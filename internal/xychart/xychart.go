// Package xychart parses and renders Mermaid xychart-beta diagrams (bar and
// line charts) to SVG.
//
// Syntax:
//
//	xychart-beta [horizontal]
//	    title "Sales"
//	    x-axis "Month" [jan, feb, mar]     (or: x-axis "Size" 0 --> 100)
//	    y-axis "Revenue" 0 --> 10000       (the range is optional)
//	    bar [5000, 6000, 7500]
//	    line "Target" [4000, 6000, 9000]
package xychart

import (
	"math"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Series is one bar or line data set.
type Series struct {
	Kind   string // "bar" or "line"
	Name   string
	Values []float64
}

// Diagram is a parsed xychart.
type Diagram struct {
	Title      string
	Horizontal bool

	XLabel    string
	XCats     []string
	XMin      float64
	XMax      float64
	HasXRange bool

	YLabel    string
	YMin      float64
	YMax      float64
	HasYRange bool

	Series []*Series
}

// Parse builds a Diagram from xychart-beta source.
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
			f := strings.Fields(line)
			if w := strings.ToLower(f[0]); w != "xychart-beta" && w != "xychart" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'xychart-beta' header")
			}
			d.Horizontal = len(f) > 1 && strings.EqualFold(f[1], "horizontal")
			headerSeen = true
			continue
		}
		kw := strings.ToLower(firstWord(line))
		rest := strings.TrimSpace(line[len(kw):])
		var err error
		switch kw {
		case "title":
			d.Title = unquote(rest)
		case "x-axis":
			d.XLabel, d.XCats, d.XMin, d.XMax, d.HasXRange, err = parseAxis(rest, true)
		case "y-axis":
			d.YLabel, _, d.YMin, d.YMax, d.HasYRange, err = parseAxis(rest, false)
		case "bar", "line":
			s := &Series{Kind: kw}
			o := strings.IndexByte(rest, '[')
			if o < 0 || !strings.HasSuffix(rest, "]") {
				return nil, syntax.Errorf(lineNo, 1, "expected %s [values]", kw)
			}
			s.Name = unquote(rest[:o])
			for _, f := range splitList(rest[o+1 : len(rest)-1]) {
				v, perr := strconv.ParseFloat(f, 64)
				if perr != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e300 {
					return nil, syntax.Errorf(lineNo, 1, "invalid value %q", f)
				}
				s.Values = append(s.Values, v)
			}
			d.Series = append(d.Series, s)
		case "accdescr", "acctitle":
		}
		if err != nil {
			return nil, syntax.Errorf(lineNo, 1, "%v", err)
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'xychart-beta' header")
	}
	if len(d.Series) == 0 {
		return nil, syntax.Errorf(1, 1, "xychart has no data series")
	}
	return d, nil
}

type axisError string

func (e axisError) Error() string { return string(e) }

// parseAxis reads `"title" [a, b]`, `"title" lo --> hi` or `"title"`, the
// title being optional and possibly unquoted.
func parseAxis(s string, cats bool) (title string, list []string, lo, hi float64, ranged bool, err error) {
	if o := strings.IndexByte(s, '['); o >= 0 && cats {
		if !strings.HasSuffix(s, "]") {
			return "", nil, 0, 0, false, axisError("unclosed [")
		}
		return unquote(s[:o]), splitList(s[o+1 : len(s)-1]), 0, 0, false, nil
	}
	left, right, ok := strings.Cut(s, "-->")
	if !ok {
		return unquote(s), nil, 0, 0, false, nil
	}
	f := fieldsQuoted(left)
	if len(f) == 0 {
		return "", nil, 0, 0, false, axisError("range has no start")
	}
	lo, err1 := strconv.ParseFloat(f[len(f)-1], 64)
	hi, err2 := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if err1 != nil || err2 != nil || math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) ||
		math.Abs(lo) > 1e300 || math.Abs(hi) > 1e300 {
		return "", nil, 0, 0, false, axisError("invalid axis range")
	}
	if hi < lo {
		lo, hi = hi, lo
	}
	return unquote(strings.Join(f[:len(f)-1], " ")), nil, lo, hi, true, nil
}

// fieldsQuoted splits on spaces, keeping "quoted text" as one field.
func fieldsQuoted(s string) []string {
	var out []string
	var cur strings.Builder
	inQ := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == '"':
			inQ = !inQ
			cur.WriteRune(r)
		case (r == ' ' || r == '\t') && !inQ:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// splitList splits a bracketed list on commas outside quotes.
func splitList(s string) []string {
	var out []string
	var cur strings.Builder
	inQ := false
	for _, r := range s {
		switch {
		case r == '"':
			inQ = !inQ
			cur.WriteRune(r)
		case r == ',' && !inQ:
			out = append(out, unquote(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if t := strings.TrimSpace(cur.String()); t != "" || len(out) > 0 {
		out = append(out, unquote(t))
	}
	return out
}

// Bounds returns the y range: the given one, or the data's extent widened
// to take in zero when there are bars, so every bar has a base.
func (d *Diagram) Bounds() (lo, hi float64) {
	if d.HasYRange {
		return d.YMin, d.YMax
	}
	first, bars := true, false
	for _, s := range d.Series {
		bars = bars || s.Kind == "bar"
		for _, v := range s.Values {
			if first {
				lo, hi, first = v, v, false
			}
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	if bars {
		lo, hi = math.Min(lo, 0), math.Max(hi, 0)
	}
	if hi == lo {
		hi = lo + 1
	}
	return lo, hi
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t[\""); i >= 0 {
		return s[:i]
	}
	return s
}
