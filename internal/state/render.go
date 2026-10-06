package state

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/svgid"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls state diagram appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

const (
	startR     = 7.0  // filled start circle
	endR       = 8.0  // ringed end circle
	forkLen    = 70.0 // length of a <<fork>> / <<join>> bar
	forkThick  = 8.0  // thickness of a <<fork>> / <<join>> bar
	choiceSize = 28.0 // width and height of a <<choice>> diamond
	compPad    = 14.0 // inside a composite state, around its machine
	regionGap  = 10.0 // between concurrent regions
	noteGap    = 24.0 // between a state and its note
	maxTextW   = 200.0
	selfLoop   = 36.0 // how far a transition to the same state bulges out
)

// Render parses and renders state diagram source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	c := &ctx{d: d, o: o, pal: theme.For(o.Theme), face: svgutil.FaceFor(o.FontFace), fs: fs, lh: fs * 1.3,
		id: svgid.Prefix(src), dir: directionOf(d.Direction), done: map[string]bool{}}
	root, err := c.region("", 0, c.dir)
	if err != nil {
		return nil, err
	}
	return c.svg(root), nil
}

type ctx struct {
	d    *Diagram
	o    RenderOptions
	pal  theme.Palette
	face svgutil.Face
	fs   float64
	lh   float64
	id   string
	dir  domain.Direction
	done map[string]bool // composites already laid out, against cycles
}

// block is a laid-out piece of the picture, drawn with its top-left corner
// at the origin.
type block struct {
	w, h float64
	body string
}

// directionOf maps a `direction` line onto a layout direction, defaulting to
// top-to-bottom when the source does not ask for one.
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

func vertical(dir domain.Direction) bool { return dir == domain.TopBottom || dir == domain.BottomTop }

// lift returns the member of scope (parent, region) that is or contains
// the state id, or "" when the state lies outside that scope.
func (c *ctx) lift(id, parent string, region int) string {
	for depth := 0; depth < maxDepth+1; depth++ {
		s := c.d.state(id)
		if s == nil {
			return ""
		}
		if s.Parent == parent && s.Region == region {
			return s.ID
		}
		if s.Parent == "" {
			return ""
		}
		id = s.Parent
	}
	return ""
}

func (c *ctx) wrap(s string, size float64) []string {
	var lines []string
	for _, para := range svgutil.SplitLines(s) {
		cur := ""
		for _, wd := range strings.Fields(para) {
			try := wd
			if cur != "" {
				try = cur + " " + wd
			}
			if cur != "" && c.face.Width(try, size) > maxTextW {
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

func (c *ctx) textBlock(lines []string, size float64) (w, h float64) {
	for _, l := range lines {
		w = max(w, c.face.Width(l, size))
	}
	return w, float64(len(lines)) * size * 1.3
}

// region lays out the states of one region of a composite (or the whole
// diagram) and the transitions between them.
func (c *ctx) region(parent string, region int, dir domain.Direction) (block, error) {
	g := &domain.Graph{Direction: dir}
	inner := map[string]block{}
	var members []*State
	for _, s := range c.d.States {
		if s.Parent != parent || s.Region != region || (parent == "" && s.ID == "") {
			continue
		}
		members = append(members, s)
		n := &domain.Node{ID: s.ID, Label: " ", Shape: domain.ShapeRound}
		switch comp := c.d.composite(s.ID); {
		case comp != nil:
			if c.done[comp.ID] {
				continue
			}
			c.done[comp.ID] = true
			bl, err := c.composite(comp, dir)
			if err != nil {
				return block{}, err
			}
			inner[s.ID] = bl
			n.Shape = domain.ShapeRect
			n.Size = domain.Size{W: bl.w, H: bl.h}
		case s.Start:
			n.Shape = domain.ShapeCircle
			n.Size = domain.Size{W: 2 * startR, H: 2 * startR}
		case s.End:
			n.Shape = domain.ShapeCircle
			n.Size = domain.Size{W: 2 * endR, H: 2 * endR}
		case s.Kind == KindFork || s.Kind == KindJoin:
			n.Shape = domain.ShapeRect
			n.Size = domain.Size{W: forkLen, H: forkThick}
			if !vertical(dir) {
				n.Size = domain.Size{W: forkThick, H: forkLen}
			}
		case s.Kind == KindChoice:
			n.Shape = domain.ShapeDiamond
			n.Size = domain.Size{W: choiceSize, H: choiceSize}
		default:
			n.Size = c.stateSize(s)
		}
		g.Nodes = append(g.Nodes, n)
	}

	type edge struct {
		t    *Transition
		e    *domain.Edge
		self string
	}
	var edges []edge
	labelled := false
	for _, t := range c.d.Transitions {
		from, to := c.lift(t.From, parent, region), c.lift(t.To, parent, region)
		if from == "" || to == "" {
			continue
		}
		if from == to {
			// Only a transition from a state to itself is drawn here; one
			// between two states inside the same child is drawn in there.
			if t.From == t.To && t.From == from {
				edges = append(edges, edge{t: t, self: from})
			}
			continue
		}
		e := &domain.Edge{From: from, To: to, Label: t.Label}
		if t.Label != "" {
			labelled = true
			// Reserve the label's room in the layout.
			lines := c.wrap(t.Label, c.fs*0.9)
			e.Label = strings.Join(lines, "\n")
		}
		g.Edges = append(g.Edges, e)
		edges = append(edges, edge{t: t, e: e})
	}

	var nodeBoxes []box
	if len(g.Nodes) > 0 {
		rankSep := 50.0
		if labelled {
			rankSep = 60
		}
		if _, err := layout.Compute(g, layout.Options{NodeSep: 40, RankSep: rankSep, FontSize: c.fs * 0.9, FontFace: c.o.FontFace}); err != nil {
			return block{}, err
		}
		for _, n := range g.Nodes {
			nodeBoxes = append(nodeBoxes, box{n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H})
		}
	}

	var bd svgutil.Bounds
	for _, n := range g.Nodes {
		bd.AddRect(n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H)
	}

	// Edge shapes and label boxes.
	type drawnEdge struct {
		t      *Transition
		sh     edgeShape
		lx, ly float64
		lines  []string
	}
	var drawn []drawnEdge
	for _, ed := range edges {
		lines := []string(nil)
		if ed.t.Label != "" {
			lines = c.wrap(ed.t.Label, c.fs*0.9)
		}
		if ed.self != "" {
			n := g.NodeByID(ed.self)
			if n == nil {
				continue
			}
			tw, th := c.textBlock(lines, c.fs*0.9)
			var sh edgeShape
			var lx, ly float64
			num := svgutil.Num
			if vertical(dir) {
				// Out of the right side and back in.
				x, cy := n.Pos.X+n.Size.W, n.Pos.Y+n.Size.H/2
				sh = edgeShape{d: fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s", num(x), num(cy-6), num(x+selfLoop), num(cy-selfLoop*0.9),
					num(x+selfLoop), num(cy+selfLoop*0.9), num(x+1), num(cy+6)), curved: true}
				lx, ly = x+selfLoop*0.75+6+tw/2, cy
				bd.AddRect(x, cy-selfLoop, selfLoop+8, 2*selfLoop)
			} else {
				// Out of the top and back in, near the right end.
				cx, y := n.Pos.X+n.Size.W-min(30, n.Size.W/3), n.Pos.Y
				sh = edgeShape{d: fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s", num(cx-6), num(y), num(cx-selfLoop*0.9), num(y-selfLoop),
					num(cx+selfLoop*0.9), num(y-selfLoop), num(cx+6), num(y-1)), curved: true}
				lx, ly = cx, y-selfLoop*0.75-th/2-2
				bd.AddRect(cx-selfLoop, y-selfLoop, 2*selfLoop, selfLoop)
			}
			drawn = append(drawn, drawnEdge{ed.t, sh, lx, ly, lines})
			if lines != nil {
				bd.AddRect(lx-tw/2-4, ly-th/2-2, tw+8, th+4)
			}
			continue
		}
		if len(ed.e.Points) < 2 {
			continue
		}
		var obs []box
		for _, n := range g.Nodes {
			if n.ID != ed.e.From && n.ID != ed.e.To {
				obs = append(obs, box{n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H})
			}
		}
		pts := c.barEnds(g, ed.e, dir)
		sh := shapeEdge(pts, vertical(dir), obs, 0, 0)
		for _, p := range ed.e.Points {
			bd.Add(p.X, p.Y)
		}
		lx, ly := ed.e.LabelPos.X, ed.e.LabelPos.Y
		if sh.curved {
			lx, ly = sh.mid.X, sh.mid.Y
		} else if len(lines) > 0 {
			// LabelPos is the baseline of the label's last line.
			ly -= float64(len(lines)-1)*c.fs*0.9*1.3/2 + c.fs*0.3
		}
		if lines != nil {
			tw, th := c.textBlock(lines, c.fs*0.9)
			bd.AddRect(lx-tw/2-4, ly-th/2-2, tw+8, th+4)
		}
		drawn = append(drawn, drawnEdge{ed.t, sh, lx, ly, lines})
	}

	// Notes beside their states, pushed clear of other boxes.
	type placedNote struct {
		n          *Note
		x, y, w, h float64
		lines      []string
		target     *domain.Node
	}
	var notes []placedNote
	for _, nt := range c.d.Notes {
		tn := g.NodeByID(c.lift(nt.Target, parent, region))
		if tn == nil || tn.ID != nt.Target {
			continue
		}
		lines := c.wrap(nt.Text, c.fs*0.9)
		tw, th := c.textBlock(lines, c.fs*0.9)
		w, h := tw+20, th+14
		hits := func(x, y float64) bool {
			for _, o := range nodeBoxes {
				if x < o.x+o.w+8 && x+w > o.x-8 && y < o.y+o.h+8 && y+h > o.y-8 {
					return true
				}
			}
			return false
		}
		cy := tn.Pos.Y + tn.Size.H/2 - h/2
		left, right := tn.Pos.X-noteGap-w, tn.Pos.X+tn.Size.W+noteGap
		cx := tn.Pos.X + tn.Size.W/2 - w/2
		cands := [][2]float64{{right, cy}, {left, cy}, {cx, tn.Pos.Y - noteGap - h}, {cx, tn.Pos.Y + tn.Size.H + noteGap}}
		if nt.Side == SideLeft {
			cands[0], cands[1] = cands[1], cands[0]
		}
		x, y := cands[0][0], cands[0][1]
		placed := false
		for _, cd := range cands {
			if !hits(cd[0], cd[1]) {
				x, y, placed = cd[0], cd[1], true
				break
			}
		}
		for tries := 0; !placed && tries < len(nodeBoxes)+1; tries++ {
			moved := false
			for _, o := range nodeBoxes {
				if x < o.x+o.w+8 && x+w > o.x-8 && y < o.y+o.h+8 && y+h > o.y-8 {
					if nt.Side == SideLeft {
						x = o.x - noteGap - w
					} else {
						x = o.x + o.w + noteGap
					}
					moved = true
				}
			}
			if !moved {
				break
			}
		}
		nodeBoxes = append(nodeBoxes, box{x, y, w, h})
		bd.AddRect(x, y, w, h)
		notes = append(notes, placedNote{nt, x, y, w, h, lines, tn})
	}

	if bd.Empty() {
		return block{w: 40, h: 20}, nil
	}
	ox, oy := -bd.MinX, -bd.MinY
	var b strings.Builder
	fmt.Fprintf(&b, `<g transform="translate(%s,%s)">`+"\n", svgutil.Num(ox), svgutil.Num(oy))
	edgeCol := svgutil.Esc(c.pal.Edge)
	for _, pn := range notes {
		t := pn.target
		a := domain.Point{X: pn.x + pn.w/2, Y: pn.y + pn.h/2}
		z := t.Center()
		switch {
		case pn.x >= t.Pos.X+t.Size.W:
			a.X, z.X = pn.x, t.Pos.X+t.Size.W
			z.Y = min(max(a.Y, t.Pos.Y+4), t.Pos.Y+t.Size.H-4)
		case pn.x+pn.w <= t.Pos.X:
			a.X, z.X = pn.x+pn.w, t.Pos.X
			z.Y = min(max(a.Y, t.Pos.Y+4), t.Pos.Y+t.Size.H-4)
		case pn.y+pn.h <= t.Pos.Y:
			a.Y, z.Y = pn.y+pn.h, t.Pos.Y
		default:
			a.Y, z.Y = pn.y, t.Pos.Y+t.Size.H
		}
		fmt.Fprintf(&b, `<path d="M%s,%s L%s,%s" fill="none" stroke="%s" stroke-dasharray="5 5"/>`+"\n",
			svgutil.Num(a.X), svgutil.Num(a.Y), svgutil.Num(z.X), svgutil.Num(z.Y), edgeCol)
	}
	for _, de := range drawn {
		fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="1.3" marker-end="url(#%s-arrow)"/>`+"\n", de.sh.d, edgeCol, c.id)
	}
	for _, s := range members {
		n := g.NodeByID(s.ID)
		if n == nil {
			continue
		}
		if bl, ok := inner[s.ID]; ok {
			fmt.Fprintf(&b, `<g transform="translate(%s,%s)">`+"\n%s</g>\n", svgutil.Num(n.Pos.X), svgutil.Num(n.Pos.Y), bl.body)
			continue
		}
		c.writeState(&b, s, n)
	}
	for _, pn := range notes {
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="#fff5ad" stroke="#aaaa33"/>`+"\n",
			svgutil.Num(pn.x), svgutil.Num(pn.y), svgutil.Num(pn.w), svgutil.Num(pn.h))
		c.lines(&b, pn.lines, pn.x+pn.w/2, pn.y+pn.h/2, c.fs*0.9, "#333333", "")
	}
	for _, de := range drawn {
		if de.lines == nil {
			continue
		}
		tw, th := c.textBlock(de.lines, c.fs*0.9)
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="#e8e8e8" fill-opacity="0.85"/>`+"\n",
			svgutil.Num(de.lx-tw/2-4), svgutil.Num(de.ly-th/2-2), svgutil.Num(tw+8), svgutil.Num(th+4))
		c.lines(&b, de.lines, de.lx, de.ly, c.fs*0.9, svgutil.Esc(c.pal.Text), "")
	}
	b.WriteString("</g>\n")
	return block{w: bd.MaxX - bd.MinX, h: bd.MaxY - bd.MinY, body: b.String()}, nil
}

// barEnds spreads the arrows that meet a fork or join bar along it, each
// toward the state at its other end, instead of all at one point.
func (c *ctx) barEnds(g *domain.Graph, e *domain.Edge, dir domain.Direction) []domain.Point {
	pts := e.Points
	isBar := func(id string) bool {
		s := c.d.state(id)
		return s != nil && (s.Kind == KindFork || s.Kind == KindJoin)
	}
	from, to := g.NodeByID(e.From), g.NodeByID(e.To)
	if from == nil || to == nil || !isBar(e.From) && !isBar(e.To) {
		return pts
	}
	out := append([]domain.Point(nil), pts...)
	fix := func(i int, bar, other *domain.Node) {
		oc, p := other.Center(), out[i]
		if vertical(dir) {
			p.X = min(max(oc.X, bar.Pos.X+6), bar.Pos.X+bar.Size.W-6)
			p.Y = bar.Pos.Y
			if oc.Y > bar.Center().Y {
				p.Y = bar.Pos.Y + bar.Size.H
			}
		} else {
			p.Y = min(max(oc.Y, bar.Pos.Y+6), bar.Pos.Y+bar.Size.H-6)
			p.X = bar.Pos.X
			if oc.X > bar.Center().X {
				p.X = bar.Pos.X + bar.Size.W
			}
		}
		out[i] = p
	}
	if isBar(e.From) {
		fix(0, from, to)
	}
	if isBar(e.To) {
		fix(len(out)-1, to, from)
	}
	if len(out) > 2 {
		out = []domain.Point{out[0], out[len(out)-1]}
	}
	return out
}

// composite lays out a composite state: its title bar over its machine,
// each concurrent region in a dashed box of its own.
func (c *ctx) composite(comp *Composite, dir domain.Direction) (block, error) {
	var regions []block
	for r := 0; r < max(comp.Regions, 1); r++ {
		has := false
		for _, s := range c.d.States {
			if s.Parent == comp.ID && s.Region == r {
				has = true
				break
			}
		}
		if !has {
			continue
		}
		bl, err := c.region(comp.ID, r, dir)
		if err != nil {
			return block{}, err
		}
		regions = append(regions, bl)
	}
	label := comp.Label
	if label == "" {
		label = comp.ID
	}
	titleLines := c.wrap(label, c.fs)
	tw, th := c.textBlock(titleLines, c.fs)
	titleH := th + 10

	// Stack the regions: down the page in a top-to-bottom diagram, across
	// it otherwise, as Mermaid does.
	stackDown := vertical(dir)
	var cw, ch float64
	type at struct{ x, y float64 }
	var pos []at
	rpad := 0.0
	if len(regions) > 1 {
		rpad = 12
	}
	for i, r := range regions {
		rw, rh := r.w+2*rpad, r.h+2*rpad
		if stackDown {
			if i > 0 {
				ch += regionGap
			}
			pos = append(pos, at{0, ch})
			ch += rh
			cw = max(cw, rw)
		} else {
			if i > 0 {
				cw += regionGap
			}
			pos = append(pos, at{cw, 0})
			cw += rw
			ch = max(ch, rh)
		}
	}
	if len(regions) == 0 {
		cw, ch = 40, 10
	}
	w := max(cw+2*compPad, tw+2*compPad)
	h := titleH + ch + 2*compPad

	var b strings.Builder
	stroke, fill := svgutil.Esc(c.pal.NodeStroke), svgutil.Esc(c.pal.NodeFill)
	n := svgutil.Num
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%s" height="%s" rx="5" fill="%s" stroke="%s"/>`+"\n", n(w), n(h), fill, stroke)
	fmt.Fprintf(&b, `<rect x="1" y="%s" width="%s" height="%s" fill="%s"/>`+"\n", n(titleH), n(w-2), n(h-titleH-1), svgutil.Esc(c.pal.Background))
	fmt.Fprintf(&b, `<line x1="0" y1="%s" x2="%s" y2="%s" stroke="%s"/>`+"\n", n(titleH), n(w), n(titleH), stroke)
	c.lines(&b, titleLines, w/2, titleH/2, c.fs, svgutil.Esc(c.pal.Text), "")
	x0 := (w - cw) / 2
	for i, r := range regions {
		p := pos[i]
		rx, ry := x0+p.x, titleH+compPad+p.y
		if len(regions) > 1 {
			rw, rh := r.w+2*rpad, r.h+2*rpad
			if stackDown {
				rw = cw
			} else {
				rh = ch
			}
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s" stroke-dasharray="6 4"/>`+"\n",
				n(rx), n(ry), n(rw), n(rh), mix(c.pal.NodeFill, c.pal.Background, 0.5, fill), stroke)
			rx += (rw - r.w) / 2
			ry += (rh - r.h) / 2
		} else {
			rx = (w - r.w) / 2
		}
		fmt.Fprintf(&b, `<g transform="translate(%s,%s)">`+"\n%s</g>\n", n(rx), n(ry), r.body)
	}
	return block{w: w, h: h, body: b.String()}, nil
}

func (c *ctx) stateSize(s *State) domain.Size {
	tw, th := c.textBlock(c.wrap(s.Label, c.fs), c.fs)
	return domain.Size{W: math.Ceil(max(tw+2*c.fs*1.1, c.fs*4.5)), H: math.Ceil(th + c.fs*1.1)}
}

// style resolves a state's classDef styles.
func (c *ctx) style(s *State) Style {
	var st Style
	for _, name := range s.Classes {
		cd := c.d.ClassDefs[name]
		pick := func(a *string, v string) {
			if v != "" {
				*a = v
			}
		}
		pick(&st.Fill, cd.Fill)
		pick(&st.Stroke, cd.Stroke)
		pick(&st.StrokeWidth, cd.StrokeWidth)
		pick(&st.Dash, cd.Dash)
		pick(&st.Color, cd.Color)
		pick(&st.FontWeight, cd.FontWeight)
	}
	return st
}

func (c *ctx) writeState(b *strings.Builder, s *State, n *domain.Node) {
	ctr := n.Center()
	edge := svgutil.Esc(c.pal.Edge)
	num := svgutil.Num
	switch {
	case s.Start:
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", num(ctr.X), num(ctr.Y), num(startR), edge)
	case s.End:
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="none" stroke="%s" stroke-width="1.5"/>`+"\n", num(ctr.X), num(ctr.Y), num(endR-0.75), edge)
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", num(ctr.X), num(ctr.Y), num(endR-3.5), edge)
	case s.Kind == KindFork || s.Kind == KindJoin:
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"/>`+"\n",
			num(n.Pos.X), num(n.Pos.Y), num(n.Size.W), num(n.Size.H), edge)
	default:
		st := c.style(s)
		fill, stroke, text := svgutil.Esc(c.pal.NodeFill), svgutil.Esc(c.pal.NodeStroke), svgutil.Esc(c.pal.Text)
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
		if s.Kind == KindChoice {
			x, y, w, h := n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H
			fmt.Fprintf(b, `<polygon points="%s,%s %s,%s %s,%s %s,%s" fill="%s" stroke="%s"%s/>`+"\n",
				num(x+w/2), num(y), num(x+w), num(y+h/2), num(x+w/2), num(y+h), num(x), num(y+h/2), fill, stroke, extra)
			return
		}
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="5" fill="%s" stroke="%s"%s/>`+"\n",
			num(n.Pos.X), num(n.Pos.Y), num(n.Size.W), num(n.Size.H), fill, stroke, extra)
		fw := ""
		if st.FontWeight != "" {
			fw = ` font-weight="` + svgutil.Esc(st.FontWeight) + `"`
		}
		c.lines(b, c.wrap(s.Label, c.fs), ctr.X, ctr.Y, c.fs, text, fw)
	}
}

// lines writes text lines centred on (cx, cy).
func (c *ctx) lines(b *strings.Builder, lines []string, cx, cy, size float64, fill, extra string) {
	if len(lines) == 0 {
		return
	}
	lh := size * 1.3
	y0 := cy - float64(len(lines)-1)*lh/2 + size*0.35
	fmt.Fprintf(b, `<text fill="%s" font-size="%s" text-anchor="middle"%s>`, fill, svgutil.Num(size), extra)
	for i, l := range lines {
		fmt.Fprintf(b, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(cx), svgutil.Num(y0+float64(i)*lh), svgutil.Esc(l))
	}
	b.WriteString("</text>\n")
}

func (c *ctx) svg(root block) []byte {
	pad := c.o.Padding
	titleH := svgutil.TitleHeight(c.o.Title, c.fs)
	w := root.w + 2*pad
	h := root.h + titleH + 2*pad
	if c.o.Title != "" {
		w = max(w, c.face.Width(c.o.Title, c.fs)+2*pad)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(fontFamily(c.o.FontFace)), svgutil.Num(c.fs))
	fmt.Fprintf(&b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(c.pal.Background))
	fmt.Fprintf(&b, `  <defs><marker id="%s-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" markerUnits="userSpaceOnUse" orient="auto"><path d="M0,0 L10,5 L0,10 z" fill="%s"/></marker></defs>`+"\n",
		c.id, svgutil.Esc(c.pal.Edge))
	if c.o.Title != "" {
		fmt.Fprintf(&b, `  <text x="%s" y="%s" fill="%s" text-anchor="middle" font-weight="bold">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+c.fs), svgutil.Esc(c.pal.Text), svgutil.Esc(c.o.Title))
	}
	fmt.Fprintf(&b, `  <g transform="translate(%s,%s)">`+"\n%s  </g>\n", svgutil.Num((w-root.w)/2), svgutil.Num(pad+titleH), root.body)
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

func path(pts []domain.Point) string {
	var d strings.Builder
	for i, p := range pts {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&d, "%s%s,%s ", cmd, svgutil.Num(p.X), svgutil.Num(p.Y))
	}
	return strings.TrimSpace(d.String())
}

var plainFont = regexp.MustCompile(`^[A-Za-z0-9 ,'"_-]{1,200}$`)

// fontFamily returns face when it is a plain font list, else sans-serif:
// a font option is written into an attribute, so it must carry nothing else.
func fontFamily(face string) string {
	l := strings.ToLower(face)
	if !plainFont.MatchString(face) || strings.Contains(l, "javascript") || strings.Contains(l, "expression") {
		return "sans-serif"
	}
	return face
}

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
