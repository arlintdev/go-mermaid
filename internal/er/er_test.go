package er

import (
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

var defaultOptions = RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16}

func render(src string) ([]byte, error) { return Render(src, defaultOptions) }

func TestGolden(t *testing.T) { goldentest.Golden(t, "testdata", render) }

func TestParseRelationsAndAttributes(t *testing.T) {
	d, err := Parse("erDiagram\nCUSTOMER ||--o{ ORDER : places\nCUSTOMER {\nstring name PK, FK \"the name\"\nstring email\n}\nA |o..|{ B : \"two words\"\np[Person] }o--|| c[\"A car\"] : owns")
	if err != nil {
		t.Fatal(err)
	}
	c := d.entity("CUSTOMER")
	a := c.Attributes[0]
	if len(c.Attributes) != 2 || a.Type != "string" || a.Name != "name" || strings.Join(a.Keys, ",") != "PK,FK" || a.Comment != "the name" {
		t.Errorf("attributes: %+v", c.Attributes)
	}
	r := d.Relationships
	if r[0].LeftKind != CardOne || r[0].RightKind != CardZeroMany || r[0].Dashed || r[0].Label != "places" {
		t.Errorf("r0: %+v", r[0])
	}
	if r[1].LeftKind != CardZeroOne || r[1].RightKind != CardOneMany || !r[1].Dashed || r[1].Label != "two words" {
		t.Errorf("r1: %+v", r[1])
	}
	if d.entity("p").Label() != "Person" || d.entity("c").Label() != "A car" || r[2].LeftKind != CardZeroMany || r[2].RightKind != CardOne {
		t.Errorf("aliases: %+v %+v %+v", d.entity("p"), d.entity("c"), r[2])
	}
	for _, bad := range []string{"CUSTOMER ||--o{ ORDER", "erDiagram\nA ||--x{ B : y", "erDiagram\nA maybe to B : x", "erDiagram\nA ||--o{ B C : x"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("expected an error for %q", bad)
		}
	}
}

func TestWordCardinalities(t *testing.T) {
	d, err := Parse("erDiagram\nA only one to zero or more B : x\nB one or more optionally to zero or one C : y\nC 1+ to 0+ D : z")
	if err != nil {
		t.Fatal(err)
	}
	want := [][3]any{{CardOne, CardZeroMany, false}, {CardOneMany, CardZeroOne, true}, {CardOneMany, CardZeroMany, false}}
	for i, w := range want {
		r := d.Relationships[i]
		if r.LeftKind != w[0] || r.RightKind != w[1] || r.Dashed != w[2] {
			t.Errorf("relation %d: %+v", i, r)
		}
	}
	if d.Relationships[2].To != "D" {
		t.Errorf("target: %+v", d.Relationships[2])
	}
}

func TestHostile(t *testing.T) {
	x := goldentest.Injection
	q := strings.ReplaceAll(x, `"`, "'")
	src := strings.Join([]string{
		"erDiagram",
		`A["` + q + `"] ||--o{ B : "` + q + `"`,
		"A {",
		"  " + q + " " + q + ` PK "` + q + `"`,
		"}",
		`"` + q + `" }|..|{ B : ` + x,
		"B ||--o{ B : " + x,
		"classDef c fill:" + x + ",stroke:" + x + ",color:" + x,
		"class A c",
		"style B fill:" + x + ",stroke-width:" + x,
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
