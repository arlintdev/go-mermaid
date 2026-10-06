// Package render turns a laid-out graph into SVG bytes. Output is
// deterministic so it can be compared against golden files.
//
// The SVG is static and plain: presentation attributes only, no style
// attribute or element, no foreignObject, no links or scripts. Every text
// node and attribute value is either a number, a constant, a value checked
// by cssval, or escaped.
package render

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// Options controls SVG appearance.
type Options struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
	Curved   bool
	// Vars overrides palette colors (from a diagram's init directive); its
	// values must already be validated.
	Vars theme.Palette
	// IDPrefix starts every id in the picture (markers), so two pictures on
	// one page never share one. It must be letters, digits, '-' or '_' and
	// start with a letter; anything else is replaced by "m".
	IDPrefix string
}

var idPrefixRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

func (o Options) prefix() string {
	if idPrefixRe.MatchString(o.IDPrefix) {
		return o.IDPrefix
	}
	return "m"
}

// Edge stroke widths and marker sizes, as mermaid.js draws them.
const (
	edgeWidth      = 1.5
	thickWidth     = 3.5
	arrowLen       = 11.0
	arrowWide      = 12.0
	circleMarker   = 13.0
	crossMarker    = 13.0
	cornerRadius   = 5.0
	curvedRadius   = 14.0
	titleFontScale = 1.15
)

// renderer carries the per-picture state.
type renderer struct {
	b       strings.Builder
	opts    Options
	pal     theme.Palette
	face    svgutil.Face
	markers map[string]string // kind|color -> id
	defs    strings.Builder
}

// SVG renders a laid-out graph to an SVG document. Graphs laid out by
// layout.Flow carry wrapped lines and subgraph boxes; older layouts are
// drawn with their labels split at explicit line breaks.
func SVG(res *layout.Result, opts Options) ([]byte, error) {
	if opts.FontSize <= 0 {
		opts.FontSize = 16
	}
	r := &renderer{
		opts:    opts,
		pal:     theme.For(opts.Theme).Over(opts.Vars).Flow(),
		face:    svgutil.FaceFor(opts.FontFace),
		markers: map[string]string{},
	}
	return r.render(res), nil
}

func (r *renderer) render(res *layout.Result) []byte {
	g := res.Graph
	pad := r.opts.Padding
	titleH := 0.0
	titleSize := r.opts.FontSize * titleFontScale
	if r.opts.Title != "" {
		titleH = titleSize*1.5 + 8
	}
	contentW := res.Width
	if tw := r.face.Width(r.opts.Title, titleSize); tw > contentW {
		contentW = tw
	}
	shiftX := (contentW - res.Width) / 2
	w := contentW + 2*pad
	h := res.Height + titleH + 2*pad

	var body strings.Builder
	r.drawBody(&body, g)

	fmt.Fprintf(&r.b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		num(w), num(h), num(w), num(h), esc(r.opts.FontFace), num(r.opts.FontSize))
	if r.defs.Len() > 0 {
		r.b.WriteString("  <defs>\n")
		r.b.WriteString(r.defs.String())
		r.b.WriteString("  </defs>\n")
	}
	fmt.Fprintf(&r.b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", esc(r.pal.Background))
	if r.opts.Title != "" {
		fmt.Fprintf(&r.b, `  <text x="%s" y="%s" fill="%s" text-anchor="middle" font-size="%s" font-weight="bold">%s</text>`+"\n",
			num(w/2), num(pad+titleSize), esc(r.pal.Text), num(titleSize), esc(r.opts.Title))
	}
	fmt.Fprintf(&r.b, `  <g transform="translate(%s,%s)">`+"\n", num(pad+shiftX), num(pad+titleH))
	r.b.WriteString(body.String())
	r.b.WriteString("  </g>\n</svg>\n")
	return []byte(r.b.String())
}

func (r *renderer) drawBody(b *strings.Builder, g *domain.Graph) {
	clusters := clusterOrder(g)
	for _, sg := range clusters {
		r.drawCluster(b, sg)
	}
	for _, e := range g.Edges {
		r.drawEdge(b, e)
	}
	// Titles go over the edges: one an edge must cross is drawn on a patch
	// of its box's fill, so the line passes behind the words.
	for _, sg := range clusters {
		r.drawClusterTitle(b, sg, g.Edges)
	}
	for _, e := range g.Edges {
		r.drawEdgeLabel(b, e)
	}
	for _, n := range g.Nodes {
		r.drawNode(b, n)
	}
}

// clusterOrder lists subgraphs outermost first, so inner boxes are drawn on
// top of the boxes around them.
func clusterOrder(g *domain.Graph) []*domain.Subgraph {
	depth := func(sg *domain.Subgraph) int {
		d := 0
		for p := sg.Parent; p != "" && d <= len(g.Subgraphs); d++ {
			parent := g.SubgraphByID(p)
			if parent == nil {
				break
			}
			p = parent.Parent
		}
		return d
	}
	out := append([]*domain.Subgraph(nil), g.Subgraphs...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && depth(out[j]) < depth(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func (r *renderer) drawCluster(b *strings.Builder, sg *domain.Subgraph) {
	box := sg.Box
	if box.Size.W <= 0 || box.Size.H <= 0 {
		return
	}
	fill, stroke := r.pal.ClusterFill, r.pal.ClusterStroke
	extra := ""
	if st := sg.Style; st != nil {
		fill, stroke = pick(st.Fill, fill), pick(st.Stroke, stroke)
		extra = strokeExtras(st)
	}
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"%s/>`+"\n",
		num(box.Min.X), num(box.Min.Y), num(box.Size.W), num(box.Size.H), esc(fill), esc(stroke), extra)
}

func (r *renderer) drawClusterTitle(b *strings.Builder, sg *domain.Subgraph, edges []*domain.Edge) {
	box := sg.Box
	lines := sg.TitleLines
	if len(lines) == 0 && sg.Title != "" {
		lines = svgutil.SplitLines(sg.Title)
	}
	if box.Size.W <= 0 || len(lines) == 0 {
		return
	}
	fill, text := r.pal.ClusterFill, r.pal.Text
	if st := sg.Style; st != nil {
		if st.Fill != "" {
			fill, text = st.Fill, theme.TextOn(st.Fill, text)
		}
		text = pick(st.Color, text)
	}
	lh := r.opts.FontSize * 1.5
	top := box.Min.Y + 6
	cx := box.Min.X + box.Size.W/2
	if sg.TitleX != 0 {
		cx = sg.TitleX
	}
	tw := r.face.LinesWidth(lines, r.opts.FontSize) + 8
	th := lh * float64(len(lines))
	x0, x1, y0, y1 := cx-tw/2, cx+tw/2, top, top+th
	for _, e := range edges {
		if e.Line == domain.LineInvisible {
			continue
		}
		crossed := false
		for i := 1; i < len(e.Points); i++ {
			p, q := e.Points[i-1], e.Points[i]
			if math.Min(p.X, q.X) < x1 && math.Max(p.X, q.X) > x0 && math.Min(p.Y, q.Y) < y1 && math.Max(p.Y, q.Y) > y0 {
				crossed = true
				break
			}
		}
		if crossed {
			fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"/>`+"\n",
				num(x0), num(y0), num(tw), num(th), esc(fill))
			break
		}
	}
	r.writeLines(b, lines, cx, top+th/2, text, fontAttrs(sg.Style))
}

func pick(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// strokeExtras returns the stroke-width and dash attributes a style sets.
func strokeExtras(st *domain.Style) string {
	s := ""
	if st.StrokeWidth != "" {
		s += fmt.Sprintf(` stroke-width="%s"`, esc(st.StrokeWidth))
	}
	if st.StrokeDash != "" {
		s += fmt.Sprintf(` stroke-dasharray="%s"`, esc(st.StrokeDash))
	}
	return s
}

func fontAttrs(st *domain.Style) string {
	if st == nil {
		return ""
	}
	s := ""
	if st.FontWeight != "" {
		s += fmt.Sprintf(` font-weight="%s"`, esc(st.FontWeight))
	}
	if st.FontStyle != "" {
		s += fmt.Sprintf(` font-style="%s"`, esc(st.FontStyle))
	}
	return s
}

// writeLines writes centred lines of text around (cx, cy).
func (r *renderer) writeLines(b *strings.Builder, lines []string, cx, cy float64, fill, attrs string) {
	if len(lines) == 0 {
		return
	}
	lh := r.opts.FontSize * 1.5
	first := cy - lh*float64(len(lines)-1)/2 + r.opts.FontSize*0.35
	fmt.Fprintf(b, `    <text fill="%s" text-anchor="middle"%s>`, esc(fill), attrs)
	for i, ln := range lines {
		fmt.Fprintf(b, `<tspan x="%s" y="%s">%s</tspan>`, num(cx), num(first+lh*float64(i)), esc(ln))
	}
	b.WriteString("</text>\n")
}

// markerID returns the id of a marker of the given kind and colour,
// defining it on first use.
func (r *renderer) markerID(kind domain.Marker, color string) string {
	key := string(kind) + "|" + color
	if id, ok := r.markers[key]; ok {
		return id
	}
	id := fmt.Sprintf("%s-%s-%d", r.opts.prefix(), kind, len(r.markers))
	r.markers[key] = id
	c := esc(color)
	switch kind {
	case domain.MarkerCircle:
		fmt.Fprintf(&r.defs, `    <marker id="%s" viewBox="0 0 10 10" refX="0" refY="5" markerUnits="userSpaceOnUse" markerWidth="%s" markerHeight="%s" orient="auto-start-reverse"><circle cx="5" cy="5" r="4.5" fill="%s"/></marker>`+"\n",
			id, num(circleMarker), num(circleMarker), c)
	case domain.MarkerCross:
		fmt.Fprintf(&r.defs, `    <marker id="%s" viewBox="0 0 11 11" refX="5.5" refY="5.5" markerUnits="userSpaceOnUse" markerWidth="%s" markerHeight="%s" orient="auto-start-reverse"><path d="M1.5,1.5 L9.5,9.5 M9.5,1.5 L1.5,9.5" stroke="%s" stroke-width="2" fill="none"/></marker>`+"\n",
			id, num(crossMarker), num(crossMarker), c)
	default:
		fmt.Fprintf(&r.defs, `    <marker id="%s" viewBox="0 0 %s %s" refX="0" refY="%s" markerUnits="userSpaceOnUse" markerWidth="%s" markerHeight="%s" orient="auto-start-reverse"><path d="M0,0 L%s,%s L0,%s z" fill="%s"/></marker>`+"\n",
			id, num(arrowLen), num(arrowWide), num(arrowWide/2), num(arrowLen), num(arrowWide),
			num(arrowLen), num(arrowWide/2), num(arrowWide), c)
	}
	return id
}

// markerSetback is how far the line stops short of the end so the marker
// reaches it exactly.
func markerSetback(m domain.Marker) float64 {
	switch m {
	case domain.MarkerArrow:
		return arrowLen
	case domain.MarkerCircle:
		return circleMarker
	case domain.MarkerCross:
		return crossMarker / 2
	}
	return 0
}

func (r *renderer) drawEdge(b *strings.Builder, e *domain.Edge) {
	if len(e.Points) < 2 || e.Line == domain.LineInvisible {
		return
	}
	start, end := e.Start, e.End
	if start == "" && end == "" && e.Arrow != domain.ArrowOpen && e.Arrow != "" {
		end = domain.MarkerArrow // laid out by an older pipeline
	}
	pts := append([]domain.Point(nil), e.Points...)
	pts[0] = setBack(pts[0], pts[1], markerSetback(start))
	n := len(pts)
	pts[n-1] = setBack(pts[n-1], pts[n-2], markerSetback(end))

	stroke := r.pal.Edge
	width := edgeWidth
	dash := ""
	if e.Line == domain.LineThick || e.Arrow == domain.ArrowThick {
		width = thickWidth
	}
	if e.Line == domain.LineDotted || e.Arrow == domain.ArrowDotted {
		dash = "3 4"
	}
	widthAttr := num(width)
	if st := e.Style; st != nil {
		stroke = pick(st.Stroke, stroke)
		if st.StrokeWidth != "" {
			widthAttr = st.StrokeWidth
		}
		if st.StrokeDash != "" {
			dash = st.StrokeDash
		}
	}
	radius := cornerRadius
	if r.opts.Curved {
		radius = curvedRadius
	}
	fmt.Fprintf(b, `    <path d="%s" fill="none" stroke="%s" stroke-width="%s"`, roundedPath(pts, radius), esc(stroke), esc(widthAttr))
	if dash != "" {
		fmt.Fprintf(b, ` stroke-dasharray="%s"`, esc(dash))
	}
	if start != domain.MarkerNone {
		fmt.Fprintf(b, ` marker-start="url(#%s)"`, r.markerID(start, stroke))
	}
	if end != domain.MarkerNone {
		fmt.Fprintf(b, ` marker-end="url(#%s)"`, r.markerID(end, stroke))
	}
	b.WriteString("/>\n")
}

// setBack moves p toward q by d, keeping at least a sliver of the segment.
func setBack(p, q domain.Point, d float64) domain.Point {
	l := math.Hypot(q.X-p.X, q.Y-p.Y)
	if d <= 0 || l == 0 {
		return p
	}
	d = math.Min(d, l*0.9)
	return domain.Point{X: p.X + (q.X-p.X)/l*d, Y: p.Y + (q.Y-p.Y)/l*d}
}

// roundedPath draws the polyline with its corners rounded.
func roundedPath(pts []domain.Point, radius float64) string {
	var d strings.Builder
	fmt.Fprintf(&d, "M%s,%s", num(pts[0].X), num(pts[0].Y))
	for i := 1; i < len(pts)-1; i++ {
		a, p, c := pts[i-1], pts[i], pts[i+1]
		l1 := math.Hypot(p.X-a.X, p.Y-a.Y)
		l2 := math.Hypot(c.X-p.X, c.Y-p.Y)
		rr := math.Min(radius, math.Min(l1, l2)/2)
		if rr < 0.5 || l1 == 0 || l2 == 0 {
			fmt.Fprintf(&d, " L%s,%s", num(p.X), num(p.Y))
			continue
		}
		in := domain.Point{X: p.X - (p.X-a.X)/l1*rr, Y: p.Y - (p.Y-a.Y)/l1*rr}
		out := domain.Point{X: p.X + (c.X-p.X)/l2*rr, Y: p.Y + (c.Y-p.Y)/l2*rr}
		fmt.Fprintf(&d, " L%s,%s Q%s,%s %s,%s", num(in.X), num(in.Y), num(p.X), num(p.Y), num(out.X), num(out.Y))
	}
	last := pts[len(pts)-1]
	fmt.Fprintf(&d, " L%s,%s", num(last.X), num(last.Y))
	return d.String()
}

func (r *renderer) drawEdgeLabel(b *strings.Builder, e *domain.Edge) {
	if e.Label == "" || len(e.Points) < 2 || e.Line == domain.LineInvisible {
		return
	}
	lines := e.LabelLines
	size := e.LabelSize
	if len(lines) == 0 {
		lines = svgutil.SplitLines(e.Label)
		size = domain.Size{W: r.face.LinesWidth(lines, r.opts.FontSize) + 8, H: r.opts.FontSize * 1.5 * float64(len(lines))}
	}
	c := e.LabelPos
	if !e.LabelCenter {
		c.Y -= size.H/2 - 4
	}
	text := r.pal.Text
	if e.Style != nil {
		text = pick(e.Style.Color, text)
	}
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s" fill-opacity="0.85"/>`+"\n",
		num(c.X-size.W/2), num(c.Y-size.H/2), num(size.W), num(size.H), esc(r.pal.LabelBackground))
	r.writeLines(b, lines, c.X, c.Y, text, "")
}

func (r *renderer) drawNode(b *strings.Builder, n *domain.Node) {
	x, y, w, h := n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H
	fill, stroke, text := r.pal.NodeFill, r.pal.NodeStroke, r.pal.Text
	extra := ""
	if st := n.Style; st != nil {
		fill, stroke, text = r.pal.Node(st.Fill, st.Stroke, st.Color)
		extra = strokeExtras(st)
	}
	paint := fmt.Sprintf(` fill="%s" stroke="%s"%s`, esc(fill), esc(stroke), extra)
	cx, cy := x+w/2, y+h/2
	b.WriteString("    ")
	switch n.Shape {
	case domain.ShapeRound:
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="5"%s/>`, num(x), num(y), num(w), num(h), paint)
	case domain.ShapeStadium:
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s"%s/>`, num(x), num(y), num(w), num(h), num(h/2), paint)
	case domain.ShapeCircle:
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s"%s/>`, num(cx), num(cy), num(w/2), paint)
	case domain.ShapeDoubleCircle:
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s"%s/>`, num(cx), num(cy), num(w/2), paint)
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s"%s/>`, num(cx), num(cy), num(w/2-5), paint)
	case domain.ShapeSmallCircle:
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s"/>`, num(cx), num(cy), num(w/2), esc(text), esc(text))
	case domain.ShapeFramedCircle:
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="none" stroke="%s" stroke-width="1.5"/>`, num(cx), num(cy), num(w/2), esc(text))
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, num(cx), num(cy), num(w/2-4), esc(text))
	case domain.ShapeDiamond:
		polygon(b, paint, cx, y, x+w, cy, cx, y+h, x, cy)
	case domain.ShapeHexagon:
		k := h / 4
		polygon(b, paint, x, cy, x+k, y, x+w-k, y, x+w, cy, x+w-k, y+h, x+k, y+h)
	case domain.ShapeParallelogram:
		k := h / 3
		polygon(b, paint, x+k, y, x+w, y, x+w-k, y+h, x, y+h)
	case domain.ShapeParallelogramAlt:
		k := h / 3
		polygon(b, paint, x, y, x+w-k, y, x+w, y+h, x+k, y+h)
	case domain.ShapeTrapezoid:
		k := h / 3
		polygon(b, paint, x+k, y, x+w-k, y, x+w, y+h, x, y+h)
	case domain.ShapeTrapezoidAlt:
		k := h / 3
		polygon(b, paint, x, y, x+w, y, x+w-k, y+h, x+k, y+h)
	case domain.ShapeAsymmetric:
		k := h / 3
		polygon(b, paint, x, y, x+w, y, x+w, y+h, x, y+h, x+k, cy)
	case domain.ShapeCylinder:
		rx, ry := w/2, layout.CylinderRY(w)
		fmt.Fprintf(b, `<path d="M%s,%s A%s,%s 0 0 1 %s,%s L%s,%s A%s,%s 0 0 1 %s,%s Z"%s/>`,
			num(x), num(y+ry), num(rx), num(ry), num(x+w), num(y+ry), num(x+w), num(y+h-ry),
			num(rx), num(ry), num(x), num(y+h-ry), paint)
		fmt.Fprintf(b, `<path d="M%s,%s A%s,%s 0 0 0 %s,%s" fill="none" stroke="%s"%s/>`,
			num(x), num(y+ry), num(rx), num(ry), num(x+w), num(y+ry), esc(stroke), extra)
		cy += ry / 2
	case domain.ShapeSubroutine:
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s"%s/>`, num(x), num(y), num(w), num(h), paint)
		fmt.Fprintf(b, `<path d="M%s,%s V%s M%s,%s V%s" fill="none" stroke="%s"%s/>`,
			num(x+8), num(y), num(y+h), num(x+w-8), num(y), num(y+h), esc(stroke), extra)
	case domain.ShapeDocument:
		wave := h * 0.15 / (1 + 0.15)
		base := y + h - wave
		fmt.Fprintf(b, `<path d="M%s,%s H%s V%s C%s,%s %s,%s %s,%s S%s,%s %s,%s Z"%s/>`,
			num(x), num(y), num(x+w), num(base),
			num(x+w*0.75), num(base-wave), num(x+w*0.75), num(base+wave), num(x+w/2), num(base),
			num(x+w*0.25), num(base-wave), num(x), num(base), paint)
		cy -= wave / 2
	case domain.ShapeText:
	default:
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s"%s/>`, num(x), num(y), num(w), num(h), paint)
	}
	b.WriteByte('\n')
	lines := n.Lines
	if len(lines) == 0 && n.Shape != domain.ShapeSmallCircle && n.Shape != domain.ShapeFramedCircle {
		label := n.Label
		if label == "" {
			label = n.ID
		}
		lines = svgutil.SplitLines(label)
	}
	r.writeLines(b, lines, cx, cy, text, fontAttrs(n.Style))
}

func polygon(b *strings.Builder, paint string, xy ...float64) {
	b.WriteString(`<polygon points="`)
	for i := 0; i+1 < len(xy); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(b, "%s,%s", num(xy[i]), num(xy[i+1]))
	}
	fmt.Fprintf(b, `"%s/>`, paint)
}
