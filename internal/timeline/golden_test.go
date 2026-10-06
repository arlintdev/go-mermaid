package timeline

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
		"timeline",
		"title " + x,
		"section " + x,
		x + " : " + x + " : " + x,
		"  : " + x,
		x,
	}, "\n")
	goldentest.Hostile(t, render, src)
	out, err := render(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), x) {
		t.Error("injection written unescaped")
	}
}
