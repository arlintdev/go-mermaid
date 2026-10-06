package block

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgid"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls block diagram appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

const (
	padX      = 16.0  // text to block edge, across
	padY      = 11.0  // text to block edge, down
	minH      = 40.0  // shortest block
	maxLabelW = 200.0 // labels wrap near this width, as Mermaid's do
	plainGap  = 10.0  // between blocks when no edge needs the room
	edgeGap   = 36.0  // between blocks when edges run between them
	compPad   = 10.0  // inside a composite block
)

// Render parses and renders block-beta source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return svg(d, o, svgid.Prefix(src)), nil
}

type rect struct{ x, y, w, h float64 }

func (r rect) cx() float64 { return r.x + r.w/2 }
func (r rect) cy() float64 { return r.y + r.h/2 }

type layout struct {
	face  svgutil.Face
	fs    float64
	lh    float64
	gap   float64
	nat   map[*Block][2]float64
	cells map[*Block]rect // where each block was placed
	lines map[*Block][]string
	order []*Block // blocks in drawing order, composites before their children
}

type slot struct {
	b              *Block
	row, col, span int
}

func (l *layout) grid(b *Block) (slots []slot, cols, rows int) {
	cols = b.Columns
	if cols <= 0 {
		for _, c := range b.Children {
			cols += c.Span
		}
		cols = max(cols, 1)
	}
	row, col := 0, 0
	for _, c := range b.Children {
		span := min(max(c.Span, 1), cols)
		if col+span > cols {
			row, col = row+1, 0
		}
		slots = append(slots, slot{c, row, col, span})
		col += span
	}
	return slots, cols, row + 1
}

// natural returns the size a block needs for its label (leaf) or its grid
// (composite).
func (l *layout) natural(b *Block) (float64, float64) {
	if s, ok := l.nat[b]; ok {
		return s[0], s[1]
	}
	var w, h float64
	switch {
	case b.Space:
		w, h = 30, 0
	case b.Composite:
		slots, cols, rows := l.grid(b)
		unit := 30.0
		rowH := make([]float64, rows)
		for _, s := range slots {
			cw, ch := l.natural(s.b)
			unit = max(unit, (cw-float64(s.span-1)*l.gap)/float64(s.span))
			rowH[s.row] = max(rowH[s.row], ch)
		}
		w = float64(cols)*unit + float64(cols-1)*l.gap
		for _, r := range rowH {
			h += max(r, 20)
		}
		h += float64(rows-1) * l.gap
		w += 2 * compPad
		h += 2 * compPad
	default:
		lines := wrap(l.face, b.Label, l.fs, maxLabelW)
		l.lines[b] = lines
		tw := 0.0
		for _, ln := range lines {
			tw = max(tw, l.face.Width(ln, l.fs))
		}
		th := float64(len(lines)) * l.lh
		w, h = tw+2*padX, max(th+2*padY, minH)
		switch b.Shape {
		case ShapeCircle, ShapeDoubleCircle:
			w = max(tw, th) + 24
			if b.Shape == ShapeDoubleCircle {
				w += 10
			}
			h = w
		case ShapeRhombus:
			w = tw + 1.9*th + 34
			h = max(w*0.55, th+30)
		case ShapeHexagon, ShapeStadium, ShapeAsymmetric:
			w += h / 2
		case ShapeParallelogram, ShapeParallelogramAlt, ShapeTrapezoid, ShapeTrapezoidAlt:
			w += h * 0.6
		case ShapeSubroutine:
			w += 16
		case ShapeCylinder:
			h += 16
		case ShapeArrow:
			w += h
			h += 16
		}
	}
	l.nat[b] = [2]float64{w, h}
	return w, h
}

// place puts b in the cell r and, for a composite, lays its children out
// in r, stretching the columns and rows to fill it.
func (l *layout) place(b *Block, r rect, root bool) {
	if b.Space {
		return
	}
	l.cells[b] = r
	l.order = append(l.order, b)
	if !b.Composite {
		return
	}
	slots, cols, rows := l.grid(b)
	inner := r
	if !root {
		inner = rect{r.x + compPad, r.y + compPad, r.w - 2*compPad, r.h - 2*compPad}
	}
	unit := (inner.w - float64(cols-1)*l.gap) / float64(cols)
	rowH := make([]float64, rows)
	for _, s := range slots {
		_, ch := l.natural(s.b)
		rowH[s.row] = max(rowH[s.row], ch, 20)
	}
	total := float64(rows-1) * l.gap
	for _, h := range rowH {
		total += h
	}
	extra := (inner.h - total) / float64(rows)
	rowY := make([]float64, rows)
	y := inner.y
	for i := range rowH {
		rowH[i] += max(extra, 0)
		rowY[i] = y
		y += rowH[i] + l.gap
	}
	for _, s := range slots {
		x := inner.x + float64(s.col)*(unit+l.gap)
		w := float64(s.span)*unit + float64(s.span-1)*l.gap
		l.place(s.b, rect{x, rowY[s.row], w, rowH[s.row]}, false)
	}
}

// shapeRect is the outline a leaf block is drawn in: the whole cell, or
// for round and diamond shapes their own size centred in it.
func (l *layout) shapeRect(b *Block) rect {
	r := l.cells[b]
	switch b.Shape {
	case ShapeCircle, ShapeDoubleCircle:
		nw, _ := l.natural(b)
		s := min(nw, r.w, r.h)
		return rect{r.cx() - s/2, r.cy() - s/2, s, s}
	case ShapeRhombus:
		nw, nh := l.natural(b)
		w, h := min(nw, r.w), min(nh, r.h)
		return rect{r.cx() - w/2, r.cy() - h/2, w, h}
	}
	return r
}

func svg(d *Diagram, o RenderOptions, id string) []byte {
	pal := theme.For(o.Theme)
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	l := &layout{face: svgutil.FaceFor(o.FontFace), fs: fs, lh: fs * 1.25, gap: plainGap,
		nat: map[*Block][2]float64{}, cells: map[*Block]rect{}, lines: map[*Block][]string{}}
	for _, e := range d.Edges {
		l.gap = max(l.gap, edgeGap)
		if e.Label != "" {
			// Room for the label between the blocks it joins, within reason.
			w := 0.0
			for _, ln := range wrap(l.face, e.Label, fs*0.9, maxLabelW) {
				w = max(w, l.face.Width(ln, fs*0.9))
			}
			l.gap = max(l.gap, min(w+28, 110))
		}
	}
	pad := o.Padding
	titleH := 0.0
	if o.Title != "" {
		titleH = fs*1.4 + 12
	}
	nw, nh := l.natural(d.Root)
	nw -= 2 * compPad
	nh -= 2 * compPad
	l.place(d.Root, rect{pad, pad + titleH, nw, nh}, true)

	// Edges first, so labels and arrows can be measured for the canvas.
	type drawn struct {
		e   Edge
		pts [][2]float64
	}
	var edges []drawn
	bounds := rect{0, 0, pad + nw + pad, pad + titleH + nh + pad}
	for _, e := range d.Edges {
		from, to := d.byID[e.From], d.byID[e.To]
		if from == nil || to == nil || from == to || e.Invisible {
			continue
		}
		if _, ok := l.cells[from]; !ok {
			continue
		}
		if _, ok := l.cells[to]; !ok {
			continue
		}
		edges = append(edges, drawn{e, l.route(from, to)})
	}

	// A route through the outer gap may reach past the blocks; grow the
	// canvas (and shift the drawing) so it stays inside.
	dx, dy := 0.0, 0.0
	for _, de := range edges {
		for _, pt := range de.pts {
			dx = max(dx, 6-pt[0])
			dy = max(dy, titleH+6-pt[1])
			bounds.w = max(bounds.w, pt[0]+6)
			bounds.h = max(bounds.h, pt[1]+6)
		}
	}
	w, h := bounds.w+dx, bounds.h+dy
	if o.Title != "" {
		w = max(w, l.face.Width(o.Title, fs*1.15)+2*pad)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	edgeColor := svgutil.Esc(pal.Edge)
	if len(edges) > 0 {
		fmt.Fprintf(&b, `  <defs><marker id="%s-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="%s"/></marker></defs>`+"\n", id, edgeColor)
	}
	if o.Title != "" {
		fmt.Fprintf(&b, `  <text x="%s" y="%s" fill="%s" font-size="%s" font-weight="bold" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+fs*1.15), svgutil.Esc(pal.Text), svgutil.Num(fs*1.15), svgutil.Esc(o.Title))
	}

	if dx > 0 || dy > 0 {
		fmt.Fprintf(&b, `  <g transform="translate(%s %s)">`+"\n", svgutil.Num(dx), svgutil.Num(dy))
	}
	for _, blk := range l.order {
		if blk == d.Root {
			continue
		}
		l.drawBlock(&b, d, blk, pal)
	}

	for _, de := range edges {
		e := de.e
		var p strings.Builder
		for i, pt := range de.pts {
			if i == 0 {
				p.WriteString("M")
			} else {
				p.WriteString(" L")
			}
			p.WriteString(svgutil.Num(pt[0]) + "," + svgutil.Num(pt[1]))
		}
		width := "1.5"
		if e.Thick {
			width = "3"
		}
		fmt.Fprintf(&b, `  <path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="round"`, p.String(), edgeColor, width)
		if e.Dotted {
			b.WriteString(` stroke-dasharray="3 3"`)
		}
		if e.ArrowEnd {
			fmt.Fprintf(&b, ` marker-end="url(#%s-arrow)"`, id)
		}
		if e.ArrowBack {
			fmt.Fprintf(&b, ` marker-start="url(#%s-arrow)"`, id)
		}
		b.WriteString("/>\n")
	}
	// Labels last so no line crosses them.
	for _, de := range edges {
		if de.e.Label == "" {
			continue
		}
		mx, my := midpoint(de.pts)
		lines := wrap(l.face, de.e.Label, fs*0.9, maxLabelW)
		tw := 0.0
		for _, ln := range lines {
			tw = max(tw, l.face.Width(ln, fs*0.9))
		}
		lh := fs * 0.9 * 1.25
		th := float64(len(lines)) * lh
		fmt.Fprintf(&b, `  <rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="#e8e8e8" fill-opacity="0.9"/>`+"\n",
			svgutil.Num(mx-tw/2-4), svgutil.Num(my-th/2-2), svgutil.Num(tw+8), svgutil.Num(th+4))
		writeLines(&b, lines, mx, my, lh, fs*0.9, svgutil.Esc(pal.Text), "")
	}
	if dx > 0 || dy > 0 {
		b.WriteString("  </g>\n")
	}

	b.WriteString("</svg>\n")
	return []byte(b.String())
}

func (l *layout) drawBlock(b *strings.Builder, d *Diagram, blk *Block, pal theme.Palette) {
	st := blk.Style
	for i := len(blk.Classes) - 1; i >= 0; i-- {
		st = st.over(d.Classes[blk.Classes[i]])
	}
	fill, stroke, text := svgutil.Esc(pal.NodeFill), svgutil.Esc(pal.NodeStroke), svgutil.Esc(pal.Text)
	if blk.Composite {
		fill = mix(pal.NodeFill, pal.Background, 0.55, fill)
	}
	if st.Fill != "" {
		fill = svgutil.Esc(st.Fill)
	}
	if st.Stroke != "" {
		stroke = svgutil.Esc(st.Stroke)
	}
	if st.Color != "" {
		text = svgutil.Esc(st.Color)
	}
	attrs := fmt.Sprintf(`fill="%s" stroke="%s"`, fill, stroke)
	if st.StrokeWidth != "" {
		attrs += ` stroke-width="` + svgutil.Esc(st.StrokeWidth) + `"`
	}
	if st.Dash != "" {
		attrs += ` stroke-dasharray="` + svgutil.Esc(st.Dash) + `"`
	}
	r := l.shapeRect(blk)
	n := svgutil.Num
	x, y, w, h := r.x, r.y, r.w, r.h
	switch {
	case blk.Composite:
		fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" rx="4" %s/>`+"\n", n(x), n(y), n(w), n(h), attrs)
		return
	}
	poly := func(pts ...float64) {
		var s []string
		for i := 0; i+1 < len(pts); i += 2 {
			s = append(s, n(pts[i])+","+n(pts[i+1]))
		}
		fmt.Fprintf(b, `  <polygon points="%s" %s/>`+"\n", strings.Join(s, " "), attrs)
	}
	switch blk.Shape {
	case ShapeRound:
		fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" rx="8" %s/>`+"\n", n(x), n(y), n(w), n(h), attrs)
	case ShapeStadium:
		fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" rx="%s" %s/>`+"\n", n(x), n(y), n(w), n(h), n(h/2), attrs)
	case ShapeSubroutine:
		fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" %s/>`+"\n", n(x), n(y), n(w), n(h), attrs)
		fmt.Fprintf(b, `  <path d="M%s,%s V%s M%s,%s V%s" fill="none" stroke="%s"/>`+"\n", n(x+8), n(y), n(y+h), n(x+w-8), n(y), n(y+h), stroke)
	case ShapeCylinder:
		ry := min(8.0, h/6)
		rx := w / 2
		fmt.Fprintf(b, `  <path d="M%s,%s A%s,%s 0 0 1 %s,%s V%s A%s,%s 0 0 1 %s,%s Z" %s/>`+"\n",
			n(x), n(y+ry), n(rx), n(ry), n(x+w), n(y+ry), n(y+h-ry), n(rx), n(ry), n(x), n(y+h-ry), attrs)
		fmt.Fprintf(b, `  <path d="M%s,%s A%s,%s 0 0 0 %s,%s" fill="none" stroke="%s"/>`+"\n", n(x), n(y+ry), n(rx), n(ry), n(x+w), n(y+ry), stroke)
		y += ry
		h -= ry
	case ShapeCircle:
		fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="%s" %s/>`+"\n", n(r.cx()), n(r.cy()), n(w/2), attrs)
	case ShapeDoubleCircle:
		fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="%s" %s/>`+"\n", n(r.cx()), n(r.cy()), n(w/2), attrs)
		fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="%s" fill="none" stroke="%s"/>`+"\n", n(r.cx()), n(r.cy()), n(w/2-5), stroke)
	case ShapeRhombus:
		poly(x+w/2, y, x+w, y+h/2, x+w/2, y+h, x, y+h/2)
	case ShapeHexagon:
		k := h / 4
		poly(x+k, y, x+w-k, y, x+w, y+h/2, x+w-k, y+h, x+k, y+h, x, y+h/2)
	case ShapeAsymmetric:
		k := h / 3
		poly(x, y, x+w, y, x+w, y+h, x, y+h, x+k, y+h/2)
	case ShapeParallelogram:
		k := h * 0.3
		poly(x+k, y, x+w, y, x+w-k, y+h, x, y+h)
	case ShapeParallelogramAlt:
		k := h * 0.3
		poly(x, y, x+w-k, y, x+w, y+h, x+k, y+h)
	case ShapeTrapezoid:
		k := h * 0.3
		poly(x+k, y, x+w-k, y, x+w, y+h, x, y+h)
	case ShapeTrapezoidAlt:
		k := h * 0.3
		poly(x, y, x+w, y, x+w-k, y+h, x+k, y+h)
	case ShapeArrow:
		l.drawArrow(b, blk.ArrowDir, r, attrs)
	default:
		fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" rx="3" %s/>`+"\n", n(x), n(y), n(w), n(h), attrs)
	}
	extra := ""
	if st.FontWeight != "" {
		extra = ` font-weight="` + svgutil.Esc(st.FontWeight) + `"`
	}
	writeLines(b, l.lines[blk], x+w/2, y+h/2, l.lh, l.fs, text, extra)
}

// drawArrow draws a block arrow pointing dir inside r.
func (l *layout) drawArrow(b *strings.Builder, dir string, r rect, attrs string) {
	x, y, w, h := r.x, r.y, r.w, r.h
	var pts []float64
	switch dir {
	case "left", "right", "x":
		k := min(h/2, w/3)
		t := h * 0.22
		switch dir {
		case "right":
			pts = []float64{x, y + t, x + w - k, y + t, x + w - k, y, x + w, y + h/2, x + w - k, y + h, x + w - k, y + h - t, x, y + h - t}
		case "left":
			pts = []float64{x + w, y + t, x + k, y + t, x + k, y, x, y + h/2, x + k, y + h, x + k, y + h - t, x + w, y + h - t}
		default:
			pts = []float64{x, y + h/2, x + k, y, x + k, y + t, x + w - k, y + t, x + w - k, y, x + w, y + h/2, x + w - k, y + h, x + w - k, y + h - t, x + k, y + h - t, x + k, y + h}
		}
	default:
		cx := x + w/2
		k := min(w/2, h/3)
		hw := min(w/2, max(w*0.3, 30))
		t := hw * 0.6
		switch dir {
		case "up":
			pts = []float64{cx - t, y + h, cx - t, y + k, cx - hw, y + k, cx, y, cx + hw, y + k, cx + t, y + k, cx + t, y + h}
		case "down":
			pts = []float64{cx - t, y, cx - t, y + h - k, cx - hw, y + h - k, cx, y + h, cx + hw, y + h - k, cx + t, y + h - k, cx + t, y}
		default:
			pts = []float64{cx, y, cx + hw, y + k, cx + t, y + k, cx + t, y + h - k, cx + hw, y + h - k, cx, y + h, cx - hw, y + h - k, cx - t, y + h - k, cx - t, y + k, cx - hw, y + k}
		}
	}
	var s []string
	for i := 0; i+1 < len(pts); i += 2 {
		s = append(s, svgutil.Num(pts[i])+","+svgutil.Num(pts[i+1]))
	}
	fmt.Fprintf(b, `  <polygon points="%s" %s/>`+"\n", strings.Join(s, " "), attrs)
}

// route returns the path of an edge from a to b: a straight line when it
// crosses no other block, else the shortest of a few right-angled routes
// through the gaps between blocks that crosses none.
func (l *layout) route(a, b *Block) [][2]float64 {
	ra, rb := l.shapeRect(a), l.shapeRect(b)
	ac := [2]float64{ra.cx(), ra.cy()}
	bc := [2]float64{rb.cx(), rb.cy()}
	var obstacles []rect
	for _, o := range l.order {
		if o == a || o == b || o.Space {
			continue
		}
		r := l.shapeRect(o)
		if o.Composite && (contains(r, ra) || contains(r, rb)) {
			continue
		}
		if contains(ra, r) || contains(rb, r) {
			continue
		}
		obstacles = append(obstacles, r)
	}
	finish := func(pts [][2]float64) [][2]float64 {
		out := append([][2]float64{}, pts...)
		out[0] = clipTo(a, ra, pts[0], pts[1])
		out[len(out)-1] = clipTo(b, rb, pts[len(pts)-1], pts[len(pts)-2])
		return out
	}
	clear := func(pts [][2]float64) bool {
		for i := 0; i+1 < len(pts); i++ {
			for _, o := range obstacles {
				if segHits(pts[i], pts[i+1], o) {
					return false
				}
			}
		}
		// The middle of the route must not run back through either end.
		for i := 1; i+2 < len(pts); i++ {
			if segHits(pts[i], pts[i+1], ra) || segHits(pts[i], pts[i+1], rb) {
				return false
			}
		}
		return true
	}
	straight := finish([][2]float64{ac, bc})
	if clear(straight) {
		return straight
	}
	g := l.gap / 2
	above := min(ra.y, rb.y) - g
	below := max(ra.y+ra.h, rb.y+rb.h) + g
	leftX := min(ra.x, rb.x) - g
	rightX := max(ra.x+ra.w, rb.x+rb.w) + g
	midX := (ac[0] + bc[0]) / 2
	if ra.x+ra.w < rb.x {
		midX = (ra.x + ra.w + rb.x) / 2
	} else if rb.x+rb.w < ra.x {
		midX = (rb.x + rb.w + ra.x) / 2
	}
	midY := (ac[1] + bc[1]) / 2
	if ra.y+ra.h < rb.y {
		midY = (ra.y + ra.h + rb.y) / 2
	} else if rb.y+rb.h < ra.y {
		midY = (rb.y + rb.h + ra.y) / 2
	}
	cands := [][][2]float64{
		{ac, {midX, ac[1]}, {midX, bc[1]}, bc},
		{ac, {ac[0], midY}, {bc[0], midY}, bc},
		{ac, {bc[0], ac[1]}, bc},
		{ac, {ac[0], bc[1]}, bc},
		{ac, {ac[0], below}, {bc[0], below}, bc},
		{ac, {ac[0], above}, {bc[0], above}, bc},
		{ac, {rightX, ac[1]}, {rightX, bc[1]}, bc},
		{ac, {leftX, ac[1]}, {leftX, bc[1]}, bc},
	}
	var best [][2]float64
	bestLen := math.Inf(1)
	for _, c := range cands {
		c = dedupe(c)
		if len(c) < 2 {
			continue
		}
		p := finish(c)
		if !clear(p) {
			continue
		}
		if L := length(p) + float64(len(p))*4; L < bestLen {
			best, bestLen = p, L
		}
	}
	if best != nil {
		return best
	}
	return straight
}

func dedupe(pts [][2]float64) [][2]float64 {
	var out [][2]float64
	for _, p := range pts {
		if len(out) > 0 && math.Abs(out[len(out)-1][0]-p[0]) < 0.5 && math.Abs(out[len(out)-1][1]-p[1]) < 0.5 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func length(pts [][2]float64) float64 {
	t := 0.0
	for i := 0; i+1 < len(pts); i++ {
		t += math.Hypot(pts[i+1][0]-pts[i][0], pts[i+1][1]-pts[i][1])
	}
	return t
}

// midpoint is the point halfway along a polyline.
func midpoint(pts [][2]float64) (float64, float64) {
	half := length(pts) / 2
	for i := 0; i+1 < len(pts); i++ {
		s := math.Hypot(pts[i+1][0]-pts[i][0], pts[i+1][1]-pts[i][1])
		if s >= half && s > 0 {
			t := half / s
			return pts[i][0] + (pts[i+1][0]-pts[i][0])*t, pts[i][1] + (pts[i+1][1]-pts[i][1])*t
		}
		half -= s
	}
	return pts[0][0], pts[0][1]
}

func contains(outer, inner rect) bool {
	return inner.x >= outer.x-0.5 && inner.y >= outer.y-0.5 && inner.x+inner.w <= outer.x+outer.w+0.5 && inner.y+inner.h <= outer.y+outer.h+0.5
}

// segHits reports whether segment p-q passes through the inside of r.
func segHits(p, q [2]float64, r rect) bool {
	const in = 2.0
	x0, y0, x1, y1 := r.x+in, r.y+in, r.x+r.w-in, r.y+r.h-in
	if x1 <= x0 || y1 <= y0 {
		return false
	}
	t0, t1 := 0.0, 1.0
	dx, dy := q[0]-p[0], q[1]-p[1]
	for _, c := range [4][2]float64{{-dx, p[0] - x0}, {dx, x1 - p[0]}, {-dy, p[1] - y0}, {dy, y1 - p[1]}} {
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

// clipTo moves the end point at the centre of block blk's outline r out to
// the outline, along the segment toward the next point.
func clipTo(blk *Block, r rect, at, toward [2]float64) [2]float64 {
	dx, dy := toward[0]-at[0], toward[1]-at[1]
	dist := math.Hypot(dx, dy)
	if dist == 0 {
		return at
	}
	ux, uy := dx/dist, dy/dist
	hw, hh := r.w/2, r.h/2
	// Distance from the centre to the outline along (ux, uy); at is not
	// always the centre, so measure from where it lies.
	ox, oy := at[0]-r.cx(), at[1]-r.cy()
	var t float64
	switch blk.Shape {
	case ShapeCircle, ShapeDoubleCircle:
		// Solve |o + t u| = radius.
		bq := ox*ux + oy*uy
		cq := ox*ox + oy*oy - hw*hw
		t = -bq + math.Sqrt(max(bq*bq-cq, 0))
	case ShapeRhombus:
		t = 1 / (math.Abs(ux)/hw + math.Abs(uy)/hh)
		if ox != 0 || oy != 0 {
			t = rectExit(ox, oy, ux, uy, hw, hh)
		}
	default:
		t = rectExit(ox, oy, ux, uy, hw, hh)
	}
	t = min(t, dist)
	return [2]float64{at[0] + ux*t, at[1] + uy*t}
}

// rectExit is how far from (ox, oy), inside a box of half-size hw x hh
// centred on the origin, the ray along (ux, uy) leaves the box.
func rectExit(ox, oy, ux, uy, hw, hh float64) float64 {
	t := math.Inf(1)
	if ux > 0 {
		t = min(t, (hw-ox)/ux)
	} else if ux < 0 {
		t = min(t, (-hw-ox)/ux)
	}
	if uy > 0 {
		t = min(t, (hh-oy)/uy)
	} else if uy < 0 {
		t = min(t, (-hh-oy)/uy)
	}
	if math.IsInf(t, 1) {
		return 0
	}
	return max(t, 0)
}

// writeLines writes lines centred on (cx, cy).
func writeLines(b *strings.Builder, lines []string, cx, cy, lh, fs float64, fill, extra string) {
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return
	}
	y0 := cy - float64(len(lines)-1)*lh/2 + fs*0.35
	fmt.Fprintf(b, `  <text fill="%s" font-size="%s" text-anchor="middle"%s>`, fill, svgutil.Num(fs), extra)
	for i, ln := range lines {
		fmt.Fprintf(b, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(cx), svgutil.Num(y0+float64(i)*lh), svgutil.Esc(ln))
	}
	b.WriteString("</text>\n")
}

// wrap breaks s into lines no wider than maxW, at spaces where it can and
// inside a word only when the word alone is too wide.
func wrap(face svgutil.Face, s string, size, maxW float64) []string {
	var lines []string
	for _, para := range svgutil.SplitLines(s) {
		cur := ""
		for _, wd := range strings.Fields(para) {
			for face.Width(wd, size) > maxW && len([]rune(wd)) > 1 {
				if cur != "" {
					lines = append(lines, cur)
					cur = ""
				}
				// The longest prefix that fits, measured rune by rune so a
				// very long word costs linear time.
				r := []rune(wd)
				k, w := 0, 0.0
				for k < len(r) {
					cw := face.Width(string(r[k]), size)
					if k > 0 && w+cw > maxW {
						break
					}
					w += cw
					k++
				}
				lines = append(lines, string(r[:k]))
				wd = string(r[k:])
			}
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
// is not a #rgb or #rrggbb colour it returns fallback.
func mix(a, b string, t float64, fallback string) string {
	ra, ga, ba, ok1 := hexRGB(a)
	rb, gb, bb, ok2 := hexRGB(b)
	if !ok1 || !ok2 {
		return fallback
	}
	c := func(x, y int) int { return x + int(float64(y-x)*t+0.5) }
	return fmt.Sprintf("#%02x%02x%02x", c(ra, rb), c(ga, gb), c(ba, bb))
}

func hexRGB(s string) (r, g, b int, ok bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v >> 16), int(v >> 8 & 0xff), int(v & 0xff), true
}
