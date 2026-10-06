package state

import (
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

var defaultOptions = RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16}

func render(src string) ([]byte, error) { return Render(src, defaultOptions) }

func TestGolden(t *testing.T) { goldentest.Golden(t, "testdata", render) }

func TestHostile(t *testing.T) {
	x := goldentest.Injection
	q := strings.ReplaceAll(x, `"`, "'")
	src := strings.Join([]string{
		"stateDiagram-v2",
		"classDef hot fill:" + x + ",stroke:" + x + ",color:" + x + ",stroke-width:" + x,
		"[*] --> A : " + x,
		"A : " + x,
		`state "` + q + `" as B`,
		"A --> B:::hot",
		"state C {",
		"  [*] --> D",
		"  --",
		"  [*] --> E : " + x,
		"}",
		"note right of A : " + x,
		"note left of B",
		x,
		"end note",
		"style A fill:" + x,
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

func TestRegionsAndClasses(t *testing.T) {
	d, err := Parse("stateDiagram-v2\nstate P {\n[*] --> A\n--\n[*] --> B\n}\nclassDef c fill:#f00,stroke:url(#x)\nclass A c\nB:::c\nA --> A")
	if err != nil {
		t.Fatal(err)
	}
	if d.composite("P").Regions != 2 || d.state("A").Region != 0 || d.state("B").Region != 1 {
		t.Errorf("regions: P=%d A=%d B=%d", d.composite("P").Regions, d.state("A").Region, d.state("B").Region)
	}
	if regionPseudoID("P", 0, false) == regionPseudoID("P", 1, false) || d.state(regionPseudoID("P", 1, false)) == nil {
		t.Error("each region needs its own start")
	}
	if cd := d.ClassDefs["c"]; cd.Fill != "#f00" || cd.Stroke != "" {
		t.Errorf("classDef: %+v", cd)
	}
	if len(d.state("A").Classes) != 1 || len(d.state("B").Classes) != 1 {
		t.Errorf("classes: A=%v B=%v", d.state("A").Classes, d.state("B").Classes)
	}
	out, err := render("stateDiagram-v2\nA --> A : again")
	if err != nil || !strings.Contains(string(out), ">again<") {
		t.Errorf("self transition: %v", err)
	}
}
