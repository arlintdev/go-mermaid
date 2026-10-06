package layout

import (
	"math"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

// token is one step of a layer read left to right with its cluster
// borders: a node, a cluster opening, or a cluster closing.
type token struct {
	n     *fnode
	cl    int
	close bool
}

// layerTokens lists layer r with an open and a close token around every run
// of a cluster's members.
func (f *flowGraph) layerTokens(r int) []token {
	var toks []token
	var stack []int
	for _, n := range f.layers[r] {
		common := len(lca(stack, n.path))
		for len(stack) > common {
			toks = append(toks, token{cl: stack[len(stack)-1], close: true})
			stack = stack[:len(stack)-1]
		}
		for i := common; i < len(n.path); i++ {
			toks = append(toks, token{cl: n.path[i]})
			stack = append(stack, n.path[i])
		}
		toks = append(toks, token{n: n, cl: -1})
	}
	for len(stack) > 0 {
		toks = append(toks, token{cl: stack[len(stack)-1], close: true})
		stack = stack[:len(stack)-1]
	}
	return toks
}

func (f *flowGraph) sepOf(n *fnode) float64 {
	switch n.sepKind {
	case sepReal:
		return f.opts.NodeSep
	case sepLabel:
		return f.opts.NodeSep / 2
	}
	return edgeSep
}

// titleOnCross reports whether subgraph titles sit on the cross axis's
// leading side (left-to-right charts keep titles on top, which is the cross
// axis there).
func (f *flowGraph) titleOnCross() bool { return !f.vertical }

// placeCross sets every node's cross coordinate.
//
// First, chains of one-to-one links are aligned into rigid blocks, so a
// plain edge between two nodes, and every long edge, runs straight; a node
// with several links on one side is left free so it can be centred on them.
// Alignments never cross and never pull a node outside a cluster into a
// layer the cluster spans.
//
// Then each block is a variable pulled toward its neighbours, each cluster
// has a variable for each border, and the order of every layer becomes
// separation constraints between consecutive tokens. A few rounds of "move
// toward the neighbours, then satisfy the constraints" settle parents over
// children, while no two boxes, and no box and a node not in it, can
// overlap.
func (f *flowGraph) placeCross() {
	blockOf := f.alignBlocks()
	s := &vpsc{}
	bv := map[*fnode]*vvar{} // block root -> variable
	for _, layer := range f.layers {
		x := 0.0
		for _, n := range layer {
			x += n.cs / 2
			root := blockOf[n]
			if bv[root] == nil {
				bv[root] = s.addVar(x, 1)
			}
			x += n.cs/2 + f.sepOf(n)
		}
	}
	varOf := func(n *fnode) *vvar { return bv[blockOf[n]] }
	lo := make([]*vvar, len(f.clusters))
	hi := make([]*vvar, len(f.clusters))
	for i, c := range f.clusters {
		if !c.used {
			continue
		}
		lo[i] = s.addVar(0, 1e-3)
		hi[i] = s.addVar(0, 1e-3)
		minW := 2 * clusterPad
		if !f.titleOnCross() {
			minW = math.Max(minW, c.titleW)
		}
		s.constrain(lo[i], hi[i], minW)
	}
	titleRoom := func(cl int) float64 {
		if f.titleOnCross() {
			return f.clusters[cl].titleH
		}
		return 0
	}
	for r := range f.layers {
		toks := f.layerTokens(r)
		for i := 1; i < len(toks); i++ {
			a, b := toks[i-1], toks[i]
			switch {
			case a.n != nil && b.n != nil:
				s.constrain(varOf(a.n), varOf(b.n), (a.n.cs+b.n.cs)/2+(f.sepOf(a.n)+f.sepOf(b.n))/2)
			case a.n != nil && !b.close:
				s.constrain(varOf(a.n), lo[b.cl], a.n.cs/2+f.outerGap(a.n))
			case a.n != nil && b.close:
				s.constrain(varOf(a.n), hi[b.cl], a.n.cs/2+f.innerGap(a.n))
			case !a.close && b.n != nil:
				s.constrain(lo[a.cl], varOf(b.n), b.n.cs/2+f.innerGap(b.n)+titleRoom(a.cl))
			case a.close && b.n != nil:
				s.constrain(hi[a.cl], varOf(b.n), b.n.cs/2+f.outerGap(b.n))
			case !a.close && !b.close:
				s.constrain(lo[a.cl], lo[b.cl], clusterPad+titleRoom(a.cl))
			case a.close && b.close:
				s.constrain(hi[a.cl], hi[b.cl], clusterPad)
			case a.close && !b.close:
				s.constrain(hi[a.cl], lo[b.cl], clusterOuter)
			}
		}
	}
	s.prepare()
	s.solve()
	// Links from each block to the blocks it neighbours.
	type link struct {
		v *vvar
		w float64
	}
	links := map[*vvar][]link{}
	for _, n := range f.nodes {
		v := varOf(n)
		for _, a := range append(append([]nbr(nil), n.ups...), n.downs...) {
			if u := varOf(a.n); u != v {
				links[v] = append(links[v], link{u, a.w})
			}
		}
	}
	for iter := 0; iter < 80; iter++ {
		for _, v := range s.vars {
			ls := links[v]
			if len(ls) == 0 {
				v.desired, v.weight = v.x, 1e-3
				continue
			}
			var sum, w float64
			for _, l := range ls {
				sum += l.v.x * l.w
				w += l.w
			}
			// Half a step toward the neighbours: a full step makes two
			// groups that pull on each other swap places forever.
			v.desired, v.weight = (v.x+sum/w)/2, w
		}
		s.solve()
	}
	minX := math.Inf(1)
	for _, n := range f.nodes {
		n.x = varOf(n).x
		minX = math.Min(minX, n.x-n.cs/2)
	}
	for i, c := range f.clusters {
		if lo[i] != nil {
			c.lo, c.hi = lo[i].x, hi[i].x
			minX = math.Min(minX, c.lo)
		}
	}
	for _, n := range f.nodes {
		n.x -= minX
	}
	for _, c := range f.clusters {
		c.lo -= minX
		c.hi -= minX
	}
}

// alignBlocks joins nodes into vertical blocks and returns each node's block
// root. A link joins its ends when it is the only link on that side of each
// real end (dummies always qualify), it crosses no link already joined, and
// no cluster border would have to pass through the block.
func (f *flowGraph) alignBlocks() map[*fnode]*fnode {
	type cand struct {
		u, v     *fnode // u on layer r, v on layer r+1
		priority int
	}
	var cands []cand
	for _, v := range f.nodes {
		for _, a := range v.ups {
			u := a.n
			if u.rank+1 != v.rank {
				continue
			}
			if u.real != nil && len(u.downs) != 1 || v.real != nil && len(v.ups) != 1 {
				continue
			}
			if !f.alignable(u, v) {
				continue
			}
			p := 2
			if u.real == nil && v.real == nil {
				p = 0
			} else if u.isLabel || v.isLabel {
				p = 1
			}
			cands = append(cands, cand{u, v, p})
		}
	}
	sortStable(len(cands), func(i, j int) bool { return cands[i].priority < cands[j].priority }, func(i, j int) {
		cands[i], cands[j] = cands[j], cands[i]
	})
	down := map[*fnode]*fnode{}
	up := map[*fnode]*fnode{}
	accepted := map[int][][2]int{}
	for _, c := range cands {
		if down[c.u] != nil || up[c.v] != nil {
			continue
		}
		crosses := false
		for _, a := range accepted[c.u.rank] {
			if (a[0]-c.u.order)*(a[1]-c.v.order) < 0 {
				crosses = true
				break
			}
		}
		if crosses {
			continue
		}
		accepted[c.u.rank] = append(accepted[c.u.rank], [2]int{c.u.order, c.v.order})
		down[c.u], up[c.v] = c.v, c.u
	}
	root := map[*fnode]*fnode{}
	for _, layer := range f.layers {
		for _, n := range layer {
			if u := up[n]; u != nil {
				root[n] = root[u]
			} else {
				root[n] = n
			}
		}
	}
	return root
}

// alignable reports whether u (one layer up) and v may share an x: neither
// may sit, outside a cluster, on a layer that cluster spans while the other
// is inside it.
func (f *flowGraph) alignable(u, v *fnode) bool {
	for _, c := range v.path {
		if !containsInt(u.path, c) && f.clusters[c].minRank <= u.rank {
			return false
		}
	}
	for _, c := range u.path {
		if !containsInt(v.path, c) && f.clusters[c].maxRank >= v.rank {
			return false
		}
	}
	return true
}

func sortStable(n int, less func(i, j int) bool, swap func(i, j int)) {
	for i := 1; i < n; i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			swap(j, j-1)
		}
	}
}

// innerGap is the room between a cluster border and a member next to it.
func (f *flowGraph) innerGap(n *fnode) float64 {
	if n.sepKind == sepDummy {
		return clusterPad / 2
	}
	return clusterPad
}

// outerGap is the room between a cluster border and an outside node next to
// it.
func (f *flowGraph) outerGap(n *fnode) float64 {
	if n.sepKind == sepDummy {
		return edgeSep / 2
	}
	return clusterOuter
}

// placePrimary stacks the layers. Each layer is as deep as its deepest node
// and nodes are centred in it. The gap between two layers holds, in order,
// the bottom padding of the clusters that end above it, a clear band where
// edges may turn (at least half the rank separation, more when many edges
// turn there), and the title and top padding of the clusters that start
// below it.
func (f *flowGraph) placePrimary() {
	nl := len(f.layers)
	f.bandH = make([]float64, nl)
	for r, layer := range f.layers {
		for _, n := range layer {
			f.bandH[r] = math.Max(f.bandH[r], n.ps)
		}
	}
	// Extra room at the top (start) and bottom (end) of each layer for
	// cluster borders: nested clusters that start on the same layer stack
	// their headers, siblings share the room.
	startRoom := make([]float64, nl)
	endRoom := make([]float64, nl)
	headOff := make([]float64, len(f.clusters))
	tailOff := make([]float64, len(f.clusters))
	for i, c := range f.clusters {
		if !c.used {
			continue
		}
		headOff[i] = f.stackOffset(i, true)
		tailOff[i] = f.stackOffset(i, false)
		startRoom[c.minRank] = math.Max(startRoom[c.minRank], headOff[i])
		endRoom[c.maxRank] = math.Max(endRoom[c.maxRank], tailOff[i])
	}
	// Left-to-right charts: a title wider than its cluster's ranks widens
	// the gaps on either side.
	if !f.vertical {
		f.widenForTitles(startRoom, endRoom)
	}
	tracks := f.countTracks()
	f.trackCount = tracks
	f.bandTop = make([]float64, nl)
	f.trackTop = make([]float64, nl)
	f.trackBottom = make([]float64, nl)
	p := startRoom[0]
	for r := 0; r < nl; r++ {
		f.bandTop[r] = p
		p += f.bandH[r]
		if r == nl-1 {
			break
		}
		p += endRoom[r]
		clear := f.opts.RankSep / 2
		if need := float64(tracks[r]+1) * trackGap; need > clear {
			clear = need
		}
		f.trackTop[r] = p
		p += clear
		f.trackBottom[r] = p
		p += startRoom[r+1]
	}
	// Nodes line up on the side of their layer the edges come from, as in
	// mermaid.js, so a row of boxes shares its top edge; labels and dummies
	// sit in the middle.
	for r, layer := range f.layers {
		for _, n := range layer {
			n.p = f.bandTop[r] + f.bandH[r]/2
			if n.real != nil {
				n.p = f.bandTop[r] + n.ps/2
			}
		}
	}
	for i, c := range f.clusters {
		if !c.used {
			continue
		}
		c.top = f.bandTop[c.minRank] - headOff[i]
		c.bottom = f.bandTop[c.maxRank] + f.bandH[c.maxRank] + tailOff[i]
	}
}

// stackOffset is how far cluster i's border sits outside the layer it
// starts on (head) or ends on: its own padding and title, plus that of any
// child cluster starting or ending on the same layer, nested inside it.
func (f *flowGraph) stackOffset(i int, head bool) float64 {
	c := f.clusters[i]
	own := clusterPad
	if head && f.dir == domain.TopBottom || !head && f.dir == domain.BottomTop {
		own += c.titleH
	}
	inner := 0.0
	for _, ch := range c.children {
		cc := f.clusters[ch]
		if !cc.used {
			continue
		}
		if head && cc.minRank == c.minRank || !head && cc.maxRank == c.maxRank {
			inner = math.Max(inner, f.stackOffset(ch, head))
		}
	}
	return own + inner
}

// widenForTitles makes room along the rank axis for subgraph titles wider
// than the ranks the subgraph spans.
func (f *flowGraph) widenForTitles(startRoom, endRoom []float64) {
	for _, c := range f.clusters {
		if !c.used {
			continue
		}
		span := 0.0
		for r := c.minRank; r <= c.maxRank; r++ {
			span += f.bandH[r] + f.opts.RankSep/2
		}
		span += 2 * clusterPad
		if c.titleW > span {
			extra := (c.titleW - span) / 2
			startRoom[c.minRank] += extra
			endRoom[c.maxRank] += extra
		}
	}
}
