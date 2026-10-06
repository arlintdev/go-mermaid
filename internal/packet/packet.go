// Package packet parses and renders Mermaid packet-beta diagrams to SVG as a
// bit/byte field table.
//
// Syntax:
//
//	packet-beta
//	0-15: "Source Port"
//	16-31: "Destination Port"
//	32-63: "Sequence Number"
//	+8: "Flags"
//	title "TCP"
package packet

import (
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Field is a contiguous range of bits with a label.
type Field struct {
	Start int
	End   int
	Label string
}

// Diagram is a parsed packet diagram.
type Diagram struct {
	Title  string
	Fields []*Field
}

// maxBits bounds a diagram's size so hostile ranges cannot make the
// renderer draw millions of rows.
const maxBits = 1 << 16

// Parse builds a Diagram from packet-beta source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{}
	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(syntax.StripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			if w := strings.ToLower(syntax.FirstWord(line)); w != "packet-beta" && w != "packet" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'packet-beta' header")
			}
			headerSeen = true
			continue
		}
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "title ") {
			d.Title = strings.Trim(strings.TrimSpace(line[len("title "):]), `"`)
			continue
		}
		if strings.HasPrefix(low, "acctitle") || strings.HasPrefix(low, "accdescr") {
			continue
		}
		next := 0
		if n := len(d.Fields); n > 0 {
			next = d.Fields[n-1].End + 1
		}
		f, err := parseField(line, lineNo, next)
		if err != nil {
			return nil, err
		}
		d.Fields = append(d.Fields, f)
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'packet-beta' header")
	}
	if len(d.Fields) == 0 {
		return nil, syntax.Errorf(1, 1, "packet has no fields")
	}
	return d, nil
}

func parseField(line string, lineNo, next int) (*Field, error) {
	rng, label, ok := strings.Cut(line, ":")
	if !ok {
		return nil, syntax.Errorf(lineNo, 1, "expected 'start-end: label'")
	}
	rng = strings.TrimSpace(rng)
	start, end := 0, 0
	if n, ok := strings.CutPrefix(rng, "+"); ok {
		// +N is N bits from where the previous field ended.
		bits, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil || bits < 1 || bits > maxBits {
			return nil, syntax.Errorf(lineNo, 1, "invalid bit count")
		}
		start, end = next, next+bits-1
	} else if lo, hi, isRange := strings.Cut(rng, "-"); isRange {
		var e1, e2 error
		start, e1 = strconv.Atoi(strings.TrimSpace(lo))
		end, e2 = strconv.Atoi(strings.TrimSpace(hi))
		if e1 != nil || e2 != nil {
			return nil, syntax.Errorf(lineNo, 1, "invalid bit range")
		}
	} else {
		v, err := strconv.Atoi(strings.TrimSpace(rng))
		if err != nil {
			return nil, syntax.Errorf(lineNo, 1, "invalid bit index")
		}
		start, end = v, v
	}
	if end < start || start < 0 {
		return nil, syntax.Errorf(lineNo, 1, "bit range end before start")
	}
	if end >= maxBits {
		return nil, syntax.Errorf(lineNo, 1, "bit range too large")
	}
	return &Field{Start: start, End: end, Label: strings.Trim(strings.TrimSpace(label), `"`)}, nil
}
