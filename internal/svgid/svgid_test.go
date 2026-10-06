package svgid

import "testing"

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
