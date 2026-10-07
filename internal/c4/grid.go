package c4

import (
	"math"
	"sort"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// gridRow is how many shapes a row of a grid holds, as Mermaid's
// c4ShapeInRow.
const gridRow = 4

const (
	gridColGap = 50.0 // least gap between two columns
	gridRowGap = 60.0 // least gap between two rows
	portGap    = 24.0 // most room between lines meeting one side of a shape
)

func graphBounds(g *domain.Graph) svgutil.Bounds {
	var bd svgutil.Bounds
	for _, n := range g.Nodes {
		bd.AddRect(n.Pos.X, n.Pos.Y, n.Size.W, n.Size.H)
	}
	for _, e := range g.Edges {
		for _, p := range e.Points {
			bd.Add(p.X, p.Y)
		}
	}
	return bd
}

// gridPlan is how one relationship crosses the grid.
type gridPlan struct {
	e      *domain.Edge
	a, b   int     // node indices, from and to
	across bool    // neighbours in a row: straight across
	lw, lh float64 // label size
	// Otherwise the line leaves a and enters b by a top or bottom side, at
	// xa and xb, and runs along the gap below row rows[0] (and, when the
	// rows are not neighbours, down the gap left of column col and along
	// the gap below rows[1]). tracks are its places in those row gaps.
	xa, xb float64
	rows   []int
	col    int
	tracks []int
}

// grid places g's nodes in rows of gridRow, in source order, as Mermaid
// does, and routes every edge along the gaps between them: straight
// between neighbours, else through the gap below a row and, when it must,
// the gap beside a column. Lines meeting one side of a shape spread along
// it, and lines sharing a gap run on tracks of their own, far enough apart
// for their labels. LabelPos is set to the centre of each label.
func (c *ctx) grid(g *domain.Graph) {
	idx := map[string]int{}
	for i, n := range g.Nodes {
		idx[n.ID] = i
	}
	rows := (len(g.Nodes) + gridRow - 1) / gridRow
	row := func(i int) int { return i / gridRow }
	col := func(i int) int { return i % gridRow }
	colW := make([]float64, gridRow)
	rowH := make([]float64, rows)
	for i, n := range g.Nodes {
		colW[col(i)] = max(colW[col(i)], n.Size.W)
		rowH[row(i)] = max(rowH[row(i)], n.Size.H)
	}

	var plans []*gridPlan
	colGap := gridColGap
	colUse := make([]int, gridRow+1)
	for _, e := range g.Edges {
		a, ok1 := idx[e.From]
		b, ok2 := idx[e.To]
		if !ok1 || !ok2 || a == b {
			continue
		}
		p := &gridPlan{e: e, a: a, b: b, col: -1}
		if e.Label != "" {
			p.lw, p.lh = c.textBox(svgutil.SplitLines(e.Label))
			p.lw, p.lh = p.lw+8, p.lh+4
		}
		ra, rb, ca, cb := row(a), row(b), col(a), col(b)
		switch {
		case ra == rb && abs(ca-cb) == 1:
			p.across = true
			colGap = max(colGap, p.lw+40)
		case abs(ra-rb) <= 1:
			p.rows = []int{min(ra, rb)}
		default:
			// Along the gap below the higher row, down the gap beside the
			// target's column, and along the gap above the lower row.
			p.rows = []int{min(ra, rb), max(ra, rb) - 1}
			p.col = cb + 1
			if cb > ca {
				p.col = cb
			}
			colUse[p.col]++
		}
		plans = append(plans, p)
	}
	for _, n := range colUse {
		colGap = max(colGap, float64(n+1)*portGap)
	}
	colX := make([]float64, gridRow+1)
	for i := 1; i <= gridRow; i++ {
		colX[i] = colX[i-1] + colW[i-1] + colGap
	}
	for i, n := range g.Nodes {
		n.Pos.X = colX[col(i)] + (colW[col(i)]-n.Size.W)/2
	}
	colTrack := make([]int, gridRow+1)
	gapX := func(p *gridPlan) float64 {
		x := colX[p.col] - colGap/2 + (float64(colTrack[p.col])-float64(colUse[p.col]-1)/2)*portGap
		colTrack[p.col]++
		return x
	}

	// Spread the lines that meet the top or bottom sides facing one row
	// gap in one column, ordered by where they head, so no two share a
	// point or run along each other there.
	type end struct {
		p          *gridPlan
		from, both bool    // which end; both when the line runs straight down the column
		to         float64 // where the line heads
	}
	type slot struct{ col, gap int }
	faces := map[slot][]end{}
	for _, p := range plans {
		if p.across {
			continue
		}
		ra, rb := row(p.a), row(p.b)
		ga, gb := g.Nodes[p.a].Center().X, g.Nodes[p.b].Center().X
		if p.col >= 0 {
			gx := colX[p.col] - colGap/2
			ga, gb = gx, gx
		}
		// The gap a side faces: below its row, or above it (the gap
		// below the row before).
		gapA, gapB := ra, rb
		if ra > rb {
			gapA = ra - 1
		}
		if rb > ra {
			gapB = rb - 1
		}
		ka, kb := slot{col(p.a), gapA}, slot{col(p.b), gapB}
		if ka == kb {
			faces[ka] = append(faces[ka], end{p, true, true, ga})
			continue
		}
		faces[ka] = append(faces[ka], end{p, true, false, gb})
		faces[kb] = append(faces[kb], end{p, false, false, ga})
	}
	keys := make([]slot, 0, len(faces))
	for k := range faces {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].col != keys[j].col {
			return keys[i].col < keys[j].col
		}
		return keys[i].gap < keys[j].gap
	})
	for _, k := range keys {
		ends := faces[k]
		sort.SliceStable(ends, func(i, j int) bool { return ends[i].to < ends[j].to })
		w := colW[k.col]
		for _, en := range ends {
			if en.from || en.both {
				w = math.Min(w, g.Nodes[en.p.a].Size.W)
			}
			if !en.from || en.both {
				w = math.Min(w, g.Nodes[en.p.b].Size.W)
			}
		}
		step := math.Min(portGap, w/float64(len(ends)+1))
		cx := colX[k.col] + colW[k.col]/2
		for i, en := range ends {
			x := cx + (float64(i)-float64(len(ends)-1)/2)*step
			if en.both {
				en.p.xa, en.p.xb = x, x
			} else if en.from {
				en.p.xa = x
			} else {
				en.p.xb = x
			}
		}
	}

	// Tracks: a line that changes x in a row gap takes a track of its own
	// there; the gap grows to hold them and their labels.
	use := make([][]*gridPlan, rows)
	for _, p := range plans {
		for k, r := range p.rows {
			straight := len(p.rows) == 1 && math.Abs(p.xa-p.xb) < 0.5
			p.tracks = append(p.tracks, -1)
			if !straight {
				p.tracks[k] = len(use[r])
				use[r] = append(use[r], p)
			}
		}
	}
	rowGap := make([]float64, rows)
	trackH := make([]float64, rows)
	for r := range rowGap {
		rowGap[r] = gridRowGap
		trackH[r] = 12
		for _, p := range use[r] {
			trackH[r] = max(trackH[r], p.lh+6)
		}
		rowGap[r] = max(rowGap[r], float64(len(use[r])+1)*trackH[r])
	}
	for _, p := range plans {
		if len(p.rows) == 1 && p.tracks[0] < 0 {
			rowGap[p.rows[0]] = max(rowGap[p.rows[0]], p.lh+30)
		}
	}
	rowY := make([]float64, rows+1)
	for i := 1; i <= rows; i++ {
		rowY[i] = rowY[i-1] + rowH[i-1] + rowGap[i-1]
	}
	for i, n := range g.Nodes {
		n.Pos.Y = rowY[row(i)] + (rowH[row(i)]-n.Size.H)/2
	}
	trackY := func(r, k int) float64 {
		return rowY[r] + rowH[r] + rowGap[r]/2 + (float64(k)-float64(len(use[r])-1)/2)*trackH[r]
	}

	for _, p := range plans {
		na, nb := g.Nodes[p.a], g.Nodes[p.b]
		var pts []domain.Point
		switch {
		case p.across:
			y := (math.Max(na.Pos.Y, nb.Pos.Y) + math.Min(na.Pos.Y+na.Size.H, nb.Pos.Y+nb.Size.H)) / 2
			xa, xb := na.Pos.X+na.Size.W, nb.Pos.X
			if col(p.b) < col(p.a) {
				xa, xb = na.Pos.X, nb.Pos.X+nb.Size.W
			}
			pts = []domain.Point{{X: xa, Y: y}, {X: xb, Y: y}}
		default:
			ra, rb := row(p.a), row(p.b)
			ya, yb := na.Pos.Y+na.Size.H, nb.Pos.Y+nb.Size.H
			if ra > rb {
				ya = na.Pos.Y
			}
			if rb > ra {
				yb = nb.Pos.Y
			}
			switch {
			case p.tracks[0] < 0:
				pts = []domain.Point{{X: p.xa, Y: ya}, {X: p.xb, Y: yb}}
			case len(p.rows) == 1:
				gy := trackY(p.rows[0], p.tracks[0])
				pts = []domain.Point{{X: p.xa, Y: ya}, {X: p.xa, Y: gy}, {X: p.xb, Y: gy}, {X: p.xb, Y: yb}}
			default:
				gy, gy2 := trackY(p.rows[0], p.tracks[0]), trackY(p.rows[1], p.tracks[1])
				if ra > rb {
					gy, gy2 = gy2, gy
				}
				gx := gapX(p)
				pts = []domain.Point{{X: p.xa, Y: ya}, {X: p.xa, Y: gy}, {X: gx, Y: gy}, {X: gx, Y: gy2}, {X: p.xb, Y: gy2}, {X: p.xb, Y: yb}}
			}
		}
		p.e.Points = pts
		p.e.LabelPos = longestRun(pts)
	}
}

// longestRun returns the middle of the longest straight piece of pts.
func longestRun(pts []domain.Point) domain.Point {
	best, at := -1.0, pts[0]
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		if l := math.Hypot(b.X-a.X, b.Y-a.Y); l > best {
			best, at = l, domain.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
		}
	}
	return at
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
