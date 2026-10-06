package layout

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/parser"
)

// The flowchart corpus and probes live with the renderer's golden files.
func corpus(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "render", "testdata", "*.mmd"))
	if err != nil || len(files) == 0 {
		t.Fatal("no corpus")
	}
	out := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		// Front matter is the root package's to read.
		if strings.HasPrefix(src, "---\n") {
			if i := strings.Index(src[4:], "\n---\n"); i >= 0 {
				src = src[4+i+5:]
			}
		}
		out[strings.TrimSuffix(filepath.Base(f), ".mmd")] = src
	}
	return out
}

type box struct{ x0, y0, x1, y1 float64 }

func nodeBox(n *domain.Node) box {
	return box{n.Pos.X, n.Pos.Y, n.Pos.X + n.Size.W, n.Pos.Y + n.Size.H}
}

func (a box) overlaps(b box, margin float64) bool {
	return a.x0 < b.x1-margin && b.x0 < a.x1-margin && a.y0 < b.y1-margin && b.y0 < a.y1-margin
}

func (a box) inside(b box) bool {
	return a.x0 >= b.x0-0.01 && a.y0 >= b.y0-0.01 && a.x1 <= b.x1+0.01 && a.y1 <= b.y1+0.01
}

// segmentCrosses reports whether the axis-aligned segment p-q passes through
// the interior of b, shrunk by margin.
func segmentCrosses(p, q domain.Point, b box, margin float64) bool {
	b = box{b.x0 + margin, b.y0 + margin, b.x1 - margin, b.y1 - margin}
	if b.x0 >= b.x1 || b.y0 >= b.y1 {
		return false
	}
	lx, hx := math.Min(p.X, q.X), math.Max(p.X, q.X)
	ly, hy := math.Min(p.Y, q.Y), math.Max(p.Y, q.Y)
	return lx < b.x1 && hx > b.x0 && ly < b.y1 && hy > b.y0
}

func labelBox(e *domain.Edge) box {
	s := e.LabelSize
	return box{e.LabelPos.X - s.W/2, e.LabelPos.Y - s.H/2, e.LabelPos.X + s.W/2, e.LabelPos.Y + s.H/2}
}

func TestFlowQuality(t *testing.T) {
	for name, src := range corpus(t) {
		t.Run(name, func(t *testing.T) {
			g, err := parser.Flowchart(src)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Flow(g, Options{FontSize: 16})
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range checkFlow(res) {
				t.Error(p)
			}
		})
	}
}

// checkFlow returns every way the layout breaks the rules a reader relies
// on.
func checkFlow(res *Result) []string {
	g := res.Graph
	var problems []string
	add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	member := map[string]map[string]bool{} // subgraph -> every node inside, at any depth
	for _, sg := range g.Subgraphs {
		member[sg.ID] = map[string]bool{}
	}
	for _, sg := range g.Subgraphs {
		for _, id := range sg.NodeIDs {
			for p := sg; p != nil; p = g.SubgraphByID(p.Parent) {
				member[p.ID][id] = true
				if p.Parent == "" {
					break
				}
			}
		}
	}
	within := func(r box) bool {
		return r.x0 >= -0.01 && r.y0 >= -0.01 && r.x1 <= res.Width+0.01 && r.y1 <= res.Height+0.01
	}
	for i, a := range g.Nodes {
		ba := nodeBox(a)
		if !within(ba) {
			add("node %s is off the canvas", a.ID)
		}
		for _, b := range g.Nodes[i+1:] {
			if ba.overlaps(nodeBox(b), 0.5) {
				add("nodes %s and %s overlap", a.ID, b.ID)
			}
		}
		for _, sg := range g.Subgraphs {
			cb := box{sg.Box.Min.X, sg.Box.Min.Y, sg.Box.Min.X + sg.Box.Size.W, sg.Box.Min.Y + sg.Box.Size.H}
			if member[sg.ID][a.ID] && !ba.inside(cb) {
				add("node %s sticks out of subgraph %s", a.ID, sg.ID)
			}
			if !member[sg.ID][a.ID] && ba.overlaps(cb, 0.5) {
				add("node %s lies on subgraph %s, which it is not in", a.ID, sg.ID)
			}
		}
	}
	for i, a := range g.Subgraphs {
		ab := box{a.Box.Min.X, a.Box.Min.Y, a.Box.Min.X + a.Box.Size.W, a.Box.Min.Y + a.Box.Size.H}
		for _, b := range g.Subgraphs[i+1:] {
			bb := box{b.Box.Min.X, b.Box.Min.Y, b.Box.Min.X + b.Box.Size.W, b.Box.Min.Y + b.Box.Size.H}
			nested := ab.inside(bb) || bb.inside(ab)
			if !nested && ab.overlaps(bb, 0.5) {
				add("subgraphs %s and %s overlap", a.ID, b.ID)
			}
		}
	}
	var labels []*domain.Edge
	for _, e := range g.Edges {
		if e.Line == domain.LineInvisible {
			continue
		}
		if len(e.Points) < 2 {
			add("edge %s->%s has no route", e.From, e.To)
			continue
		}
		for i := 1; i < len(e.Points); i++ {
			p, q := e.Points[i-1], e.Points[i]
			if math.Abs(p.X-q.X) > 0.01 && math.Abs(p.Y-q.Y) > 0.01 {
				add("edge %s->%s has a slanted segment %v-%v", e.From, e.To, p, q)
			}
			for _, n := range g.Nodes {
				if n.ID == e.From || n.ID == e.To {
					continue
				}
				if segmentCrosses(p, q, nodeBox(n), 1) {
					add("edge %s->%s runs through node %s", e.From, e.To, n.ID)
				}
			}
		}
		for _, end := range []struct {
			id string
			p  domain.Point
		}{{e.From, e.Points[0]}, {e.To, e.Points[len(e.Points)-1]}} {
			n := g.NodeByID(end.id)
			if n == nil {
				continue // a subgraph end
			}
			b := nodeBox(n)
			grown := box{b.x0 - 1, b.y0 - 1, b.x1 + 1, b.y1 + 1}
			if !(box{end.p.X, end.p.Y, end.p.X, end.p.Y}).inside(grown) {
				add("edge %s->%s does not reach %s", e.From, e.To, end.id)
			}
		}
		if e.Label != "" {
			labels = append(labels, e)
			if !within(labelBox(e)) {
				add("label of %s->%s is off the canvas", e.From, e.To)
			}
		}
	}
	for i, a := range labels {
		la := labelBox(a)
		for _, n := range g.Nodes {
			if la.overlaps(nodeBox(n), 0.5) {
				add("label %q lies on node %s", a.Label, n.ID)
			}
		}
		for _, b := range labels[i+1:] {
			if la.overlaps(labelBox(b), 0.5) {
				add("labels %q and %q overlap", a.Label, b.Label)
			}
		}
	}
	return problems
}

// TestFlowQualityRandom builds many small random flowcharts, with cycles,
// labels, nested subgraphs and every direction, and checks the same rules.
func TestFlowQualityRandom(t *testing.T) {
	seed := uint64(7)
	rnd := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int((seed >> 33) % uint64(n))
	}
	dirs := []string{"TD", "LR", "BT", "RL"}
	words := []string{"alpha", "beta gamma", "a much longer label that wraps", "x", "delta-epsilon"}
	iters := 300
	if os.Getenv("FLOW_RANDOM") != "" {
		iters = 3000
	}
	for iter := 0; iter < iters; iter++ {
		var b strings.Builder
		fmt.Fprintf(&b, "flowchart %s\n", dirs[rnd(len(dirs))])
		n := 3 + rnd(9)
		if iter%10 == 9 {
			n = 12 + rnd(20)
		}
		sgs := rnd(4)
		open := 0
		for i := 0; i < n; i++ {
			if open < sgs && rnd(3) == 0 {
				fmt.Fprintf(&b, "subgraph S%d_%d[Group %d]\n", iter, i, i)
				open++
			}
			fmt.Fprintf(&b, "N%d[%s %d]\n", i, words[rnd(len(words))], i)
			if open > 0 && rnd(3) == 0 {
				b.WriteString("end\n")
				open--
			}
		}
		for ; open > 0; open-- {
			b.WriteString("end\n")
		}
		links := []string{"-->", "---", "-.->", "==>", "<-->", "--o"}
		for e := 0; e < n+rnd(n); e++ {
			from, to := rnd(n), rnd(n)
			label := ""
			if rnd(3) == 0 {
				label = "|" + words[rnd(len(words))] + "|"
			}
			fmt.Fprintf(&b, "N%d %s%s N%d\n", from, links[rnd(len(links))], label, to)
		}
		src := b.String()
		g, err := parser.Flowchart(src)
		if err != nil {
			t.Fatalf("parse: %v\n%s", err, src)
		}
		res, err := Flow(g, Options{FontSize: 16})
		if err != nil {
			t.Fatal(err)
		}
		if ps := checkFlow(res); len(ps) > 0 {
			t.Errorf("%d problems, first: %s\n%s", len(ps), ps[0], src)
		}
	}
}

func TestFlowSubgraphDirection(t *testing.T) {
	g, err := parser.Flowchart("flowchart LR\n  A --> B\n  subgraph S[Steps]\n    direction TB\n    s1 --> s2 --> s3\n  end\n  subgraph T[Linked, so it follows the chart]\n    direction TB\n    t1 --> t2\n  end\n  B --> t1")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Flow(g, Options{FontSize: 16})
	if err != nil {
		t.Fatal(err)
	}
	if ps := checkFlow(res); len(ps) > 0 {
		t.Fatal(ps)
	}
	s1, s2, s3 := g.NodeByID("s1"), g.NodeByID("s2"), g.NodeByID("s3")
	if !(s1.Pos.X == s2.Pos.X && s2.Pos.X == s3.Pos.X && s1.Pos.Y < s2.Pos.Y && s2.Pos.Y < s3.Pos.Y) {
		t.Errorf("an unlinked subgraph's own direction was not kept: %v %v %v", s1.Pos, s2.Pos, s3.Pos)
	}
	t1, t2 := g.NodeByID("t1"), g.NodeByID("t2")
	if !(t1.Pos.Y == t2.Pos.Y && t1.Pos.X < t2.Pos.X) {
		t.Errorf("a linked subgraph should follow the chart's direction: %v %v", t1.Pos, t2.Pos)
	}
	box := g.SubgraphByID("S").Box
	for _, n := range []*domain.Node{s1, s2, s3} {
		if !nodeBox(n).inside(box2(box)) {
			t.Errorf("node %s outside its box", n.ID)
		}
	}
	if len(g.Nodes) != 7 || len(g.Subgraphs) != 2 {
		t.Errorf("the graph was not restored: %d nodes, %d subgraphs", len(g.Nodes), len(g.Subgraphs))
	}
}

func box2(r domain.Rect) box { return box{r.Min.X, r.Min.Y, r.Min.X + r.Size.W, r.Min.Y + r.Size.H} }
