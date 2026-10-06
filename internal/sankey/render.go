package sankey

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/syntax"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls sankey appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// nodeColors is d3's Tableau10 scheme, which Mermaid colours nodes with in
// the order they first appear.
var nodeColors = []string{"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949", "#af7aa1", "#ff9da7", "#9c755f", "#bab0ab"}

const (
	chartW = 600.0
	nodeW  = 10.0
)

// Render parses and renders sankey source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return svg(d, o)
}

type node struct {
	name           string
	color          string
	in, out        []*link
	value          float64
	col            int
	x, y0, y1      float64
	inUsed, outUse float64
}

type link struct {
	src, dst *node
	value    float64
	w        float64
	sy, ty   float64 // centre of the band at each end
}

func (n *node) center() float64 { return (n.y0 + n.y1) / 2 }

// errCycle reports a flow that loops back, which a sankey cannot draw.
var errCycle = errors.New("circular flow")

func svg(d *Diagram, o RenderOptions) ([]byte, error) {
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	o.FontFace = fontFamily(o.FontFace)
	pal := theme.For(o.Theme)
	face := svgutil.FaceFor(o.FontFace)
	fs := o.FontSize
	pad := o.Padding

	nodes, links, cols, err := build(d)
	if err != nil {
		return nil, syntax.Errorf(1, 1, "%v", err)
	}
	py := fs + 8
	maxCount := 0
	for _, c := range cols {
		maxCount = max(maxCount, len(c))
	}
	chartH := math.Max(160, math.Min(400, float64(maxCount)*72))
	layoutColumns(cols, links, chartH, py)

	titleFs := math.Round(fs * 1.3)
	top := pad
	if o.Title != "" {
		top += titleFs*1.4 + 8
	}
	w := chartW + 2*pad
	if tw := face.Width(o.Title, titleFs) + 2*pad; tw > w {
		w = tw
	}
	h := top + chartH + pad
	left := (w - chartW) / 2
	ncols := len(cols)
	for _, n := range nodes {
		if ncols > 1 {
			n.x = left + float64(n.col)*(chartW-nodeW)/float64(ncols-1)
		} else {
			n.x = left
		}
		n.y0 += top
		n.y1 += top
	}
	for _, l := range links {
		l.sy += top
		l.ty += top
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+titleFs), pal.Text, svgutil.Num(titleFs), svgutil.Esc(o.Title))
	}

	// Links: a band as wide as its value, in the source's colour.
	for _, l := range links {
		if l.w <= 0 {
			continue
		}
		x0, x1 := l.src.x+nodeW, l.dst.x
		mx := (x0 + x1) / 2
		fmt.Fprintf(&b, `<path d="M%s,%s C%s,%s %s,%s %s,%s" fill="none" stroke="%s" stroke-opacity="0.5" stroke-width="%s"/>`+"\n",
			svgutil.Num(x0), svgutil.Num(l.sy), svgutil.Num(mx), svgutil.Num(l.sy), svgutil.Num(mx), svgutil.Num(l.ty),
			svgutil.Num(x1), svgutil.Num(l.ty), l.src.color, svgutil.Num(math.Max(l.w, 1)))
	}
	for _, n := range nodes {
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n",
			svgutil.Num(n.x), svgutil.Num(n.y0), svgutil.Num(nodeW), svgutil.Num(math.Max(n.y1-n.y0, 1)), n.color)
	}
	for _, n := range nodes {
		label := n.name + " " + svgutil.Num(n.value)
		x, anchor := n.x+nodeW+6, "start"
		if n.x+nodeW/2 > w/2 {
			x, anchor = n.x-6, "end"
		}
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" text-anchor="%s">%s</text>`+"\n",
			svgutil.Num(x), svgutil.Num(n.center()+fs*0.35), pal.Text, anchor, svgutil.Esc(label))
	}
	b.WriteString("</svg>\n")
	return []byte(b.String()), nil
}

// build makes the graph and its columns: each node's column is its depth
// from the sources, and nodes with no outflow go to the last column, as
// d3-sankey's justify alignment (Mermaid's default) places them.
func build(d *Diagram) ([]*node, []*link, [][]*node, error) {
	byName := map[string]*node{}
	var nodes []*node
	for i, name := range d.Nodes {
		n := &node{name: name, color: nodeColors[i%len(nodeColors)]}
		byName[name] = n
		nodes = append(nodes, n)
	}
	var links []*link
	for _, f := range d.Flows {
		l := &link{src: byName[f.Source], dst: byName[f.Target], value: f.Value}
		if l.src == l.dst {
			return nil, nil, nil, errCycle
		}
		l.src.out = append(l.src.out, l)
		l.dst.in = append(l.dst.in, l)
		links = append(links, l)
	}
	for _, n := range nodes {
		var in, out float64
		for _, l := range n.in {
			in += l.value
		}
		for _, l := range n.out {
			out += l.value
		}
		n.value = math.Max(in, out)
	}
	// Depth by Kahn's algorithm; anything left over is a cycle.
	indeg := map[*node]int{}
	for _, l := range links {
		indeg[l.dst]++
	}
	var queue []*node
	for _, n := range nodes {
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	done, maxCol := 0, 0
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		done++
		for _, l := range n.out {
			l.dst.col = max(l.dst.col, n.col+1)
			maxCol = max(maxCol, l.dst.col)
			if indeg[l.dst]--; indeg[l.dst] == 0 {
				queue = append(queue, l.dst)
			}
		}
	}
	if done != len(nodes) {
		return nil, nil, nil, errCycle
	}
	cols := make([][]*node, maxCol+1)
	for _, n := range nodes {
		if len(n.out) == 0 {
			n.col = maxCol
		}
		cols[n.col] = append(cols[n.col], n)
	}
	return nodes, links, cols, nil
}

// layoutColumns sizes and places nodes as d3-sankey (Mermaid's layout)
// does: stack each column, spread the spare height, then relax each node
// towards where its links attach on the other side, resolving overlaps,
// and finally order each node's links by the position of their other end.
func layoutColumns(cols [][]*node, links []*link, height, py float64) {
	ky := math.Inf(1)
	for _, c := range cols {
		sum := 0.0
		for _, n := range c {
			sum += n.value
		}
		if sum > 0 {
			ky = math.Min(ky, (height-float64(len(c)-1)*py)/sum)
		}
	}
	if math.IsInf(ky, 0) || ky < 0 {
		ky = 0
	}
	for _, l := range links {
		l.w = l.value * ky
	}
	for _, c := range cols {
		y := 0.0
		for _, n := range c {
			n.y0, n.y1 = y, y+n.value*ky
			y = n.y1 + py
		}
		spare := (height - y + py) / float64(len(c)+1)
		for i, n := range c {
			n.y0 += spare * float64(i+1)
			n.y1 += spare * float64(i+1)
		}
		for _, n := range c {
			reorderLinks(n)
		}
	}
	const iterations = 6
	for i := 0; i < iterations; i++ {
		alpha := math.Pow(0.99, float64(i))
		beta := math.Max(1-alpha, float64(i+1)/iterations)
		for k := len(cols) - 2; k >= 0; k-- {
			for _, src := range cols[k] {
				var y, w float64
				for _, l := range src.out {
					v := l.value * float64(l.dst.col-src.col)
					y += sourceTop(src, l.dst, py) * v
					w += v
				}
				if w > 0 {
					dy := (y/w - src.y0) * alpha
					src.y0 += dy
					src.y1 += dy
					reorderLinks(src)
				}
			}
			sortColumn(cols[k])
			resolveCollisions(cols[k], beta, height, py)
		}
		for k := 1; k < len(cols); k++ {
			for _, dst := range cols[k] {
				var y, w float64
				for _, l := range dst.in {
					v := l.value * float64(dst.col-l.src.col)
					y += targetTop(l.src, dst, py) * v
					w += v
				}
				if w > 0 {
					dy := (y/w - dst.y0) * alpha
					dst.y0 += dy
					dst.y1 += dy
					reorderLinks(dst)
				}
			}
			sortColumn(cols[k])
			resolveCollisions(cols[k], beta, height, py)
		}
	}
	for _, c := range cols {
		for _, n := range c {
			reorderLinks(n)
			y := n.y0
			for _, l := range n.out {
				l.sy = y + l.w/2
				y += l.w
			}
			y = n.y0
			for _, l := range n.in {
				l.ty = y + l.w/2
				y += l.w
			}
		}
	}
}

func sortColumn(c []*node) {
	sort.SliceStable(c, func(i, j int) bool { return c[i].y0 < c[j].y0 })
}

func reorderLinks(n *node) {
	sort.SliceStable(n.out, func(i, j int) bool { return n.out[i].dst.y0 < n.out[j].dst.y0 })
	sort.SliceStable(n.in, func(i, j int) bool { return n.in[i].src.y0 < n.in[j].src.y0 })
}

// targetTop is the y0 that dst would need for the link from src to
// arrive level with where it leaves src.
func targetTop(src, dst *node, py float64) float64 {
	y := src.y0 - float64(len(src.out)-1)*py/2
	for _, l := range src.out {
		if l.dst == dst {
			break
		}
		y += l.w + py
	}
	for _, l := range dst.in {
		if l.src == src {
			break
		}
		y -= l.w
	}
	return y
}

// sourceTop is the y0 that src would need for its link to dst to leave
// level with where it arrives.
func sourceTop(src, dst *node, py float64) float64 {
	y := dst.y0 - float64(len(dst.in)-1)*py/2
	for _, l := range dst.in {
		if l.src == src {
			break
		}
		y += l.w + py
	}
	for _, l := range src.out {
		if l.dst == dst {
			break
		}
		y -= l.w
	}
	return y
}

func resolveCollisions(c []*node, alpha, height, py float64) {
	if len(c) == 0 {
		return
	}
	i := len(c) / 2
	subject := c[i]
	bottomToTop(c, subject.y0-py, i-1, alpha, py)
	topToBottom(c, subject.y1+py, i+1, alpha, py)
	bottomToTop(c, height, len(c)-1, alpha, py)
	topToBottom(c, 0, 0, alpha, py)
}

func topToBottom(c []*node, y float64, i int, alpha, py float64) {
	for ; i < len(c); i++ {
		n := c[i]
		if dy := (y - n.y0) * alpha; dy > 1e-6 {
			n.y0 += dy
			n.y1 += dy
		}
		y = n.y1 + py
	}
}

func bottomToTop(c []*node, y float64, i int, alpha, py float64) {
	for ; i >= 0; i-- {
		n := c[i]
		if dy := (n.y1 - y) * alpha; dy > 1e-6 {
			n.y0 -= dy
			n.y1 -= dy
		}
		y = n.y0 - py
	}
}

// fontFamily keeps a font-family option only when it is a plain list of
// family names, so the option can never carry markup into the picture.
func fontFamily(s string) string {
	if strings.TrimSpace(s) == "" || len(s) > 200 {
		return "sans-serif"
	}
	for _, r := range s {
		if !(r == ' ' || r == ',' || r == '-' || r == '_' || r == '\'' || r == '"' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return "sans-serif"
		}
	}
	return s
}
