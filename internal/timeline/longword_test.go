package timeline

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
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
	if w := columnWidth(d, svgutil.FaceSans, svgutil.FaceSans.Bold(), svgutil.FaceSans, 14); w < need+2*boxPadX {
		t.Errorf("a column is %.0f wide, the word needs %.0f", w, need+2*boxPadX)
	}
	if !regexp.MustCompile(">" + word + "<").Match(out) {
		t.Error("the long word is not drawn whole on one line")
	}
}

// Section names (and periods, when there are no sections) are drawn bold, so
// they are measured bold: a section name that only just fits at regular
// weight gets a wider box.
func TestBoldTextMeasuredBold(t *testing.T) {
	name := "Supercalifragilisticexpialidociously"
	src := "timeline\nsection " + name + "\n2024 : a\n"
	out, err := render(src)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := Parse(src)
	if w := columnWidth(d, svgutil.FaceSans, svgutil.FaceSans.Bold(), svgutil.FaceSans, 14); w < svgutil.FaceSans.BoldWidth(name, 14)+2*boxPadX {
		t.Errorf("column %.0f is narrower than the bold name needs", w)
	}
	boxes, _ := goldentest.TextBoxes(out)
	rects := regexp.MustCompile(`<rect x="([0-9.]+)" y="[0-9.]+" width="([0-9.]+)"[^>]*rx="5"`).FindStringSubmatch(string(out))
	x, _ := strconv.ParseFloat(rects[1], 64)
	w, _ := strconv.ParseFloat(rects[2], 64)
	for _, b := range boxes {
		if b.Text == name && (b.X0 < x+boxPadX-1 || b.X1 > x+w-boxPadX+1) {
			t.Errorf("bold name %v reaches into its box's padding %v..%v", b, x, x+w)
		}
	}
	d, _ = Parse("timeline\nNoparagraphbreakslongperiodnamehere : a")
	if w := columnWidth(d, svgutil.FaceSans, svgutil.FaceSans.Bold(), svgutil.FaceSans.Bold(), 14); w < svgutil.FaceSans.BoldWidth("Noparagraphbreakslongperiodnamehere", 14)+2*boxPadX {
		t.Errorf("bold period not measured bold: %.0f", w)
	}
}

// A colon inside a word (a URL, a time) does not start a new event; only a
// colon followed by a space does, as in Mermaid.
func TestColonInsideWordKeepsEvent(t *testing.T) {
	d, err := Parse("timeline\n2024 : Deploy https://example.com/a : second\n     : at 10:30\n")
	if err != nil {
		t.Fatal(err)
	}
	evs := d.Sections[0].Periods[0].Events
	if len(evs) != 3 || evs[0] != "Deploy https://example.com/a" || evs[1] != "second" || evs[2] != "at 10:30" {
		t.Errorf("events: %q", evs)
	}
}
