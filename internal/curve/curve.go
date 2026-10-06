// Package curve draws the relationship lines of the diagram types laid out
// by layout.Compute (class, state, ER, requirement, C4): a smooth curve
// between the two ends when it crosses no box, as Mermaid draws them, else
// the routed polyline.
package curve

import (
	"fmt"
	"math"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// Shape is how one relationship line is drawn: a smooth curve between
// its two ends when that crosses no box, as Mermaid draws them, else the
// layout's routed polyline.
type Shape struct {
	D                string       // path data
	Start, End       domain.Point // the two tips
	StartDir, EndDir [2]float64   // unit vectors from each tip back along the line
	Mid              domain.Point // halfway along
	Curved           bool
	// Pts is the drawn line as a polyline (a curve sampled finely enough
	// to test what it passes through).
	Pts []domain.Point
}

// Box is an obstacle: a laid-out node's rectangle.
type Box struct{ X, Y, W, H float64 }

// Hits reports whether p lies inside the box, a little in from its edges.
func (b Box) Hits(p domain.Point) bool {
	const in = 3
	return p.X > b.X+in && p.X < b.X+b.W-in && p.Y > b.Y+in && p.Y < b.Y+b.H-in
}

// Edge builds the drawn line for routed points pts. vertical says the
// layout flows top to bottom (or bottom to top); the curve leaves and
// enters along that axis. trimStart and trimEnd pull the line back from
// each tip to make room for an end decoration.
func Edge(pts []domain.Point, vertical bool, obstacles []Box, trimStart, trimEnd float64) Shape {
	a, z := pts[0], pts[len(pts)-1]
	var c1, c2 domain.Point
	if vertical {
		k := (z.Y - a.Y) / 2
		c1, c2 = domain.Point{X: a.X, Y: a.Y + k}, domain.Point{X: z.X, Y: z.Y - k}
	} else {
		k := (z.X - a.X) / 2
		c1, c2 = domain.Point{X: a.X + k, Y: a.Y}, domain.Point{X: z.X - k, Y: z.Y}
	}
	bez := func(t float64) domain.Point {
		u := 1 - t
		return domain.Point{
			X: u*u*u*a.X + 3*u*u*t*c1.X + 3*u*t*t*c2.X + t*t*t*z.X,
			Y: u*u*u*a.Y + 3*u*u*t*c1.Y + 3*u*t*t*c2.Y + t*t*t*z.Y,
		}
	}
	clear := math.Hypot(z.X-a.X, z.Y-a.Y) > 1 && (len(pts) <= 2 || bendy(pts))
	for i := 1; i < 40 && clear; i++ {
		p := bez(float64(i) / 40)
		for _, o := range obstacles {
			if o.Hits(p) {
				clear = false
				break
			}
		}
	}
	if clear {
		s := Shape{Start: a, End: z, Curved: true, Mid: bez(0.5), Pts: Bezier(a, c1, c2, z)}
		s.StartDir = UnitTo(a, c1, z)
		s.EndDir = UnitTo(z, c2, a)
		a2 := domain.Point{X: a.X + s.StartDir[0]*trimStart, Y: a.Y + s.StartDir[1]*trimStart}
		z2 := domain.Point{X: z.X + s.EndDir[0]*trimEnd, Y: z.Y + s.EndDir[1]*trimEnd}
		n := svgutil.Num
		s.D = fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s", n(a2.X), n(a2.Y), n(c1.X), n(c1.Y), n(c2.X), n(c2.Y), n(z2.X), n(z2.Y))
		return s
	}
	s := Shape{Start: a, End: z, Mid: domain.PolylineMidpoint(pts), Pts: append([]domain.Point(nil), pts...)}
	s.StartDir = UnitTo(a, pts[1], z)
	s.EndDir = UnitTo(z, pts[len(pts)-2], a)
	q := append([]domain.Point(nil), pts...)
	q[0] = domain.Point{X: a.X + s.StartDir[0]*trimStart, Y: a.Y + s.StartDir[1]*trimStart}
	q[len(q)-1] = domain.Point{X: z.X + s.EndDir[0]*trimEnd, Y: z.Y + s.EndDir[1]*trimEnd}
	s.D = Path(q)
	return s
}

// bezierSteps is how many straight pieces stand for one curve.
const bezierSteps = 24

// Bezier samples the cubic curve a-c1-c2-z as a polyline.
func Bezier(a, c1, c2, z domain.Point) []domain.Point {
	pts := make([]domain.Point, 0, bezierSteps+1)
	for i := 0; i <= bezierSteps; i++ {
		t := float64(i) / bezierSteps
		u := 1 - t
		pts = append(pts, domain.Point{
			X: u*u*u*a.X + 3*u*u*t*c1.X + 3*u*t*t*c2.X + t*t*t*z.X,
			Y: u*u*u*a.Y + 3*u*u*t*c1.Y + 3*u*t*t*c2.Y + t*t*t*z.Y,
		})
	}
	return pts
}

// bendy reports whether a routed polyline is only the layout's way around
// a corner (every point on one of the two ends' axes), which a curve
// replaces cleanly; a longer route that weaves between boxes is kept.
func bendy(pts []domain.Point) bool { return len(pts) <= 4 }

// UnitTo is the unit vector from p toward q, or toward r when q is p.
func UnitTo(p, q, r domain.Point) [2]float64 {
	dx, dy := q.X-p.X, q.Y-p.Y
	if math.Hypot(dx, dy) < 0.01 {
		dx, dy = r.X-p.X, r.Y-p.Y
	}
	d := math.Hypot(dx, dy)
	if d == 0 {
		return [2]float64{0, 0}
	}
	return [2]float64{dx / d, dy / d}
}

// Path writes pts as SVG path data of straight segments.
func Path(pts []domain.Point) string {
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
