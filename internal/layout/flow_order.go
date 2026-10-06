package layout

import (
	"math"
	"sort"
)

// endPath is the cluster path an edge end lives in: a node's own path, or
// for an end that names a subgraph, the path of the subgraph's parent, since
// the edge meets the subgraph from outside.
func (f *flowGraph) endPath(n *fnode, cl int) []int {
	if cl >= 0 {
		return f.clusterPath(f.clusters[cl].parent)
	}
	return n.path
}

// buildLayers buckets nodes by rank and threads every edge through a dummy
// node on each rank it crosses. A labelled edge's label takes the dummy on
// the half-rank nearest its middle. Each cluster gets a filler on any layer
// it spans but has nothing on, so its box is held open there.
func (f *flowGraph) buildLayers() {
	maxRank := 0
	for _, n := range f.nodes {
		maxRank = max(maxRank, n.rank)
	}
	for _, e := range f.edges {
		if e.self {
			e.chain = []*fnode{e.from}
			continue
		}
		fromCl, toCl := e.fromCl, e.toCl
		if e.reversed {
			fromCl, toCl = toCl, fromCl
		}
		path := lca(f.endPath(e.from, fromCl), f.endPath(e.to, toCl))
		cl := -1
		if len(path) > 0 {
			cl = path[len(path)-1]
		}
		labelRank := -1
		if e.e.Label != "" && !e.invisible {
			span := e.to.rank - e.from.rank
			mid := e.from.rank + span/2
			if mid%2 == 0 {
				mid--
			}
			labelRank = max(mid, e.from.rank+1)
		}
		chain := []*fnode{e.from}
		for r := e.from.rank + 1; r < e.to.rank; r++ {
			d := &fnode{edge: e, cluster: cl, path: path, rank: r, sepKind: sepDummy}
			if r == labelRank {
				d.isLabel, d.sepKind = true, sepLabel
				d.ps, d.cs = f.toRank(e.e.LabelSize)
				e.label = d
			}
			f.nodes = append(f.nodes, d)
			chain = append(chain, d)
		}
		chain = append(chain, e.to)
		e.chain = chain
		for i := 1; i < len(chain); i++ {
			a, b := chain[i-1], chain[i]
			w := 1.0
			switch {
			case a.real == nil && b.real == nil:
				w = 8
			case a.real == nil || b.real == nil:
				w = 2
			}
			a.downs = append(a.downs, nbr{b, w})
			b.ups = append(b.ups, nbr{a, w})
		}
	}
	f.reserveLoops()
	f.addFillers(maxRank)
	f.layers = make([][]*fnode, maxRank+1)
	for _, n := range f.nodes {
		f.layers[n.rank] = append(f.layers[n.rank], n)
	}
}

// Self-loop geometry: the first loop reaches loopOut beyond the node, each
// further one loopStep more, and labels sit past the outermost loop.
const (
	loopOut  = 18.0
	loopStep = 12.0
)

// reserveLoops makes room beside each node for its self-loops and their
// labels, so neither lands on a neighbour.
func (f *flowGraph) reserveLoops() {
	count := map[*fnode]int{}
	for _, e := range f.edges {
		if !e.self || e.invisible {
			continue
		}
		n := e.from
		e.loop = count[n]
		count[n]++
		n.loopRoom = math.Max(n.loopRoom, loopOut+loopStep*float64(e.loop)+2)
	}
	labelW := map[*fnode]float64{}
	labelH := map[*fnode]float64{}
	for _, e := range f.edges {
		if !e.self || e.invisible || e.e.Label == "" {
			continue
		}
		ps, cs := f.toRank(e.e.LabelSize)
		labelW[e.from] = math.Max(labelW[e.from], cs)
		labelH[e.from] += ps + 4
	}
	for n, w := range labelW {
		n.loopRoom += 6 + w
		n.loopPS = labelH[n]
	}
}

func (f *flowGraph) addFillers(maxRank int) {
	for i, c := range f.clusters {
		c.minRank, c.maxRank = 1<<30, -1
		present := map[int]bool{}
		for _, n := range f.nodes {
			if containsInt(n.path, i) {
				c.minRank = min(c.minRank, n.rank)
				c.maxRank = max(c.maxRank, n.rank)
				present[n.rank] = true
			}
		}
		c.used = c.maxRank >= 0
		if !c.used {
			continue
		}
		for r := c.minRank; r <= c.maxRank && r <= maxRank; r++ {
			if !present[r] {
				f.nodes = append(f.nodes, &fnode{cluster: i, path: f.clusterPath(i), rank: r, sepKind: sepDummy})
			}
		}
	}
}

// order arranges each layer to cut edge crossings: an initial depth-first
// order, then barycenter sweeps down and up, keeping the best order seen.
// Every sort keeps each subgraph's members together and keeps sibling
// subgraphs in one order on every layer, so their boxes can neither
// interleave nor overlap.
func (f *flowGraph) order() {
	f.initOrder()
	best := f.snapshot()
	bestCross := f.crossings()
	stale := 0
	for iter := 0; iter < 24 && bestCross > 0 && stale < 6; iter++ {
		down := iter%2 == 0
		keys := f.clusterKeys()
		if down {
			for r := 1; r < len(f.layers); r++ {
				f.sortLayer(r, true, keys)
			}
		} else {
			for r := len(f.layers) - 2; r >= 0; r-- {
				f.sortLayer(r, false, keys)
			}
		}
		f.transpose()
		if c := f.crossings(); c < bestCross {
			bestCross, best, stale = c, f.snapshot(), 0
		} else {
			stale++
		}
	}
	f.restore(best)
}

// initOrder places nodes in the order a depth-first walk from the sources,
// in source order, first reaches them.
func (f *flowGraph) initOrder() {
	visited := map[*fnode]bool{}
	next := make([]int, len(f.layers))
	var visit func(n *fnode)
	visit = func(n *fnode) {
		if visited[n] {
			return
		}
		visited[n] = true
		n.order = next[n.rank]
		next[n.rank]++
		for _, d := range n.downs {
			visit(d.n)
		}
	}
	for _, n := range f.nodes {
		if len(n.ups) == 0 {
			visit(n)
		}
	}
	for _, n := range f.nodes {
		visit(n)
	}
	keys := f.clusterKeys()
	for r := range f.layers {
		layer := f.layers[r]
		sort.SliceStable(layer, func(i, j int) bool { return layer[i].order < layer[j].order })
		vals := map[*fnode]float64{}
		for _, n := range layer {
			vals[n] = float64(n.order)
		}
		f.arrange(r, vals, keys)
	}
}

// clusterKeys gives each cluster its mean relative position over every
// layer, so sibling clusters can be put in the same order on all layers.
func (f *flowGraph) clusterKeys() []float64 {
	sum := make([]float64, len(f.clusters))
	cnt := make([]float64, len(f.clusters))
	for _, layer := range f.layers {
		for _, n := range layer {
			rel := (float64(n.order) + 0.5) / float64(len(layer))
			for _, c := range n.path {
				sum[c] += rel
				cnt[c]++
			}
		}
	}
	for i := range sum {
		if cnt[i] > 0 {
			sum[i] /= cnt[i]
		}
	}
	return sum
}

// sortLayer reorders layer r by the barycenter of each node's neighbours on
// the layer above (down) or below (up).
func (f *flowGraph) sortLayer(r int, down bool, keys []float64) {
	layer := f.layers[r]
	vals := map[*fnode]float64{}
	for _, n := range layer {
		adj := n.downs
		if down {
			adj = n.ups
		}
		if len(adj) == 0 {
			vals[n] = float64(n.order)
			continue
		}
		var s, w float64
		for _, a := range adj {
			s += float64(a.n.order) * a.w
			w += a.w
		}
		// Scale into this layer's index range so nodes without neighbours,
		// which keep their own index, compare fairly.
		other := f.layers[adj[0].n.rank]
		vals[n] = s / w * float64(len(layer)) / float64(max(len(other), 1))
	}
	f.arrange(r, vals, keys)
}

// arrange sorts layer r by vals, nesting clusters: at each level a cluster
// is one unit valued at its members' mean, and sibling clusters fill their
// slots in the order of keys.
func (f *flowGraph) arrange(r int, vals map[*fnode]float64, keys []float64) {
	out := f.arrangeLevel(f.layers[r], 0, vals, keys)
	for i, n := range out {
		n.order = i
	}
	f.layers[r] = out
}

func (f *flowGraph) arrangeLevel(items []*fnode, depth int, vals map[*fnode]float64, keys []float64) []*fnode {
	type unit struct {
		node    *fnode
		cl      int
		members []*fnode
		val     float64
	}
	var units []*unit
	byCl := map[int]*unit{}
	for _, n := range items {
		if len(n.path) <= depth {
			units = append(units, &unit{node: n, cl: -1, val: vals[n]})
			continue
		}
		c := n.path[depth]
		u, ok := byCl[c]
		if !ok {
			u = &unit{cl: c}
			byCl[c] = u
			units = append(units, u)
		}
		u.members = append(u.members, n)
		u.val += vals[n]
	}
	for _, u := range units {
		if u.cl >= 0 {
			u.val /= float64(len(u.members))
		}
	}
	sort.SliceStable(units, func(i, j int) bool { return units[i].val < units[j].val })
	// Sibling clusters take their slots in the global order.
	var slots []int
	var cls []*unit
	for i, u := range units {
		if u.cl >= 0 {
			slots = append(slots, i)
			cls = append(cls, u)
		}
	}
	sort.SliceStable(cls, func(i, j int) bool {
		if keys[cls[i].cl] != keys[cls[j].cl] {
			return keys[cls[i].cl] < keys[cls[j].cl]
		}
		return cls[i].cl < cls[j].cl
	})
	for k, s := range slots {
		units[s] = cls[k]
	}
	out := make([]*fnode, 0, len(items))
	for _, u := range units {
		if u.cl < 0 {
			out = append(out, u.node)
			continue
		}
		out = append(out, f.arrangeLevel(u.members, depth+1, vals, keys)...)
	}
	return out
}

// transpose swaps neighbours in a layer when that cuts crossings and keeps
// the cluster nesting intact.
func (f *flowGraph) transpose() {
	for pass := 0; pass < 4; pass++ {
		improved := false
		for r, layer := range f.layers {
			for i := 0; i+1 < len(layer); i++ {
				a, b := layer[i], layer[i+1]
				if !samePath(a.path, b.path) {
					continue
				}
				before := f.pairCrossings(r, a, b)
				after := f.pairCrossings(r, b, a)
				if after < before {
					layer[i], layer[i+1] = b, a
					a.order, b.order = b.order, a.order
					improved = true
				}
			}
		}
		if !improved {
			return
		}
	}
}

func samePath(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// pairCrossings counts crossings between the edges of u and v (u left of v)
// with both neighbouring layers.
func (f *flowGraph) pairCrossings(_ int, u, v *fnode) int {
	c := 0
	for _, adj := range [2]func(*fnode) []nbr{func(n *fnode) []nbr { return n.ups }, func(n *fnode) []nbr { return n.downs }} {
		for _, a := range adj(u) {
			for _, b := range adj(v) {
				if a.n.order > b.n.order {
					c++
				}
			}
		}
	}
	return c
}

// crossings counts edge crossings between every pair of adjacent layers.
func (f *flowGraph) crossings() int {
	total := 0
	for r := 0; r+1 < len(f.layers); r++ {
		type seg struct{ a, b int }
		var segs []seg
		for _, n := range f.layers[r] {
			for _, d := range n.downs {
				segs = append(segs, seg{n.order, d.n.order})
			}
		}
		for i := range segs {
			for j := i + 1; j < len(segs); j++ {
				if (segs[i].a-segs[j].a)*(segs[i].b-segs[j].b) < 0 {
					total++
				}
			}
		}
	}
	return total
}

func (f *flowGraph) snapshot() [][]*fnode {
	s := make([][]*fnode, len(f.layers))
	for i, l := range f.layers {
		s[i] = append([]*fnode(nil), l...)
	}
	return s
}

func (f *flowGraph) restore(s [][]*fnode) {
	f.layers = s
	for _, l := range f.layers {
		for i, n := range l {
			n.order = i
		}
	}
}
