package svgutil

import (
	"strings"
	"testing"
)

func TestWrap(t *testing.T) {
	long := "A very long label that goes on and on to see how the renderer wraps or does not wrap long text"
	lines := FaceSans.Wrap(long, 16, 200)
	if len(lines) < 3 {
		t.Fatalf("want several lines, got %q", lines)
	}
	for _, l := range lines {
		if w := FaceSans.Width(l, 16); w > 200 {
			t.Errorf("line %q is %.0f wide, over 200", l, w)
		}
	}
	if got := strings.Join(lines, " "); got != long {
		t.Errorf("wrapping lost words: %q", got)
	}
	for _, c := range []struct {
		in   string
		want int
	}{
		{"one<br>two", 2}, {"one<BR/>two", 2}, {"one<br />two", 2}, {`one\ntwo`, 2}, {"one\ntwo\nthree", 3}, {"", 1},
	} {
		if got := FaceSans.Wrap(c.in, 16, 200); len(got) != c.want {
			t.Errorf("Wrap(%q) = %q, want %d lines", c.in, got, c.want)
		}
	}
	if got := FaceSans.Wrap(strings.Repeat("x", 200), 16, 100); len(got) < 2 {
		t.Errorf("a very long word is cut, got %q", got)
	}
	if got := FaceSans.Wrap("DIGIN_DATA_DIR/files", 16, 120); len(got) != 1 {
		t.Errorf("a word a little too wide stays whole, got %q", got)
	}
}
