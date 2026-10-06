package syntax

import "testing"

func TestStripComment(t *testing.T) {
	for in, want := range map[string]string{"a --> b %% note": "a --> b ", "%% all": "", "plain": "plain"} {
		if got := StripComment(in); got != want {
			t.Errorf("StripComment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFirstWord(t *testing.T) {
	for in, want := range map[string]string{"title A b": "title", "x\ty": "x", "one": "one", "": ""} {
		if got := FirstWord(in); got != want {
			t.Errorf("FirstWord(%q) = %q, want %q", in, got, want)
		}
	}
}
