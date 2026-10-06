package packet

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
	goldentest.Hostile(t, render, "packet-beta\ntitle "+inj+"\n0-15: \""+inj+"\"\n16: \""+inj+"\"\n+3: "+inj)
	goldentest.Hostile(t, render, "packet-beta\n0-7: \"a\"")
}

func TestRelativeFieldsAndLimits(t *testing.T) {
	d, err := Parse("packet-beta\ntitle \"T\"\n0-3: \"a\"\n+4: \"b\"\n+1: \"c\"")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "T" || d.Fields[1].Start != 4 || d.Fields[1].End != 7 || d.Fields[2].Start != 8 || d.Fields[2].End != 8 {
		t.Errorf("fields %+v %+v", d.Fields[1], d.Fields[2])
	}
	for _, src := range []string{"packet-beta\n0-99999999: \"x\"", "packet-beta\n+0: \"x\"", "packet-beta\n+99999999: \"x\""} {
		if _, err := Parse(src); err == nil {
			t.Errorf("no error for %q", src)
		}
	}
}
