package svgid

import (
	"strings"
	"testing"
)

func TestPrefix(t *testing.T) {
	a, b := Prefix("graph TD\nA-->B"), Prefix("graph TD\nA-->C")
	if a == b {
		t.Fatalf("different sources share prefix %q", a)
	}
	if a != Prefix("graph TD\nA-->B") {
		t.Fatal("prefix is not deterministic")
	}
	for _, r := range a {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			t.Fatalf("prefix %q has a character outside [a-z0-9]", a)
		}
	}
}

func TestFor(t *testing.T) {
	src := "graph TD\nA-->B"
	for _, c := range []struct{ prefix, want string }{
		{"", Prefix(src)},
		{"light", "light"},
		{"d-1_x", "d-1_x"},
		{"1abc", "m"},
		{`a"><script>`, "m"},
		{strings.Repeat("a", 65), "m"},
	} {
		if got := For(c.prefix, src); got != c.want {
			t.Errorf("For(%q) = %q, want %q", c.prefix, got, c.want)
		}
	}
}
