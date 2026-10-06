package quadrant

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
	src := strings.Join([]string{
		"quadrantChart",
		"title " + x,
		"x-axis " + x + " --> " + x,
		"y-axis " + x + " --> " + x,
		"quadrant-1 " + x,
		"quadrant-2 " + x,
		x + ": [0.3, 0.6] radius: " + x + ", color: " + x + ", stroke-color: " + x + ", stroke-width: " + x,
		"P:::hot: [0.5, 0.5]",
		"classDef hot color: " + x + ", stroke-color: red;" + x + ", stroke-width: 2px" + x,
	}, "\n")
	goldentest.Hostile(t, render, src)
	out, err := render(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), x) {
		t.Errorf("injection written unescaped")
	}
	f := defaultOptions
	f.FontFace = x
	goldentest.Hostile(t, func(s string) ([]byte, error) { return Render(s, f) }, src)
}
