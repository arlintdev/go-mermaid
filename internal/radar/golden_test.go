package radar

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
	goldentest.Hostile(t, render, "radar-beta\ntitle "+inj+"\naxis a[\""+inj+"\"], "+inj+"\ncurve c[\""+inj+"\"]{1, 2}\ngraticule "+inj+"\nticks "+inj)
	o.FontFace = inj
	goldentest.Hostile(t, render, "radar-beta\naxis a, b, c\ncurve x{1,2,3}")
}

func TestOptionsAndKeyedCurves(t *testing.T) {
	d, err := Parse("radar-beta\naxis m[\"Math\"], s[\"Science\"]\naxis e\ncurve a[\"A\"]{ e: 3, m: 1 }\ncurve b{4, 5, 6}\nmax 10\nmin 1\ngraticule polygon\nticks 3\nshowLegend false")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Axes) != 3 || d.Axes[0] != "Math" || d.Axes[2] != "e" {
		t.Errorf("axes %v", d.Axes)
	}
	if v := d.Curves[0].Values; len(v) != 3 || v[0] != 1 || v[1] != 0 || v[2] != 3 {
		t.Errorf("keyed values %v", v)
	}
	if d.Max() != 10 || d.Min() != 1 || !d.Polygon || d.Ticks != 3 || !d.HideLegend {
		t.Errorf("options %+v", d)
	}
	for _, src := range []string{
		"radar-beta\naxis a\ncurve x{NaN}",
		"radar-beta\naxis a\ncurve x{ z: 1 }",
		"radar-beta\naxis a\ncurve x{1}\nmax 0\nmin 5",
	} {
		if _, err := Parse(src); err == nil {
			t.Errorf("no error for %q", src)
		}
	}
}
