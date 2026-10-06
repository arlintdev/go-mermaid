// Package quadrant parses and renders Mermaid quadrant charts to SVG.
//
// Syntax:
//
//	quadrantChart
//	    title Reach and engagement
//	    x-axis Low Reach --> High Reach
//	    y-axis Low Engagement --> High Engagement
//	    quadrant-1 We should expand
//	    Campaign A: [0.3, 0.6] radius: 8, color: #ff3300
//	    Campaign B:::hot: [0.45, 0.23]
//	    classDef hot color: #d64541, stroke-color: #000, stroke-width: 2px
package quadrant

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Style is the look a point may be given, inline or through a class. Every
// field is already validated; an empty field means "not set".
type Style struct {
	Radius      float64
	Color       string
	StrokeColor string
	StrokeWidth float64
	hasWidth    bool
}

// Point is a plotted item with coordinates in the unit square.
type Point struct {
	Label string
	X, Y  float64
	Class string
	Style Style
}

// Diagram is a parsed quadrant chart.
type Diagram struct {
	Title    string
	XLeft    string
	XRight   string
	YBottom  string
	YTop     string
	Quadrant [4]string // quadrant-1..4
	Points   []*Point
	Classes  map[string]Style
}

const (
	maxRadius      = 40
	maxStrokeWidth = 12
)

var pointLine = regexp.MustCompile(`^(.*?)(?::::([A-Za-z0-9_-]+))?\s*:\s*\[([^\]]*)\]\s*(.*)$`)

// Parse builds a Diagram from quadrant chart source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{Classes: map[string]Style{}}
	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			if firstWord(line) != "quadrantChart" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'quadrantChart' header")
			}
			headerSeen = true
			continue
		}
		key := firstWord(line)
		rest := strings.TrimSpace(strings.TrimPrefix(line, key))
		switch key {
		case "title":
			d.Title = unquote(rest)
		case "x-axis":
			d.XLeft, d.XRight = splitAxis(rest)
		case "y-axis":
			d.YBottom, d.YTop = splitAxis(rest)
		case "quadrant-1", "quadrant-2", "quadrant-3", "quadrant-4":
			d.Quadrant[key[len(key)-1]-'1'] = unquote(rest)
		case "classDef":
			name := firstWord(rest)
			if name != "" {
				d.Classes[name] = parseStyle(strings.TrimSpace(strings.TrimPrefix(rest, name)))
			}
		case "accTitle", "accTitle:", "accDescr", "accDescr:":
		default:
			p, err := parsePoint(line, lineNo)
			if err != nil {
				return nil, err
			}
			d.Points = append(d.Points, p)
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'quadrantChart' header")
	}
	return d, nil
}

func parsePoint(line string, lineNo int) (*Point, error) {
	m := pointLine.FindStringSubmatch(line)
	if m == nil {
		return nil, syntax.Errorf(lineNo, 1, "expected 'label: [x, y]'")
	}
	xs, ys, ok := strings.Cut(m[3], ",")
	if !ok {
		return nil, syntax.Errorf(lineNo, 1, "expected two coordinates")
	}
	x, err1 := strconv.ParseFloat(strings.TrimSpace(xs), 64)
	y, err2 := strconv.ParseFloat(strings.TrimSpace(ys), 64)
	if err1 != nil || err2 != nil || x != x || y != y {
		return nil, syntax.Errorf(lineNo, 1, "invalid coordinates")
	}
	return &Point{
		Label: unquote(strings.TrimSpace(m[1])),
		X:     x, Y: y,
		Class: m[2],
		Style: parseStyle(m[4]),
	}, nil
}

// parseStyle reads "radius: 10, color: #f00, stroke-color: #000,
// stroke-width: 2px". A value that fails validation is dropped.
func parseStyle(s string) Style {
	var st Style
	for _, part := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "radius":
			if r, ok := cssval.Pixels(v, maxRadius); ok && r > 0 {
				st.Radius = r
			}
		case "color":
			if c, ok := cssval.Color(v); ok {
				st.Color = c
			}
		case "stroke-color":
			if c, ok := cssval.Color(v); ok {
				st.StrokeColor = c
			}
		case "stroke-width":
			if w, ok := cssval.Pixels(v, maxStrokeWidth); ok {
				st.StrokeWidth, st.hasWidth = w, true
			}
		}
	}
	return st
}

// merge returns s with any field it lacks taken from base.
func (s Style) merge(base Style) Style {
	if s.Radius == 0 {
		s.Radius = base.Radius
	}
	if s.Color == "" {
		s.Color = base.Color
	}
	if s.StrokeColor == "" {
		s.StrokeColor = base.StrokeColor
	}
	if !s.hasWidth {
		s.StrokeWidth, s.hasWidth = base.StrokeWidth, base.hasWidth
	}
	return s
}

// splitAxis splits "Low --> High" into its two ends.
func splitAxis(s string) (lo, hi string) {
	if l, r, ok := strings.Cut(s, "-->"); ok {
		return unquote(strings.TrimSpace(l)), unquote(strings.TrimSpace(r))
	}
	return unquote(strings.TrimSpace(s)), ""
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
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
