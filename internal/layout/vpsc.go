package layout

// A small solver for placing variables on a line under separation
// constraints (left + gap <= right) as close as it can to where each
// variable wants to be, weighted. It is the "satisfy" pass of Dwyer,
// Marriott and Stuckey's VPSC: variables are taken in an order consistent
// with the constraints and merged into blocks that move as one whenever a
// constraint would be broken, each block sitting at the weighted mean of
// its members' wishes. A final push pass makes the result feasible even in
// the rare cases the merging leaves a constraint slightly broken.

type vvar struct {
	desired float64
	weight  float64
	offset  float64
	block   *vblock
	in, out []*vcon
	x       float64
}

type vcon struct {
	left, right *vvar
	gap         float64
}

type vblock struct {
	vars    []*vvar
	wsum    float64 // sum of weight * (desired - offset)
	weight  float64
	posn    float64
	mergeID int
}

type vpsc struct {
	vars []*vvar
	cons []*vcon
	// order is a topological order of vars under the constraints.
	order []*vvar
}

func (s *vpsc) addVar(desired, weight float64) *vvar {
	v := &vvar{desired: desired, weight: weight}
	s.vars = append(s.vars, v)
	return v
}

// constrain requires left + gap <= right. A pair constrained twice keeps the
// larger gap.
func (s *vpsc) constrain(left, right *vvar, gap float64) {
	if left == right {
		return
	}
	for _, c := range left.out {
		if c.right == right {
			if gap > c.gap {
				c.gap = gap
			}
			return
		}
	}
	c := &vcon{left: left, right: right, gap: gap}
	left.out = append(left.out, c)
	right.in = append(right.in, c)
	s.cons = append(s.cons, c)
}

// prepare computes a topological order of the variables. Constraints that
// would close a cycle are dropped; the layout never makes one, but the
// solver must not loop on bad input.
func (s *vpsc) prepare() {
	indeg := map[*vvar]int{}
	for _, c := range s.cons {
		indeg[c.right]++
	}
	var queue []*vvar
	for _, v := range s.vars {
		if indeg[v] == 0 {
			queue = append(queue, v)
		}
	}
	s.order = s.order[:0]
	done := map[*vvar]bool{}
	for len(queue) > 0 || len(s.order) < len(s.vars) {
		if len(queue) == 0 {
			// A cycle: take the first unplaced variable and drop its
			// incoming constraints from unplaced variables.
			for _, v := range s.vars {
				if !done[v] {
					var keep []*vcon
					for _, c := range v.in {
						if done[c.left] {
							keep = append(keep, c)
						}
					}
					v.in = keep
					queue = append(queue, v)
					break
				}
			}
		}
		v := queue[0]
		queue = queue[1:]
		if done[v] {
			continue
		}
		done[v] = true
		s.order = append(s.order, v)
		for _, c := range v.out {
			indeg[c.right]--
			if indeg[c.right] == 0 && !done[c.right] {
				queue = append(queue, c.right)
			}
		}
	}
	// Keep only constraints whose left comes first.
	pos := map[*vvar]int{}
	for i, v := range s.order {
		pos[v] = i
	}
	for _, v := range s.vars {
		var in []*vcon
		for _, c := range v.in {
			if pos[c.left] < pos[v] {
				in = append(in, c)
			}
		}
		v.in = in
	}
}

// solve places every variable; read the result from vvar.x.
func (s *vpsc) solve() {
	for _, v := range s.order {
		b := &vblock{vars: []*vvar{v}, weight: v.weight, wsum: v.weight * v.desired}
		v.offset = 0
		v.block = b
		b.posn = v.desired
		s.mergeLeft(b)
	}
	for _, v := range s.order {
		v.x = v.block.posn + v.offset
	}
	// Push pass: guarantees every constraint holds.
	for _, v := range s.order {
		for _, c := range v.in {
			if need := c.left.x + c.gap; v.x < need-1e-9 {
				v.x = need
			}
		}
	}
}

func (s *vpsc) mergeLeft(b *vblock) {
	for {
		var worst *vcon
		viol := 1e-9
		for _, v := range b.vars {
			for _, c := range v.in {
				if c.left.block == b {
					continue
				}
				lx := c.left.block.posn + c.left.offset
				rx := b.posn + v.offset
				if d := lx + c.gap - rx; d > viol {
					viol, worst = d, c
				}
			}
		}
		if worst == nil {
			return
		}
		bl := worst.left.block
		// Shift b's offsets so the constraint is tight inside the merged
		// block, then fold b into bl.
		d := worst.left.offset + worst.gap - worst.right.offset
		for _, v := range b.vars {
			v.offset += d
			v.block = bl
			bl.vars = append(bl.vars, v)
		}
		bl.weight += b.weight
		bl.wsum += b.wsum - d*b.weight
		bl.posn = bl.wsum / bl.weight
		b = bl
	}
}
