package git

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
	for _, dir := range []string{"", " TB:", " BT:"} {
		src := strings.Join([]string{
			"gitGraph" + dir,
			`commit id: "` + q + `" tag: "` + q + `"`,
			`branch "` + q + `"`,
			`commit tag: "` + q + `" type: HIGHLIGHT`,
			"checkout main",
			`merge "` + q + `" id: "m` + q + `" tag: "` + q + `"`,
			`cherry-pick id: "` + q + `"`,
		}, "\n")
		goldentest.Hostile(t, render, src)
		out, err := render(src)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), q) {
			t.Error("injection written unescaped")
		}
	}
}
