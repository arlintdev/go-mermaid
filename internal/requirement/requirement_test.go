package requirement

import (
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

var defaultOptions = RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16}

func render(src string) ([]byte, error) { return Render(src, defaultOptions) }

func TestGolden(t *testing.T) { goldentest.Golden(t, "testdata", render) }

func TestParse(t *testing.T) {
	d, err := Parse("requirementDiagram\ndirection LR\nrequirement test_req {\nid: 1\ntext: \"quoted: text\"\nrisk: High\nverifyMethod: test\n}\n" +
		"designConstraint \"c 1\" {\nid: 2\n}\nelement e {\ntype: sim\ndocRef: x\n}\ne - satisfies -> test_req\ntest_req <- contains - \"c 1\"\nclassDef k fill:#f00\nclass e k")
	if err != nil {
		t.Fatal(err)
	}
	r := d.node("test_req")
	if r.Kind != "requirement" || r.Fields["text"] != "quoted: text" || r.Fields["verifymethod"] != "test" {
		t.Errorf("requirement: %+v", r)
	}
	if c := d.node("c 1"); c == nil || c.Kind != "designconstraint" {
		t.Errorf("quoted name: %+v", c)
	}
	if d.Rels[0].From != "e" || d.Rels[0].To != "test_req" || d.Rels[0].Type != "satisfies" {
		t.Errorf("forward: %+v", d.Rels[0])
	}
	if d.Rels[1].From != "c 1" || d.Rels[1].To != "test_req" || d.Rels[1].Type != "contains" {
		t.Errorf("backward: %+v", d.Rels[1])
	}
	if d.Direction != "LR" || len(d.node("e").Classes) != 1 {
		t.Errorf("direction/class: %q %+v", d.Direction, d.node("e"))
	}
	got := fields(r)
	if len(got) != 4 || got[2] != [2]string{"Risk", "High"} || got[3] != [2]string{"Verification", "Test"} {
		t.Errorf("fields: %v", got)
	}
	for _, bad := range []string{"requirement x {", "requirementDiagram\nwidget w {\n}", "requirementDiagram\na b c"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("expected an error for %q", bad)
		}
	}
}

func TestHostile(t *testing.T) {
	x := goldentest.Injection
	q := strings.ReplaceAll(x, `"`, "'")
	src := strings.Join([]string{
		"requirementDiagram",
		"requirement r {",
		"  id: " + x,
		"  text: " + x,
		"  risk: " + x,
		"}",
		`element "` + q + `" {`,
		"  type: " + x,
		"  docref: " + x,
		"}",
		`"` + q + `" - satisfies -> r`,
		"classDef k fill:" + x + ",stroke:" + x + ",color:" + x,
		"class r k",
		"style r stroke-width:" + x,
	}, "\n")
	goldentest.Hostile(t, render, src)
	out, err := render(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), x) || strings.Contains(string(out), q) {
		t.Error("injection written unescaped")
	}
}
