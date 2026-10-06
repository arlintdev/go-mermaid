package sequence

import (
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

var defaultOpts = RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16}

func renderDefault(src string) ([]byte, error) { return Render(src, defaultOpts) }

func TestGolden(t *testing.T) {
	goldentest.Golden(t, "testdata", renderDefault)
}

func TestHostile(t *testing.T) {
	inj := goldentest.Injection
	src := strings.Join([]string{
		"sequenceDiagram",
		"title " + inj,
		"autonumber",
		"box " + inj,
		"participant P1 as " + inj,
		"end",
		"box red " + inj,
		"actor P2 as " + inj,
		"end",
		"box rgb(1,2,3);fill:url(javascript:x) " + inj,
		"participant P3",
		"end",
		"create participant P4 as " + inj,
		"P1->>P4: " + inj,
		"link P1: " + inj + " @ javascript:alert(1)",
		"links P1: {\"x\": \"" + inj + "\"}",
		"rect " + inj,
		"P1->>+P2: " + inj,
		"end",
		"rect rgb(0,0,0\"><a)",
		"Note right of P1: " + inj,
		"end",
		"rect #12345g",
		"Note over P1,P2: " + inj,
		"end",
		"loop " + inj,
		"P2-->>-P1: " + inj + "<br>" + inj,
		"end",
		"alt " + inj,
		"P1-xP2: " + inj,
		"else " + inj,
		"P1--)P2: " + inj,
		"end",
		"critical " + inj,
		"P1->>P1: " + inj,
		"option " + inj,
		"P2->>P2: #34;#60;#62;" + inj,
		"end",
		"destroy P4",
		"P4->>P1: " + inj,
		"Note left of P2: " + inj,
	}, "\n")
	goldentest.Hostile(t, renderDefault, src)

	out, err := renderDefault(src)
	if err != nil {
		t.Fatalf("hostile source refused: %v", err)
	}
	s := string(out)
	for _, bad := range []string{`fill="#12345g"`, `fill="rgb(0,0,0`, `fill="rgb(1,2,3);`, `<a `} {
		if strings.Contains(s, bad) {
			t.Errorf("output carries %q", bad)
		}
	}

	// A different font face is escaped too.
	o := defaultOpts
	goldentest.Hostile(t, func(src string) ([]byte, error) { return Render(src, o) }, "sequenceDiagram\nA->>B: x")
}

func TestUnclosedAndDeepNesting(t *testing.T) {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	for i := 0; i < 500; i++ {
		b.WriteString("loop x\nA->>+B: y\n")
	}
	got, err := renderDefault(b.String())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := goldentest.CheckSVG(got); err != nil {
		t.Fatal(err)
	}
}
