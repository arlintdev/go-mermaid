package journey

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
	goldentest.Hostile(t, render, "journey\ntitle "+inj+"\nsection "+inj+"\n"+inj+": 5: "+inj+", b")
	o.FontFace, o.Title = inj, inj
	goldentest.Hostile(t, render, "journey\nsection s\na: 1: x")
}

func TestActorsAndScores(t *testing.T) {
	out, err := Render("journey\nsection S\nA: 5: Zed, Amy\nB: 1: Amy\nC: 3: Zed", opts())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Actors are coloured in name order: Amy first, Zed second.
	if !strings.Contains(s, `fill="#8fbc8f" stroke="#000000"><title>Amy</title>`) ||
		!strings.Contains(s, `fill="#7cfc00" stroke="#000000"><title>Zed</title>`) {
		t.Errorf("actor colours wrong:\n%s", s)
	}
	if strings.Count(s, `r="15"`) != 3 {
		t.Error("want one face per task")
	}
	if !strings.Contains(s, "marker-end=\"url(#m") {
		t.Error("arrow marker id is not derived from the source")
	}
}
