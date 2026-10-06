package journey

import (
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// wrap breaks s into lines no wider than maxW, on spaces and on <br>.
func wrap(face svgutil.Face, s string, fs, maxW float64) []string {
	var out []string
	for _, para := range svgutil.SplitLines(s) {
		line := ""
		for _, word := range strings.Fields(para) {
			try := word
			if line != "" {
				try = line + " " + word
			}
			if line != "" && face.Width(try, fs) > maxW {
				out = append(out, line)
				line = word
			} else {
				line = try
			}
		}
		out = append(out, line)
	}
	return out
}
