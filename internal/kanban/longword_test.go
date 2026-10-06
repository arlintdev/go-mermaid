package kanban

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

var rectWidthRe = regexp.MustCompile(`<rect [^>]*width="([0-9.]+)"`)

// TestLongWordWidensBox checks that a word too long to break is held whole
// by a box at least as wide as it, never cut between letters or left
// hanging over a neighbour.
func TestLongWordWidensBox(t *testing.T) {
	b, err := os.ReadFile("testdata/long_words.mmd")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	out, err := Render(src, opts())
	if err != nil {
		t.Fatal(err)
	}
	const word = "Pneumonoultramicroscopicsilicovolcanoconiosisdiagnosis"
	need := svgutil.FaceSans.Width(word, 14)
	widest := 0.0
	for _, m := range rectWidthRe.FindAllStringSubmatch(string(out), -1) {
		if w, err := strconv.ParseFloat(m[1], 64); err == nil {
			widest = max(widest, w)
		}
	}
	if widest < need {
		t.Errorf("widest box is %.0f, the word needs %.0f", widest, need)
	}
	if !regexp.MustCompile(">" + word + "<").Match(out) {
		t.Error("the long word is not drawn whole on one line")
	}
}
