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

func TestWrapBreaksAfterHyphens(t *testing.T) {
	got := FaceSans.Wrap("OnFailure=: digin-backup-failed.service, make backup-failed", 16, 120)
	for _, l := range got {
		if FaceSans.Width(l, 16) > 120*1.5 {
			t.Errorf("line %q too wide", l)
		}
	}
	joined := strings.Join(got, "")
	if strings.ReplaceAll(joined, " ", "") != strings.ReplaceAll("OnFailure=: digin-backup-failed.service, make backup-failed", " ", "") {
		t.Errorf("lost text: %q", got)
	}
	found := false
	for _, l := range got {
		if strings.HasSuffix(l, "-") {
			found = true
		}
	}
	if !found {
		t.Errorf("no break after a hyphen: %q", got)
	}
}

func TestWrapHard(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  float64
		want []string
	}{
		{"words fill each line", "The internal Microsoft Exchange e-mail system.", 220,
			[]string{"The internal Microsoft Exchange", "e-mail system."}},
		{"explicit breaks", "a<br>b", 200, []string{"a", "b"}},
		{"empty text", "", 200, []string{""}},
		{"a word wider than the box is cut", "xxxxxxxxxxxxxxxxxxxx", 60, []string{"xxxxxxxx", "xxxxxxxx", "xxxx"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FaceSans.WrapHard(tc.in, 14, tc.max)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("WrapHard(%q) = %q, want %q", tc.in, got, tc.want)
			}
			for _, l := range got {
				if w := FaceSans.Width(l, 14); w > tc.max {
					t.Errorf("line %q is %.1f wide, over %.0f", l, w, tc.max)
				}
			}
		})
	}
}

func TestCutIsLinear(t *testing.T) {
	word := strings.Repeat("́", 50000) + strings.Repeat("x", 50000)
	got := FaceSans.WrapHard(word, 14, 100)
	if strings.Join(got, "") != word {
		t.Error("cutting lost text")
	}
}
