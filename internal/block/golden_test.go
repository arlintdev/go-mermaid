package block

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
		"block-beta",
		`a["` + q + `"] b(("` + q + `")) c<["` + q + `"]>(right)`,
		`a -- "` + q + `" --> b`,
		"b -->|" + q + "| c",
		"style a fill:" + x + ",stroke:" + x + ",stroke-width:" + x + ",color:" + x + ",stroke-dasharray:" + x,
		"classDef hot fill:" + x,
		"class b hot",
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
