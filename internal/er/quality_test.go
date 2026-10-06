package er

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/curve"
	"github.com/arlintdev/go-mermaid/internal/domain"
)

var (
	relPathRe  = regexp.MustCompile(`<path d="(M[^"M]*)" fill="none"`)
	labelRe    = regexp.MustCompile(`<rect x="([-0-9.]+)" y="([-0-9.]+)" width="([0-9.]+)" height="([0-9.]+)" rx="2" [^>]*fill-opacity="0.85"/>`)
	pathNumsRe = regexp.MustCompile(`[-0-9.]+`)
)

// drawnLines reads the relationship lines out of a rendered diagram: every
// path with a single move, as polylines (curves sampled).
func drawnLines(svg string) [][]domain.Point {
	var out [][]domain.Point
	for _, m := range relPathRe.FindAllStringSubmatch(svg, -1) {
		var v []float64
		for _, s := range pathNumsRe.FindAllString(m[1], -1) {
			f, _ := strconv.ParseFloat(s, 64)
			v = append(v, f)
		}
		var pts []domain.Point
		for i := 0; i+1 < len(v); i += 2 {
			pts = append(pts, domain.Point{X: v[i], Y: v[i+1]})
		}
		if strings.Contains(m[1], "C") && len(pts) == 4 {
			pts = curve.Bezier(pts[0], pts[1], pts[2], pts[3])
		}
		out = append(out, pts)
	}
	return out
}

// hiddenLines counts the labels drawn over a line other than their own: a
// label sits on its own line, so one crossed by two lines hides another.
func hiddenLines(svg string) int {
	lines := drawnLines(svg)
	n := 0
	for _, m := range labelRe.FindAllStringSubmatch(svg, -1) {
		var v [4]float64
		for i := range v {
			v[i], _ = strconv.ParseFloat(m[i+1], 64)
		}
		b := curve.Box{X: v[0], Y: v[1], W: v[2], H: v[3]}
		hits := 0
		for _, l := range lines {
			if curve.Crosses(l, b) {
				hits++
			}
		}
		if hits > 1 {
			n++
		}
	}
	return n
}

// TestLabelsOffOtherLines checks that a relationship label is moved along
// its own line off the others where it can be. The counts left are labels
// whose whole line runs beside or across another one.
func TestLabelsOffOtherLines(t *testing.T) {
	most := map[string]int{"dig66_1": 0, "er01_er": 0}
	files, _ := filepath.Glob("testdata/*.mmd")
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".mmd")
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out, err := render(string(src))
		if err != nil {
			t.Fatal(err)
		}
		if got := hiddenLines(string(out)); got > most[name] {
			t.Errorf("%s: %d labels lie on another line, want at most %d", name, got, most[name])
		}
	}
}
