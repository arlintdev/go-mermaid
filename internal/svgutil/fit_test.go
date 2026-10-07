package svgutil

import (
	"strings"
	"testing"
)

func TestFit(t *testing.T) {
	f := FaceFor("")
	long := "Pneumonoultramicroscopicsilicovolcanoconiosis"
	for _, tc := range []struct {
		name     string
		text     string
		maxW     float64
		maxLines int
		size     float64 // the size it should be drawn at
		lines    int
		cut      bool
	}{
		{"fits as it is", "Reach", 200, 1, 14, 1, false},
		{"wraps between words", "Very important and urgent", 120, 2, 14, 2, false},
		{"shrinks a little to stay whole", long, f.Width(long, 13), 1, 13, 1, false},
		{"breaks a word when shrinking is not enough", long, 160, 3, 14, 2, false},
		{"cuts only as the last resort", long, 100, 1, 11, 1, true},
	} {
		lines, size := f.Fit(tc.text, 14, 11, tc.maxW, tc.maxLines)
		if size != tc.size || len(lines) != tc.lines {
			t.Errorf("%s: %d lines at %v, want %d at %v (%q)", tc.name, len(lines), size, tc.lines, tc.size, lines)
		}
		if cut := strings.HasSuffix(lines[len(lines)-1], "…"); cut != tc.cut {
			t.Errorf("%s: cut is %v, want %v (%q)", tc.name, cut, tc.cut, lines)
		}
		if w := f.LinesWidth(lines, size); w > tc.maxW {
			t.Errorf("%s: %v wide, want at most %v", tc.name, w, tc.maxW)
		}
	}
}
