package kanban

import (
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
	o := opts()
	render := func(s string) ([]byte, error) { return Render(s, o) }
	goldentest.Hostile(t, render, "kanban\n  c["+inj+"]\n    a["+inj+"]@{ ticket: "+inj+", assigned: '"+inj+"', priority: '"+inj+"' }\n    "+inj)
	o.Title = inj
	goldentest.Hostile(t, render, "kanban\n  c\n    a")
}

func TestMetadata(t *testing.T) {
	d, err := Parse("kanban\n  todo[Todo]\n    id1[Docs]@{ ticket: MC-1, assigned: 'knsv', priority: 'very high' }\n    id2[X]@{ priority: 'urgent' }")
	if err != nil {
		t.Fatal(err)
	}
	if d.Columns[0].Title != "Todo" {
		t.Errorf("column title %q", d.Columns[0].Title)
	}
	c := d.Columns[0].Cards[0]
	if c.Text != "Docs" || c.Ticket != "MC-1" || c.Assigned != "knsv" || c.Priority != "Very High" {
		t.Errorf("card %+v", c)
	}
	if p := d.Columns[0].Cards[1].Priority; p != "" {
		t.Errorf("unknown priority kept: %q", p)
	}
}
