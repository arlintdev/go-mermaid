// Package radar parses and renders Mermaid radar-beta charts to SVG as a polar
// plot with one smooth closed curve per data series.
//
// Syntax:
//
//	radar-beta
//	    title Skills
//	    axis a["Speed"], b["Power"], c["Range"]
//	    curve s1["Team A"]{80, 60, 90}
//	    curve s2["Team B"]{ c: 40, a: 50, b: 90 }
//	    max 100
//	    min 0
//	    graticule polygon
//	    ticks 4
//	    showLegend false
package radar

import (
	"math"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Curve is a named series of values, one per axis.
type Curve struct {
	Name   string
	Values []float64
}

// Diagram is a parsed radar chart.
type Diagram struct {
	Title  string
	Axes   []string
	Curves []*Curve

	MaxSet, MinSet bool
	MaxV, MinV     float64
	Polygon        bool // graticule polygon (default circle)
	Ticks          int
	HideLegend     bool

	axisIDs []string
}

// Max returns the outer value of the chart: the given max, else the
// largest value across curves (at least 1).
func (d *Diagram) Max() float64 {
	if d.MaxSet {
		return d.MaxV
	}
	m := 1.0
	for _, c := range d.Curves {
		for _, v := range c.Values {
			m = math.Max(m, v)
		}
	}
	return m
}

// Min returns the value at the centre: the given min, else 0.
func (d *Diagram) Min() float64 {
	if d.MinSet {
		return d.MinV
	}
	return 0
}

// Parse builds a Diagram from radar-beta source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{Ticks: 5}
	type pending struct {
		c    *Curve
		body string
		line int
	}
	var curves []pending
	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			if w := strings.ToLower(firstWord(line)); w != "radar-beta" && w != "radar" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'radar-beta' header")
			}
			headerSeen = true
			continue
		}
		kw := strings.ToLower(firstWord(line))
		rest := strings.TrimSpace(line[len(kw):])
		switch kw {
		case "title":
			d.Title = rest
		case "axis":
			for _, tok := range splitTop(rest) {
				id, label := idLabel(tok)
				if id != "" || label != "" {
					d.axisIDs = append(d.axisIDs, id)
					d.Axes = append(d.Axes, label)
				}
			}
		case "curve":
			for _, tok := range splitTop(rest) {
				o := strings.IndexByte(tok, '{')
				if o < 0 || !strings.HasSuffix(tok, "}") {
					return nil, syntax.Errorf(lineNo, 1, "expected curve name{values}")
				}
				_, name := idLabel(strings.TrimSpace(tok[:o]))
				curves = append(curves, pending{&Curve{Name: name}, tok[o+1 : len(tok)-1], lineNo})
			}
		case "max", "min":
			v, err := strconv.ParseFloat(rest, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e300 {
				return nil, syntax.Errorf(lineNo, 1, "invalid %s", kw)
			}
			if kw == "max" {
				d.MaxV, d.MaxSet = v, true
			} else {
				d.MinV, d.MinSet = v, true
			}
		case "graticule":
			d.Polygon = strings.EqualFold(rest, "polygon")
		case "ticks":
			if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= 20 {
				d.Ticks = n
			}
		case "showlegend":
			d.HideLegend = strings.EqualFold(rest, "false")
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'radar-beta' header")
	}
	if len(d.Axes) == 0 {
		return nil, syntax.Errorf(1, 1, "radar chart has no axes")
	}
	for _, p := range curves {
		if err := d.fill(p.c, p.body); err != nil {
			return nil, syntax.Errorf(p.line, 1, "%v", err)
		}
		d.Curves = append(d.Curves, p.c)
	}
	if d.Max() <= d.Min() {
		return nil, syntax.Errorf(1, 1, "radar max must be above min")
	}
	return d, nil
}

type parseError string

func (e parseError) Error() string { return string(e) }

// fill reads a curve's values, either in axis order or as "axis: value"
// pairs.
func (d *Diagram) fill(c *Curve, body string) error {
	fields := strings.Split(body, ",")
	if strings.TrimSpace(body) == "" {
		fields = nil
	}
	keyed := len(fields) > 0 && strings.Contains(fields[0], ":")
	if keyed {
		c.Values = make([]float64, len(d.Axes))
	}
	for _, f := range fields {
		key, val := "", f
		if keyed {
			k, v, ok := strings.Cut(f, ":")
			if !ok {
				return parseError("mixed keyed and plain values")
			}
			key, val = strings.TrimSpace(k), v
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e300 {
			return parseError("invalid value " + strconv.Quote(strings.TrimSpace(val)))
		}
		if !keyed {
			c.Values = append(c.Values, v)
			continue
		}
		found := false
		for i, id := range d.axisIDs {
			if id == key {
				c.Values[i], found = v, true
			}
		}
		if !found {
			return parseError("unknown axis " + strconv.Quote(key))
		}
	}
	return nil
}

// splitTop splits on commas outside [] and {}.
func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if t := strings.TrimSpace(s[start:]); t != "" {
		out = append(out, t)
	}
	return out
}

// idLabel splits `id["Label"]` into its id and label; a bare token is both.
func idLabel(tok string) (string, string) {
	tok = strings.TrimSpace(tok)
	if o := strings.IndexByte(tok, '['); o >= 0 && strings.HasSuffix(tok, "]") {
		label := strings.TrimSpace(tok[o+1 : len(tok)-1])
		if len(label) >= 2 && label[0] == '"' && label[len(label)-1] == '"' {
			label = label[1 : len(label)-1]
		}
		return strings.TrimSpace(tok[:o]), label
	}
	return tok, tok
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

func stripComment(s string) string {
	if i := strings.Index(s, "%%"); i >= 0 {
		return s[:i]
	}
	return s
}
