package sequence

import (
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// maxLines caps how many lines one label may wrap into, so hostile source
// cannot make a picture of unbounded height from one statement.
const maxLines = 200

// wrap splits text on explicit breaks (<br>, \n) and then word-wraps each
// line so no line is wider than width. A word longer than width is broken
// between characters. Empty text yields no lines.
func wrap(text string, width float64, face svgutil.Face, fs float64) []string {
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
			for face.Width(w, fs) > width && len([]rune(w)) > 1 {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				head, tail := breakWord(w, width, face, fs)
				out = append(out, head)
				w = tail
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

// breakWord returns the longest prefix of w (at least one rune) that fits in
// width, and the rest.
func breakWord(w string, width float64, face svgutil.Face, fs float64) (string, string) {
	rs := []rune(w)
	n := 1
	for n < len(rs) && face.Width(string(rs[:n+1]), fs) <= width {
		n++
	}
	return string(rs[:n]), string(rs[n:])
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
