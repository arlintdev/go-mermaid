package state

import (
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

// TestLabelsOffOtherLines checks that a relationship label is moved along
// its own line off other lines, labels and boxes where it can be. The
// counts left are labels whose whole line runs beside or across another.
func TestLabelsOffOtherLines(t *testing.T) {
	goldentest.LabelsClear(t, "testdata", func(src string) ([]byte, error) { return Render(src, defaultOptions) }, map[string]int{"crowded_labels": 5})
}
