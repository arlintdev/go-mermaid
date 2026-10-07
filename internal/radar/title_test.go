package radar

import (
	"os"
	"regexp"
	"testing"
)

// TestPlotUnderLongTitle checks that a title wider than the chart, which
// widens the canvas, has the plot centred under it.
func TestPlotUnderLongTitle(t *testing.T) {
	src, err := os.ReadFile("testdata/long_title.mmd")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Render(string(src), opts())
	if err != nil {
		t.Fatal(err)
	}
	title := regexp.MustCompile(`<text x="([0-9.]+)"[^>]*>How each`).FindSubmatch(out)
	plot := regexp.MustCompile(`<circle cx="([0-9.]+)"`).FindSubmatch(out)
	if title == nil || plot == nil {
		t.Fatalf("no title or plot in\n%s", out)
	}
	if string(title[1]) != string(plot[1]) {
		t.Errorf("plot centred at %s, title at %s", plot[1], title[1])
	}
}
