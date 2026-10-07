package packet

import (
	"os"
	"strings"
	"testing"
)

// TestLabelsShrinkBeforeCut checks that a field label a little too wide
// for its block is drawn whole in a smaller size, and that one far too
// wide is broken onto two lines before it is cut.
func TestLabelsShrinkBeforeCut(t *testing.T) {
	src, err := os.ReadFile("testdata/long_fields.mmd")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Render(string(src), opts())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, w := range []string{"Acknowledgement", "Retransmission"} {
		if !strings.Contains(s, ">"+w+"<") {
			t.Errorf("%q is not drawn whole", w)
		}
	}
	if !strings.Contains(s, ">Pneumonoultramic") || strings.Count(s, "…") != 1 {
		t.Errorf("the far too long label should fill two lines and be cut once:\n%s", s)
	}
}
