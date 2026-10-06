package curve

import (
	"math"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

// Label is one relationship label to place: the line it belongs to (an
// index into the lines given to PlaceLabels), its size, and the centre the
// renderer chose for it.
type Label struct {
	Line int
	W, H float64
	X, Y float64
}

// endRoom keeps a label this far from either tip of its own line, where the
// line's end decorations are drawn.
const endRoom = 20.0

// besideCost is what moving a label off its line, to sit beside it, costs.
const besideCost = 0.5

// labelSpots are the fractions of its own line's length where a label may
// sit instead of its first spot, nearest the middle first.
var labelSpots = []float64{0.5, 0.42, 0.58, 0.34, 0.66, 0.26, 0.74, 0.18, 0.82}

// PlaceLabels moves each label along its own line where its first spot
// would cover another line, another label or a box, to the spot that
// covers the least. A label already clear stays where it is. The work is
// bounded (a fixed number of spots per label) and the result depends only
// on the input order, so output stays deterministic.
func PlaceLabels(lines []Shape, labels []Label, boxes []Box) {
	p := placer{lines: lines, labels: labels, boxes: boxes, bounds: make([]rect, len(lines))}
	for j, s := range lines {
		p.bounds[j] = boundsOf(s.Pts)
	}
	for i := range labels {
		l := &labels[i]
		if l.Line < 0 || l.Line >= len(lines) {
			continue
		}
		best := p.cost(i, l.X, l.Y, nil)
		if best == 0 {
			continue
		}
		own := lines[l.Line]
		total := length(own.Pts)
		for _, f := range labelSpots {
			d := f * total
			at, dx, dy := pointAt(own.Pts, d)
			// How far the label reaches along its line from its centre.
			reach := math.Abs(dx)*l.W/2 + math.Abs(dy)*l.H/2
			if d < endRoom+reach || total-d < endRoom+reach {
				continue
			}
			if c := p.cost(i, at.X, at.Y, nil); c < best {
				best, l.X, l.Y = c, at.X, at.Y
			}
			// Beside the line, touching it, when no spot on it is clear;
			// it costs a little more than a spot on the line, and nothing
			// may pass between the label and its line.
			side := math.Abs(dy)*l.W/2 + math.Abs(dx)*l.H/2 + 2
			for _, k := range []float64{1, -1} {
				x, y := at.X-k*dy*side, at.Y+k*dx*side
				if c := p.cost(i, x, y, &at) + besideCost; c < best {
					best, l.X, l.Y = c, x, y
				}
			}
		}
	}
}

// cost scores label i centred at x, y: each other line it covers
// counts one, each other label or box it overlaps counts more, since
// those hide text. When anchor is set, the label sits beside its line at
// that point, and a line passing between the two counts as covered.
func (p *placer) cost(i int, x, y float64, anchor *domain.Point) float64 {
	l := p.labels[i]
	r := rect{x - l.W/2, y - l.H/2, x + l.W/2, y + l.H/2}
	reach := r
	if anchor != nil {
		reach = rect{math.Min(r.x0, anchor.X-3), math.Min(r.y0, anchor.Y-3), math.Max(r.x1, anchor.X+3), math.Max(r.y1, anchor.Y+3)}
	}
	cost := 0.0
	for j, s := range p.lines {
		if j != l.Line && reach.overlaps(p.bounds[j]) && reach.crossedBy(s.Pts) {
			cost++
		}
	}
	for j, o := range p.labels {
		if j != i && r.overlaps(rect{o.X - o.W/2, o.Y - o.H/2, o.X + o.W/2, o.Y + o.H/2}) {
			cost += 4
		}
	}
	for _, b := range p.boxes {
		if r.overlaps(rect{b.X, b.Y, b.X + b.W, b.Y + b.H}) {
			cost += 4
		}
	}
	return cost
}

// placer holds what PlaceLabels works over, with each line's bounds so
// most lines are ruled out without walking them.
type placer struct {
	lines  []Shape
	labels []Label
	boxes  []Box
	bounds []rect
}

func boundsOf(pts []domain.Point) rect {
	r := rect{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, q := range pts {
		r = rect{math.Min(r.x0, q.X), math.Min(r.y0, q.Y), math.Max(r.x1, q.X), math.Max(r.y1, q.Y)}
	}
	return r
}

type rect struct{ x0, y0, x1, y1 float64 }

func (a rect) overlaps(b rect) bool {
	return a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1
}

// Crosses reports whether the polyline pts passes through the interior of
// box b, a pixel in from its edges.
func Crosses(pts []domain.Point, b Box) bool {
	return rect{b.X, b.Y, b.X + b.W, b.Y + b.H}.crossedBy(pts)
}

// crossedBy reports whether the polyline passes through the rectangle's
// interior, a pixel in from its edges.
func (a rect) crossedBy(pts []domain.Point) bool {
	in := rect{a.x0 + 1, a.y0 + 1, a.x1 - 1, a.y1 - 1}
	for k := 1; k < len(pts); k++ {
		if in.segment(pts[k-1], pts[k]) {
			return true
		}
	}
	return false
}

// segment reports whether segment p-q meets the rectangle (Liang-Barsky).
func (a rect) segment(p, q domain.Point) bool {
	t0, t1 := 0.0, 1.0
	dx, dy := q.X-p.X, q.Y-p.Y
	for _, c := range [4][2]float64{{-dx, p.X - a.x0}, {dx, a.x1 - p.X}, {-dy, p.Y - a.y0}, {dy, a.y1 - p.Y}} {
		pp, qq := c[0], c[1]
		if pp == 0 {
			if qq < 0 {
				return false
			}
			continue
		}
		t := qq / pp
		if pp < 0 {
			t0 = math.Max(t0, t)
		} else {
			t1 = math.Min(t1, t)
		}
		if t0 > t1 {
			return false
		}
	}
	return true
}

func length(pts []domain.Point) float64 {
	total := 0.0
	for k := 1; k < len(pts); k++ {
		total += math.Hypot(pts[k].X-pts[k-1].X, pts[k].Y-pts[k-1].Y)
	}
	return total
}

// pointAt returns the point at distance d along the polyline, and the unit
// direction of the line there.
func pointAt(pts []domain.Point, d float64) (p domain.Point, dx, dy float64) {
	for k := 1; k < len(pts); k++ {
		ux, uy := pts[k].X-pts[k-1].X, pts[k].Y-pts[k-1].Y
		seg := math.Hypot(ux, uy)
		if seg == 0 {
			continue
		}
		if d <= seg || k == len(pts)-1 {
			f := math.Min(d/seg, 1)
			return domain.Point{X: pts[k-1].X + f*ux, Y: pts[k-1].Y + f*uy}, ux / seg, uy / seg
		}
		d -= seg
	}
	return pts[len(pts)-1], 0, 0
}
