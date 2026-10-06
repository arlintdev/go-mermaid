package layout

import (
	"math"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

// total primary extent, for flipping BT and RL.
func (f *flowGraph) primaryExtent() float64 {
	ext := 0.0
	for r := range f.layers {
		ext = math.Max(ext, f.bandTop[r]+f.bandH[r])
	}
	for _, c := range f.clusters {
		if c.used {
			ext = math.Max(ext, c.bottom)
		}
	}
	return ext
}

// screen maps a rank-space point to the picture.
func (f *flowGraph) screen(x, p, ext float64) domain.Point {
	switch f.dir {
	case domain.BottomTop:
		return domain.Point{X: x, Y: ext - p}
	case domain.LeftRight:
		return domain.Point{X: p, Y: x}
	case domain.RightLeft:
		return domain.Point{X: ext - p, Y: x}
	}
	return domain.Point{X: x, Y: p}
}

// writeBack moves the layout from rank space onto the domain graph.
func (f *flowGraph) writeBack() {
	ext := f.primaryExtent()
	for _, n := range f.nodes {
		if n.real == nil {
			continue
		}
		c := f.screen(n.x, n.p, ext)
		n.real.Pos = domain.Point{X: c.X - n.real.Size.W/2, Y: c.Y - n.real.Size.H/2}
	}
	for _, e := range f.edges {
		e.e.Points = nil
		e.e.LabelCenter = true
		if e.invisible {
			continue
		}
		pts := make([]domain.Point, len(e.rpts))
		for i, p := range e.rpts {
			pts[i] = f.screen(p.x, p.p, ext)
		}
		if len(pts) >= 2 && !e.self {
			pts[0] = clipToOutline(e.from.real, pts[0], pts[1])
			last := len(pts) - 1
			pts[last] = clipToOutline(e.to.real, pts[last], pts[last-1])
		}
		if e.reversed {
			for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
				pts[i], pts[j] = pts[j], pts[i]
			}
		}
		e.e.Points = pts
		if e.e.Label != "" {
			e.e.LabelPos = f.screen(e.labelAt.x, e.labelAt.p, ext)
		}
	}
	for _, c := range f.clusters {
		if !c.used {
			continue
		}
		a := f.screen(c.lo, c.top, ext)
		b := f.screen(c.hi, c.bottom, ext)
		c.sg.Box = domain.Rect{
			Min:  domain.Point{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y)},
			Size: domain.Size{W: math.Abs(b.X - a.X), H: math.Abs(b.Y - a.Y)},
		}
	}
	for _, e := range f.edges {
		f.clipToClusters(e)
	}
}

// clipToClusters trims an edge that names a subgraph so it stops at, or
// starts from, the subgraph's box.
func (f *flowGraph) clipToClusters(e *fedge) {
	pts := e.e.Points
	if len(pts) < 2 {
		return
	}
	if e.toCl >= 0 {
		box := f.clusters[e.toCl].sg.Box
		for i := 1; i < len(pts); i++ {
			if inRect(box, pts[i]) && !inRect(box, pts[i-1]) {
				pts = append(pts[:i:i], crossRect(box, pts[i-1], pts[i]))
				break
			}
		}
	}
	if e.fromCl >= 0 {
		box := f.clusters[e.fromCl].sg.Box
		for i := len(pts) - 2; i >= 0; i-- {
			if inRect(box, pts[i]) && !inRect(box, pts[i+1]) {
				pts = append([]domain.Point{crossRect(box, pts[i+1], pts[i])}, pts[i+1:]...)
				break
			}
		}
	}
	e.e.Points = pts
}

func inRect(r domain.Rect, p domain.Point) bool {
	return p.X >= r.Min.X && p.X <= r.Min.X+r.Size.W && p.Y >= r.Min.Y && p.Y <= r.Min.Y+r.Size.H
}

// crossRect returns where the axis-aligned segment from out (outside r) to
// in (inside r) crosses r's border.
func crossRect(r domain.Rect, out, in domain.Point) domain.Point {
	switch {
	case out.X == in.X && out.Y < r.Min.Y:
		return domain.Point{X: out.X, Y: r.Min.Y}
	case out.X == in.X:
		return domain.Point{X: out.X, Y: r.Min.Y + r.Size.H}
	case out.X < r.Min.X:
		return domain.Point{X: r.Min.X, Y: out.Y}
	}
	return domain.Point{X: r.Min.X + r.Size.W, Y: out.Y}
}

// clipToOutline moves end, which lies on n's bounding box, onto the shape's
// outline along the axis-aligned segment toward next.
func clipToOutline(n *domain.Node, end, next domain.Point) domain.Point {
	if n == nil {
		return end
	}
	c := n.Center()
	w, h := n.Size.W, n.Size.H
	if math.Abs(end.X-next.X) < 0.5 {
		// Vertical segment: find the outline at x = end.X on the side
		// facing next.
		dy := halfExtent(n.Shape, w, h, end.X-c.X, true)
		if next.Y < c.Y {
			return domain.Point{X: end.X, Y: c.Y - dy}
		}
		return domain.Point{X: end.X, Y: c.Y + dy}
	}
	dx := halfExtent(n.Shape, h, w, end.Y-c.Y, false)
	if next.X < c.X {
		return domain.Point{X: c.X - dx, Y: end.Y}
	}
	return domain.Point{X: c.X + dx, Y: end.Y}
}

// halfExtent returns how far the outline of a shape of size along x along
// the cross axis and depth along the travel axis reaches from its centre,
// at offset off across. vertical is true when travel is vertical.
func halfExtent(s domain.Shape, along, depth, off float64, vertical bool) float64 {
	a, d := along/2, depth/2
	off = math.Abs(off)
	if off > a {
		off = a
	}
	switch s {
	case domain.ShapeCircle, domain.ShapeDoubleCircle, domain.ShapeSmallCircle, domain.ShapeFramedCircle:
		return math.Sqrt(math.Max(d*d-off*off, 0))
	case domain.ShapeDiamond:
		return d * (1 - off/a)
	case domain.ShapeHexagon:
		if vertical {
			k := depth / 4
			if off > a-k {
				return d * (a - off) / k
			}
			return d
		}
		k := along / 4
		_ = k
		return d - depth/4*off/a
	case domain.ShapeStadium:
		if vertical {
			r := depth / 2
			if off > a-r {
				u := off - (a - r)
				return math.Sqrt(math.Max(r*r-u*u, 0))
			}
			return d
		}
		return d - (along/2)*(1-math.Sqrt(math.Max(1-off*off/(a*a), 0)))
	case domain.ShapeCylinder:
		if vertical {
			ry := CylinderRY(along)
			return d - ry + ry*math.Sqrt(math.Max(1-off*off/(a*a), 0))
		}
		return d
	}
	return d
}

// normalize shifts everything so the top-left corner of what is drawn is at
// the origin and returns the size.
func (f *flowGraph) normalize() (w, h float64) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	grow := func(x0, y0, x1, y1 float64) {
		minX, minY = math.Min(minX, x0), math.Min(minY, y0)
		maxX, maxY = math.Max(maxX, x1), math.Max(maxY, y1)
	}
	for _, n := range f.g.Nodes {
		grow(n.Pos.X, n.Pos.Y, n.Pos.X+n.Size.W, n.Pos.Y+n.Size.H)
	}
	for _, e := range f.g.Edges {
		for _, p := range e.Points {
			grow(p.X, p.Y, p.X, p.Y)
		}
		if e.Label != "" && len(e.Points) > 0 {
			s := e.LabelSize
			grow(e.LabelPos.X-s.W/2, e.LabelPos.Y-s.H/2, e.LabelPos.X+s.W/2, e.LabelPos.Y+s.H/2)
		}
	}
	for i, c := range f.clusters {
		if c.used {
			b := f.clusters[i].sg.Box
			grow(b.Min.X, b.Min.Y, b.Min.X+b.Size.W, b.Min.Y+b.Size.H)
		}
	}
	if math.IsInf(minX, 1) {
		return 0, 0
	}
	dx, dy := -minX, -minY
	for _, n := range f.g.Nodes {
		n.Pos.X += dx
		n.Pos.Y += dy
	}
	for _, e := range f.g.Edges {
		for i := range e.Points {
			e.Points[i].X += dx
			e.Points[i].Y += dy
		}
		e.LabelPos.X += dx
		e.LabelPos.Y += dy
	}
	for _, c := range f.clusters {
		if c.used {
			c.sg.Box.Min.X += dx
			c.sg.Box.Min.Y += dy
		}
	}
	return maxX - minX, maxY - minY
}
