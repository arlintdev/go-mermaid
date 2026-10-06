package class

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/curve"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls class diagram appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
	// IDPrefix starts every id in the picture; empty derives one from the
	// source (see svgid.For).
	IDPrefix string
}

const (
	clusterPad = 16.0 // gap between a namespace box and its classes
	cardGap    = 6.0  // gap between a relationship end and its multiplicity
	noteMaxW   = 200.0
)

// noteID names the layout node of the i-th note; the NUL keeps it apart
// from every class name.
func noteID(i int) string { return "\x00note" + strconv.Itoa(i) }

// metrics are the box measurements, scaled to the font size.
type metrics struct {
	face                      svgutil.Face
	fs, lh, padX, headPad, cp float64
	empty                     float64 // height of an empty compartment
}

func newMetrics(o RenderOptions) metrics {
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	return metrics{face: svgutil.FaceFor(o.FontFace), fs: fs, lh: fs * 1.45, padX: fs * 0.8, headPad: fs * 0.6, cp: fs * 0.45, empty: fs * 0.55}
}

// Render parses and renders class diagram source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	m := newMetrics(o)
	o.FontSize = m.fs

	g := &domain.Graph{Direction: domain.DirectionOf(d.Direction)}
	for _, c := range d.Classes {
		n := &domain.Node{ID: c.Name, Label: c.Name, Shape: domain.ShapeRect}
		n.Size = classSize(c, m)
		g.Nodes = append(g.Nodes, n)
	}
	for i, nt := range d.Notes {
		w, h := noteSize(nt.Text, m)
		g.Nodes = append(g.Nodes, &domain.Node{ID: noteID(i), Label: " ", Shape: domain.ShapeRect, Size: domain.Size{W: w, H: h}})
	}
	for _, r := range d.Relations {
		g.Edges = append(g.Edges, &domain.Edge{From: r.From, To: r.To, Label: r.Label})
	}
	for i, nt := range d.Notes {
		if nt.For != "" {
			g.Edges = append(g.Edges, &domain.Edge{From: noteID(i), To: nt.For})
		}
	}
	for _, ns := range d.Namespaces {
		if len(ns.Members) > 0 {
			g.Subgraphs = append(g.Subgraphs, &domain.Subgraph{ID: ns.Name, Title: ns.Name, NodeIDs: ns.Members})
		}
	}

	// Multiplicities sit at both ends of a line and the label between
	// them; give such lines the length to hold all three.
	rankSep := 60.0
	across := d.Direction == "LR" || d.Direction == "RL"
	for _, r := range d.Relations {
		need := 0.0
		if r.Label != "" {
			need = 70
			if across {
				need = m.face.Width(generics(r.Label), m.fs*0.9) + 40
			}
		}
		if r.LeftCard != "" || r.RightCard != "" {
			if across {
				need += m.face.Width(r.LeftCard, m.fs*0.85) + m.face.Width(r.RightCard, m.fs*0.85) + 40
			} else {
				need = max(need, 70) + m.fs*2
			}
		}
		rankSep = max(rankSep, need)
	}
	res, err := layout.Compute(g, layout.Options{NodeSep: 50, RankSep: rankSep, FontSize: m.fs * 0.9, FontFace: o.FontFace})
	if err != nil {
		return nil, err
	}
	return svg(d, g, res, o, m), nil
}

// namespaceBox returns the box enclosing a namespace's classes, with room for
// its title. ok is false when no member was placed.
func namespaceBox(ns *Namespace, g *domain.Graph, m metrics) (x, y, w, h float64, ok bool) {
	var bd svgutil.Bounds
	for _, name := range ns.Members {
		n := g.NodeByID(name)
		if n == nil {
			continue
		}
		bd.AddRect(n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H)
	}
	if bd.Empty() {
		return 0, 0, 0, 0, false
	}
	titleH := m.fs + 8
	w = max(bd.MaxX-bd.MinX+clusterPad*2, m.face.Width(ns.Name, m.fs)+20)
	cx := (bd.MinX + bd.MaxX) / 2
	return cx - w/2, bd.MinY - clusterPad - titleH, w, bd.MaxY - bd.MinY + clusterPad*2 + titleH, true
}

func svg(d *Diagram, g *domain.Graph, res *layout.Result, o RenderOptions, m metrics) []byte {
	pal := theme.For(o.Theme)
	pad := o.Padding
	titleH := svgutil.TitleHeight(o.Title, m.fs)

	// Namespace boxes, labels and multiplicities reach outside the class
	// extents the layout reported.
	var bd svgutil.Bounds
	bd.AddRect(0, 0, res.Width, res.Height)
	for _, ns := range d.Namespaces {
		if nx, ny, nw, nh, ok := namespaceBox(ns, g, m); ok {
			bd.AddRect(nx, ny, nw, nh)
		}
	}
	shapes, labelAt, moved := relationShapes(d, g, m)
	for i, r := range d.Relations {
		e := g.Edges[i]
		if r.Label != "" {
			lw := m.face.Width(r.Label, m.fs*0.9) + 8
			bd.AddRect(e.LabelPos.X-lw/2, e.LabelPos.Y-m.fs, lw, m.fs*1.4)
			if moved[i] {
				fs := m.fs * 0.9
				tw := m.face.Width(generics(r.Label), fs)
				bd.AddRect(labelAt[i].X-tw/2-4, labelAt[i].Y-fs*0.95, tw+8, fs*1.3)
			}
		}
		for _, p := range e.Points {
			bd.AddRect(p.X-20, p.Y-20, 40, 40)
		}
	}
	shiftX, shiftY := bd.Offset()
	contentW, contentH := bd.Size()
	w := contentW + pad*2
	h := contentH + titleH + pad*2
	if o.Title != "" {
		w = max(w, m.face.Bold().Width(o.Title, m.fs)+2*pad)
	}
	shiftX += (w - contentW - pad*2) / 2

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(m.fs))
	fmt.Fprintf(&b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `  <text x="%s" y="%s" fill="%s" text-anchor="middle" font-weight="bold">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+m.fs), svgutil.Esc(pal.Text), svgutil.Esc(o.Title))
	}
	fmt.Fprintf(&b, `  <g transform="translate(%s,%s)">`+"\n", svgutil.Num(pad+shiftX), svgutil.Num(pad+titleH+shiftY))

	for _, ns := range d.Namespaces {
		writeNamespace(&b, ns, g, pal, m)
	}
	for i, r := range d.Relations {
		if len(g.Edges[i].Points) < 2 {
			continue
		}
		writeRelation(&b, r, shapes[i], pal, m)
	}
	for i, nt := range d.Notes {
		writeNote(&b, i, nt, g, len(d.Relations), pal, m)
	}
	for _, c := range d.Classes {
		writeClass(&b, d, c, g.NodeByID(c.Name), pal, m)
	}
	// Relationship labels last, so no line or box covers them.
	for i, r := range d.Relations {
		writeEdgeLabel(&b, r, g.Edges[i], labelAt[i], pal, m)
	}

	b.WriteString("  </g>\n</svg>\n")
	return []byte(b.String())
}

func writeClass(b *strings.Builder, d *Diagram, c *Class, n *domain.Node, pal theme.Palette, m metrics) {
	if n == nil {
		return
	}
	st := c.Style
	for i := len(c.Classes) - 1; i >= 0; i-- {
		st = st.Over(d.ClassDefs[c.Classes[i]])
	}
	fill, stroke, text := pal.Node(st.Fill, st.Stroke, st.Color)
	fill, stroke, text = svgutil.Esc(fill), svgutil.Esc(stroke), svgutil.Esc(text)
	extra := ""
	if st.StrokeWidth != "" {
		extra += ` stroke-width="` + svgutil.Esc(st.StrokeWidth) + `"`
	}
	if st.Dash != "" {
		extra += ` stroke-dasharray="` + svgutil.Esc(st.Dash) + `"`
	}

	x, y, w := n.Pos.X, n.Pos.Y, n.Size.W
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"%s/>`+"\n",
		svgutil.Num(x), svgutil.Num(y), svgutil.Num(w), svgutil.Num(n.Size.H), fill, stroke, extra)

	cy := y + m.headPad
	if c.Annotation != "" {
		cy += m.lh
		fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" text-anchor="middle" font-size="%s">%s</text>`+"\n",
			svgutil.Num(x+w/2), svgutil.Num(cy-m.lh*0.3), text, svgutil.Num(m.fs*0.9), svgutil.Esc("«"+c.Annotation+"»"))
	}
	cy += m.lh
	fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" text-anchor="middle" font-weight="bold">%s</text>`+"\n",
		svgutil.Num(x+w/2), svgutil.Num(cy-m.lh*0.3), text, svgutil.Esc(c.Label()))
	cy += m.headPad

	compartment := func(rows []string, method bool) {
		fmt.Fprintf(b, `    <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`+"\n",
			svgutil.Num(x), svgutil.Num(cy), svgutil.Num(x+w), svgutil.Num(cy), stroke)
		if len(rows) == 0 {
			cy += m.empty
			return
		}
		cy += m.cp
		for _, raw := range rows {
			mb := formatMember(raw, method)
			cy += m.lh
			ty := cy - m.lh*0.3
			style := ""
			if mb.italic {
				style = ` font-style="italic"`
			}
			fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s"%s>%s</text>`+"\n",
				svgutil.Num(x+m.padX), svgutil.Num(ty), text, style, svgutil.Esc(mb.text))
			if mb.underline {
				uw := m.face.Width(mb.text, m.fs)
				fmt.Fprintf(b, `    <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`+"\n",
					svgutil.Num(x+m.padX), svgutil.Num(ty+2), svgutil.Num(x+m.padX+uw), svgutil.Num(ty+2), text)
			}
		}
		cy += m.cp
	}
	compartment(c.Attributes, false)
	compartment(c.Methods, true)
}

// classSize computes a box size that fits the name and all members.
func classSize(c *Class, m metrics) domain.Size {
	maxW := m.face.Width(c.Label(), m.fs) * 1.05 // bold
	if c.Annotation != "" {
		maxW = max(maxW, m.face.Width("«"+c.Annotation+"»", m.fs*0.9))
	}
	for _, a := range c.Attributes {
		maxW = max(maxW, m.face.Width(formatMember(a, false).text, m.fs))
	}
	for _, a := range c.Methods {
		maxW = max(maxW, m.face.Width(formatMember(a, true).text, m.fs))
	}
	w := max(maxW+m.padX*2, m.fs*4.5)
	h := 2*m.headPad + m.lh
	if c.Annotation != "" {
		h += m.lh
	}
	for _, rows := range [][]string{c.Attributes, c.Methods} {
		if len(rows) == 0 {
			h += m.empty
		} else {
			h += float64(len(rows))*m.lh + 2*m.cp
		}
	}
	return domain.Size{W: math.Ceil(w), H: math.Ceil(h)}
}

func writeRelation(b *strings.Builder, r *Relation, sh curve.Shape, pal theme.Palette, m metrics) {
	edge := svgutil.Esc(pal.Edge)
	dash := ""
	if r.Dashed {
		dash = ` stroke-dasharray="5 4"`
	}
	fmt.Fprintf(b, `    <path d="%s" fill="none" stroke="%s"%s/>`+"\n", sh.D, edge, dash)
	writeHead(b, r.Left, sh.Start, sh.StartDir[0], sh.StartDir[1], pal)
	writeHead(b, r.Right, sh.End, sh.EndDir[0], sh.EndDir[1], pal)
	writeCardinality(b, r.LeftCard, sh.Start, sh.StartDir, pal, m)
	writeCardinality(b, r.RightCard, sh.End, sh.EndDir, pal, m)
}

// writeEdgeLabel draws a relationship's label on a soft background at the
// position the layout reserved for it.
func writeEdgeLabel(b *strings.Builder, r *Relation, e *domain.Edge, at domain.Point, pal theme.Palette, m metrics) {
	if r.Label == "" || len(e.Points) < 2 {
		return
	}
	fs := m.fs * 0.9
	text := generics(r.Label)
	tw := m.face.Width(text, fs)
	x, y := at.X, at.Y
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s" fill-opacity="0.85"/>`+"\n",
		svgutil.Num(x-tw/2-4), svgutil.Num(y-fs*0.95), svgutil.Num(tw+8), svgutil.Num(fs*1.3), svgutil.Esc(pal.RelationLabel))
	fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" text-anchor="middle" font-size="%s">%s</text>`+"\n",
		svgutil.Num(x), svgutil.Num(y), svgutil.Esc(pal.Text), svgutil.Num(fs), svgutil.Esc(text))
}

// relationShapes draws every relationship line and places its label: at
// the layout's label position (the text baseline), or a curved line's
// middle, then moved along its own line where it would hide another line,
// label or class. moved reports the labels that left their first spot.
func relationShapes(d *Diagram, g *domain.Graph, m metrics) (shapes []curve.Shape, at []domain.Point, moved []bool) {
	shapes = make([]curve.Shape, len(d.Relations))
	at = make([]domain.Point, len(d.Relations))
	moved = make([]bool, len(d.Relations))
	vertical := g.Direction == domain.TopBottom || g.Direction == domain.BottomTop
	fs := m.fs * 0.9
	var labels []curve.Label
	var idx []int
	for i, r := range d.Relations {
		e := g.Edges[i]
		if len(e.Points) < 2 {
			continue
		}
		var obs []curve.Box
		for _, n := range g.Nodes {
			if n.ID != r.From && n.ID != r.To {
				obs = append(obs, curve.Box{X: n.Pos.X, Y: n.Pos.Y, W: n.Size.W, H: n.Size.H})
			}
		}
		shapes[i] = curve.Edge(e.Points, vertical, obs, headLen(r.Left), headLen(r.Right))
		at[i] = e.LabelPos
		if shapes[i].Curved {
			at[i] = domain.Point{X: shapes[i].Mid.X, Y: shapes[i].Mid.Y + fs*0.35}
		}
		if r.Label == "" {
			continue
		}
		line := i
		if r.From == r.To {
			line = -1
		}
		tw := m.face.Width(generics(r.Label), fs)
		// The plate runs from 0.95em above the baseline to 0.35em below.
		labels = append(labels, curve.Label{Line: line, W: tw + 8, H: fs * 1.3, X: at[i].X, Y: at[i].Y - fs*0.3})
		idx = append(idx, i)
	}
	var boxes []curve.Box
	for _, n := range g.Nodes {
		boxes = append(boxes, curve.Box{X: n.Pos.X, Y: n.Pos.Y, W: n.Size.W, H: n.Size.H})
	}
	curve.PlaceLabels(shapes, labels, boxes)
	for k, l := range labels {
		i := idx[k]
		if p := (domain.Point{X: l.X, Y: l.Y + fs*0.3}); math.Abs(p.X-at[i].X) > 1e-9 || math.Abs(p.Y-at[i].Y) > 1e-9 {
			at[i], moved[i] = p, true
		}
	}
	return shapes, at, moved
}

func headLen(k headKind) float64 {
	switch k {
	case headTriangle:
		return 14
	case headDiamondFilled, headDiamondHollow:
		return 16
	case headLollipop:
		return 10
	}
	return 0
}

// writeHead draws a relationship decoration at tip pointing in direction (dx,dy).
func writeHead(b *strings.Builder, kind headKind, tip domain.Point, dx, dy float64, pal theme.Palette) {
	if kind == headNone {
		return
	}
	edge, bg := svgutil.Esc(pal.Edge), svgutil.Esc(pal.Background)
	px, py := -dy, dx // perpendicular
	n := svgutil.Num
	switch kind {
	case headArrow:
		const l, hw = 10.0, 5.0
		bx, by := tip.X+dx*l, tip.Y+dy*l
		fmt.Fprintf(b, `    <path d="M%s,%s L%s,%s L%s,%s Z" fill="%s"/>`+"\n",
			n(tip.X), n(tip.Y), n(bx+px*hw), n(by+py*hw), n(bx-px*hw), n(by-py*hw), edge)
	case headTriangle:
		const l, hw = 14.0, 8.0
		bx, by := tip.X+dx*l, tip.Y+dy*l
		fmt.Fprintf(b, `    <path d="M%s,%s L%s,%s L%s,%s Z" fill="%s" stroke="%s"/>`+"\n",
			n(tip.X), n(tip.Y), n(bx+px*hw), n(by+py*hw), n(bx-px*hw), n(by-py*hw), bg, edge)
	case headDiamondFilled, headDiamondHollow:
		const l, hw = 16.0, 6.0
		bx, by := tip.X+dx*l, tip.Y+dy*l
		mx, my := tip.X+dx*l/2, tip.Y+dy*l/2
		fill := edge
		if kind == headDiamondHollow {
			fill = bg
		}
		fmt.Fprintf(b, `    <path d="M%s,%s L%s,%s L%s,%s L%s,%s Z" fill="%s" stroke="%s"/>`+"\n",
			n(tip.X), n(tip.Y), n(mx+px*hw), n(my+py*hw), n(bx), n(by), n(mx-px*hw), n(my-py*hw), fill, edge)
	case headLollipop:
		fmt.Fprintf(b, `    <circle cx="%s" cy="%s" r="5" fill="%s" stroke="%s"/>`+"\n", n(tip.X+dx*5), n(tip.Y+dy*5), bg, edge)
	}
}

// writeCardinality draws a multiplicity label just inside the end of a
// relationship line. tip is the end point and next is the neighbouring
// waypoint, so the label sits along the line rather than on top of the class.
func writeCardinality(b *strings.Builder, card string, tip domain.Point, dir [2]float64, pal theme.Palette, m metrics) {
	if card == "" {
		return
	}
	dx, dy := dir[0], dir[1]
	fs := m.fs * 0.85
	// Step along the line past the end decoration, then to the side, far
	// enough that the line does not strike through the text.
	off := 7 + m.face.Width(card, fs)/2
	x := tip.X + dx*(cardGap+12) - dy*off
	y := tip.Y + dy*(cardGap+12) + dx*off*0.6 + fs*0.35
	fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" text-anchor="middle" font-size="%s">%s</text>`+"\n",
		svgutil.Num(x), svgutil.Num(y), svgutil.Esc(pal.Text), svgutil.Num(fs), svgutil.Esc(card))
}

// writeNamespace draws the box and title of a namespace block, in
// Mermaid's cluster colours.
func writeNamespace(b *strings.Builder, ns *Namespace, g *domain.Graph, pal theme.Palette, m metrics) {
	x, y, w, h, ok := namespaceBox(ns, g, m)
	if !ok {
		return
	}
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="4" fill="%s" stroke="%s"/>`+"\n",
		svgutil.Num(x), svgutil.Num(y), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(pal.ClusterFill), svgutil.Esc(pal.ClusterStroke))
	fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" text-anchor="middle">%s</text>`+"\n",
		svgutil.Num(x+w/2), svgutil.Num(y+m.fs+4), svgutil.Esc(pal.Text), svgutil.Esc(ns.Name))
}

func noteLines(text string, m metrics) []string { return m.face.Wrap(text, m.fs, noteMaxW) }

func noteSize(text string, m metrics) (float64, float64) {
	lines := noteLines(text, m)
	tw := 0.0
	for _, l := range lines {
		tw = max(tw, m.face.Width(l, m.fs))
	}
	return math.Ceil(tw + 2*m.padX), math.Ceil(float64(len(lines))*m.lh + 2*m.cp + 4)
}

// writeNote draws a note box and the dashed line to its class.
func writeNote(b *strings.Builder, i int, nt *Note, g *domain.Graph, edgeBase int, pal theme.Palette, m metrics) {
	n := g.NodeByID(noteID(i))
	if n == nil {
		return
	}
	if nt.For != "" {
		for _, e := range g.Edges[edgeBase:] {
			if e.From == noteID(i) && len(e.Points) >= 2 {
				var obs []curve.Box
				for _, o := range g.Nodes {
					if o.ID != e.From && o.ID != e.To {
						obs = append(obs, curve.Box{X: o.Pos.X, Y: o.Pos.Y, W: o.Size.W, H: o.Size.H})
					}
				}
				sh := curve.Edge([]domain.Point{e.Points[0], e.Points[len(e.Points)-1]}, g.Direction == domain.TopBottom || g.Direction == domain.BottomTop, obs, 0, 0)
				if !sh.Curved {
					sh.D = curve.Path(e.Points)
				}
				fmt.Fprintf(b, `    <path d="%s" fill="none" stroke="%s" stroke-dasharray="3 3"/>`+"\n", sh.D, svgutil.Esc(pal.Edge))
				break
			}
		}
	}
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"/>`+"\n",
		svgutil.Num(n.Pos.X), svgutil.Num(n.Pos.Y), svgutil.Num(n.Size.W), svgutil.Num(n.Size.H), svgutil.Esc(pal.NoteFill), svgutil.Esc(pal.NoteStroke))
	y := n.Pos.Y + m.cp + 2
	for _, l := range noteLines(nt.Text, m) {
		y += m.lh
		fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s">%s</text>`+"\n",
			svgutil.Num(n.Pos.X+m.padX), svgutil.Num(y-m.lh*0.3), svgutil.Esc(pal.NoteText), svgutil.Esc(l))
	}
}
