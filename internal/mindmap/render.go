package mindmap

import (
	"fmt"
	"math"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls mindmap appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// Mermaid's default mindmap colours: the root, then one colour per branch
// (child of the root), with a lighter underline for plain nodes.
const (
	rootFill = "#0000ec"
	rootText = "#ffffff"
)

var (
	branchFills = []string{"#ffff78", "#d7ff86", "#c286ff", "#ff86ff", "#ff86c2", "#ff8686", "#ffc286", "#c2ff86", "#86ffc2", "#86ffff", "#86c2ff"}
	branchLines = []string{"#ababff", "#d0b9ff", "#dcffb9", "#b9ffb9", "#b9ffdc", "#b9ffff", "#b9dcff", "#dcb9ff", "#ffb9dc", "#ffb9b9", "#ffdcb9"}
)

const (
	maxTextW = 200.0
	levelGap = 56.0
	siblingG = 14.0
)

// Render parses and renders mindmap source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return svg(d, o), nil
}

type layout struct {
	face svgutil.Face
	fs   float64
	lh   float64
}

func (l *layout) measure(n *Node, depth int) {
	n.Depth = depth
	limit := maxTextW
	if n.Shape == ShapeCircle {
		limit = maxTextW * 0.6 // a circle grows in both directions
	}
	n.lines = wrap(l.face, n.Text, l.fs, limit)
	tw := 0.0
	for _, ln := range n.lines {
		tw = math.Max(tw, l.face.Width(ln, l.fs))
	}
	th := float64(len(n.lines)) * l.lh
	switch n.Shape {
	case ShapeCircle:
		n.W = math.Hypot(tw, th) + 16
		n.H = n.W
	case ShapeHexagon:
		n.H = th + 20
		n.W = tw + 28 + n.H/2
	case ShapeBang:
		n.W, n.H = tw+48, th+44
	case ShapeCloud:
		n.W, n.H = tw+44, th+36
	default:
		n.W, n.H = tw+28, th+20
	}
	for _, c := range n.Children {
		l.measure(c, depth+1)
	}
}

// span is the height a subtree needs, worked out once by measure.
func span(n *Node) float64 { return n.span }

// setSpan stores every subtree's height, bottom up, so layout stays linear
// in the number of nodes however deep the map is.
func setSpan(n *Node) {
	for _, c := range n.Children {
		setSpan(c)
	}
	n.span = n.H
	if len(n.Children) > 0 {
		n.span = math.Max(n.H, childrenSpan(n.Children))
	}
}

func childrenSpan(cs []*Node) float64 {
	t := 0.0
	for i, c := range cs {
		if i > 0 {
			t += siblingG
		}
		t += span(c)
	}
	return t
}

// place lays a subtree out on one side of the root: inner is the x of the
// node's edge nearest the root, top the top of its band.
func place(n *Node, inner, top, side float64, section int) {
	n.side, n.section = side, section
	n.X = inner + side*n.W/2
	n.Y = top + span(n)/2
	y := n.Y - childrenSpan(n.Children)/2
	for _, c := range n.Children {
		place(c, n.X+side*(n.W/2+levelGap), y, side, section)
		y += span(c) + siblingG
	}
}

func svg(d *Diagram, o RenderOptions) []byte {
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	pal := theme.For(o.Theme)
	l := &layout{face: svgutil.FaceFor(o.FontFace), fs: o.FontSize, lh: o.FontSize * 1.3}
	pad := o.Padding
	root := d.Root
	l.measure(root, 0)
	setSpan(root)
	root.section = -1

	// Split the branches between the two sides, about half the height on
	// each, first ones on the right as Mermaid's layout tends to.
	total := childrenSpan(root.Children)
	var right, left []*Node
	acc := 0.0
	for i, c := range root.Children {
		if i == 0 || acc+span(c)/2 <= total/2 {
			right = append(right, c)
			acc += span(c) + siblingG
		} else {
			left = append(left, c)
		}
	}
	sections := map[*Node]int{}
	for i, c := range root.Children {
		sections[c] = i
	}
	for _, side := range []struct {
		nodes []*Node
		dir   float64
	}{{right, 1}, {left, -1}} {
		y := -childrenSpan(side.nodes) / 2
		for _, c := range side.nodes {
			place(c, side.dir*(root.W/2+levelGap), y, side.dir, sections[c])
			y += span(c) + siblingG
		}
	}

	// Bounds.
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	var walk func(n *Node, f func(*Node))
	walk = func(n *Node, f func(*Node)) {
		f(n)
		for _, c := range n.Children {
			walk(c, f)
		}
	}
	walk(root, func(n *Node) {
		bump := 0.0
		if n.Shape == ShapeBang || n.Shape == ShapeCloud {
			bump = 8
		}
		minX = math.Min(minX, n.X-n.W/2-bump)
		maxX = math.Max(maxX, n.X+n.W/2+bump)
		minY = math.Min(minY, n.Y-n.H/2-bump)
		maxY = math.Max(maxY, n.Y+n.H/2+bump+2)
	})
	titleFs := math.Round(o.FontSize * 1.3)
	titleH := 0.0
	if o.Title != "" {
		titleH = titleFs*1.4 + 8
		tw := l.face.Width(o.Title, titleFs)
		if tw > maxX-minX {
			grow := (tw - (maxX - minX)) / 2
			minX -= grow
			maxX += grow
		}
	}
	dx, dy := pad-minX, pad+titleH-minY
	w := maxX - minX + 2*pad
	h := maxY - minY + 2*pad + titleH

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(o.FontSize))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+titleFs), pal.Text, svgutil.Num(titleFs), svgutil.Esc(o.Title))
	}
	walk(root, func(n *Node) { n.X += dx; n.Y += dy })

	// Edges first, so nodes sit on top of them.
	walk(root, func(n *Node) {
		for _, c := range n.Children {
			x1 := n.X + c.side*n.W/2*0.9
			if n == root {
				x1 = n.X
			}
			x2 := c.X - c.side*c.W/2
			mx := (x1 + x2) / 2
			fmt.Fprintf(&b, `<path d="M%s,%s C%s,%s %s,%s %s,%s" fill="none" stroke="%s" stroke-width="%s" stroke-linecap="round"/>`+"\n",
				svgutil.Num(x1), svgutil.Num(n.Y), svgutil.Num(mx), svgutil.Num(n.Y), svgutil.Num(mx), svgutil.Num(c.Y),
				svgutil.Num(x2), svgutil.Num(c.Y), branchFills[c.section%len(branchFills)],
				svgutil.Num(math.Max(2, 14-3*float64(c.Depth))))
		}
	})
	walk(root, func(n *Node) { writeNode(&b, n, l) })
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

func writeNode(b *strings.Builder, n *Node, l *layout) {
	fill, text, line := rootFill, rootText, ""
	if n.section >= 0 {
		k := n.section % len(branchFills)
		fill, text, line = branchFills[k], "#000000", branchLines[k]
	}
	x0, y0, x1, y1 := n.X-n.W/2, n.Y-n.H/2, n.X+n.W/2, n.Y+n.H/2
	switch n.Shape {
	case ShapeCircle:
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", svgutil.Num(n.X), svgutil.Num(n.Y), svgutil.Num(n.W/2), fill)
	case ShapeSquare, ShapeRounded:
		rx := 0.0
		if n.Shape == ShapeRounded {
			rx = 8
		}
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"/>`+"\n",
			svgutil.Num(x0), svgutil.Num(y0), svgutil.Num(n.W), svgutil.Num(n.H), svgutil.Num(rx), fill)
	case ShapeHexagon:
		m := n.H / 4
		fmt.Fprintf(b, `<polygon points="%s,%s %s,%s %s,%s %s,%s %s,%s %s,%s" fill="%s"/>`+"\n",
			svgutil.Num(x0), svgutil.Num(n.Y), svgutil.Num(x0+m), svgutil.Num(y0), svgutil.Num(x1-m), svgutil.Num(y0),
			svgutil.Num(x1), svgutil.Num(n.Y), svgutil.Num(x1-m), svgutil.Num(y1), svgutil.Num(x0+m), svgutil.Num(y1), fill)
	case ShapeBang:
		fmt.Fprintf(b, `<path d="%s" fill="%s"/>`+"\n", scallop(n, 16, 0.55), fill)
	case ShapeCloud:
		fmt.Fprintf(b, `<path d="%s" fill="%s"/>`+"\n", scallop(n, 9, 0.62), fill)
	default:
		if n.section < 0 {
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="5" fill="%s"/>`+"\n",
				svgutil.Num(x0), svgutil.Num(y0), svgutil.Num(n.W), svgutil.Num(n.H), fill)
			break
		}
		// Mermaid's plain node: rounded top corners and an underline.
		fmt.Fprintf(b, `<path d="M%s,%s V%s Q%s,%s %s,%s H%s Q%s,%s %s,%s V%s Z" fill="%s"/>`+"\n",
			svgutil.Num(x0), svgutil.Num(y1), svgutil.Num(y0+5), svgutil.Num(x0), svgutil.Num(y0), svgutil.Num(x0+5), svgutil.Num(y0),
			svgutil.Num(x1-5), svgutil.Num(x1), svgutil.Num(y0), svgutil.Num(x1), svgutil.Num(y0+5), svgutil.Num(y1), fill)
		fmt.Fprintf(b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="3"/>`+"\n",
			svgutil.Num(x0), svgutil.Num(y1), svgutil.Num(x1), svgutil.Num(y1), line)
	}
	svgutil.MultilineText(b, n.lines, n.X, n.Y+l.fs*0.35, l.lh, text, "")
	b.WriteByte('\n')
}

// scallop outlines an ellipse with k outward bumps: many small ones for a
// bang, a few large ones for a cloud.
func scallop(n *Node, k int, r float64) string {
	var p strings.Builder
	rx, ry := n.W/2, n.H/2
	pt := func(i int) (float64, float64) {
		a := 2*math.Pi*float64(i)/float64(k) - math.Pi/2
		return n.X + rx*math.Cos(a), n.Y + ry*math.Sin(a)
	}
	x, y := pt(0)
	fmt.Fprintf(&p, "M%s,%s", svgutil.Num(x), svgutil.Num(y))
	for i := 1; i <= k; i++ {
		nx, ny := pt(i)
		rad := math.Hypot(nx-x, ny-y) * r
		fmt.Fprintf(&p, " A%s,%s 0 0 1 %s,%s", svgutil.Num(rad), svgutil.Num(rad), svgutil.Num(nx), svgutil.Num(ny))
		x, y = nx, ny
	}
	p.WriteString(" Z")
	return p.String()
}
