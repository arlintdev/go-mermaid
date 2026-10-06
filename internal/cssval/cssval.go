// Package cssval validates the style values a diagram source may give
// (colours, lengths, dash patterns, opacities) before a renderer writes them
// into an SVG presentation attribute. Anything that is not a plain value of
// the expected kind is refused, so a style line can never carry markup, a
// url() reference or a script into the picture.
package cssval

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	hexColor  = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	funcColor = regexp.MustCompile(`^(?:rgb|rgba|hsl|hsla)\(\s*-?[0-9.]+(?:deg|%)?\s*(?:[,\s]\s*-?[0-9.]+%?\s*){2}(?:[,/]\s*[0-9.]+%?\s*)?\)$`)
	length    = regexp.MustCompile(`^[0-9]*\.?[0-9]+(?:px|em|rem|pt|%)?$`)
)

// Color returns s as a colour fit for a fill or stroke attribute: a hex
// colour, an rgb()/hsl() colour with plain numbers, or a CSS colour name.
// It reports false for anything else.
func Color(s string) (string, bool) {
	s = strings.TrimSpace(s)
	switch {
	case hexColor.MatchString(s):
		return strings.ToLower(s), true
	case funcColor.MatchString(strings.ToLower(s)):
		return strings.ToLower(s), true
	}
	l := strings.ToLower(s)
	if namedColors[l] {
		return l, true
	}
	return "", false
}

// Length returns s as a non-negative length (a number with an optional px,
// em, rem, pt or % unit) and reports whether it is one.
func Length(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) > 16 || !length.MatchString(s) {
		return "", false
	}
	return s, true
}

// Pixels parses a length given in pixels (with or without "px") and reports
// whether it is a finite, non-negative number no larger than max.
func Pixels(s string, max float64) (float64, bool) {
	s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "px")
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f != f || f < 0 || f > max {
		return 0, false
	}
	return f, true
}

// Dash returns s as a stroke-dasharray: one to eight non-negative numbers
// separated by commas or spaces, or "none".
func Dash(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "none") {
		return "none", true
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	if len(parts) == 0 || len(parts) > 8 {
		return "", false
	}
	for i, p := range parts {
		v, ok := Length(p)
		if !ok || strings.HasSuffix(v, "%") {
			return "", false
		}
		parts[i] = v
	}
	return strings.Join(parts, " "), true
}

// Opacity returns s as an opacity between 0 and 1 (or a percentage).
func Opacity(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		f, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil || f < 0 || f > 100 {
			return "", false
		}
		return strconv.FormatFloat(f/100, 'f', -1, 64), true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || f > 1 {
		return "", false
	}
	return strconv.FormatFloat(f, 'f', -1, 64), true
}

// FontWeight returns s as a font-weight keyword or number.
func FontWeight(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "normal", "bold", "bolder", "lighter", "100", "200", "300", "400", "500", "600", "700", "800", "900":
		return s, true
	}
	return "", false
}

// FontStyle returns s as a font-style keyword.
func FontStyle(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "normal", "italic", "oblique":
		return s, true
	}
	return "", false
}

// namedColors is the CSS Color Module Level 4 named-colour list plus
// transparent.
var namedColors = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(`aliceblue antiquewhite aqua aquamarine azure beige bisque black
blanchedalmond blue blueviolet brown burlywood cadetblue chartreuse chocolate coral
cornflowerblue cornsilk crimson cyan darkblue darkcyan darkgoldenrod darkgray darkgreen
darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon
darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink
deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro
ghostwhite gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo
ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan
lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen
lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen
magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen
mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream
mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid
palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum
powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown
seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen
steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow
yellowgreen transparent none`) {
		m[n] = true
	}
	return m
}()
