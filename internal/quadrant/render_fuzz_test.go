package quadrant

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzRender feeds mutated diagrams to the renderer: untrusted source must
// never panic or hang, only render or return an error.
func FuzzRender(f *testing.F) {
	files, _ := filepath.Glob(filepath.Join("testdata", "*.mmd"))
	for _, name := range files {
		if src, err := os.ReadFile(name); err == nil {
			f.Add(string(src))
		}
	}
	f.Fuzz(func(_ *testing.T, src string) {
		_, _ = Render(src, defaultOptions)
	})
}
