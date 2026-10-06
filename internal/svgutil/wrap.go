package svgutil

import (
	"math"
	"regexp"
	"strings"
	"unicode"
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
// between words, and inside a word after a hyphen. A token still wider than
// A token a little too wide stays whole, as in a browser; one far wider
// (over twice maxWidth, and over 360) may also break after '/', '-' or '_'
// and between wide (CJK) characters. Letters are never split from each
// other, so a token with no such place stays whole on a line of its own.
// maxWidth <= 0 only splits at explicit breaks.
func (f Face) Wrap(text string, fontSize, maxWidth float64) []string {
	return f.wrap(text, fontSize, maxWidth, math.Max(maxWidth*2, 360), true)
}

// WrapWide is Wrap for a label holding a token wider than its usual wrap
// width: lines fill to maxWidth, and only a token wider than maxWidth breaks
// after '/', '-' or '_' (and between CJK characters).
func (f Face) WrapWide(text string, fontSize, maxWidth float64) []string {
	return f.wrap(text, fontSize, maxWidth, maxWidth, true)
}

// LongestWord returns the width of the widest run of text between spaces
// and line breaks.
func (f Face) LongestWord(text string, fontSize float64) float64 {
	w := 0.0
	for _, line := range Breaks(text) {
		for _, word := range strings.Fields(line) {
			w = math.Max(w, f.Width(word, fontSize))
		}
	}
	return w
}

// WrapHard is for text drawn inside a box: it breaks between words, and
// breaks a token wider than maxWidth only after '/', '-' or '_' or between
// wide (CJK) characters. A token with no such place stays whole, so a caller
// sizing a box should measure the lines it gets back (see MinWidth).
func (f Face) WrapHard(text string, fontSize, maxWidth float64) []string {
	return f.wrap(text, fontSize, maxWidth, maxWidth, false)
}

// MinWidth returns the width of the widest piece of text that WrapHard cannot
// break: the narrowest a box can be and still hold every line.
func (f Face) MinWidth(text string, fontSize float64) float64 {
	return f.LinesWidth(f.WrapHard(text, fontSize, 1), fontSize)
}

// wrap breaks between words, after hyphens when hyphens is set, and at the
// soft break places of any token wider than softAt.
func (f Face) wrap(text string, fontSize, maxWidth, softAt float64, hyphens bool) []string {
	var out []string
	for _, line := range Breaks(text) {
		line = strings.Join(strings.Fields(line), " ")
		if maxWidth <= 0 || f.Width(line, fontSize) <= maxWidth {
			out = append(out, line)
			continue
		}
		cur := ""
		for _, word := range strings.Fields(line) {
			for i, piece := range f.pieces(word, fontSize, softAt, hyphens) {
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

// pieces splits a word after each hyphen when hyphens is set, then splits any
// piece wider than softAt at its soft break places.
func (f Face) pieces(word string, fontSize, softAt float64, hyphens bool) []string {
	parts := []string{word}
	if hyphens {
		parts = splitAfter(word, func(r rune) bool { return r == '-' }, false)
	}
	var out []string
	for _, p := range parts {
		if f.Width(p, fontSize) <= softAt {
			out = append(out, p)
			continue
		}
		out = append(out, splitAfter(p, isSoftBreak, true)...)
	}
	return out
}

// zeroWidth reports a combining mark or format control, which belongs to the
// character before it.
func zeroWidth(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r)
}

func isSoftBreak(r rune) bool { return r == '/' || r == '-' || r == '_' }

// splitAfter splits word after each run of runes matching after, where the
// piece so far holds some other rune and more follows, and, when wide is
// set, on either side of a wide (CJK) character. It never splits between
// two letters.
func splitAfter(word string, after func(rune) bool, wide bool) []string {
	var parts []string
	start, prev, body := 0, rune(-1), false
	for i, r := range word {
		if prev >= 0 {
			breakHere := body && after(prev) && !after(r)
			if wide && (inRanges(wideRanges, prev) || inRanges(wideRanges, r)) && !zeroWidth(r) {
				breakHere = true
			}
			if breakHere {
				parts = append(parts, word[start:i])
				start, body = i, false
			}
		}
		if !after(r) {
			body = true
		}
		prev = r
	}
	return append(parts, word[start:])
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

// WrapWithin is Wrap for text that has a fixed room and nowhere else to go:
// a line Wrap leaves wider than maxWidth (one long token) is cut between
// characters into pieces that fit, as a browser's overflow-wrap does.
func (f Face) WrapWithin(text string, fontSize, maxWidth float64) []string {
	lines := f.Wrap(text, fontSize, maxWidth)
	if maxWidth <= 0 {
		return lines
	}
	var out []string
	for _, l := range lines {
		for f.Width(l, fontSize) > maxWidth {
			r := []rune(l)
			n := 1
			for n < len(r) && f.Width(string(r[:n+1]), fontSize) <= maxWidth {
				n++
			}
			for n < len(r) && zeroWidth(r[n]) {
				n++
			}
			out = append(out, string(r[:n]))
			l = string(r[n:])
		}
		out = append(out, l)
	}
	return out
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
