package curve

import (
	"fmt"
	"math"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// cornerRadius rounds the bends of a routed line, as flowchart edges are.
const cornerRadius = 5.0

// Routed builds the drawn line for a route that layout.Flow found: the
// orthogonal polyline as it is, corners rounded, never replaced by a curve
// (a curve would leave the label room the layout kept on the route). mid
// is where the layout put the line's label. trimStart and trimEnd pull the
// line back from each tip to make room for an end decoration.
func Routed(pts []domain.Point, mid domain.Point, trimStart, trimEnd float64) Shape {
	a, z := pts[0], pts[len(pts)-1]
	s := Shape{Start: a, End: z, Mid: mid, Pts: append([]domain.Point(nil), pts...)}
	s.StartDir = UnitTo(a, pts[1], z)
	s.EndDir = UnitTo(z, pts[len(pts)-2], a)
	q := append([]domain.Point(nil), pts...)
	q[0] = domain.Point{X: a.X + s.StartDir[0]*trimStart, Y: a.Y + s.StartDir[1]*trimStart}
	q[len(q)-1] = domain.Point{X: z.X + s.EndDir[0]*trimEnd, Y: z.Y + s.EndDir[1]*trimEnd}
	s.D = rounded(q, cornerRadius)
	return s
}

// rounded writes pts as path data with each bend rounded by radius.
func rounded(pts []domain.Point, radius float64) string {
	n := svgutil.Num
	var d strings.Builder
	fmt.Fprintf(&d, "M%s,%s", n(pts[0].X), n(pts[0].Y))
	for i := 1; i < len(pts)-1; i++ {
		a, p, c := pts[i-1], pts[i], pts[i+1]
		l1 := math.Hypot(p.X-a.X, p.Y-a.Y)
		l2 := math.Hypot(c.X-p.X, c.Y-p.Y)
		r := math.Min(radius, math.Min(l1, l2)/2)
		if r < 0.5 || l1 == 0 || l2 == 0 {
			fmt.Fprintf(&d, " L%s,%s", n(p.X), n(p.Y))
			continue
		}
		in := domain.Point{X: p.X - (p.X-a.X)/l1*r, Y: p.Y - (p.Y-a.Y)/l1*r}
		out := domain.Point{X: p.X + (c.X-p.X)/l2*r, Y: p.Y + (c.Y-p.Y)/l2*r}
		fmt.Fprintf(&d, " L%s,%s Q%s,%s %s,%s", n(in.X), n(in.Y), n(p.X), n(p.Y), n(out.X), n(out.Y))
	}
	last := pts[len(pts)-1]
	fmt.Fprintf(&d, " L%s,%s", n(last.X), n(last.Y))
	return d.String()
}

// Hidden reports whether a placed label hides something: a line other
// than its own runs through it, or it overlaps another label. A diagram
// laid out by the older layered layout falls back to layout.Flow when it
// does.
func Hidden(lines []Shape, labels []Label) bool {
	for i, l := range labels {
		r := rect{l.X - l.W/2, l.Y - l.H/2, l.X + l.W/2, l.Y + l.H/2}
		for j, s := range lines {
			if j != l.Line && len(s.Pts) > 1 && r.crossedBy(s.Pts) {
				return true
			}
		}
		for _, o := range labels[i+1:] {
			if math.Min(r.x1, o.X+o.W/2)-math.Max(r.x0, o.X-o.W/2) > 1 && math.Min(r.y1, o.Y+o.H/2)-math.Max(r.y0, o.Y-o.H/2) > 1 {
				return true
			}
		}
	}
	return false
}
