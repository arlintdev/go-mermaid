package cssval

import (
	"strconv"
	"strings"
)

// Style is what a style or classDef line of a diagram says about a shape,
// each value already validated. An empty field was not given.
type Style struct {
	Fill, Stroke, StrokeWidth, Dash, Color, FontWeight string
}

// Parse reads CSS declarations such as "fill:#f9f,stroke:#333,
// stroke-width:4px", separated by commas or semicolons. It keeps only the
// properties a shape can take, and only values that validate; the rest is
// dropped. background and background-color stand for fill, border-color
// for stroke.
func Parse(decls string) Style {
	var st Style
	for _, part := range strings.FieldsFunc(decls, func(r rune) bool { return r == ',' || r == ';' }) {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "fill", "background", "background-color":
			if c, ok := Color(v); ok {
				st.Fill = c
			}
		case "stroke", "border-color":
			if c, ok := Color(v); ok {
				st.Stroke = c
			}
		case "color":
			if c, ok := Color(v); ok {
				st.Color = c
			}
		case "stroke-width":
			if w, ok := Pixels(v, 20); ok {
				st.StrokeWidth = strconv.FormatFloat(w, 'f', -1, 64)
			}
		case "stroke-dasharray":
			if d, ok := Dash(v); ok {
				st.Dash = d
			}
		case "font-weight":
			if fw, ok := FontWeight(v); ok {
				st.FontWeight = fw
			}
		}
	}
	return st
}

// Over returns s with every field it leaves empty taken from base.
func (s Style) Over(base Style) Style {
	pick := func(a, b string) string {
		if a != "" {
			return a
		}
		return b
	}
	return Style{
		Fill: pick(s.Fill, base.Fill), Stroke: pick(s.Stroke, base.Stroke), StrokeWidth: pick(s.StrokeWidth, base.StrokeWidth),
		Dash: pick(s.Dash, base.Dash), Color: pick(s.Color, base.Color), FontWeight: pick(s.FontWeight, base.FontWeight),
	}
}
