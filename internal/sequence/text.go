package sequence

import (
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// maxLines caps how many lines one label may wrap into, so hostile source
// cannot make a picture of unbounded height from one statement.
const maxLines = 200

// maxText caps the length of one label for the same reason.
const maxText = 8000

// wrap splits text on explicit breaks (<br>, \n) and then word-wraps each
// line at its spaces so no line is wider than width. A word wider than width
// stays whole on a line of its own, and the lifelines move apart to fit it,
// as mermaid.js draws it. Empty text yields no lines. When a label needs more
// than one line, the lines are balanced: the narrowest width that still
// gives the same number of lines is used, so no lone word is left over.
func wrap(text string, width float64, face svgutil.Face, fs float64) []string {
	if len(text) > maxText {
		text = strings.ToValidUTF8(text[:maxText], "")
	}
	lines := greedy(text, width, face, fs)
	if len(lines) < 2 || len(lines) >= maxLines {
		return lines
	}
	lo, hi := width/float64(len(lines)+1), width
	for i := 0; i < 12 && hi-lo > 1; i++ {
		mid := (lo + hi) / 2
		if len(greedy(text, mid, face, fs)) == len(lines) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return greedy(text, hi, face, fs)
}

func greedy(text string, width float64, face svgutil.Face, fs float64) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var out []string
	for _, para := range svgutil.SplitLines(text) {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			if len(out) >= maxLines {
				break
			}
			switch {
			case line == "":
				line = w
			case face.Width(line+" "+w, fs) <= width:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		out = append(out, line)
		if len(out) >= maxLines {
			break
		}
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	if len(out) > maxLines {
		out = out[:maxLines]
	}
	return out
}

// widest returns the width of the widest line.
func widest(lines []string, face svgutil.Face, fs float64) float64 {
	var w float64
	for _, l := range lines {
		if lw := face.Width(l, fs); lw > w {
			w = lw
		}
	}
	return w
}
