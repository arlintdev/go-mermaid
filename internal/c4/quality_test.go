package c4

import (
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

// TestLabelsOffOtherLines checks that no relationship label lies on a line
// other than its own or on another label: a diagram whose layered layout
// would put one there is routed by layout.Flow instead.
func TestLabelsOffOtherLines(t *testing.T) {
	goldentest.LabelsClear(t, "testdata", func(src string) ([]byte, error) { return Render(src, defaultOptions) }, map[string]int{})
}
