package class

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
		"classDiagram",
		"class A {",
		"  <<" + strings.ReplaceAll(q, ">", "") + ">>",
		"  +" + x,
		"  +run(" + x + ") " + x,
		"}",
		`class B["` + q + `"]`,
		`A "` + q + `" --> "` + q + `" B : ` + x,
		"A : " + x,
		`note for A "` + q + `"`,
		`note "` + q + `"`,
		"namespace N {",
		"  class C",
		"}",
		"style A fill:" + x + ",stroke:" + x + ",stroke-width:" + x + ",color:" + x + ",stroke-dasharray:" + x,
		"classDef hot fill:" + x,
		"cssClass \"B\" hot",
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

func TestNewSyntax(t *testing.T) {
	d, err := Parse("classDiagram\nclass A\n<<interface>> A\nnote for A \"a note\"\nnote \"free\"\nA <|..|> B\nA ()-- C\nclass D[\"Shown\"]:::hot\nclassDef hot fill:#f00\nclick A href \"x\"")
	if err != nil {
		t.Fatal(err)
	}
	if d.class("A").Annotation != "interface" {
		t.Error("annotation line")
	}
	if len(d.Notes) != 2 || d.Notes[0].For != "A" || d.Notes[0].Text != "a note" || d.Notes[1].For != "" {
		t.Errorf("notes: %+v", d.Notes)
	}
	r := d.Relations[0]
	if r.Left != headTriangle || r.Right != headTriangle || !r.Dashed {
		t.Errorf("two-way realization: %+v", r)
	}
	if d.Relations[1].Left != headLollipop {
		t.Errorf("lollipop: %+v", d.Relations[1])
	}
	if c := d.class("D"); c.Label() != "Shown" || c.Classes[0] != "hot" || d.ClassDefs["hot"].Fill != "#f00" {
		t.Errorf("label and class: %+v", c)
	}
	if d.class("Duo") != nil {
		t.Error("unexpected class")
	}
	d, err = Parse("classDiagram\nDuo -- Trio")
	if err != nil || d.Relations[0].From != "Duo" || d.Relations[0].Left != headNone {
		t.Errorf("a name ending in o is not an aggregation: %+v %v", d.Relations, err)
	}
}

func TestMemberFormat(t *testing.T) {
	cases := []struct {
		in     string
		method bool
		want   member
	}{
		{"+makeSound()* void", true, member{text: "+makeSound() : void", italic: true}},
		{"+move(int distance) bool", true, member{text: "+move(int distance) : bool"}},
		{"+make(T seed)$ Shape~T~", true, member{text: "+make(T seed) : Shape<T>", underline: true}},
		{"+run()", true, member{text: "+run()"}},
		{"+count$", false, member{text: "+count", underline: true}},
		{"+List~List~int~~ points", false, member{text: "+List<List<int>> points"}},
		{"Map~K, V~ m", false, member{text: "Map<K, V> m"}},
	}
	for _, c := range cases {
		if got := formatMember(c.in, c.method); got != c.want {
			t.Errorf("%q: got %+v, want %+v", c.in, got, c.want)
		}
	}
}
