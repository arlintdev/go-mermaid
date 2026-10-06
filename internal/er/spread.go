package er

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

// spreadEnds moves line ends that the layout put on the same spot of a
// box apart along that side, ordered by where their other ends lie, so two
// relationships never share one end and their glyphs never stack.
func spreadEnds(g *domain.Graph, edges []*domain.Edge) {
	type end struct {
		e     *domain.Edge
		idx   int     // 0 or len(Points)-1
		along float64 // position along the side
		other float64 // the far end's position along the same axis
	}
	groups := map[string][]end{}
	var keys []string
	for _, e := range edges {
		if e == nil || len(e.Points) < 2 {
			continue
		}
		for _, idx := range []int{0, len(e.Points) - 1} {
			id := e.From
			far := e.Points[len(e.Points)-1]
			if idx > 0 {
				id, far = e.To, e.Points[0]
			}
			n := g.NodeByID(id)
			if n == nil {
				continue
			}
			p := e.Points[idx]
			var side string
			var along, other float64
			switch {
			case math.Abs(p.Y-n.Pos.Y) < 1.5:
				side, along, other = "t", p.X, far.X
			case math.Abs(p.Y-n.Pos.Y-n.Size.H) < 1.5:
				side, along, other = "b", p.X, far.X
			case math.Abs(p.X-n.Pos.X) < 1.5:
				side, along, other = "l", p.Y, far.Y
			case math.Abs(p.X-n.Pos.X-n.Size.W) < 1.5:
				side, along, other = "r", p.Y, far.Y
			default:
				continue
			}
			k := id + "\x00" + side + "\x00" + fmt.Sprint(math.Round(along/4))
			if _, ok := groups[k]; !ok {
				keys = append(keys, k)
			}
			groups[k] = append(groups[k], end{e, idx, along, other})
		}
	}
	for _, k := range keys {
		grp := groups[k]
		if len(grp) < 2 {
			continue
		}
		sort.SliceStable(grp, func(i, j int) bool { return grp[i].other < grp[j].other })
		const gap = 26.0
		for i, en := range grp {
			off := (float64(i) - float64(len(grp)-1)/2) * gap
			pts := en.e.Points
			p := pts[en.idx]
			nb := 1
			if en.idx > 0 {
				nb = len(pts) - 2
			}
			horizontalSide := strings.Contains(k, "\x00t\x00") || strings.Contains(k, "\x00b\x00")
			if horizontalSide {
				if len(pts) > 2 && math.Abs(pts[nb].X-p.X) < 0.5 {
					pts[nb].X += off
				}
				pts[en.idx].X += off
			} else {
				if len(pts) > 2 && math.Abs(pts[nb].Y-p.Y) < 0.5 {
					pts[nb].Y += off
				}
				pts[en.idx].Y += off
			}
		}
	}
}
