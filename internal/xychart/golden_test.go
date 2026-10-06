package xychart

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
	for _, head := range []string{"xychart-beta", "xychart-beta horizontal"} {
		goldentest.Hostile(t, render, head+"\ntitle "+inj+"\nx-axis "+inj+" ["+inj+", b]\ny-axis "+inj+"\nbar "+inj+" [1, 2]\nline [2, 1]")
		goldentest.Hostile(t, render, head+"\nx-axis \""+inj+"\" 0 --> 10\ny-axis \""+inj+"\" 0 --> 5\nline [1, 2]")
	}
	o.FontFace = inj
	goldentest.Hostile(t, render, "xychart-beta\nbar [1]")
}

func TestAxisForms(t *testing.T) {
	d, err := Parse(`xychart-beta horizontal
title "T"
x-axis "Region" [North, "South, East", West]
y-axis "Sales (k)" 10 --> 0
bar "2025" [1, 2.5, -3]`)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Horizontal || d.XLabel != "Region" || len(d.XCats) != 3 || d.XCats[1] != "South, East" {
		t.Errorf("x axis: %+v", d)
	}
	if d.YLabel != "Sales (k)" || d.YMin != 0 || d.YMax != 10 || !d.HasYRange {
		t.Errorf("y axis: %q %v %v", d.YLabel, d.YMin, d.YMax)
	}
	if d.Series[0].Name != "2025" || d.Series[0].Values[2] != -3 {
		t.Errorf("series: %+v", d.Series[0])
	}
	d, err = Parse("xychart-beta\nx-axis \"Load\" 0 --> 100\nline [1, 2]")
	if err != nil || !d.HasXRange || d.XMax != 100 || d.XLabel != "Load" {
		t.Errorf("numeric x axis: %+v %v", d, err)
	}
}

func TestRejectsBadValues(t *testing.T) {
	for _, src := range []string{
		"xychart-beta\nbar [1, x]",
		"xychart-beta\nbar [NaN]",
		"xychart-beta\nline [Inf]",
		"xychart-beta\ny-axis 0 --> NaN\nbar [1]",
	} {
		if _, err := Parse(src); err == nil {
			t.Errorf("no error for %q", src)
		}
	}
}

// Bars start from zero even when Mermaid's auto range would start at the
// smallest value and leave that bar with no length.
func TestBarsStartAtZero(t *testing.T) {
	d, _ := Parse("xychart-beta\nbar [3, 7]")
	if lo, _ := d.Bounds(); lo != 0 {
		t.Errorf("auto range starts at %v", lo)
	}
	out, _ := Render("xychart-beta\nbar [3, 7]", opts())
	if !strings.Contains(string(out), ">0<") {
		t.Error("no zero tick")
	}
}
