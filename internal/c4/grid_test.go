package c4

import (
	"os"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// laidOut lays out the C4 source in testdata/name.mmd as Render does.
func laidOut(t *testing.T, name string) *ctx {
	t.Helper()
	src, err := os.ReadFile("testdata/" + name + ".mmd")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	c := &ctx{d: d, m: metrics{face: svgutil.FaceFor(""), fs: 14}, pal: theme.For(""), rects: map[string]rect{}, local: map[*Boundary]sub{}}
	if _, _, err := c.layout(d.Root, true); err != nil {
		t.Fatal(err)
	}
	c.place(d.Root, 0, 0)
	return c
}

func TestGridRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows int
	}{
		{"stress_many", 3}, // twelve systems in a chain: rows of four
		{"grid_routes", 2},
		{"grid_open_boundary", 0}, // a line comes in from outside: no grid
	} {
		c := laidOut(t, tc.name)
		rows := map[float64]int{}
		for _, e := range c.d.Elements {
			r := c.rects[e.ID]
			rows[r.y+r.h/2]++
		}
		if !c.gridded {
			if tc.rows != 0 {
				t.Errorf("%s: not laid out in rows", tc.name)
			}
			continue
		}
		if len(rows) != tc.rows {
			t.Errorf("%s: %d rows, want %d", tc.name, len(rows), tc.rows)
		}
		for y, n := range rows {
			if n > gridRow {
				t.Errorf("%s: %d shapes in the row at %v, want at most %d", tc.name, n, y, gridRow)
			}
		}
	}
}
