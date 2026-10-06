package er

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/curve"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls ER diagram appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

const (
	maxLabelW = 200.0
	selfLoop  = 56.0 // how far a relationship of an entity to itself bulges
)

type metrics struct {
	face                     svgutil.Face
	fs, cellPad, headH, rowH float64
}

// Render parses and renders ER diagram source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	m := metrics{face: svgutil.FaceFor(o.FontFace), fs: fs, cellPad: fs * 0.6, headH: fs * 2.3, rowH: fs * 1.9}

	g := &domain.Graph{Direction: directionOf(d.Direction)}
	vert := g.Direction == domain.TopBottom || g.Direction == domain.BottomTop
	cols := map[*Entity][]float64{}
	// A relationship of an entity to itself loops out of its side; the
	// layout keeps room for the loop and its label beside the box.
	loopRoom := map[string]float64{}
	for _, r := range d.Relationships {
		if r.From == r.To {
			tw, th := textSize(m.face, wrap(m.face, r.Label, fs*0.9, maxLabelW), fs*0.9)
			if !vert {
				tw = th
			}
			loopRoom[r.From] = max(loopRoom[r.From], selfLoop+tw+12)
		}
	}
	real := map[string]domain.Size{}
	for _, e := range d.Entities {
		w, h, c := entitySize(e, m)
		cols[e] = c
		real[e.Name] = domain.Size{W: w, H: h}
		sz := domain.Size{W: w, H: h}
		if vert {
			sz.W += loopRoom[e.Name]
		} else {
			sz.H += loopRoom[e.Name]
		}
		g.Nodes = append(g.Nodes, &domain.Node{ID: e.Name, Label: " ", Shape: domain.ShapeRect, Size: sz})
	}
	edgeOf := make([]*domain.Edge, len(d.Relationships))
	for i, r := range d.Relationships {
		if r.From == r.To {
			continue
		}
		e := &domain.Edge{From: r.From, To: r.To}
		if r.Label != "" {
			e.Label = strings.Join(wrap(m.face, r.Label, fs*0.9, maxLabelW), "\n")
		}
		edgeOf[i] = e
		g.Edges = append(g.Edges, e)
	}
	// Each line holds a glyph at both ends and its label between them;
	// across the page the label's width is what needs the room.
	rankSep := 96.0
	if !vert {
		for _, r := range d.Relationships {
			tw, _ := textSize(m.face, wrap(m.face, r.Label, fs*0.9, maxLabelW), fs*0.9)
			rankSep = max(rankSep, tw+80)
		}
	}
	res, err := layout.Compute(g, layout.Options{NodeSep: 50, RankSep: rankSep, FontSize: fs * 0.9, FontFace: o.FontFace})
	if err != nil {
		return nil, err
	}
	for _, n := range g.Nodes {
		if loopRoom[n.ID] == 0 {
			continue
		}
		// Give the box back its own size; move line ends onto it.
		rs := real[n.ID]
		if !vert {
			// The room was added above the box.
			n.Pos.Y += n.Size.H - rs.H
		}
		for _, e := range g.Edges {
			if len(e.Points) < 2 {
				continue
			}
			var ends []int
			if e.From == n.ID {
				ends = append(ends, 0)
			}
			if e.To == n.ID {
				ends = append(ends, len(e.Points)-1)
			}
			for _, i := range ends {
				p := &e.Points[i]
				nb := 1
				if i > 0 {
					nb = len(e.Points) - 2
				}
				q := &e.Points[nb]
				if vert && p.X > n.Pos.X+1 && p.X < n.Pos.X+n.Size.W-1 {
					// An end on the top or bottom: squeeze it onto the box,
					// keeping a vertical first leg vertical.
					nx := n.Pos.X + 8 + (p.X-n.Pos.X)*(rs.W-16)/n.Size.W
					if len(e.Points) > 2 && math.Abs(q.X-p.X) < 0.5 {
						q.X = nx
					}
					p.X = nx
				} else if !vert && p.Y > n.Pos.Y+1 && p.Y < n.Pos.Y+rs.H-1 {
					// Left or right side ends stay where they are.
				}
				p.X = min(max(p.X, n.Pos.X), n.Pos.X+rs.W)
				p.Y = min(max(p.Y, n.Pos.Y), n.Pos.Y+rs.H)
			}
		}
		n.Size = rs
	}
	spreadEnds(g, edgeOf)
	return svg(d, g, res, edgeOf, cols, o, m), nil
}

// directionOf maps a `direction` line onto a layout direction.
func directionOf(dir string) domain.Direction {
	switch dir {
	case "LR":
		return domain.LeftRight
	case "RL":
		return domain.RightLeft
	case "BT":
		return domain.BottomTop
	default:
		return domain.TopBottom
	}
}

// entitySize sizes an entity's table and returns its column widths: type,
// name, then keys and comment when any row has them.
func entitySize(e *Entity, m metrics) (w, h float64, cols []float64) {
	head := m.face.Width(e.Label(), m.fs) + 2*m.fs*1.2
	if len(e.Attributes) == 0 {
		return math.Ceil(max(head, m.fs*7)), math.Ceil(m.fs * 3.2), nil
	}
	hasKeys, hasComment := false, false
	for _, a := range e.Attributes {
		hasKeys = hasKeys || len(a.Keys) > 0
		hasComment = hasComment || a.Comment != ""
	}
	n := 2
	if hasKeys {
		n++
	}
	if hasComment {
		n++
	}
	cols = make([]float64, n)
	for _, a := range e.Attributes {
		cells := rowCells(a, hasKeys, hasComment)
		for i, c := range cells {
			cols[i] = max(cols[i], m.face.Width(c, m.fs*0.95)+2*m.cellPad)
		}
	}
	for _, c := range cols {
		w += c
	}
	if w < head {
		cols[len(cols)-1] += head - w
		w = head
	}
	return math.Ceil(w), math.Ceil(m.headH + float64(len(e.Attributes))*m.rowH), cols
}

func rowCells(a Attribute, hasKeys, hasComment bool) []string {
	cells := []string{a.Type, a.Name}
	if hasKeys {
		cells = append(cells, strings.Join(a.Keys, ", "))
	}
	if hasComment {
		cells = append(cells, a.Comment)
	}
	return cells
}

func svg(d *Diagram, g *domain.Graph, res *layout.Result, edgeOf []*domain.Edge, cols map[*Entity][]float64, o RenderOptions, m metrics) []byte {
	pal := theme.For(o.Theme)
	pad := o.Padding
	titleH := svgutil.TitleHeight(o.Title, m.fs)
	vert := g.Direction == domain.TopBottom || g.Direction == domain.BottomTop

	type drawn struct {
		r      *Relationship
		sh     curve.Shape
		lx, ly float64
		lines  []string
	}
	var rels []drawn
	var bd svgutil.Bounds
	bd.AddRect(0, 0, res.Width, res.Height)
	for _, n := range g.Nodes {
		bd.AddRect(n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H)
	}
	lfs := m.fs * 0.9
	for i, r := range d.Relationships {
		var lines []string
		if r.Label != "" {
			lines = wrap(m.face, r.Label, lfs, maxLabelW)
		}
		tw, th := textSize(m.face, lines, lfs)
		if r.From == r.To {
			n := g.NodeByID(r.From)
			if n == nil {
				continue
			}
			sh, lx, ly := selfShape(n, vert, tw, th)
			rels = append(rels, drawn{r, sh, lx, ly, lines})
			if vert {
				bd.AddRect(n.Pos.X+n.Size.W, n.Pos.Y, selfLoop+tw+12, n.Size.H)
			} else {
				bd.AddRect(n.Pos.X, n.Pos.Y-selfLoop-th-8, n.Size.W, selfLoop+th+8)
			}
			continue
		}
		e := edgeOf[i]
		if e == nil || len(e.Points) < 2 {
			continue
		}
		var obs []curve.Box
		for _, n := range g.Nodes {
			if n.ID != r.From && n.ID != r.To {
				obs = append(obs, curve.Box{X: n.Pos.X, Y: n.Pos.Y, W: n.Size.W, H: n.Size.H})
			}
		}
		sh := curve.Edge(e.Points, vert, obs, 0, 0)
		lx, ly := e.LabelPos.X, e.LabelPos.Y
		if sh.Curved {
			lx, ly = sh.Mid.X, sh.Mid.Y
		} else if len(lines) > 0 {
			ly -= float64(len(lines)-1)*lfs*1.3/2 + lfs*0.3
		}
		for _, p := range e.Points {
			bd.Add(p.X, p.Y)
		}
		if lines != nil {
			bd.AddRect(lx-tw/2-4, ly-th/2-2, tw+8, th+4)
		}
		rels = append(rels, drawn{r, sh, lx, ly, lines})
	}
	// Nudge labels apart where two would overlap.
	type lb struct{ x0, y0, x1, y1 float64 }
	boxOf := func(de drawn) lb {
		tw, th := textSize(m.face, de.lines, lfs)
		return lb{de.lx - tw/2 - 4, de.ly - th/2 - 2, de.lx + tw/2 + 4, de.ly + th/2 + 2}
	}
	for i := range rels {
		if rels[i].lines == nil {
			continue
		}
		for pass := 0; pass < 4; pass++ {
			moved := false
			a := boxOf(rels[i])
			for j := 0; j < i; j++ {
				if rels[j].lines == nil {
					continue
				}
				o := boxOf(rels[j])
				if a.x0 < o.x1 && a.x1 > o.x0 && a.y0 < o.y1 && a.y1 > o.y0 {
					if rels[i].ly >= rels[j].ly {
						rels[i].ly += o.y1 - a.y0 + 2
					} else {
						rels[i].ly -= a.y1 - o.y0 + 2
					}
					moved = true
					a = boxOf(rels[i])
				}
			}
			if !moved {
				break
			}
		}
		a := boxOf(rels[i])
		bd.AddRect(a.x0, a.y0, a.x1-a.x0, a.y1-a.y0)
	}
	shiftX, shiftY := bd.Offset()
	contentW, contentH := bd.Size()
	w := contentW + pad*2
	h := contentH + titleH + pad*2
	if o.Title != "" {
		w = max(w, m.face.Width(o.Title, m.fs)+2*pad)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(m.fs))
	fmt.Fprintf(&b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `  <text x="%s" y="%s" fill="%s" text-anchor="middle" font-weight="bold">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+m.fs), svgutil.Esc(pal.Text), svgutil.Esc(o.Title))
	}
	fmt.Fprintf(&b, `  <g transform="translate(%s,%s)">`+"\n", svgutil.Num(pad+shiftX), svgutil.Num(pad+titleH+shiftY))

	edge := svgutil.Esc(pal.Edge)
	for _, de := range rels {
		dash := ""
		if de.r.Dashed {
			dash = ` stroke-dasharray="6 4"`
		}
		fmt.Fprintf(&b, `    <path d="%s" fill="none" stroke="%s" stroke-width="1.3"%s/>`+"\n", de.sh.D, edge, dash)
	}
	for _, e := range d.Entities {
		writeEntity(&b, d, e, g.NodeByID(e.Name), cols[e], pal, m)
	}
	// Glyphs sit over the entity border, and labels over everything.
	for _, de := range rels {
		writeCrow(&b, de.r.LeftKind, de.sh.Start, de.sh.StartDir, pal)
		writeCrow(&b, de.r.RightKind, de.sh.End, de.sh.EndDir, pal)
	}
	for _, de := range rels {
		if de.lines == nil {
			continue
		}
		tw, th := textSize(m.face, de.lines, lfs)
		fmt.Fprintf(&b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="#e8e8e8" fill-opacity="0.85"/>`+"\n",
			svgutil.Num(de.lx-tw/2-4), svgutil.Num(de.ly-th/2-2), svgutil.Num(tw+8), svgutil.Num(th+4))
		writeLines(&b, de.lines, de.lx, de.ly, lfs, svgutil.Esc(pal.Text), "middle")
	}
	b.WriteString("  </g>\n</svg>\n")
	return []byte(b.String())
}

// selfShape is the loop of a relationship from an entity to itself, with
// where its label goes.
func selfShape(n *domain.Node, vert bool, tw, th float64) (curve.Shape, float64, float64) {
	num := svgutil.Num
	if vert {
		x, cy := n.Pos.X+n.Size.W, n.Pos.Y+n.Size.H/2
		a, z := domain.Point{X: x, Y: cy - 14}, domain.Point{X: x, Y: cy + 14}
		c1, c2 := domain.Point{X: x + selfLoop, Y: cy - 14 - selfLoop*0.6}, domain.Point{X: x + selfLoop, Y: cy + 14 + selfLoop*0.6}
		sh := curve.Shape{Start: a, End: z, Curved: true, StartDir: curve.UnitTo(a, c1, z), EndDir: curve.UnitTo(z, c2, a),
			D: fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s", num(a.X), num(a.Y), num(c1.X), num(c1.Y), num(c2.X), num(c2.Y), num(z.X), num(z.Y))}
		return sh, x + selfLoop*0.75 + 8 + tw/2, cy
	}
	cx, y := n.Pos.X+n.Size.W/2, n.Pos.Y
	a, z := domain.Point{X: cx - 14, Y: y}, domain.Point{X: cx + 14, Y: y}
	c1, c2 := domain.Point{X: cx - 14 - selfLoop*0.6, Y: y - selfLoop}, domain.Point{X: cx + 14 + selfLoop*0.6, Y: y - selfLoop}
	sh := curve.Shape{Start: a, End: z, Curved: true, StartDir: curve.UnitTo(a, c1, z), EndDir: curve.UnitTo(z, c2, a),
		D: fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s", num(a.X), num(a.Y), num(c1.X), num(c1.Y), num(c2.X), num(c2.Y), num(z.X), num(z.Y))}
	return sh, cx, y - selfLoop*0.75 - th/2 - 4
}

func writeEntity(b *strings.Builder, d *Diagram, e *Entity, n *domain.Node, cols []float64, pal theme.Palette, m metrics) {
	if n == nil {
		return
	}
	var st Style
	for _, c := range e.Classes {
		cd := d.ClassDefs[c]
		for _, f := range [][2]*string{{&st.Fill, &cd.Fill}, {&st.Stroke, &cd.Stroke}, {&st.StrokeWidth, &cd.StrokeWidth}, {&st.Dash, &cd.Dash}, {&st.Color, &cd.Color}} {
			if *f[1] != "" {
				*f[0] = *f[1]
			}
		}
	}
	fill, stroke, text := svgutil.Esc(pal.NodeFill), svgutil.Esc(pal.NodeStroke), svgutil.Esc(pal.Text)
	if st.Fill != "" {
		fill = svgutil.Esc(st.Fill)
	}
	if st.Stroke != "" {
		stroke = svgutil.Esc(st.Stroke)
	}
	if st.Color != "" {
		text = svgutil.Esc(st.Color)
	}
	extra := ""
	if st.StrokeWidth != "" {
		extra += ` stroke-width="` + svgutil.Esc(st.StrokeWidth) + `"`
	}
	if st.Dash != "" {
		extra += ` stroke-dasharray="` + svgutil.Esc(st.Dash) + `"`
	}
	num := svgutil.Num
	x, y, w, h := n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"%s/>`+"\n", num(x), num(y), num(w), num(h), fill, stroke, extra)
	if len(e.Attributes) == 0 {
		writeLines(b, []string{e.Label()}, x+w/2, y+h/2, m.fs, text, "middle")
		return
	}
	writeLines(b, []string{e.Label()}, x+w/2, y+m.headH/2, m.fs, text, "middle")
	hasKeys, hasComment := false, false
	for _, a := range e.Attributes {
		hasKeys = hasKeys || len(a.Keys) > 0
		hasComment = hasComment || a.Comment != ""
	}
	odd, even := svgutil.Esc(pal.Background), mix(pal.NodeFill, pal.Background, 0.55, svgutil.Esc(pal.Background))
	ry := y + m.headH
	for i, a := range e.Attributes {
		rowFill := odd
		if i%2 == 1 {
			rowFill = even
		}
		fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n", num(x+0.5), num(ry), num(w-1), num(m.rowH), rowFill)
		cx := x
		for ci, c := range rowCells(a, hasKeys, hasComment) {
			if c != "" {
				writeLines(b, []string{c}, cx+m.cellPad, ry+m.rowH/2, m.fs*0.95, text, "start")
			}
			cx += cols[ci]
		}
		ry += m.rowH
	}
	// Grid: the header rule, row rules and column rules, then the border
	// again so the row fills do not cover it.
	fmt.Fprintf(b, `    <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`+"\n", num(x), num(y+m.headH), num(x+w), num(y+m.headH), stroke)
	for i := 1; i < len(e.Attributes); i++ {
		yy := y + m.headH + float64(i)*m.rowH
		fmt.Fprintf(b, `    <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-opacity="0.35"/>`+"\n", num(x), num(yy), num(x+w), num(yy), stroke)
	}
	cx := x
	for _, c := range cols[:len(cols)-1] {
		cx += c
		fmt.Fprintf(b, `    <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-opacity="0.35"/>`+"\n", num(cx), num(y+m.headH), num(cx), num(y+h), stroke)
	}
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" fill="none" stroke="%s"%s/>`+"\n", num(x), num(y), num(w), num(h), stroke, extra)
}

// writeCrow draws the crow's-foot glyph for kind at tip, the point where
// the line meets the entity; dir points from the tip along the line.
// Reading outward from the entity: a crow's foot for "many", then a bar
// for "one" or a ring for "zero"; "exactly one" is two bars.
func writeCrow(b *strings.Builder, kind Card, tip domain.Point, dir [2]float64, pal theme.Palette) {
	dx, dy := dir[0], dir[1]
	if dx == 0 && dy == 0 {
		return
	}
	px, py := -dy, dx
	edge, bg := svgutil.Esc(pal.Edge), svgutil.Esc(pal.Background)
	num := svgutil.Num
	at := func(d float64) (float64, float64) { return tip.X + dx*d, tip.Y + dy*d }
	bar := func(d float64) {
		x, y := at(d)
		fmt.Fprintf(b, `    <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.3"/>`+"\n",
			num(x+px*7), num(y+py*7), num(x-px*7), num(y-py*7), edge)
	}
	ring := func(d float64) {
		x, y := at(d)
		fmt.Fprintf(b, `    <circle cx="%s" cy="%s" r="5" fill="%s" stroke="%s" stroke-width="1.3"/>`+"\n", num(x), num(y), bg, edge)
	}
	foot := func() {
		x, y := at(14)
		fmt.Fprintf(b, `    <path d="M%s,%s L%s,%s M%s,%s L%s,%s M%s,%s L%s,%s" fill="none" stroke="%s" stroke-width="1.3"/>`+"\n",
			num(tip.X+px*8), num(tip.Y+py*8), num(x), num(y),
			num(tip.X-px*8), num(tip.Y-py*8), num(x), num(y),
			num(tip.X), num(tip.Y), num(x), num(y), edge)
	}
	switch kind {
	case CardOne:
		bar(8)
		bar(14)
	case CardZeroOne:
		bar(8)
		ring(20)
	case CardOneMany:
		foot()
		bar(19)
	case CardZeroMany:
		foot()
		ring(23)
	}
}

func textSize(face svgutil.Face, lines []string, size float64) (w, h float64) {
	for _, l := range lines {
		w = max(w, face.Width(l, size))
	}
	return w, float64(len(lines)) * size * 1.3
}

// writeLines writes lines with their block centred vertically on cy.
func writeLines(b *strings.Builder, lines []string, x, cy, size float64, fill, anchor string) {
	lh := size * 1.3
	y0 := cy - float64(len(lines)-1)*lh/2 + size*0.35
	fmt.Fprintf(b, `    <text fill="%s" font-size="%s" text-anchor="%s">`, fill, svgutil.Num(size), anchor)
	for i, l := range lines {
		fmt.Fprintf(b, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(x), svgutil.Num(y0+float64(i)*lh), svgutil.Esc(l))
	}
	b.WriteString("</text>\n")
}

// wrap breaks s into lines no wider than maxW, at spaces.
func wrap(face svgutil.Face, s string, size, maxW float64) []string {
	var lines []string
	for _, para := range svgutil.SplitLines(s) {
		cur := ""
		for _, wd := range strings.Fields(para) {
			try := wd
			if cur != "" {
				try = cur + " " + wd
			}
			if cur != "" && face.Width(try, size) > maxW {
				lines = append(lines, cur)
				cur = wd
				continue
			}
			cur = try
		}
		lines = append(lines, cur)
	}
	return lines
}

var plainFont = regexp.MustCompile(`^[A-Za-z0-9 ,'"_-]{1,200}$`)

// mix blends hex colour a toward hex colour b by t (0 keeps a). When either
// is not a #rrggbb colour it returns fallback.
func mix(a, b string, t float64, fallback string) string {
	var ra, ga, ba, rb, gb, bb int
	if len(a) != 7 || len(b) != 7 {
		return fallback
	}
	if _, err := fmt.Sscanf(strings.ToLower(a), "#%02x%02x%02x", &ra, &ga, &ba); err != nil {
		return fallback
	}
	if _, err := fmt.Sscanf(strings.ToLower(b), "#%02x%02x%02x", &rb, &gb, &bb); err != nil {
		return fallback
	}
	c := func(x, y int) int { return x + int(float64(y-x)*t+0.5) }
	return fmt.Sprintf("#%02x%02x%02x", c(ra, rb), c(ga, gb), c(ba, bb))
}
