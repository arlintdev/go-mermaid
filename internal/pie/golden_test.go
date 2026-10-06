package pie

import (
	"strings"
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
	goldentest.Hostile(t, render, "pie showData title "+inj+"\n\""+inj+"\" : 3\n\"b\" : 0.01")
	goldentest.Hostile(t, render, "pie\ntitle "+inj+"\n"+inj+" : 3")
	o.Title = inj
	goldentest.Hostile(t, render, "pie\n\"a\" : 1")
}

func TestShowDataLegend(t *testing.T) {
	out, _ := Render("pie showData\n\"a\" : 12.5\n\"b\" : 2", opts())
	if !strings.Contains(string(out), ">a [12.5]<") {
		t.Errorf("legend lacks the value:\n%s", out)
	}
	out, _ = Render("pie\n\"a\" : 12.5\n\"b\" : 2", opts())
	if strings.Contains(string(out), "[12.5]") {
		t.Error("value shown without showData")
	}
}

// A thin slice's percentage goes outside the pie on a leader line.
func TestSmallSliceLabelOutside(t *testing.T) {
	out, _ := Render("pie\n\"big\" : 99\n\"tiny\" : 1", opts())
	if !strings.Contains(string(out), "<polyline") {
		t.Errorf("no leader line for the 1%% slice:\n%s", out)
	}
}

func TestRejectsNonFiniteValues(t *testing.T) {
	for _, v := range []string{"NaN", "Inf", "-1", "1e400"} {
		if _, err := Parse("pie\n\"a\" : " + v); err == nil {
			t.Errorf("accepted %s", v)
		}
	}
}
