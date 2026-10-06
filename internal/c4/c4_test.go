package c4

import (
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

var defaultOptions = RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16}

func render(src string) ([]byte, error) { return Render(src, defaultOptions) }

func TestGolden(t *testing.T) { goldentest.Golden(t, "testdata", render) }

func TestParse(t *testing.T) {
	d, err := Parse(`C4Container
title Containers
Person(custA, "Customer", "A bank customer")
System_Boundary(c1, "Banking") {
  Container(web, "Web App", "Java, Spring", "Serves pages")
  ContainerDb(db, "Database", "SQL")
  Boundary(b2, "Inner", "zone") {
    Component_Ext(x, "X", $techn="Go", $descr="does x")
  }
}
Rel(custA, web, "Uses", "HTTPS")
BiRel(web, db, "Reads")
Rel_Back(db, x, "Feeds")
UpdateElementStyle(custA, $bgColor="grey", $fontColor="url(#evil)")
UpdateRelStyle(custA, web, $lineColor="blue")
UpdateLayoutConfig($c4ShapeInRow="3")`)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Containers" || len(d.Elements) != 4 || len(d.Boundaries) != 2 || len(d.Rels) != 3 {
		t.Fatalf("parsed: %+v", d)
	}
	web := d.element("web")
	if web.Techn != "Java, Spring" || web.Descr != "Serves pages" {
		t.Errorf("container args: %+v", web)
	}
	if x := d.element("x"); x.Techn != "Go" || x.Descr != "does x" {
		t.Errorf("named args: %+v", x)
	}
	if b := d.boundary("b2"); b.Type != "zone" || b.parent != d.boundary("c1") || len(d.boundary("c1").Children) != 3 {
		t.Errorf("boundaries: %+v", b)
	}
	if st := d.element("custA").Style; st.Fill != "grey" || st.Text != "" {
		t.Errorf("element style must validate: %+v", st)
	}
	if d.Rels[0].Tech != "HTTPS" || d.Rels[0].Style.Stroke != "blue" || d.Rels[2].Kind != "Rel_Back" {
		t.Errorf("rels: %+v %+v", d.Rels[0], d.Rels[2])
	}
	for _, bad := range []string{"graph TD", "C4Context\nWidget(a, \"A\")", "C4Context\nRel(a)", "C4Context\nnot a call"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("expected an error for %q", bad)
		}
	}
}

func TestHostile(t *testing.T) {
	x := goldentest.Injection
	q := strings.ReplaceAll(x, `"`, "'")
	src := strings.Join([]string{
		"C4Context",
		"title " + x,
		`Person(a, "` + q + `", "` + q + `")`,
		`Enterprise_Boundary(b, "` + q + `") {`,
		`  Container(c, "` + q + `", "` + q + `", "` + q + `")`,
		"}",
		`Rel(a, c, "` + q + `", "` + q + `")`,
		`UpdateElementStyle(a, $bgColor="` + q + `", $fontColor="` + q + `", $borderColor="` + q + `")`,
		`UpdateRelStyle(a, c, $textColor="` + q + `", $lineColor="` + q + `")`,
	}, "\n")
	goldentest.Hostile(t, render, src)
	out, err := render(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), x) || strings.Contains(string(out), q) {
		t.Error("injection written unescaped")
	}
	f := defaultOptions
	f.FontFace = x
	goldentest.Hostile(t, func(s string) ([]byte, error) { return Render(s, f) }, src)
}
