package state

import (
	"fmt"
	"math"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// edgeShape is how one relationship line is drawn: a smooth curve between
// its two ends when that crosses no box, as Mermaid draws them, else the
// layout's routed polyline.
type edgeShape struct {
	d          string       // path data
	start, end domain.Point // the two tips
	sdir, edir [2]float64   // unit vectors from each tip back along the line
	mid        domain.Point // halfway along
	curved     bool
}

// box is an obstacle: a laid-out node's rectangle.
type box struct{ x, y, w, h float64 }

func (b box) hits(p domain.Point) bool {
	const in = 3
	return p.X > b.x+in && p.X < b.x+b.w-in && p.Y > b.y+in && p.Y < b.y+b.h-in
}

// shapeEdge builds the drawn line for routed points pts. vertical says the
// layout flows top to bottom (or bottom to top); the curve leaves and
// enters along that axis. trimStart and trimEnd pull the line back from
// each tip to make room for an end decoration.
func shapeEdge(pts []domain.Point, vertical bool, obstacles []box, trimStart, trimEnd float64) edgeShape {
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
			if o.hits(p) {
				clear = false
				break
			}
		}
	}
	if clear {
		s := edgeShape{start: a, end: z, curved: true, mid: bez(0.5)}
		s.sdir = unitTo(a, c1, z)
		s.edir = unitTo(z, c2, a)
		a2 := domain.Point{X: a.X + s.sdir[0]*trimStart, Y: a.Y + s.sdir[1]*trimStart}
		z2 := domain.Point{X: z.X + s.edir[0]*trimEnd, Y: z.Y + s.edir[1]*trimEnd}
		n := svgutil.Num
		s.d = fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s", n(a2.X), n(a2.Y), n(c1.X), n(c1.Y), n(c2.X), n(c2.Y), n(z2.X), n(z2.Y))
		return s
	}
	s := edgeShape{start: a, end: z, mid: domain.PolylineMidpoint(pts)}
	s.sdir = unitTo(a, pts[1], z)
	s.edir = unitTo(z, pts[len(pts)-2], a)
	q := append([]domain.Point(nil), pts...)
	q[0] = domain.Point{X: a.X + s.sdir[0]*trimStart, Y: a.Y + s.sdir[1]*trimStart}
	q[len(q)-1] = domain.Point{X: z.X + s.edir[0]*trimEnd, Y: z.Y + s.edir[1]*trimEnd}
	s.d = path(q)
	return s
}

// bendy reports whether a routed polyline is only the layout's way around
// a corner (every point on one of the two ends' axes), which a curve
// replaces cleanly; a longer route that weaves between boxes is kept.
func bendy(pts []domain.Point) bool { return len(pts) <= 4 }

// unitTo is the unit vector from p toward q, or toward r when q is p.
func unitTo(p, q, r domain.Point) [2]float64 {
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
