package mermaid_test

import (
	"strings"
	"testing"

	mermaid "github.com/arlintdev/go-mermaid"
	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

// TestEveryTypeEscapes renders every diagram type with an injection payload
// in its labels, titles and style values, and checks the output against
// Digin's allow-list: plain static SVG, every value plain, no markup let
// through. A diagram that refuses the source is safe too, but each of these
// should render, so a refusal is reported.
func TestEveryTypeEscapes(t *testing.T) {
	x := goldentest.Injection
	// Some grammars end a field at a quote or a colon; these variants carry
	// the same markup past them.
	xq := strings.ReplaceAll(x, `"`, "#quot;")
	xc := strings.ReplaceAll(x, ":", " ")
	sources := map[string]string{
		"flowchart": "flowchart TD\n  A[" + xq + "] -->|" + xq + "| B\n  subgraph S [" + xq + "]\n    B\n  end\n" +
			"  classDef c fill:" + x + ",stroke:" + x + "\n  class A c\n  style B fill:" + x + "\n  linkStyle 0 stroke:" + x,
		"sequence":    "sequenceDiagram\n  participant A as " + x + "\n  A->>B: " + x + "\n  Note over A,B: " + x + "\n  rect " + x + "\n  A->>B: x\n  end",
		"class":       "classDiagram\n  class A[\"" + xq + "\"]\n  A : +" + x + "\n  A <|-- B : " + x + "\n  style A fill:" + x,
		"state":       "stateDiagram-v2\n  [*] --> A : " + x + "\n  A : " + x + "\n  note right of A : " + x + "\n  classDef c fill:" + x + "\n  class A c",
		"er":          "erDiagram\n  A ||--o{ B : \"" + x + "\"\n  A {\n    string " + x + "\n  }",
		"pie":         "pie title " + x + "\n  \"" + x + "\" : 1\n  \"b\" : 2",
		"journey":     "journey\n  title " + x + "\n  section " + x + "\n    " + xc + ": 5: " + xc,
		"quadrant":    "quadrantChart\n  title " + x + "\n  x-axis " + x + " --> b\n  quadrant-1 " + x + "\n  " + xc + ": [0.3, 0.6]",
		"gitgraph":    "gitGraph\n  commit id: \"" + xq + "\" tag: \"" + xq + "\"\n  branch b" + "\n  commit",
		"timeline":    "timeline\n  title " + x + "\n  2002 : " + x,
		"mindmap":     "mindmap\n  root((" + x + "))\n    " + x,
		"gantt":       "gantt\n  title " + x + "\n  dateFormat YYYY-MM-DD\n  section " + x + "\n  " + x + " : a1, 2024-01-01, 10d",
		"c4":          "C4Context\n  title " + x + "\n  Person(a, \"" + x + "\", \"" + x + "\")\n  UpdateElementStyle(a, $bgColor=\"" + x + "\")",
		"requirement": "requirementDiagram\n  requirement r {\n    id: 1\n    text: " + x + "\n  }\n  element e {\n    type: " + x + "\n  }\n  e - satisfies -> r",
		"sankey":      "sankey-beta\n\n\"" + xq + "\",b,5\n",
		"xychart":     "xychart-beta\n  title \"" + x + "\"\n  x-axis [\"" + x + "\", b]\n  bar [1, 2]",
		"block":       "block-beta\n  a[\"" + xq + "\"] b\n  style a fill:" + x,
		"kanban":      "kanban\n  col[" + x + "]\n    t1[" + x + "]",
		"packet":      "packet-beta\n  0-15: \"" + x + "\"",
		"radar":       "radar-beta\n  title " + x + "\n  axis a[\"" + x + "\"], b, c\n  curve k[\"" + x + "\"]{1, 2, 3}",
	}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			for _, opts := range [][]mermaid.Option{nil, {mermaid.WithTransparentBackground()}} {
				out, err := mermaid.Render(src, opts...)
				if err != nil {
					t.Errorf("refused: %v", err)
					continue
				}
				if err := goldentest.CheckSVG(out); err != nil {
					t.Errorf("not plain static SVG: %v", err)
				}
				for _, bad := range []string{"<a ", "<script", "<foreignObject", "style="} {
					if strings.Contains(string(out), bad) {
						t.Errorf("output holds %q", bad)
					}
				}
			}
		})
	}
}
