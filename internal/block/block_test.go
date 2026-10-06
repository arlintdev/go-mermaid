package block

import (
	"testing"
)

func TestParseFlow(t *testing.T) {
	d, err := Parse("block-beta\ncolumns 3\na b c\nd[\"wide block\"]:2 e")
	if err != nil {
		t.Fatal(err)
	}
	if d.Columns() != 3 || len(d.Root.Children) != 5 {
		t.Fatalf("columns %d, children %d", d.Columns(), len(d.Root.Children))
	}
	wide := d.Root.Children[3]
	if wide.ID != "d" || wide.Label != "wide block" || wide.Span != 2 {
		t.Errorf("wide block: %+v", wide)
	}
	if _, err := Parse("columns 2"); err == nil {
		t.Error("source without a header must be refused")
	}
}

func TestParseEdges(t *testing.T) {
	d, err := Parse("block-beta\na[\"A\"] b((\"B\"))\na --> b\nb -- \"reads\" --> c\nc --- a\na <--> b\nb ==> c\nc -.-> a\na-->|lbl|b")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.Root.Children); n != 3 {
		t.Fatalf("edges must not add blocks already defined; got %d blocks", n)
	}
	if d.Block("b").Shape != ShapeCircle {
		t.Errorf("b should be a circle")
	}
	want := []Edge{
		{From: "a", To: "b", ArrowEnd: true},
		{From: "b", To: "c", Label: "reads", ArrowEnd: true},
		{From: "c", To: "a"},
		{From: "a", To: "b", ArrowEnd: true, ArrowBack: true},
		{From: "b", To: "c", ArrowEnd: true, Thick: true},
		{From: "c", To: "a", ArrowEnd: true, Dotted: true},
		{From: "a", To: "b", ArrowEnd: true, Label: "lbl"},
	}
	if len(d.Edges) != len(want) {
		t.Fatalf("got %d edges: %+v", len(d.Edges), d.Edges)
	}
	for i, e := range want {
		if d.Edges[i] != e {
			t.Errorf("edge %d: got %+v, want %+v", i, d.Edges[i], e)
		}
	}
}

func TestParseShapesSpacesComposites(t *testing.T) {
	src := "block-beta\ncolumns 3\na space:2\nb[(\"DB\")] c{{\"hex\"}} d>\"flag\"]\n" +
		"block:grp:2\n  columns 2\n  e f\nend\narr<[\"go\"]>(down)\ng[/\"p\"/] h[\\\"q\"/] i([\"s\"]) j[[\"sub\"]] k{\"r\"} l(((\"dc\")))"
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	sp := d.Root.Children[1]
	if !sp.Space || sp.Span != 2 {
		t.Errorf("space:2: %+v", sp)
	}
	shapes := map[string]Shape{"b": ShapeCylinder, "c": ShapeHexagon, "d": ShapeAsymmetric, "g": ShapeParallelogram,
		"h": ShapeTrapezoidAlt, "i": ShapeStadium, "j": ShapeSubroutine, "k": ShapeRhombus, "l": ShapeDoubleCircle, "arr": ShapeArrow}
	for id, s := range shapes {
		if b := d.Block(id); b == nil || b.Shape != s {
			t.Errorf("%s: got %+v, want shape %d", id, b, s)
		}
	}
	if d.Block("arr").ArrowDir != "down" || d.Block("arr").Label != "go" {
		t.Errorf("block arrow: %+v", d.Block("arr"))
	}
	g := d.Block("grp")
	if g == nil || !g.Composite || g.Span != 2 || g.Columns != 2 || len(g.Children) != 2 {
		t.Errorf("composite: %+v", g)
	}
	if _, err := Parse("block-beta\nend"); err == nil {
		t.Error("a stray end must be refused")
	}
}

func TestStyles(t *testing.T) {
	d, err := Parse("block-beta\na b\nstyle a fill:#f9f,stroke:#333,stroke-width:4px,color:url(#x)\nclassDef hot fill:red,stroke-dasharray:5 5\nclass b hot")
	if err != nil {
		t.Fatal(err)
	}
	a := d.Block("a").Style
	if a.Fill != "#f9f" || a.Stroke != "#333" || a.StrokeWidth != "4" || a.Color != "" {
		t.Errorf("style a: %+v", a)
	}
	if c := d.Classes["hot"]; c.Fill != "red" || c.Dash != "5 5" {
		t.Errorf("classDef hot: %+v", c)
	}
	if got := d.Block("b").Classes; len(got) != 1 || got[0] != "hot" {
		t.Errorf("class b: %v", got)
	}
}
