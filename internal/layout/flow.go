package layout

import (
	"math"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

// Flow lays out a flowchart the way mermaid.js (dagre) does, and draws it
// tidier where it can:
//
//   - labels are wrapped near WrapWidth and every node is sized to its
//     wrapped text and shape;
//   - an edge label is a node of its own on a rank between the edge's ends,
//     so it takes room in the layout and never sits on a node or another
//     label;
//   - nodes are ranked so every edge is as short as it can be, ordered to
//     cut crossings while each subgraph stays one contiguous block, and
//     placed by solving for straight edges under separation constraints,
//     subgraph borders included, so a subgraph's box never covers a node
//     that is not in it;
//   - edges are routed orthogonally through the gaps between ranks, each
//     horizontal run on a track of its own, ending on the node's outline.
//
// It fills Node.Pos/Size/Lines, Edge.Points/LabelPos/LabelLines/LabelSize
// and Subgraph.Box/TitleLines.
func Flow(g *domain.Graph, opts Options) (*Result, error) {
	opts = opts.withFlowDefaults()
	measure(g, opts)
	f := newFlowGraph(g, opts, g.Direction)
	f.run()
	f.writeBack()
	w, h := f.normalize()
	return &Result{Graph: g, Width: w, Height: h}, nil
}

// Flow layout defaults, matching mermaid.js's flowchart look at 16 px.
const (
	defaultWrapWidth = 120.0 // node labels, as mermaid.js 12
	labelWrapWidth   = 200.0 // edge labels
	nodePadX         = 16.0  // text to rectangle edge, each side
	nodePadY         = 12.0
	labelPadX        = 4.0 // text to edge-label background, each side
	labelPadY        = 2.0
	clusterPad       = 18.0 // subgraph border to its content
	clusterOuter     = 18.0 // subgraph border to what is outside it
	clusterTitlePadY = 6.0  // above and below a subgraph title
	edgeSep          = 18.0 // between edges running side by side
	trackGap         = 10.0 // between parallel horizontal runs in a gap
	portGap          = 14.0 // between edges meeting one side of a node
)

func (o Options) withFlowDefaults() Options {
	if o.FontSize <= 0 {
		o.FontSize = 16
	}
	if o.NodeSep <= 0 {
		o.NodeSep = 50
	}
	if o.RankSep <= 0 {
		o.RankSep = 50
	}
	if o.WrapWidth == 0 {
		o.WrapWidth = defaultWrapWidth
	}
	return o
}

// lineHeight is the distance between the baselines of two label lines.
func (o Options) lineHeight() float64 { return o.FontSize * 1.5 }

// measure wraps every label and sizes every node, edge label and subgraph
// title.
func measure(g *domain.Graph, opts Options) {
	face, fs, lh := opts.face(), opts.FontSize, opts.lineHeight()
	for _, n := range g.Nodes {
		label := n.Label
		if label == "" && n.Shape != domain.ShapeSmallCircle && n.Shape != domain.ShapeFramedCircle {
			label = n.ID
		}
		wrapAt := opts.WrapWidth
		if wrapAt > 0 {
			wrapAt = face.WrapWidthFor(label, fs, lh, wrapAt, math.Max(wrapAt, 2*labelWrapWidth-100))
		}
		n.Lines = face.Wrap(label, fs, wrapAt)
		if label == "" {
			n.Lines = nil
		}
		tw := face.LinesWidth(n.Lines, fs)
		th := lh * float64(len(n.Lines))
		if boxLike(n.Shape) && opts.WrapWidth > 0 {
			// mermaid.js 12 gives every box the full wrapping width, so a
			// row of boxes lines up.
			tw = math.Max(tw, math.Min(wrapAt, opts.WrapWidth))
		}
		n.Size = shapeSize(n.Shape, tw, th)
	}
	for _, e := range g.Edges {
		e.LabelLines, e.LabelSize = nil, domain.Size{}
		if e.Label == "" {
			continue
		}
		e.LabelLines = face.Wrap(e.Label, fs, math.Max(opts.WrapWidth, labelWrapWidth))
		e.LabelSize = domain.Size{
			W: face.LinesWidth(e.LabelLines, fs) + 2*labelPadX,
			H: lh*float64(len(e.LabelLines)) + 2*labelPadY,
		}
	}
	for _, sg := range g.Subgraphs {
		sg.TitleLines = nil
		if sg.Title != "" {
			sg.TitleLines = face.Wrap(sg.Title, fs, math.Max(opts.WrapWidth*2.5, labelWrapWidth))
		}
	}
}

// boxLike reports whether a shape is a box that takes the full wrapping
// width, as opposed to a circle, a diamond or bare text.
func boxLike(s domain.Shape) bool {
	switch s {
	case domain.ShapeCircle, domain.ShapeDoubleCircle, domain.ShapeSmallCircle,
		domain.ShapeFramedCircle, domain.ShapeDiamond, domain.ShapeText:
		return false
	}
	return true
}

// shapeSize returns the outline size for text of the given block size.
func shapeSize(s domain.Shape, tw, th float64) domain.Size {
	w, h := tw+2*nodePadX, th+2*nodePadY
	switch s {
	case domain.ShapeCircle:
		d := math.Max(tw, th) + 2*nodePadY
		return domain.Size{W: d, H: d}
	case domain.ShapeDoubleCircle:
		d := math.Max(tw, th) + 2*nodePadY + 10
		return domain.Size{W: d, H: d}
	case domain.ShapeSmallCircle:
		return domain.Size{W: 14, H: 14}
	case domain.ShapeFramedCircle:
		return domain.Size{W: 22, H: 22}
	case domain.ShapeDiamond:
		// mermaid.js draws a square rhombus whose side holds the text.
		s := tw + th + 2*nodePadY + 6
		return domain.Size{W: s, H: s}
	case domain.ShapeHexagon:
		return domain.Size{W: w + h/2, H: h}
	case domain.ShapeStadium:
		return domain.Size{W: w + h/3, H: h}
	case domain.ShapeParallelogram, domain.ShapeParallelogramAlt,
		domain.ShapeTrapezoid, domain.ShapeTrapezoidAlt:
		return domain.Size{W: w + h*2/3, H: h}
	case domain.ShapeAsymmetric:
		return domain.Size{W: w + h/3, H: h}
	case domain.ShapeSubroutine:
		return domain.Size{W: w + 16, H: h}
	case domain.ShapeCylinder:
		ry := CylinderRY(w)
		return domain.Size{W: w, H: h + 2*ry}
	case domain.ShapeDocument:
		return domain.Size{W: w, H: h + h*0.15}
	case domain.ShapeText:
		return domain.Size{W: tw + 8, H: th + 8}
	}
	return domain.Size{W: w, H: h}
}

// CylinderRY is the half-height of a cylinder's top ellipse for a cylinder
// of width w, as mermaid.js draws it.
func CylinderRY(w float64) float64 {
	return w / 2 / (2.5 + w/50)
}

// --- the working graph ---

type fnode struct {
	real    *domain.Node
	edge    *fedge // the edge a dummy routes, nil for real nodes and fillers
	isLabel bool
	cluster int   // innermost cluster, -1 for none
	path    []int // clusters from outermost to innermost
	rank    int
	order   int
	ps, cs  float64 // size along the primary (rank) and cross axes
	x, p    float64 // centre on the cross and primary axes
	ups     []nbr
	downs   []nbr
	sepKind int // sepReal, sepLabel or sepDummy
}

type nbr struct {
	n *fnode
	w float64
}

const (
	sepReal = iota
	sepLabel
	sepDummy
)

type fedge struct {
	e        *domain.Edge
	from, to *fnode // as laid out, after cycle breaking
	// fromCl and toCl are the clusters the edge really starts or ends at,
	// when its source names a subgraph; -1 otherwise.
	fromCl, toCl int
	reversed     bool
	minLen       int
	chain        []*fnode
	label        *fnode
	self         bool
	invisible    bool
	// routed, in rank space
	rpts    []rpt
	labelAt rpt
}

type fcluster struct {
	sg       *domain.Subgraph
	parent   int
	depth    int
	children []int
	minRank  int
	maxRank  int
	titleW   float64
	titleH   float64
	used     bool
	// laid out
	lo, hi       float64 // cross extent
	top, bottom  float64 // primary extent
	layerMembers map[int]bool
}

type flowGraph struct {
	g        *domain.Graph
	opts     Options
	dir      domain.Direction
	vertical bool
	nodes    []*fnode
	byID     map[string]*fnode
	edges    []*fedge
	clusters []*fcluster
	clByID   map[string]int
	layers   [][]*fnode
	// primary geometry, per layer
	bandTop, bandH []float64
	// gaps between layer i and i+1: where tracks may run
	trackTop, trackBottom []float64
	trackCount            []int
	routes                map[*fedge]*route
}

func newFlowGraph(g *domain.Graph, opts Options, dir domain.Direction) *flowGraph {
	f := &flowGraph{
		g: g, opts: opts, dir: dir,
		vertical: dir == domain.TopBottom || dir == domain.BottomTop,
		byID:     map[string]*fnode{},
		clByID:   map[string]int{},
	}
	f.buildClusters()
	for _, n := range g.Nodes {
		fn := &fnode{real: n, cluster: -1}
		if c, ok := f.memberOf(n.ID); ok {
			fn.cluster = c
		}
		fn.path = f.clusterPath(fn.cluster)
		fn.ps, fn.cs = f.toRank(n.Size)
		f.nodes = append(f.nodes, fn)
		f.byID[n.ID] = fn
	}
	// An empty subgraph still draws its box: give it a filler to hold it.
	for i, c := range f.clusters {
		if !f.clusterHasNodes(i) {
			fn := &fnode{cluster: i, path: f.clusterPath(i), sepKind: sepDummy}
			fn.ps, fn.cs = f.toRank(domain.Size{W: math.Max(c.titleW, 40), H: 8})
			f.nodes = append(f.nodes, fn)
			f.byID["\x00cluster:"+c.sg.ID] = fn
		}
	}
	for _, e := range g.Edges {
		fe := &fedge{e: e, fromCl: -1, toCl: -1, minLen: max(e.MinLen, 1)}
		fe.invisible = e.Line == domain.LineInvisible
		from, fc := f.endpoint(e.From, true)
		to, tc := f.endpoint(e.To, false)
		if from == nil || to == nil {
			continue
		}
		fe.from, fe.to, fe.fromCl, fe.toCl = from, to, fc, tc
		fe.self = from == to
		f.edges = append(f.edges, fe)
	}
	return f
}

// toRank converts a screen size to (primary, cross) sizes.
func (f *flowGraph) toRank(s domain.Size) (ps, cs float64) {
	if f.vertical {
		return s.H, s.W
	}
	return s.W, s.H
}

func (f *flowGraph) buildClusters() {
	for _, sg := range f.g.Subgraphs {
		if _, dup := f.clByID[sg.ID]; dup {
			continue
		}
		f.clByID[sg.ID] = len(f.clusters)
		c := &fcluster{sg: sg, parent: -1}
		face, fs := f.opts.face(), f.opts.FontSize
		if len(sg.TitleLines) > 0 {
			c.titleW = face.LinesWidth(sg.TitleLines, fs) + 2*clusterPad
			c.titleH = f.opts.lineHeight()*float64(len(sg.TitleLines)) + 2*clusterTitlePadY
		}
		f.clusters = append(f.clusters, c)
	}
	for i, c := range f.clusters {
		if p, ok := f.clByID[c.sg.Parent]; ok && p != i && !f.isAncestor(i, p) {
			c.parent = p
			f.clusters[p].children = append(f.clusters[p].children, i)
		}
	}
	for i := range f.clusters {
		f.clusters[i].depth = len(f.clusterPath(i)) - 1
	}
}

// isAncestor reports whether a is an ancestor of (or equal to) b.
func (f *flowGraph) isAncestor(a, b int) bool {
	for seen := 0; b >= 0 && seen <= len(f.clusters); seen++ {
		if a == b {
			return true
		}
		b = f.clusters[b].parent
	}
	return false
}

func (f *flowGraph) memberOf(id string) (int, bool) {
	for i, c := range f.clusters {
		for _, m := range c.sg.NodeIDs {
			if m == id {
				return i, true
			}
		}
	}
	return -1, false
}

func (f *flowGraph) clusterHasNodes(c int) bool {
	for _, n := range f.nodes {
		for _, p := range n.path {
			if p == c {
				return true
			}
		}
	}
	return false
}

// clusterPath returns the clusters from the outermost down to c.
func (f *flowGraph) clusterPath(c int) []int {
	var path []int
	for seen := 0; c >= 0 && seen <= len(f.clusters); seen++ {
		path = append([]int{c}, path...)
		c = f.clusters[c].parent
	}
	return path
}

// endpoint resolves an edge end. A subgraph id stands for one of its
// members: the deepest-ranked member is chosen later for a source, the
// shallowest for a target (see pickRepresentatives), so the edge leaves or
// enters the box on the side facing the other end.
func (f *flowGraph) endpoint(id string, _ bool) (*fnode, int) {
	if n, ok := f.byID[id]; ok {
		return n, -1
	}
	c, ok := f.clByID[id]
	if !ok {
		return nil, -1
	}
	for _, n := range f.nodes {
		if containsInt(n.path, c) {
			return n, c
		}
	}
	return nil, -1
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// lca returns the deepest common cluster path of a and b.
func lca(a, b []int) []int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return a[:n]
}

// run performs ranking, ordering, placement and routing.
func (f *flowGraph) run() {
	f.breakCycles()
	f.rank()
	f.pickRepresentatives()
	f.buildLayers()
	f.order()
	f.placeCross()
	f.placePrimary()
	f.route()
}
