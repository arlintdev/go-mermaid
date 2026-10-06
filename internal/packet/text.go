package packet

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

// fontFamily keeps a font-family option only when it is a plain list of
// family names, so the option can never carry markup into the picture.
func fontFamily(s string) string {
	if strings.TrimSpace(s) == "" || len(s) > 200 {
		return "sans-serif"
	}
	for _, r := range s {
		if !(r == ' ' || r == ',' || r == '-' || r == '_' || r == '\'' || r == '"' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return "sans-serif"
		}
	}
	return s
}
