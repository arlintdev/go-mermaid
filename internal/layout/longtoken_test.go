package layout

import (
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// A node holding a long token (a URL) grows wider rather than breaking the
// token after every slash; only a token over 2.5 wrap widths breaks, and
// then its lines fill that wider width.
func TestLongTokenWidensNode(t *testing.T) {
	url := "https://identity.example.com/realms/organisation/protocol/openid-connect/auth"
	g := graphFrom("flowchart TD\nA[\"Fetch " + url + "\"] --> B[\"See /var/lib/application/data.db\"]")
	if _, err := Flow(g, Options{NodeSep: 50, RankSep: 50, FontSize: 16}); err != nil {
		t.Fatal(err)
	}
	face := svgutil.FaceSans
	a, b := g.NodeByID("A"), g.NodeByID("B")
	if len(a.Lines) > 3 {
		t.Errorf("URL node broke into %d lines: %q", len(a.Lines), a.Lines)
	}
	if strings.ReplaceAll(strings.Join(a.Lines, ""), " ", "") != "Fetch"+url {
		t.Errorf("text changed: %q", a.Lines)
	}
	for _, l := range a.Lines {
		if w := face.Width(l, 16); w > 2.5*defaultWrapWidth+0.01 {
			t.Errorf("line %q is %.0f wide, over 2.5 wrap widths", l, w)
		}
	}
	found := false
	for _, l := range b.Lines {
		found = found || strings.Contains(l, "/var/lib/application/data.db")
	}
	if !found {
		t.Errorf("a path under 2.5 wrap widths was broken: %q", b.Lines)
	}
}
