package mindmap

import (
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

func opts() RenderOptions {
	return RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16}
}

func TestGolden(t *testing.T) {
	goldentest.Golden(t, "testdata", func(src string) ([]byte, error) { return Render(src, opts()) })
}

func TestHostile(t *testing.T) {
	inj := goldentest.Injection
	o := opts()
	render := func(s string) ([]byte, error) { return Render(s, o) }
	goldentest.Hostile(t, render, "mindmap\n  root(("+inj+"))\n    "+inj+"\n    ::icon("+inj+")\n    a["+inj+"]:::"+inj+
		"\n    b))"+inj+"((\n    c)"+inj+"(\n    d{{"+inj+"}}\n    \""+inj+"\"")
	o.FontFace, o.Title = inj, inj
	goldentest.Hostile(t, render, "mindmap\n  r\n    a")
}

func TestShapes(t *testing.T) {
	d, err := Parse("mindmap\n  root((R))\n    a[Sq]\n    b(Rd)\n    c))Bang((\n    d)Cloud(\n    e{{Hex}}\n    f\n    g[\"Quoted\"]\n    h:::urgent")
	if err != nil {
		t.Fatal(err)
	}
	if d.Root.Shape != ShapeCircle || d.Root.Text != "R" {
		t.Errorf("root %+v", d.Root)
	}
	want := []struct {
		text  string
		shape Shape
	}{{"Sq", ShapeSquare}, {"Rd", ShapeRounded}, {"Bang", ShapeBang}, {"Cloud", ShapeCloud},
		{"Hex", ShapeHexagon}, {"f", ShapeDefault}, {"Quoted", ShapeSquare}, {"h", ShapeDefault}}
	for i, w := range want {
		c := d.Root.Children[i]
		if c.Text != w.text || c.Shape != w.shape {
			t.Errorf("child %d: %q %v, want %q %v", i, c.Text, c.Shape, w.text, w.shape)
		}
	}
}
