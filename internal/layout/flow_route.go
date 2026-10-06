package layout

import (
	"math"
	"sort"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

// A route is an edge's path as cross positions per gap: xs[i] is where the
// edge runs between chain[i] and chain[i+1]'s layers; a change of x between
// consecutive entries is a horizontal run in that gap, on its own track.
type route struct {
	e      *fedge
	xs     []float64 // cross position at each chain element (ports at the ends)
	tracks []int     // per chain step, the track of its horizontal run or -1
}

// hrun is a horizontal run in a gap, for track assignment.
type hrun struct {
	r        *route
	step     int
	from, to float64
}

// countTracks plans every route (ports, then runs) and returns how many
// tracks each gap needs.
func (f *flowGraph) countTracks() []int {
	f.planPorts()
	byGap := map[int][]*hrun{}
	for _, e := range f.edges {
		if e.self || len(e.chain) < 2 {
			continue
		}
		r := f.routes[e]
		r.tracks = make([]int, len(e.chain)-1)
		for i := 0; i+1 < len(e.chain); i++ {
			r.tracks[i] = -1
			if math.Abs(r.xs[i]-r.xs[i+1]) > 0.5 {
				g := e.chain[i].rank
				byGap[g] = append(byGap[g], &hrun{r: r, step: i, from: r.xs[i], to: r.xs[i+1]})
			}
		}
	}
	counts := make([]int, len(f.layers))
	for g, runs := range byGap {
		counts[g] = assignTracks(runs)
	}
	return counts
}

// assignTracks gives overlapping runs in one gap different tracks, ordered
// so runs leaving one node do not cross each other: of the runs heading
// left, the one going furthest left takes the highest track; of those
// heading right, the one going furthest right. It returns the track count.
func assignTracks(runs []*hrun) int {
	sort.SliceStable(runs, func(i, j int) bool {
		a, b := runs[i], runs[j]
		al, bl := a.to < a.from, b.to < b.from
		if al != bl {
			return al
		}
		if al {
			return a.to < b.to
		}
		return a.to > b.to
	})
	n := 0
	var done []*hrun
	for _, h := range runs {
		lo, hi := math.Min(h.from, h.to), math.Max(h.from, h.to)
		t := 0
		for _, d := range done {
			dlo, dhi := math.Min(d.from, d.to), math.Max(d.from, d.to)
			if lo <= dhi+trackGap/2 && dlo <= hi+trackGap/2 {
				t = max(t, d.r.tracks[d.step]+1)
			}
		}
		h.r.tracks[h.step] = t
		done = append(done, h)
		n = max(n, t+1)
	}
	return n
}

// planPorts picks where each edge leaves and meets its nodes. Edges on one
// side of a node spread across it in the order of where they go, so they
// neither bunch at the centre nor cross as they leave; an edge whose next
// waypoint lies within the node's side leaves straight toward it.
func (f *flowGraph) planPorts() {
	f.routes = map[*fedge]*route{}
	type use struct {
		r      *route
		idx    int     // index in xs of this end
		toward float64 // cross position of the adjacent waypoint
	}
	sides := map[*fnode]map[bool][]use{}
	add := func(n *fnode, bottom bool, u use) {
		if sides[n] == nil {
			sides[n] = map[bool][]use{}
		}
		sides[n][bottom] = append(sides[n][bottom], u)
	}
	for _, e := range f.edges {
		if e.self || len(e.chain) < 2 {
			continue
		}
		r := &route{e: e, xs: make([]float64, len(e.chain))}
		for i, n := range e.chain {
			r.xs[i] = n.x
		}
		f.routes[e] = r
		last := len(e.chain) - 1
		add(e.chain[0], true, use{r, 0, e.chain[1].x})
		add(e.chain[last], false, use{r, last, e.chain[last-1].x})
	}
	for n, bySide := range sides {
		for _, us := range bySide {
			sort.SliceStable(us, func(i, j int) bool { return us[i].toward < us[j].toward })
			half := f.portHalfWidth(n)
			ports := make([]float64, len(us))
			gap := portGap
			if len(us) > 1 && 2*half/float64(len(us)-1) < gap {
				gap = 2 * half / float64(len(us)-1)
			}
			for i, u := range us {
				ports[i] = math.Max(n.x-half, math.Min(n.x+half, u.toward))
				if i > 0 && ports[i] < ports[i-1]+gap {
					ports[i] = ports[i-1] + gap
				}
			}
			for i := len(us) - 1; i >= 0; i-- {
				limit := n.x + half
				if i+1 < len(us) {
					limit = math.Min(limit, ports[i+1]-gap)
				}
				ports[i] = math.Min(ports[i], limit)
			}
			for i, u := range us {
				u.r.xs[u.idx] = ports[i]
			}
		}
	}
}

// portHalfWidth is how far from a node's centre an edge may meet it.
func (f *flowGraph) portHalfWidth(n *fnode) float64 {
	if n.real == nil {
		return 0
	}
	half := n.cs/2 - 8
	switch n.real.Shape {
	case domain.ShapeDiamond, domain.ShapeCircle, domain.ShapeDoubleCircle:
		half = n.cs / 4
	case domain.ShapeHexagon, domain.ShapeStadium:
		half = n.cs/2 - n.ps/2
	case domain.ShapeSmallCircle, domain.ShapeFramedCircle:
		half = 0
	}
	return math.Max(half, 0)
}

// route builds every edge's points in rank space.
func (f *flowGraph) route() {
	for _, e := range f.edges {
		if e.self {
			f.routeSelf(e)
			continue
		}
		r := f.routes[e]
		if r == nil {
			continue
		}
		var pts []rpt
		first, last := e.chain[0], e.chain[len(e.chain)-1]
		x := r.xs[0]
		pts = append(pts, rpt{x, first.p + first.ps/2})
		for i := 0; i+1 < len(e.chain); i++ {
			if t := r.tracks[i]; t >= 0 {
				y := f.trackY(e.chain[i].rank, t)
				x = r.xs[i+1]
				pts = append(pts, rpt{r.xs[i], y}, rpt{x, y})
			}
			// A step without a run differs by less than half a pixel: keep
			// the line straight.
			r.xs[i+1] = x
		}
		pts = append(pts, rpt{x, last.p - last.ps/2})
		e.rpts = simplify(pts)
		if e.label != nil {
			e.labelAt = rpt{r.xs[indexOf(e.chain, e.label)], e.label.p}
		}
	}
}

func indexOf(chain []*fnode, n *fnode) int {
	for i, c := range chain {
		if c == n {
			return i
		}
	}
	return 0
}

// trackY is the primary coordinate of track t in the gap below layer g.
func (f *flowGraph) trackY(g, t int) float64 {
	n := f.trackCount[g]
	top, bottom := f.trackTop[g], f.trackBottom[g]
	return top + (bottom-top)*float64(t+1)/float64(n+1)
}

// routeSelf draws a loop on the trailing cross side of the node.
func (f *flowGraph) routeSelf(e *fedge) {
	n := e.from
	out := 22.0
	side := n.x + n.cs/2
	q := n.ps / 4
	e.rpts = []rpt{{side, n.p - q}, {side + out, n.p - q}, {side + out, n.p + q}, {side, n.p + q}}
	if e.e.Label != "" {
		lps, lcs := f.toRank(e.e.LabelSize)
		_ = lps
		e.labelAt = rpt{side + out + 4 + lcs/2, n.p}
	}
}

// rpt is a point in rank space: x across, p along the ranks.
type rpt struct{ x, p float64 }

// simplify drops repeated points and the middle of straight runs.
func simplify(pts []rpt) []rpt {
	var out []rpt
	for _, p := range pts {
		if len(out) > 0 && math.Abs(out[len(out)-1].x-p.x) < 0.01 && math.Abs(out[len(out)-1].p-p.p) < 0.01 {
			continue
		}
		if len(out) >= 2 {
			a, b := out[len(out)-2], out[len(out)-1]
			if (math.Abs(a.x-b.x) < 0.01 && math.Abs(b.x-p.x) < 0.01) || (math.Abs(a.p-b.p) < 0.01 && math.Abs(b.p-p.p) < 0.01) {
				out[len(out)-1] = p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}
