package layout

import (
	"github.com/arlintdev/go-mermaid/internal/domain"
)

// block is a subgraph laid out on its own and stood in for, in its parent's
// layout, by one node of its size.
type block struct {
	sg          *domain.Subgraph
	stand       *domain.Node
	nodes       []*domain.Node
	edges       []*domain.Edge
	subgraphs   []*domain.Subgraph
	parent      *domain.Subgraph
	titleH      float64
	origNodes   []*domain.Node
	origEdges   []*domain.Edge
	origSGs     []*domain.Subgraph
	parentNodes []string
}

// extractIsolated lays out every outermost subgraph whose direction differs
// from the flow around it and that no edge crosses, and replaces it in g by
// a stand-in node. Undo with restore, last first.
func extractIsolated(g *domain.Graph, opts Options) []*block {
	var out []*block
	for _, sg := range g.Subgraphs {
		if sg.Direction == "" || sg.Direction == effectiveDir(g, sg) {
			continue
		}
		if hasExtractedAncestor(g, sg, out) {
			continue
		}
		members, sgs := descendants(g, sg)
		if !isolated(g, members, sgs, sg) {
			continue
		}
		b := &block{sg: sg, origNodes: g.Nodes, origEdges: g.Edges, origSGs: g.Subgraphs}
		sub := &domain.Graph{Direction: sg.Direction}
		var keepN []*domain.Node
		for _, n := range g.Nodes {
			if members[n.ID] {
				sub.Nodes = append(sub.Nodes, n)
			} else {
				keepN = append(keepN, n)
			}
		}
		var keepE []*domain.Edge
		for _, e := range g.Edges {
			if inside(e.From, members, sgs) {
				sub.Edges = append(sub.Edges, e)
			} else {
				keepE = append(keepE, e)
			}
		}
		var keepS []*domain.Subgraph
		for _, s := range g.Subgraphs {
			if sgs[s.ID] && s != sg {
				sub.Subgraphs = append(sub.Subgraphs, s)
			} else if s != sg {
				keepS = append(keepS, s)
			}
		}
		sw, sh := flowLayout(sub, opts)
		if len(sg.TitleLines) > 0 {
			b.titleH = opts.lineHeight()*float64(len(sg.TitleLines)) + 2*clusterTitlePadY
		}
		titleW := opts.face().LinesWidth(sg.TitleLines, opts.FontSize) + 2*clusterPad
		b.stand = &domain.Node{
			ID:    "\x00block:" + sg.ID,
			Shape: domain.ShapeText,
			Size:  domain.Size{W: max(sw+2*clusterPad, titleW), H: sh + 2*clusterPad + b.titleH},
		}
		b.nodes, b.edges, b.subgraphs = sub.Nodes, sub.Edges, sub.Subgraphs
		g.Nodes = append(keepN, b.stand)
		g.Edges = keepE
		g.Subgraphs = keepS
		if p := g.SubgraphByID(sg.Parent); p != nil {
			b.parent = p
			b.parentNodes = p.NodeIDs
			p.NodeIDs = append(append([]string(nil), p.NodeIDs...), b.stand.ID)
		}
		out = append(out, b)
	}
	return out
}

// restore puts the block's contents back into g, moved to where its
// stand-in was placed, and gives the subgraph the stand-in's box.
func (b *block) restore(g *domain.Graph) {
	pos, size := b.stand.Pos, b.stand.Size
	inner := 0.0
	for _, n := range b.nodes {
		inner = max(inner, n.Pos.X+n.Size.W)
	}
	for _, s := range b.subgraphs {
		inner = max(inner, s.Box.Min.X+s.Box.Size.W)
	}
	for _, e := range b.edges {
		for _, p := range e.Points {
			inner = max(inner, p.X)
		}
	}
	dx := pos.X + (size.W-inner)/2
	dy := pos.Y + clusterPad + b.titleH
	for _, n := range b.nodes {
		n.Pos.X += dx
		n.Pos.Y += dy
	}
	for _, e := range b.edges {
		for i := range e.Points {
			e.Points[i].X += dx
			e.Points[i].Y += dy
		}
		e.LabelPos.X += dx
		e.LabelPos.Y += dy
	}
	for _, s := range b.subgraphs {
		s.Box.Min.X += dx
		s.Box.Min.Y += dy
		if s.TitleX != 0 {
			s.TitleX += dx
		}
	}
	b.sg.Box = domain.Rect{Min: pos, Size: size}
	b.sg.TitleX = 0
	g.Nodes, g.Edges, g.Subgraphs = b.origNodes, b.origEdges, b.origSGs
	if b.parent != nil {
		b.parent.NodeIDs = b.parentNodes
	}
}

// effectiveDir is the direction the flow around sg runs in.
func effectiveDir(g *domain.Graph, sg *domain.Subgraph) domain.Direction {
	for p := g.SubgraphByID(sg.Parent); p != nil; p = g.SubgraphByID(p.Parent) {
		if p.Direction != "" {
			return p.Direction
		}
		if p.Parent == "" {
			break
		}
	}
	return g.Direction
}

func hasExtractedAncestor(g *domain.Graph, sg *domain.Subgraph, done []*block) bool {
	for p := g.SubgraphByID(sg.Parent); p != nil; p = g.SubgraphByID(p.Parent) {
		for _, b := range done {
			if b.sg == p {
				return true
			}
		}
		if p.Parent == "" {
			break
		}
	}
	return false
}

// descendants returns the node ids inside sg at any depth, and the ids of
// sg and every subgraph inside it.
func descendants(g *domain.Graph, sg *domain.Subgraph) (map[string]bool, map[string]bool) {
	nodes, sgs := map[string]bool{}, map[string]bool{sg.ID: true}
	for changed := true; changed; {
		changed = false
		for _, s := range g.Subgraphs {
			if !sgs[s.ID] && sgs[s.Parent] {
				sgs[s.ID] = true
				changed = true
			}
		}
	}
	for _, s := range g.Subgraphs {
		if sgs[s.ID] {
			for _, id := range s.NodeIDs {
				nodes[id] = true
			}
		}
	}
	return nodes, sgs
}

func inside(id string, nodes, sgs map[string]bool) bool { return nodes[id] || sgs[id] }

// isolated reports whether no edge joins the inside of sg to the outside,
// and no edge names sg itself.
func isolated(g *domain.Graph, nodes, sgs map[string]bool, sg *domain.Subgraph) bool {
	for _, e := range g.Edges {
		if e.From == sg.ID || e.To == sg.ID {
			return false
		}
		if inside(e.From, nodes, sgs) != inside(e.To, nodes, sgs) {
			return false
		}
	}
	return len(nodes) > 0
}
