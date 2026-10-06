package c4

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/curve"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/svgid"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls C4 appearance.
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
	minW      = 170.0
	maxTextW  = 200.0
	boxPad    = 12.0
	bPad      = 18.0 // inside a boundary, around its contents
	labelMaxW = 180.0
)

// look is an element kind's colours and its [type] wording, after
// look is how an element is drawn: its colors (escaped) and its [type].
type look struct{ fill, stroke, text, typ string }

func lookFor(e *Element, cc theme.C4Colors) look {
	ext := strings.HasSuffix(e.Kind, "_Ext")
	base := strings.TrimSuffix(e.Kind, "_Ext")
	techn := func(t string) string {
		if e.Techn != "" {
			return t + ": " + e.Techn
		}
		return t
	}
	var c theme.C4Look
	var typ string
	switch {
	case base == "Person":
		c, typ = cc.Person, "Person"
		if ext {
			c, typ = cc.ExternalPerson, "External Person"
		}
	case strings.HasPrefix(base, "System"):
		c, typ = cc.System, "Software System"
		if ext {
			c, typ = cc.ExternalSystem, "External System"
		}
	case strings.HasPrefix(base, "Container"):
		c, typ = cc.Container, techn("Container")
		if ext {
			c, typ = cc.ExternalContainer, techn("External Container")
		}
	default:
		c, typ = cc.Component, techn("Component")
		if ext {
			c, typ = cc.ExternalComponent, techn("External Component")
		}
	}
	if e.Style.Fill != "" {
		c.Fill, c.Text = e.Style.Fill, theme.TextOn(e.Style.Fill, c.Text)
	}
	if e.Style.Stroke != "" {
		c.Stroke = e.Style.Stroke
	}
	if e.Style.Text != "" {
		c.Text = e.Style.Text
	}
	return look{svgutil.Esc(c.Fill), svgutil.Esc(c.Stroke), svgutil.Esc(c.Text), typ}
}

type metrics struct {
	face svgutil.Face
	fs   float64
	ff   string // the font family the layout measures with
}

func (m metrics) lines(s string, size, maxW float64) []string { return m.face.WrapHard(s, size, maxW) }

// elementText is an element's label, [type] and description lines.
func elementText(e *Element, m metrics) (label, typ, descr []string) {
	return m.lines(e.Label, m.fs, maxTextW), m.lines("["+lookFor(e, theme.C4Colors{}).typ+"]", m.fs*0.78, maxTextW), m.lines(e.Descr, m.fs*0.85, maxTextW)
}

func headR(m metrics) float64 { return m.fs * 1.25 }

func isPerson(e *Element) bool { return strings.HasPrefix(e.Kind, "Person") }

func elementSize(e *Element, m metrics) (float64, float64) {
	label, typ, descr := elementText(e, m)
	w := 0.0
	for _, l := range label {
		w = max(w, m.face.Width(l, m.fs)*1.07)
	}
	for _, l := range typ {
		w = max(w, m.face.Width(l, m.fs*0.78))
	}
	for _, l := range descr {
		w = max(w, m.face.Width(l, m.fs*0.85))
	}
	h := 2*boxPad + float64(len(label))*m.fs*1.3 + float64(len(typ))*m.fs*0.78*1.3
	if e.Descr != "" {
		h += 6 + float64(len(descr))*m.fs*0.85*1.3
	}
	switch {
	case isPerson(e):
		h += headR(m) * 1.6
	case strings.Contains(e.Kind, "Db"):
		h += 16
	case strings.Contains(e.Kind, "Queue"):
		w += 24
	}
	return math.Ceil(max(w+2*boxPad, minW)), math.Ceil(h)
}

type rect struct{ x, y, w, h float64 }

type ctx struct {
	d     *Diagram
	m     metrics
	pal   theme.Palette
	rects map[string]rect   // absolute boxes of elements and boundaries
	local map[*Boundary]sub // each boundary's own layout
}

type sub struct {
	g      *domain.Graph
	w, h   float64
	ox, oy float64 // where the layout's origin sits inside the boundary
}

// Render parses and renders C4 source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	if o.Title == "" {
		o.Title = d.Title
	}
	c := &ctx{d: d, m: metrics{face: svgutil.FaceFor(o.FontFace), fs: fs, ff: o.FontFace}, pal: theme.For(o.Theme),
		rects: map[string]rect{}, local: map[*Boundary]sub{}}
	w, h, err := c.layout(d.Root, true)
	if err != nil {
		return nil, err
	}
	c.place(d.Root, 0, 0)
	return c.svg(o, w, h, svgid.For(o.IDPrefix, src)), nil
}

func childID(it any) string {
	switch v := it.(type) {
	case *Element:
		return v.ID
	case *Boundary:
		return v.ID
	}
	return ""
}

// lift returns the child of b that is or contains id, or "".
func (c *ctx) lift(id string, b *Boundary) string {
	var parentOf func(cur *Boundary) string
	parentOf = func(cur *Boundary) string {
		for _, it := range cur.Children {
			if childID(it) == id {
				return id
			}
			if nb, ok := it.(*Boundary); ok {
				if parentOf(nb) != "" {
					return nb.ID
				}
			}
		}
		return ""
	}
	return parentOf(b)
}

// boundaryHead is the height of a boundary's label and [type].
func (c *ctx) boundaryHead(b *Boundary) float64 {
	h := c.m.fs * 1.4
	if b.Type != "" {
		h += c.m.fs * 0.8 * 1.3
	}
	return h + 4
}

// layout lays out the children of b and returns its size.
func (c *ctx) layout(b *Boundary, root bool) (float64, float64, error) {
	g := &domain.Graph{Direction: domain.TopBottom}
	for _, it := range b.Children {
		var w, h float64
		switch v := it.(type) {
		case *Element:
			w, h = elementSize(v, c.m)
		case *Boundary:
			var err error
			if w, h, err = c.layout(v, false); err != nil {
				return 0, 0, err
			}
		}
		g.Nodes = append(g.Nodes, &domain.Node{ID: childID(it), Label: " ", Shape: domain.ShapeRect, Size: domain.Size{W: w, H: h}})
	}
	labelled := false
	for _, r := range c.d.Rels {
		from, to := c.lift(r.From, b), c.lift(r.To, b)
		if from == "" || to == "" || from == to {
			continue
		}
		if r.Kind == "Rel_U" || r.Kind == "Rel_Up" || r.Kind == "Rel_Back" {
			from, to = to, from
		}
		e := &domain.Edge{From: from, To: to}
		if r.Label != "" || r.Tech != "" {
			labelled = true
			e.Label = strings.Join(c.relLines(r), "\n")
		}
		g.Edges = append(g.Edges, e)
	}
	var w, h float64
	if len(g.Nodes) > 0 {
		rankSep := 60.0
		if labelled {
			rankSep = 80
		}
		_, err := layout.Compute(g, layout.Options{NodeSep: 50, RankSep: rankSep, FontSize: c.m.fs * 0.85, FontFace: c.m.ff})
		if err != nil {
			return 0, 0, err
		}
		var bd svgutil.Bounds
		for _, n := range g.Nodes {
			bd.AddRect(n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H)
		}
		for _, e := range g.Edges {
			for _, p := range e.Points {
				bd.Add(p.X, p.Y)
			}
		}
		ox, oy := bd.Offset()
		w, h = bd.Size()
		s := sub{g: g, w: w, h: h, ox: ox, oy: oy}
		if !root {
			s.ox += bPad
			s.oy += bPad + c.boundaryHead(b)
		}
		c.local[b] = s
	}
	if root {
		return w, h, nil
	}
	lw := c.m.face.Width(b.Label, c.m.fs) * 1.07
	return max(w, lw, 120) + 2*bPad, h + 2*bPad + c.boundaryHead(b), nil
}

// place records absolute boxes for b's children, with b's top-left at
// (x, y).
func (c *ctx) place(b *Boundary, x, y float64) {
	s, ok := c.local[b]
	if !ok {
		return
	}
	for _, it := range b.Children {
		n := s.g.NodeByID(childID(it))
		if n == nil {
			continue
		}
		r := rect{x + s.ox + n.Pos.X, y + s.oy + n.Pos.Y, n.Size.W, n.Size.H}
		c.rects[childID(it)] = r
		if nb, ok := it.(*Boundary); ok {
			c.place(nb, r.x, r.y)
		}
	}
}

// relLines is a relationship's label lines and its [technology] line.
func (c *ctx) relLines(r *Rel) []string {
	var out []string
	if r.Label != "" {
		out = append(out, c.m.lines(r.Label, c.m.fs*0.85, labelMaxW)...)
	}
	if r.Tech != "" {
		out = append(out, c.m.lines("["+r.Tech+"]", c.m.fs*0.75, labelMaxW)...)
	}
	return out
}

type drawnRel struct {
	r      *Rel
	sh     curve.Shape
	lx, ly float64
}

// route draws a relationship from its scope's layout when both ends are
// laid out together, else as a straight line between the two boxes.
func (c *ctx) route(r *Rel) (drawnRel, bool) {
	fr, ok1 := c.rects[r.From]
	tr, ok2 := c.rects[r.To]
	if !ok1 || !ok2 || r.From == r.To {
		return drawnRel{}, false
	}
	// Find the scope that lays out both ends directly.
	var scope *Boundary
	var walk func(b *Boundary)
	walk = func(b *Boundary) {
		if scope != nil {
			return
		}
		f, t := c.lift(r.From, b), c.lift(r.To, b)
		if f == r.From && t == r.To {
			scope = b
			return
		}
		for _, it := range b.Children {
			if nb, ok := it.(*Boundary); ok {
				walk(nb)
			}
		}
	}
	walk(c.d.Root)
	if scope != nil {
		s := c.local[scope]
		var bx, by float64
		if scope != c.d.Root {
			br := c.rects[scope.ID]
			bx, by = br.x, br.y
		}
		for _, e := range s.g.Edges {
			if !(e.From == r.From && e.To == r.To || e.From == r.To && e.To == r.From) || len(e.Points) < 2 {
				continue
			}
			pts := make([]domain.Point, len(e.Points))
			for i, p := range e.Points {
				pts[i] = domain.Point{X: p.X + bx + s.ox, Y: p.Y + by + s.oy}
			}
			if e.From != r.From {
				for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
					pts[i], pts[j] = pts[j], pts[i]
				}
			}
			var obs []curve.Box
			for id, rr := range c.rects {
				if id != r.From && id != r.To && c.d.element(id) != nil {
					obs = append(obs, curve.Box{X: rr.x, Y: rr.y, W: rr.w, H: rr.h})
				}
			}
			sh := curve.Edge(pts, true, obs, 0, 0)
			lx, ly := sh.Mid.X, sh.Mid.Y
			if !sh.Curved {
				lx, ly = e.LabelPos.X+bx+s.ox, e.LabelPos.Y+by+s.oy-c.m.fs*0.3
			}
			return drawnRel{r, sh, lx, ly}, true
		}
	}
	// Ends laid out in different boundaries: a straight line, or a right
	// angle around the boxes in between, with the label where no box is.
	a := domain.Point{X: fr.x + fr.w/2, Y: fr.y + fr.h/2}
	z := domain.Point{X: tr.x + tr.w/2, Y: tr.y + tr.h/2}
	var obs []rect
	for id, rr := range c.rects {
		if id != r.From && id != r.To && c.d.element(id) != nil {
			obs = append(obs, rr)
		}
	}
	hits := func(p, q domain.Point) bool {
		for _, o := range obs {
			if segHitsRect(p, q, o) {
				return true
			}
		}
		return false
	}
	pts := []domain.Point{clipRect(fr, a, z), clipRect(tr, z, a)}
	if hits(pts[0], pts[1]) {
		for _, corner := range []domain.Point{{X: z.X, Y: a.Y}, {X: a.X, Y: z.Y}} {
			if inside(fr, corner) || inside(tr, corner) {
				continue
			}
			p0, p2 := clipRect(fr, a, corner), clipRect(tr, z, corner)
			if !hits(p0, corner) && !hits(corner, p2) {
				pts = []domain.Point{p0, corner, p2}
				break
			}
		}
	}
	sh := curve.Shape{Start: pts[0], End: pts[len(pts)-1], D: curve.Path(pts), Pts: pts}
	tw, th := c.textBox(c.relLines(r))
	best := domain.PolylineMidpoint(pts)
	total := domain.PolylineLength(pts)
	for _, f := range []float64{0.5, 0.4, 0.6, 0.3, 0.7, 0.2, 0.8} {
		p := domain.PolylinePointAt(pts, total*f)
		lr := rect{p.X - tw/2 - 4, p.Y - th/2 - 2, tw + 8, th + 4}
		clear := true
		for _, o := range obs {
			if lr.x < o.x+o.w && lr.x+lr.w > o.x && lr.y < o.y+o.h && lr.y+lr.h > o.y {
				clear = false
				break
			}
		}
		if clear {
			best = p
			break
		}
	}
	sh.Mid = best
	return drawnRel{r, sh, best.X, best.Y}, true
}

func inside(r rect, p domain.Point) bool {
	return p.X > r.x && p.X < r.x+r.w && p.Y > r.y && p.Y < r.y+r.h
}

// segHitsRect reports whether segment p-q passes through the inside of r.
func segHitsRect(p, q domain.Point, r rect) bool {
	const in = 2.0
	x0, y0, x1, y1 := r.x+in, r.y+in, r.x+r.w-in, r.y+r.h-in
	t0, t1 := 0.0, 1.0
	dx, dy := q.X-p.X, q.Y-p.Y
	for _, c := range [4][2]float64{{-dx, p.X - x0}, {dx, x1 - p.X}, {-dy, p.Y - y0}, {dy, y1 - p.Y}} {
		if c[0] == 0 {
			if c[1] < 0 {
				return false
			}
			continue
		}
		t := c[1] / c[0]
		if c[0] < 0 {
			t0 = max(t0, t)
		} else {
			t1 = min(t1, t)
		}
		if t0 > t1 {
			return false
		}
	}
	return true
}

// clipRect moves p (the centre of r) to r's border toward q.
func clipRect(r rect, p, q domain.Point) domain.Point {
	dx, dy := q.X-p.X, q.Y-p.Y
	t := 1.0
	if dx != 0 {
		t = min(t, math.Abs(r.w/2/dx))
	}
	if dy != 0 {
		t = min(t, math.Abs(r.h/2/dy))
	}
	return domain.Point{X: p.X + dx*t, Y: p.Y + dy*t}
}

func (c *ctx) svg(o RenderOptions, cw, ch float64, id string) []byte {
	pad := o.Padding
	fs := c.m.fs
	titleH := 0.0
	if o.Title != "" {
		titleH = fs*1.3*1.4 + 8
	}
	var rels []drawnRel
	var bd svgutil.Bounds
	bd.AddRect(0, 0, cw, ch)
	for _, r := range c.d.Rels {
		dr, ok := c.route(r)
		if !ok {
			continue
		}
		bd.Add(dr.sh.Start.X, dr.sh.Start.Y)
		bd.Add(dr.sh.End.X, dr.sh.End.Y)
		lines := c.relLines(r)
		tw, th := c.textBox(lines)
		bd.AddRect(dr.lx-tw/2-4, dr.ly-th/2-2, tw+8, th+4)
		rels = append(rels, dr)
	}
	// Move a label that would hide another line, label or element along its
	// own line.
	shapes := make([]curve.Shape, len(rels))
	labels := make([]curve.Label, len(rels))
	for i, dr := range rels {
		shapes[i] = dr.sh
		tw, th := c.textBox(c.relLines(dr.r))
		labels[i] = curve.Label{Line: i, W: tw + 8, H: th + 4, X: dr.lx, Y: dr.ly}
	}
	ids := make([]string, 0, len(c.rects))
	for id := range c.rects {
		if c.d.element(id) != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var boxes []curve.Box
	for _, id := range ids {
		rr := c.rects[id]
		boxes = append(boxes, curve.Box{X: rr.x, Y: rr.y, W: rr.w, H: rr.h})
	}
	// A boundary's dashed border is a line to keep clear too, and its
	// title a box.
	for _, bo := range c.d.Boundaries {
		r, ok := c.rects[bo.ID]
		if !ok {
			continue
		}
		shapes = append(shapes, curve.Shape{Pts: []domain.Point{{X: r.x, Y: r.y}, {X: r.x + r.w, Y: r.y}, {X: r.x + r.w, Y: r.y + r.h}, {X: r.x, Y: r.y + r.h}, {X: r.x, Y: r.y}}})
		th := 8 + c.m.fs*1.3
		if bo.Type != "" {
			th += c.m.fs * 0.8 * 1.3
		}
		boxes = append(boxes, curve.Box{X: r.x, Y: r.y, W: c.m.face.Bold().Width(bo.Label, c.m.fs) + bPad*1.2, H: th})
	}
	curve.PlaceLabels(shapes, labels, boxes)
	for i, l := range labels {
		rels[i].lx, rels[i].ly = l.X, l.Y
	}
	// Nudge labels apart where two would overlap.
	type lb struct{ x0, y0, x1, y1 float64 }
	boxOf := func(dr drawnRel) lb {
		tw, th := c.textBox(c.relLines(dr.r))
		return lb{dr.lx - tw/2 - 4, dr.ly - th/2 - 2, dr.lx + tw/2 + 4, dr.ly + th/2 + 2}
	}
	for i := range rels {
		for pass := 0; pass < 4; pass++ {
			moved := false
			a := boxOf(rels[i])
			for j := 0; j < i; j++ {
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
	sx, sy := bd.Offset()
	w0, h0 := bd.Size()
	w := w0 + 2*pad
	h := h0 + 2*pad + titleH
	if o.Title != "" {
		w = max(w, c.m.face.Width(o.Title, fs*1.3)*1.07+2*pad)
	}

	var b strings.Builder
	edge := svgutil.Esc(c.pal.C4.Line)
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(c.pal.Background))
	fmt.Fprintf(&b, `  <defs><marker id="%s-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="9" markerHeight="9" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="%s"/></marker></defs>`+"\n", id, edge)
	if o.Title != "" {
		fmt.Fprintf(&b, `  <text x="%s" y="%s" fill="%s" font-size="%s" font-weight="bold">%s</text>`+"\n",
			svgutil.Num(pad), svgutil.Num(pad+fs*1.3), svgutil.Esc(c.pal.Text), svgutil.Num(fs*1.3), svgutil.Esc(o.Title))
	}
	fmt.Fprintf(&b, `  <g transform="translate(%s,%s)">`+"\n", svgutil.Num(pad+sx), svgutil.Num(pad+titleH+sy))
	for _, bo := range c.d.Boundaries {
		c.writeBoundary(&b, bo)
	}
	for _, dr := range rels {
		col := edge
		if dr.r.Style.Stroke != "" {
			col = svgutil.Esc(dr.r.Style.Stroke)
		}
		markers := ""
		switch dr.r.Kind {
		case "Rel_Back":
			markers = fmt.Sprintf(` marker-start="url(#%s-arrow)"`, id)
		case "BiRel":
			markers = fmt.Sprintf(` marker-start="url(#%s-arrow)" marker-end="url(#%s-arrow)"`, id, id)
		default:
			markers = fmt.Sprintf(` marker-end="url(#%s-arrow)"`, id)
		}
		fmt.Fprintf(&b, `    <path d="%s" fill="none" stroke="%s" stroke-width="1.2"%s/>`+"\n", dr.sh.D, col, markers)
	}
	for _, e := range c.d.Elements {
		c.writeElement(&b, e)
	}
	for _, dr := range rels {
		lines := c.relLines(dr.r)
		if len(lines) == 0 {
			continue
		}
		tw, th := c.textBox(lines)
		text := svgutil.Esc(c.pal.Text)
		if dr.r.Style.Text != "" {
			text = svgutil.Esc(dr.r.Style.Text)
		}
		fmt.Fprintf(&b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s" fill-opacity="0.85"/>`+"\n",
			svgutil.Num(dr.lx-tw/2-4), svgutil.Num(dr.ly-th/2-2), svgutil.Num(tw+8), svgutil.Num(th+4), svgutil.Esc(c.pal.Background))
		y := dr.ly - th/2
		nLabel := len(c.m.lines(dr.r.Label, fs*0.85, labelMaxW))
		if dr.r.Label == "" {
			nLabel = 0
		}
		for i, l := range lines {
			size, style := fs*0.85, ""
			if i >= nLabel {
				size, style = fs*0.75, ` font-style="italic"`
			}
			y += size * 1.3
			fmt.Fprintf(&b, `    <text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle"%s>%s</text>`+"\n",
				svgutil.Num(dr.lx), svgutil.Num(y-size*0.35), text, svgutil.Num(size), style, svgutil.Esc(l))
		}
	}
	b.WriteString("  </g>\n</svg>\n")
	return []byte(b.String())
}

func (c *ctx) textBox(lines []string) (float64, float64) {
	w, h := 0.0, 0.0
	for _, l := range lines {
		size := c.m.fs * 0.85
		if strings.HasPrefix(l, "[") {
			size = c.m.fs * 0.75
		}
		w = max(w, c.m.face.Width(l, size))
		h += size * 1.3
	}
	return w, h
}

func (c *ctx) writeBoundary(b *strings.Builder, bo *Boundary) {
	r, ok := c.rects[bo.ID]
	if !ok {
		return
	}
	n := svgutil.Num
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="4" fill="none" stroke="%s" stroke-dasharray="7 7"/>`+"\n",
		n(r.x), n(r.y), n(r.w), n(r.h), svgutil.Esc(c.pal.C4.Line))
	y := r.y + 8 + c.m.fs
	fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" font-weight="bold">%s</text>`+"\n", n(r.x+bPad*0.6), n(y), svgutil.Esc(c.pal.Text), svgutil.Esc(bo.Label))
	if bo.Type != "" {
		y += c.m.fs * 0.8 * 1.3
		fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" font-size="%s">%s</text>`+"\n", n(r.x+bPad*0.6), n(y), svgutil.Esc(c.pal.Text), n(c.m.fs*0.8), svgutil.Esc("["+bo.Type+"]"))
	}
}

func (c *ctx) writeElement(b *strings.Builder, e *Element) {
	r, ok := c.rects[e.ID]
	if !ok {
		return
	}
	l := lookFor(e, c.pal.C4)
	n := svgutil.Num
	x, y, w, h := r.x, r.y, r.w, r.h
	attrs := fmt.Sprintf(`fill="%s" stroke="%s"`, l.fill, l.stroke)
	switch {
	case isPerson(e):
		hr := headR(c.m)
		top := y + hr*1.6
		fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="14" %s/>`+"\n", n(x), n(top), n(w), n(y+h-top), attrs)
		fmt.Fprintf(b, `    <circle cx="%s" cy="%s" r="%s" %s/>`+"\n", n(x+w/2), n(y+hr), n(hr), attrs)
		y, h = top, y+h-top
	case strings.Contains(e.Kind, "Db"):
		ry := 8.0
		fmt.Fprintf(b, `    <path d="M%s,%s A%s,%s 0 0 1 %s,%s V%s A%s,%s 0 0 1 %s,%s Z" %s/>`+"\n",
			n(x), n(y+ry), n(w/2), n(ry), n(x+w), n(y+ry), n(y+h-ry), n(w/2), n(ry), n(x), n(y+h-ry), attrs)
		fmt.Fprintf(b, `    <path d="M%s,%s A%s,%s 0 0 0 %s,%s" fill="none" stroke="%s"/>`+"\n", n(x), n(y+ry), n(w/2), n(ry), n(x+w), n(y+ry), l.stroke)
		y, h = y+ry*2, h-ry*2
	case strings.Contains(e.Kind, "Queue"):
		rx := 12.0
		fmt.Fprintf(b, `    <path d="M%s,%s H%s A%s,%s 0 0 1 %s,%s H%s A%s,%s 0 0 1 %s,%s Z" %s/>`+"\n",
			n(x+rx), n(y), n(x+w-rx), n(rx), n(h/2), n(x+w-rx), n(y+h), n(x+rx), n(rx), n(h/2), n(x+rx), n(y), attrs)
		fmt.Fprintf(b, `    <path d="M%s,%s A%s,%s 0 0 0 %s,%s" fill="none" stroke="%s"/>`+"\n", n(x+w-rx), n(y), n(rx), n(h/2), n(x+w-rx), n(y+h), l.stroke)
		w -= rx
	default:
		fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="4" %s/>`+"\n", n(x), n(y), n(w), n(h), attrs)
	}
	label, typ, descr := elementText(e, c.m)
	total := float64(len(label))*c.m.fs*1.3 + float64(len(typ))*c.m.fs*0.78*1.3
	if e.Descr != "" {
		total += 6 + float64(len(descr))*c.m.fs*0.85*1.3
	}
	ty := y + h/2 - total/2
	cx := x + w/2
	write := func(lines []string, size float64, extra string) {
		for _, ln := range lines {
			ty += size * 1.3
			fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle"%s>%s</text>`+"\n",
				n(cx), n(ty-size*0.35), l.text, n(size), extra, svgutil.Esc(ln))
		}
	}
	write(label, c.m.fs, ` font-weight="bold"`)
	write(typ, c.m.fs*0.78, "")
	if e.Descr != "" {
		ty += 6
		write(descr, c.m.fs*0.85, "")
	}
}
