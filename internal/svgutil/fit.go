package svgutil

import "strings"

// Fit fits text into a fixed room maxWidth wide and at most maxLines lines
// tall, for a chart that cannot grow to hold it. It prefers, in order:
// wrapping between words at fontSize, or at a size shrunk a step at a
// time down to minSize; breaking a long word between characters at
// fontSize, then at the smaller sizes; and last, cutting the text short
// with an ellipsis at minSize. It returns the lines and the size to draw
// them at.
func (f Face) Fit(text string, fontSize, minSize, maxWidth float64, maxLines int) ([]string, float64) {
	maxLines = max(maxLines, 1)
	minSize = min(minSize, fontSize)
	sizes := []float64{fontSize}
	for s := fontSize - 1; s > minSize; s-- {
		sizes = append(sizes, s)
	}
	if minSize < fontSize {
		sizes = append(sizes, minSize)
	}
	ok := func(lines []string, s float64) bool {
		return len(lines) <= maxLines && f.LinesWidth(lines, s) <= maxWidth
	}
	for _, s := range sizes {
		if l := f.Wrap(text, s, maxWidth); ok(l, s) {
			return l, s
		}
	}
	for _, s := range sizes {
		if l := f.WrapWithin(text, s, maxWidth); ok(l, s) {
			return l, s
		}
	}
	l := f.WrapWithin(text, minSize, maxWidth)
	rest := strings.Join(l[maxLines-1:], "")
	return append(l[:maxLines-1:maxLines-1], f.Clip(rest, minSize, maxWidth)), minSize
}

// Clip shortens s to fit maxWidth, ending it with an ellipsis when it cut
// anything.
func (f Face) Clip(s string, fontSize, maxWidth float64) string {
	if f.Width(s, fontSize) <= maxWidth {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && f.Width(string(r)+"…", fontSize) > maxWidth {
		r = r[:len(r)-1]
	}
	return strings.TrimSpace(string(r)) + "…"
}
