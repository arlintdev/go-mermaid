package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/parser"
)

func BenchmarkFlowCorpus(b *testing.B) {
	for _, name := range []string{"dig99_1", "dig65_3", "dig58_2"} {
		src, err := os.ReadFile(filepath.Join("..", "render", "testdata", name+".mmd"))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				g, _ := parser.Flowchart(string(src))
				if _, err := Flow(g, Options{FontSize: 16}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
