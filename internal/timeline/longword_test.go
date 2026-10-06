package timeline

import (
	"os"
	"regexp"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// TestLongWordWidensBox checks that a word too long to break is held whole
// by a box at least as wide as it, never cut between letters or left
// hanging over a neighbour.
func TestLongWordWidensBox(t *testing.T) {
	b, err := os.ReadFile("testdata/long_words.mmd")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	out, err := render(src)
	if err != nil {
		t.Fatal(err)
	}
	const word = "Pneumonoultramicroscopicsilicovolcanoconiosisdiagnosis"
	need := svgutil.FaceSans.Width(word, 14)
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if w := columnWidth(d, svgutil.FaceSans, 14); w < need+2*boxPadX {
		t.Errorf("a column is %.0f wide, the word needs %.0f", w, need+2*boxPadX)
	}
	if !regexp.MustCompile(">" + word + "<").Match(out) {
		t.Error("the long word is not drawn whole on one line")
	}
}
