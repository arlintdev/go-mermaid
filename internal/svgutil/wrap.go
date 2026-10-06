package svgutil

import (
	"math"
	"regexp"
	"strings"
)

var breakTag = regexp.MustCompile(`(?i)<br\s*/?\s*>`)

// Breaks splits label text at its explicit line breaks: <br>, <br/>,
// <br /> in any case, a literal "\n" and a newline. It returns at least one
// line.
func Breaks(s string) []string {
	s = breakTag.ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, `\n`, "\n")
	return strings.Split(s, "\n")
}

// Wrap breaks text into lines no wider than maxWidth at the given font size,
// the way a browser wraps a label: at its explicit breaks first, then
// between words, and inside a word only after a hyphen. A piece
// wider than maxWidth stays whole unless it is far wider, when it is cut.
// maxWidth <= 0 only splits at explicit breaks.
func (f Face) Wrap(text string, fontSize, maxWidth float64) []string {
	var out []string
	for _, line := range Breaks(text) {
		line = strings.Join(strings.Fields(line), " ")
		if maxWidth <= 0 || f.Width(line, fontSize) <= maxWidth {
			out = append(out, line)
			continue
		}
		cur := ""
		for _, word := range strings.Fields(line) {
			for i, piece := range f.pieces(word, fontSize, maxWidth) {
				joined := cur + piece
				if i == 0 && cur != "" {
					joined = cur + " " + piece
				}
				switch {
				case cur == "":
					cur = piece
				case f.Width(joined, fontSize) <= maxWidth:
					cur = joined
				default:
					out = append(out, cur)
					cur = piece
				}
			}
		}
		out = append(out, cur)
	}
	return out
}

// pieces splits a word after each hyphen, and cuts any piece far
// wider than maxWidth.
func (f Face) pieces(word string, fontSize, maxWidth float64) []string {
	var parts []string
	start := 0
	for i, r := range word {
		if r == '-' && i > start && i+1 < len(word) {
			parts = append(parts, word[start:i+1])
			start = i + 1
		}
	}
	parts = append(parts, word[start:])
	var out []string
	for _, p := range parts {
		out = append(out, f.cut(p, fontSize, math.Max(maxWidth*2, 360))...)
	}
	return out
}

// WrapWidthFor returns the width to wrap text at: base, widened for long
// text so the wrapped block does not grow far taller than it is wide.
func (f Face) WrapWidthFor(text string, fontSize, lineHeight, base, most float64) float64 {
	total := 0.0
	for _, l := range Breaks(text) {
		total += f.Width(l, fontSize)
	}
	w := math.Sqrt(total * lineHeight * 0.6)
	return math.Max(base, math.Min(w, most))
}

// LinesWidth returns the width of the widest of lines.
func (f Face) LinesWidth(lines []string, fontSize float64) float64 {
	w := 0.0
	for _, l := range lines {
		if lw := f.Width(l, fontSize); lw > w {
			w = lw
		}
	}
	return w
}

// JoinBreaks rewrites every explicit line break in s (see Breaks) as "\n".
func JoinBreaks(s string) string { return strings.Join(Breaks(s), "\n") }

// Wrap breaks text into lines no wider than maxWidth at fontSize, measured
// with sans-serif metrics; see Face.Wrap.
func Wrap(text string, maxWidth, fontSize float64) []string {
	return FaceSans.Wrap(text, fontSize, maxWidth)
}

// cut splits a word wider than limit into pieces no wider than limit.
func (f Face) cut(word string, fontSize, limit float64) []string {
	if f.Width(word, fontSize) <= limit {
		return []string{word}
	}
	var pieces []string
	var cur []rune
	for _, r := range word {
		if len(cur) > 0 && f.Width(string(append(cur, r)), fontSize) > limit {
			pieces = append(pieces, string(cur))
			cur = cur[:0]
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		pieces = append(pieces, string(cur))
	}
	return pieces
}
