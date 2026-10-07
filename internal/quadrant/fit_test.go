package quadrant

import (
	"os"
	"strings"
	"testing"
)

// TestWordsKeptWhole checks that a word a little too wide for its room is
// drawn whole in a slightly smaller size, never cut, and that even a word
// far too wide is broken onto lines rather than cut.
func TestWordsKeptWhole(t *testing.T) {
	for name, words := range map[string][]string{
		"slightly_long": {"Hippopotomonstrosesquippedaliophobia", "Electroencephalographically", "Counterrevolutionaries"},
		"stress_word":   nil,
	} {
		src, err := os.ReadFile("testdata/" + name + ".mmd")
		if err != nil {
			t.Fatal(err)
		}
		out, err := render(string(src))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), "…") {
			t.Errorf("%s: a label is cut", name)
		}
		for _, w := range words {
			if !strings.Contains(string(out), ">"+w+"<") {
				t.Errorf("%s: %q is not drawn whole", name, w)
			}
		}
	}
}
