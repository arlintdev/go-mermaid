package layout

import "sort"

// breakCycles reverses a set of edges so the graph is acyclic: a depth-first
// search in source order reverses every edge that closes a cycle. Self-loops
// are left out of the layered graph and routed on their own.
func (f *flowGraph) breakCycles() {
	out := map[*fnode][]*fedge{}
	for _, e := range f.edges {
		if !e.self {
			out[e.from] = append(out[e.from], e)
		}
	}
	const (
		white = iota
		grey
		black
	)
	state := map[*fnode]int{}
	var visit func(n *fnode)
	visit = func(n *fnode) {
		state[n] = grey
		for _, e := range out[n] {
			switch state[e.to] {
			case grey:
				e.from, e.to = e.to, e.from
				e.reversed = !e.reversed
			case white:
				visit(e.to)
			}
		}
		state[n] = black
	}
	for _, n := range f.nodes {
		if state[n] == white {
			visit(n)
		}
	}
}

// rank assigns each node a layer. Layers are doubled, as dagre does, so an
// edge label can take the layer between its ends: an edge of length k spans
// 2k layers. Ranks start from longest-path and are then tightened node by
// node toward the weighted median of their neighbours, which shortens edges
// the way network simplex would: a source sits just above what it feeds,
// not at the top of the picture.
func (f *flowGraph) rank() {
	type link struct {
		n      *fnode
		minLen int
	}
	preds := map[*fnode][]link{}
	succs := map[*fnode][]link{}
	indeg := map[*fnode]int{}
	for _, e := range f.edges {
		if e.self {
			continue
		}
		ml := 2 * e.minLen
		preds[e.to] = append(preds[e.to], link{e.from, ml})
		succs[e.from] = append(succs[e.from], link{e.to, ml})
		indeg[e.to]++
	}
	var topo []*fnode
	var queue []*fnode
	for _, n := range f.nodes {
		n.rank = 0
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		topo = append(topo, n)
		for _, s := range succs[n] {
			if r := n.rank + s.minLen; r > s.n.rank {
				s.n.rank = r
			}
			indeg[s.n]--
			if indeg[s.n] == 0 {
				queue = append(queue, s.n)
			}
		}
	}
	// Tighten: move each node within its feasible window to where its edges
	// are shortest. A node with more edges below than above sinks toward
	// them; ties go down, toward what the node feeds.
	for iter := 0; iter < 8; iter++ {
		moved := false
		for i := len(topo) - 1; i >= 0; i-- {
			n := topo[i]
			lo, hi := -1<<30, 1<<30
			var want []int
			for _, p := range preds[n] {
				lo = max(lo, p.n.rank+p.minLen)
				want = append(want, p.n.rank+p.minLen)
			}
			for _, s := range succs[n] {
				hi = min(hi, s.n.rank-s.minLen)
				want = append(want, s.n.rank-s.minLen)
			}
			if len(want) == 0 {
				continue
			}
			if lo == -1<<30 {
				lo = hi
			}
			if hi == 1<<30 {
				hi = lo
			}
			sort.Ints(want)
			// Any point between the two middle values minimises the sum of
			// edge lengths; take the lower one (nearer the successors).
			target := want[len(want)/2]
			target = max(lo, min(hi, target))
			if target != n.rank {
				n.rank = target
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	minRank := 1 << 30
	for _, n := range f.nodes {
		minRank = min(minRank, n.rank)
	}
	for _, n := range f.nodes {
		n.rank -= minRank
	}
}

// pickRepresentatives moves an edge that names a subgraph onto the member
// facing the other end: the lowest-ranked member for a source, the
// highest-ranked for a target, then ranks again.
func (f *flowGraph) pickRepresentatives() {
	changed := false
	for _, e := range f.edges {
		origFrom, origTo := &e.from, &e.to
		if e.reversed {
			origFrom, origTo = &e.to, &e.from
		}
		for _, end := range []struct {
			cl     int
			node   **fnode
			source bool
		}{{e.fromCl, origFrom, !e.reversed}, {e.toCl, origTo, e.reversed}} {
			if end.cl < 0 {
				continue
			}
			var best *fnode
			for _, n := range f.nodes {
				if !containsInt(n.path, end.cl) {
					continue
				}
				if best == nil || (end.source && n.rank > best.rank) || (!end.source && n.rank < best.rank) {
					best = n
				}
			}
			if best != nil && best != *end.node {
				*end.node = best
				changed = true
			}
		}
		e.self = e.from == e.to
	}
	if changed {
		f.breakCycles()
		f.rank()
	}
}
