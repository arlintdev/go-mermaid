package mermaid_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mermaid "github.com/arlintdev/go-mermaid"
	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

// TestTextStaysOnCanvas renders every package's test diagrams, the stress
// probes among them (long words and titles, many items, empty sections,
// CJK and emoji, one-item diagrams), in the default and dark themes, and
// fails when a line of text reaches off the canvas or two level lines of
// text overlap.
func TestTextStaysOnCanvas(t *testing.T) {
	inputs, _ := filepath.Glob(filepath.Join("internal", "*", "testdata", "*.mmd"))
	golden, _ := filepath.Glob(filepath.Join("testdata", "golden", "*.mmd"))
	inputs = append(inputs, golden...)
	if len(inputs) == 0 {
		t.Fatal("no diagram inputs")
	}
	for _, in := range inputs {
		name := filepath.Base(filepath.Dir(filepath.Dir(in))) + "_" + strings.TrimSuffix(filepath.Base(in), ".mmd")
		src, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		for _, th := range []mermaid.Theme{mermaid.Default, mermaid.Dark} {
			svg, err := mermaid.Render(string(src), mermaid.WithTheme(th))
			if err != nil {
				continue // parse-error fixtures
			}
			off, err := goldentest.OffCanvas(svg, 1)
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			for _, b := range off {
				t.Errorf("%s (%s): text off the canvas: %v", name, th, b)
			}
			over, _ := goldentest.TextOverlaps(svg, 2)
			for _, p := range over {
				t.Errorf("%s (%s): text overlaps: %v and %v", name, th, p[0], p[1])
			}
		}
	}
}
