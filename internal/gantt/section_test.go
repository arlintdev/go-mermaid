package gantt

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

var band = regexp.MustCompile(`<rect x="[0-9.]+" y="([0-9.]+)" width="[0-9.]+" height="([0-9.]+)" fill="[^"]+" fill-opacity`)

// A section title that wraps onto more lines than its rows are tall gets a
// taller block, so its lines stay inside its own band.
func TestSectionTitleStaysInItsBand(t *testing.T) {
	src, _ := os.ReadFile("testdata/tall_section_title.mmd")
	out, err := Render(string(src), opts())
	if err != nil {
		t.Fatal(err)
	}
	m := band.FindStringSubmatch(string(out))
	if m == nil {
		t.Fatal("no section band")
	}
	top, _ := strconv.ParseFloat(m[1], 64)
	h, _ := strconv.ParseFloat(m[2], 64)
	boxes, _ := goldentest.TextBoxes(out)
	n := 0
	for _, b := range boxes {
		if strings.Contains("Design, review and sign-off with the security and compliance teams", b.Text) && len(b.Text) > 8 {
			n++
			if b.Y0 < top || b.Y1 > top+h {
				t.Errorf("title line %v leaves its band %v..%v", b, top, top+h)
			}
		}
	}
	if n < 2 {
		t.Fatalf("expected a wrapped title, found %d lines", n)
	}
	if ov, _ := goldentest.TextOverlaps(out, 1); len(ov) > 0 {
		t.Errorf("text overlaps: %v", ov)
	}
}

// A section name that is one word longer than the gutter allows is cut, so
// the gutter stops at 2.5 times the wrap width.
func TestSectionWordWidensGutterThenCuts(t *testing.T) {
	word := strings.Repeat("Pneumonoultramicroscopic", 4)
	out, err := Render("gantt\ndateFormat YYYY-MM-DD\nsection "+word+"\nA : 2024-01-01, 3d", opts())
	if err != nil {
		t.Fatal(err)
	}
	boxes, _ := goldentest.TextBoxes(out)
	joined := ""
	for _, b := range boxes {
		if strings.HasPrefix(word[len(joined):], b.Text) {
			joined += b.Text
			if b.X1-b.X0 > sectionTitleMaxW*2.5+1 {
				t.Errorf("line %v is wider than the gutter allows", b)
			}
		}
	}
	if joined != word {
		t.Errorf("section name not drawn whole: %q", joined)
	}
}
