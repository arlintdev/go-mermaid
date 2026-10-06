package render

import (
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/parser"
	"github.com/arlintdev/go-mermaid/internal/svgid"
)

// renderFlowchart is the flowchart pipeline with the library's defaults.
func renderFlowchart(src string) ([]byte, error) {
	if strings.HasPrefix(src, "---\n") {
		if i := strings.Index(src[4:], "\n---\n"); i >= 0 {
			src = src[4+i+5:]
		}
	}
	g, err := parser.Flowchart(src)
	if err != nil {
		return nil, err
	}
	res, err := layout.Flow(g, layout.Options{FontSize: 16, FontFace: "sans-serif"})
	if err != nil {
		return nil, err
	}
	return SVG(res, Options{Theme: "default", FontFace: "sans-serif", FontSize: 16, Padding: 16, IDPrefix: svgid.Prefix(src)})
}

// TestGolden renders the flowchart corpus and probes. Rewrite the golden
// files with GOLDEN_UPDATE=1 after an intended change, and look at the
// pictures before committing them.
func TestGolden(t *testing.T) {
	goldentest.Golden(t, "testdata", renderFlowchart)
}

func TestHostileFlowchart(t *testing.T) {
	inj := goldentest.Injection
	q := strings.ReplaceAll(inj, `"`, "#quot;")
	for _, src := range []string{
		"flowchart TD\n  A[\"" + q + "\"] -->|\"" + q + "\"| B(\"" + q + "\")\n  subgraph S[\"" + q + "\"]\n    B\n  end",
		"flowchart LR\n  A -- \"" + q + "\" --> B\n  classDef c fill:" + inj + ",stroke:" + inj + ",color:" + inj + ",stroke-width:" + inj + ",stroke-dasharray:" + inj + "\n  class A c\n  style B fill:" + inj + "\n  style S stroke:" + inj + "\n  linkStyle 0 stroke:" + inj + ",stroke-width:" + inj,
		"flowchart TD\n  A:::x --> B@{ shape: cyl, label: \"" + q + "\" }\n  classDef x fill:#fff\" onload=\"alert(1),font-weight:" + inj,
		"flowchart TD\n  A[\"&lt;script&gt;alert(1)&lt;/script&gt;\"] --> B[\"#lt;a href=#quot;javascript:x#quot;#gt;\"]",
	} {
		goldentest.Hostile(t, renderFlowchart, src)
		out, err := renderFlowchart(src)
		if err != nil {
			continue
		}
		if s := string(out); strings.Contains(s, "<a ") || strings.Contains(s, "<script") || strings.Contains(s, "onload=") {
			t.Errorf("markup got through:\n%s", s)
		}
	}
}
