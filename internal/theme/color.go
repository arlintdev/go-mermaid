package theme

import (
	"fmt"
	"math"

	"github.com/arlintdev/go-mermaid/internal/cssval"
)

// Mix returns the color a fraction t of the way from a to b, as #rrggbb,
// or fallback when either is not a color.
func Mix(a, b string, t float64, fallback string) string {
	ra, ga, ba, ok1 := cssval.RGB(a)
	rb, gb, bb, ok2 := cssval.RGB(b)
	if !ok1 || !ok2 {
		return fallback
	}
	c := func(x, y uint8) int { return int(x) + int(float64(int(y)-int(x))*t+0.5) }
	return fmt.Sprintf("#%02x%02x%02x", c(ra, rb), c(ga, gb), c(ba, bb))
}

// Darken returns c two thirds as bright, as #rrggbb, or c itself when it is
// not a color.
func Darken(c string) string {
	r, g, b, ok := cssval.RGB(c)
	if !ok {
		return c
	}
	f := func(x uint8) uint { return uint(x) * 2 / 3 }
	return fmt.Sprintf("#%02x%02x%02x", f(r), f(g), f(b))
}

// Luminance returns the relative luminance of c (0 for black, 1 for white),
// and false when c is not a color.
func Luminance(c string) (float64, bool) {
	r, g, b, ok := cssval.RGB(c)
	if !ok {
		return 0, false
	}
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b), true
}

// IsDark reports whether c is a dark color, one on which light text reads
// better than dark text.
func IsDark(c string) bool {
	l, ok := Luminance(c)
	return ok && l < 0.18
}

// Contrast returns the contrast ratio of two colors, from 1 to 21, or 21
// when either is not a color (so callers keep what they have).
func Contrast(a, b string) float64 {
	la, ok1 := Luminance(a)
	lb, ok2 := Luminance(b)
	if !ok1 || !ok2 {
		return 21
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TextOn returns the color to write text in on fill: text when it reads
// well there, else whichever of dark and light contrasts more with fill. A
// diagram's style line may set a fill without a text color; this keeps the
// theme's text readable on it.
func TextOn(fill, text string) string {
	if Contrast(text, fill) >= 3 {
		return text
	}
	dark, light := palettes["default"].Text, palettes["dark"].Text
	if Contrast(dark, fill) >= Contrast(light, fill) {
		return dark
	}
	return light
}

// BackdropOpacity returns the opacity to draw a color the diagram source
// chose for a background at, when text in the theme's text color sits on
// it: 1 when the text reads there, else low enough that the color only
// tints the page. A source written for a light page asks for pale
// backgrounds, which would hide a dark theme's light text.
func BackdropOpacity(fill, text string) float64 {
	if Contrast(text, fill) >= 3 {
		return 1
	}
	return 0.3
}
