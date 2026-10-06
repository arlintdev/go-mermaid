package parser

import (
	"testing"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

func mustParse(t *testing.T, src string) *domain.Graph {
	t.Helper()
	g, err := Flowchart(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return g
}

func TestShapesEverySyntax(t *testing.T) {
	cases := []struct {
		src   string
		shape domain.Shape
		label string
	}{
		{"A>Flag]", domain.ShapeAsymmetric, "Flag"},
		{"A(((Dbl)))", domain.ShapeDoubleCircle, "Dbl"},
		{"A((Circle))", domain.ShapeCircle, "Circle"},
		{"A[(DB)]", domain.ShapeCylinder, "DB"},
		{"A{{Hex}}", domain.ShapeHexagon, "Hex"},
		{"A[/Para/]", domain.ShapeParallelogram, "Para"},
		{"A[\\Alt\\]", domain.ShapeParallelogramAlt, "Alt"},
		{"A[/Trap\\]", domain.ShapeTrapezoid, "Trap"},
		{"A[\\Inv/]", domain.ShapeTrapezoidAlt, "Inv"},
		{"A{Decide}", domain.ShapeDiamond, "Decide"},
		{"A(Round)", domain.ShapeRound, "Round"},
		{"A([Stadium])", domain.ShapeStadium, "Stadium"},
		{"A[[Sub]]", domain.ShapeSubroutine, "Sub"},
		{`A["a ] b ) c"]`, domain.ShapeRect, "a ] b ) c"},
		{`A["/api/v1: JSON API"]`, domain.ShapeRect, "/api/v1: JSON API"},
		{`A[/api/v1: x]`, domain.ShapeRect, "/api/v1: x"},
		{`A@{ shape: cyl, label: "Store" }`, domain.ShapeCylinder, "Store"},
		{`A@{ shape: dbl-circ }`, domain.ShapeDoubleCircle, "A"},
		{`A@{ shape: lean-r, label: "In" }`, domain.ShapeParallelogram, "In"},
		{`A@{ shape: doc, label: "Doc" }`, domain.ShapeDocument, "Doc"},
		{"A@{ shape: stadium,\n  label: \"Two\nlines\" }", domain.ShapeStadium, "Two\nlines"},
	}
	for _, c := range cases {
		g := mustParse(t, "flowchart TD\n  "+c.src+" --> B")
		n := g.NodeByID("A")
		if n == nil || n.Shape != c.shape || n.Label != c.label {
			t.Errorf("%s: got %+v, want shape %s label %q", c.src, n, c.shape, c.label)
		}
		if len(g.Edges) != 1 {
			t.Errorf("%s: %d edges", c.src, len(g.Edges))
		}
	}
}

func TestLinkForms(t *testing.T) {
	cases := []struct {
		link       string
		start, end domain.Marker
		line       domain.Line
		minLen     int
		label      string
	}{
		{"-->", "", domain.MarkerArrow, domain.LineSolid, 1, ""},
		{"--->", "", domain.MarkerArrow, domain.LineSolid, 2, ""},
		{"---->", "", domain.MarkerArrow, domain.LineSolid, 3, ""},
		{"---", "", "", domain.LineSolid, 1, ""},
		{"----", "", "", domain.LineSolid, 2, ""},
		{"<-->", domain.MarkerArrow, domain.MarkerArrow, domain.LineSolid, 1, ""},
		{"--o", "", domain.MarkerCircle, domain.LineSolid, 1, ""},
		{"--x", "", domain.MarkerCross, domain.LineSolid, 1, ""},
		{"o--o", domain.MarkerCircle, domain.MarkerCircle, domain.LineSolid, 1, ""},
		{"x--x", domain.MarkerCross, domain.MarkerCross, domain.LineSolid, 1, ""},
		{"-.->", "", domain.MarkerArrow, domain.LineDotted, 1, ""},
		{"-..->", "", domain.MarkerArrow, domain.LineDotted, 2, ""},
		{"-.-", "", "", domain.LineDotted, 1, ""},
		{"==>", "", domain.MarkerArrow, domain.LineThick, 1, ""},
		{"===", "", "", domain.LineThick, 1, ""},
		{"~~~", "", "", domain.LineInvisible, 1, ""},
		{"-- label -->", "", domain.MarkerArrow, domain.LineSolid, 1, "label"},
		{`-- "a, b: c" -->`, "", domain.MarkerArrow, domain.LineSolid, 1, "a, b: c"},
		{"-- open label ---", "", "", domain.LineSolid, 1, "open label"},
		{"-. dotted, label .->", "", domain.MarkerArrow, domain.LineDotted, 1, "dotted, label"},
		{"== thick (label) ==>", "", domain.MarkerArrow, domain.LineThick, 1, "thick (label)"},
		{"-->|piped|", "", domain.MarkerArrow, domain.LineSolid, 1, "piped"},
		{`-->|"piped, quoted (parens)"|`, "", domain.MarkerArrow, domain.LineSolid, 1, "piped, quoted (parens)"},
		{"---|open|", "", "", domain.LineSolid, 1, "open"},
		{"e1@-->", "", domain.MarkerArrow, domain.LineSolid, 1, ""},
	}
	for _, c := range cases {
		g := mustParse(t, "flowchart TD\n  A "+c.link+" B")
		if len(g.Edges) != 1 || len(g.Nodes) != 2 {
			t.Errorf("%s: %d edges, %d nodes", c.link, len(g.Edges), len(g.Nodes))
			continue
		}
		e := g.Edges[0]
		if e.Start != c.start || e.End != c.end || e.Line != c.line || e.MinLen != c.minLen || e.Label != c.label {
			t.Errorf("%s: got start %q end %q line %q len %d label %q", c.link, e.Start, e.End, e.Line, e.MinLen, e.Label)
		}
	}
	g := mustParse(t, "flowchart TD\n  A e1@--> B\n  e1@{ animate: true }")
	if len(g.Nodes) != 2 || len(g.Edges) != 1 {
		t.Errorf("edge metadata made %d nodes, %d edges", len(g.Nodes), len(g.Edges))
	}
	g = mustParse(t, "graph TD;\n  A-->B;\n  A-->C;")
	if len(g.Edges) != 2 {
		t.Errorf("legacy semicolons: %d edges", len(g.Edges))
	}
}

func TestLabelText(t *testing.T) {
	cases := map[string]string{
		"A[\"`**Markdown** string with *emphasis*`\"]":       "Markdown string with emphasis",
		"A[\"Quotes &quot;inside&quot; and #amp; entity\"]":  `Quotes "inside" and & entity`,
		"A[\"one<br/>two\"]":                                 "one\ntwo",
		"A[\"one<b>bold</b> two\"]":                          "onebold two",
		"A[\"#35; and #quot;q#quot;\"]":                      `# and "q"`,
		"A[Ünïcödé — ✓ 日本語]":                                 "Ünïcödé — ✓ 日本語",
		"A[\"`line one\n  line two`\"]":                      "line one\nline two",
		"A[\"a < b and c > d\"]":                             "a < b and c > d",
		"A[\"<script>alert(1)</script>\"]":                   "alert(1)",
		"A[\"&lt;img src=x onerror=alert(1)&gt;\"]":          "<img src=x onerror=alert(1)>",
		"A[\"#lt;rect width=#quot;9999#quot;/#gt; plain\"]": `<rect width="9999"/> plain`,
	}
	for src, want := range cases {
		g := mustParse(t, "flowchart TD\n  "+src)
		if got := g.NodeByID("A").Label; got != want {
			t.Errorf("%s: label %q, want %q", src, got, want)
		}
	}
}

func TestSubgraphStructure(t *testing.T) {
	g := mustParse(t, `flowchart LR
  subgraph cloud[Cloud]
    direction TB
    subgraph web[Web tier]
      lb[Load balancer] --> app1[App 1]
    end
    subgraph data[Data tier]
      db[(Primary)]
    end
    app1 --> db
  end
  user((User)) --> lb
  data --> backup[Backups]`)
	if len(g.Subgraphs) != 3 {
		t.Fatalf("%d subgraphs", len(g.Subgraphs))
	}
	cloud, web, data := g.SubgraphByID("cloud"), g.SubgraphByID("web"), g.SubgraphByID("data")
	if cloud.Direction != domain.TopBottom || cloud.Title != "Cloud" {
		t.Errorf("cloud: %+v", cloud)
	}
	if web.Parent != "cloud" || data.Parent != "cloud" || cloud.Parent != "" {
		t.Errorf("parents: web %q data %q cloud %q", web.Parent, data.Parent, cloud.Parent)
	}
	if len(cloud.NodeIDs) != 0 {
		t.Errorf("cloud holds only subgraphs, got %v", cloud.NodeIDs)
	}
	if g.NodeByID("direction") != nil || g.NodeByID("TB") != nil {
		t.Error("direction was read as nodes")
	}
	if g.NodeByID("data") != nil {
		t.Error("a link to a subgraph made a node")
	}
	if e := g.Edges[len(g.Edges)-1]; e.From != "data" || e.To != "backup" {
		t.Errorf("subgraph link: %+v", e)
	}

	// A node written outside a subgraph and then named inside it joins the
	// subgraph, as in mermaid.js; nodes only named outside stay outside.
	g = mustParse(t, `flowchart TD
  U[Outside]
  A[First outside]
  subgraph S[Sub]
    A --> B
  end
  U --> A
  B --> E[After]`)
	s := g.Subgraphs[0]
	if len(s.NodeIDs) != 2 || s.NodeIDs[0] != "A" || s.NodeIDs[1] != "B" {
		t.Errorf("members %v", s.NodeIDs)
	}

	g = mustParse(t, "flowchart TD\n  subgraph \"Title with spaces\"\n    A\n  end\n  subgraph id2 [\"Quoted [title]\"]\n    B\n  end")
	if g.Subgraphs[0].Title != "Title with spaces" || g.Subgraphs[1].Title != "Quoted [title]" || g.Subgraphs[1].ID != "id2" {
		t.Errorf("titles: %+v %+v", g.Subgraphs[0], g.Subgraphs[1])
	}
}

func TestUnicodeIDs(t *testing.T) {
	g := mustParse(t, "flowchart TD\n  Ä[Ü] --> 日[本]")
	if g.NodeByID("Ä") == nil || g.NodeByID("日").Label != "本" {
		t.Errorf("nodes %+v", g.Nodes)
	}
}
