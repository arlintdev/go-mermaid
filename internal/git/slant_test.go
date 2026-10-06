package git

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

var (
	branchBox = regexp.MustCompile(`<rect x="([-0-9.]+)" y="([-0-9.]+)" width="([0-9.]+)" height="([0-9.]+)" rx="4"`)
	slantedID = regexp.MustCompile(`<text x="([-0-9.]+)" y="([-0-9.]+)" fill="[^"]+" font-size="([0-9.]+)" text-anchor="end" transform="rotate\(-45[^>]*>([^<]*)</text>`)
)

// A slanted commit id never runs over the name of a branch below it.
func TestSlantedIDsClearBranchNames(t *testing.T) {
	src, err := os.ReadFile("testdata/stress_word.mmd")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Render(string(src), RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16})
	if err != nil {
		t.Fatal(err)
	}
	f := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	boxes := branchBox.FindAllStringSubmatch(string(out), -1)
	ids := slantedID.FindAllStringSubmatch(string(out), -1)
	if len(boxes) < 2 || len(ids) == 0 {
		t.Fatalf("found %d branch names and %d ids", len(boxes), len(ids))
	}
	for _, id := range ids {
		ax, ay, fs := f(id[1]), f(id[2]), f(id[3])
		w := svgutil.FaceSans.Width(id[4], fs)
		for d := 0.0; d <= w; d += 2 {
			// Along the baseline, and a glyph's height above it.
			for _, up := range []float64{0, fs * 0.7} {
				x := ax - d*math.Sqrt2/2 - up*math.Sqrt2/2
				y := ay + d*math.Sqrt2/2 - up*math.Sqrt2/2
				for _, b := range boxes {
					if x > f(b[1]) && x < f(b[1])+f(b[3]) && y > f(b[2]) && y < f(b[2])+f(b[4]) {
						t.Fatalf("id %q runs over a branch name at %.0f,%.0f", id[4], x, y)
					}
				}
			}
		}
	}
}
