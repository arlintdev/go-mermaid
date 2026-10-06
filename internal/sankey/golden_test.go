package sankey

import (
	"fmt"
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

func opts() RenderOptions {
	return RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16}
}

func TestGolden(t *testing.T) {
	goldentest.Golden(t, "testdata", func(src string) ([]byte, error) { return Render(src, opts()) })
}

func TestHostile(t *testing.T) {
	inj := goldentest.Injection
	q := `"` + strings.ReplaceAll(inj, `"`, `""`) + `"`
	o := opts()
	render := func(s string) ([]byte, error) { return Render(s, o) }
	goldentest.Hostile(t, render, "sankey-beta\n"+q+",b,5\nb,"+q+"x,3")
	o.FontFace, o.Title = inj, inj
	goldentest.Hostile(t, render, "sankey-beta\na,b,1")
}

func TestCyclesAreRefused(t *testing.T) {
	for _, src := range []string{"sankey-beta\na,b,1\nb,a,1", "sankey-beta\na,a,1"} {
		if _, err := Render(src, opts()); err == nil {
			t.Errorf("no error for %q", src)
		}
	}
}

// Link bands are as wide as their share of the flow.
func TestLinkWidths(t *testing.T) {
	out, err := Render("sankey-beta\nA,B,6\nA,C,2", opts())
	if err != nil {
		t.Fatal(err)
	}
	var widths []string
	for _, part := range strings.Split(string(out), `stroke-width="`)[1:] {
		widths = append(widths, part[:strings.IndexByte(part, '"')])
	}
	if len(widths) != 2 {
		t.Fatalf("want 2 links, got %v", widths)
	}
	var a, b float64
	if _, err := fmtSscan(widths[0], &a); err != nil {
		t.Fatal(err)
	}
	if _, err := fmtSscan(widths[1], &b); err != nil {
		t.Fatal(err)
	}
	if r := a / b; r < 2.99 || r > 3.01 {
		t.Errorf("widths %v are not in the ratio 3:1", widths)
	}
}

func TestQuotedFields(t *testing.T) {
	d, err := Parse("sankey-beta\n\"a, \"\"b\"\"\",c,1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Flows[0].Source != `a, "b"` {
		t.Errorf("source %q", d.Flows[0].Source)
	}
}

func fmtSscan(s string, f *float64) (int, error) { return fmt.Sscan(s, f) }
