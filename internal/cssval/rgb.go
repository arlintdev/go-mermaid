package cssval

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	rgbFunc = regexp.MustCompile(`^rgba?\(\s*([0-9.]+%?)\s*[,\s]\s*([0-9.]+%?)\s*[,\s]\s*([0-9.]+%?)\s*(?:[,/]\s*[0-9.]+%?\s*)?\)$`)
	hslFunc = regexp.MustCompile(`^hsla?\(\s*(-?[0-9.]+)(?:deg)?\s*[,\s]\s*([0-9.]+)%\s*[,\s]\s*([0-9.]+)%\s*(?:[,/]\s*[0-9.]+%?\s*)?\)$`)
)

// RGB returns the red, green and blue channels of a colour that Color
// accepts: a hex colour, an rgb() or hsl() colour, or a CSS colour name.
// Any alpha is ignored. It reports false for anything else, including
// "transparent" and "none".
func RGB(c string) (r, g, b uint8, ok bool) {
	c = strings.ToLower(strings.TrimSpace(c))
	if strings.HasPrefix(c, "#") {
		h := c[1:]
		switch len(h) {
		case 3, 4:
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		case 6, 8:
			h = h[:6]
		default:
			return 0, 0, 0, false
		}
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		return uint8(v >> 16), uint8(v >> 8), uint8(v), true
	}
	if m := rgbFunc.FindStringSubmatch(c); m != nil {
		var ch [3]uint8
		for i, s := range m[1:4] {
			pct := strings.HasSuffix(s, "%")
			f, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
			if err != nil {
				return 0, 0, 0, false
			}
			if pct {
				f = f * 255 / 100
			}
			ch[i] = uint8(math.Round(math.Max(0, math.Min(255, f))))
		}
		return ch[0], ch[1], ch[2], true
	}
	if m := hslFunc.FindStringSubmatch(c); m != nil {
		h, err1 := strconv.ParseFloat(m[1], 64)
		s, err2 := strconv.ParseFloat(m[2], 64)
		l, err3 := strconv.ParseFloat(m[3], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, 0, 0, false
		}
		rf, gf, bf := HSL(h, math.Min(s, 100)/100, math.Min(l, 100)/100)
		return uint8(math.Round(rf * 255)), uint8(math.Round(gf * 255)), uint8(math.Round(bf * 255)), true
	}
	if v, ok := namedRGB[c]; ok {
		return uint8(v >> 16), uint8(v >> 8), uint8(v), true
	}
	return 0, 0, 0, false
}

// HSL converts a hue in degrees, and a saturation and lightness between 0
// and 1, to red, green and blue between 0 and 1.
func HSL(h, s, l float64) (r, g, b float64) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	h /= 360
	if s == 0 {
		return l, l, l
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	return hue(p, q, h+1.0/3), hue(p, q, h), hue(p, q, h-1.0/3)
}

func hue(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}

// namedRGB holds the value of every CSS colour name, as a browser computes it.
var namedRGB = map[string]uint32{
	"aliceblue": 0xf0f8ff, "antiquewhite": 0xfaebd7, "aqua": 0x00ffff, "aquamarine": 0x7fffd4,
	"azure": 0xf0ffff, "beige": 0xf5f5dc, "bisque": 0xffe4c4, "black": 0x000000,
	"blanchedalmond": 0xffebcd, "blue": 0x0000ff, "blueviolet": 0x8a2be2, "brown": 0xa52a2a,
	"burlywood": 0xdeb887, "cadetblue": 0x5f9ea0, "chartreuse": 0x7fff00, "chocolate": 0xd2691e,
	"coral": 0xff7f50, "cornflowerblue": 0x6495ed, "cornsilk": 0xfff8dc, "crimson": 0xdc143c,
	"cyan": 0x00ffff, "darkblue": 0x00008b, "darkcyan": 0x008b8b, "darkgoldenrod": 0xb8860b,
	"darkgray": 0xa9a9a9, "darkgreen": 0x006400, "darkgrey": 0xa9a9a9, "darkkhaki": 0xbdb76b,
	"darkmagenta": 0x8b008b, "darkolivegreen": 0x556b2f, "darkorange": 0xff8c00,
	"darkorchid": 0x9932cc, "darkred": 0x8b0000, "darksalmon": 0xe9967a, "darkseagreen": 0x8fbc8f,
	"darkslateblue": 0x483d8b, "darkslategray": 0x2f4f4f, "darkslategrey": 0x2f4f4f,
	"darkturquoise": 0x00ced1, "darkviolet": 0x9400d3, "deeppink": 0xff1493, "deepskyblue": 0x00bfff,
	"dimgray": 0x696969, "dimgrey": 0x696969, "dodgerblue": 0x1e90ff, "firebrick": 0xb22222,
	"floralwhite": 0xfffaf0, "forestgreen": 0x228b22, "fuchsia": 0xff00ff, "gainsboro": 0xdcdcdc,
	"ghostwhite": 0xf8f8ff, "gold": 0xffd700, "goldenrod": 0xdaa520, "gray": 0x808080,
	"green": 0x008000, "greenyellow": 0xadff2f, "grey": 0x808080, "honeydew": 0xf0fff0,
	"hotpink": 0xff69b4, "indianred": 0xcd5c5c, "indigo": 0x4b0082, "ivory": 0xfffff0,
	"khaki": 0xf0e68c, "lavender": 0xe6e6fa, "lavenderblush": 0xfff0f5, "lawngreen": 0x7cfc00,
	"lemonchiffon": 0xfffacd, "lightblue": 0xadd8e6, "lightcoral": 0xf08080, "lightcyan": 0xe0ffff,
	"lightgoldenrodyellow": 0xfafad2, "lightgray": 0xd3d3d3, "lightgreen": 0x90ee90,
	"lightgrey": 0xd3d3d3, "lightpink": 0xffb6c1, "lightsalmon": 0xffa07a, "lightseagreen": 0x20b2aa,
	"lightskyblue": 0x87cefa, "lightslategray": 0x778899, "lightslategrey": 0x778899,
	"lightsteelblue": 0xb0c4de, "lightyellow": 0xffffe0, "lime": 0x00ff00, "limegreen": 0x32cd32,
	"linen": 0xfaf0e6, "magenta": 0xff00ff, "maroon": 0x800000, "mediumaquamarine": 0x66cdaa,
	"mediumblue": 0x0000cd, "mediumorchid": 0xba55d3, "mediumpurple": 0x9370db,
	"mediumseagreen": 0x3cb371, "mediumslateblue": 0x7b68ee, "mediumspringgreen": 0x00fa9a,
	"mediumturquoise": 0x48d1cc, "mediumvioletred": 0xc71585, "midnightblue": 0x191970,
	"mintcream": 0xf5fffa, "mistyrose": 0xffe4e1, "moccasin": 0xffe4b5, "navajowhite": 0xffdead,
	"navy": 0x000080, "oldlace": 0xfdf5e6, "olive": 0x808000, "olivedrab": 0x6b8e23,
	"orange": 0xffa500, "orangered": 0xff4500, "orchid": 0xda70d6, "palegoldenrod": 0xeee8aa,
	"palegreen": 0x98fb98, "paleturquoise": 0xafeeee, "palevioletred": 0xdb7093,
	"papayawhip": 0xffefd5, "peachpuff": 0xffdab9, "peru": 0xcd853f, "pink": 0xffc0cb,
	"plum": 0xdda0dd, "powderblue": 0xb0e0e6, "purple": 0x800080, "rebeccapurple": 0x663399,
	"red": 0xff0000, "rosybrown": 0xbc8f8f, "royalblue": 0x4169e1, "saddlebrown": 0x8b4513,
	"salmon": 0xfa8072, "sandybrown": 0xf4a460, "seagreen": 0x2e8b57, "seashell": 0xfff5ee,
	"sienna": 0xa0522d, "silver": 0xc0c0c0, "skyblue": 0x87ceeb, "slateblue": 0x6a5acd,
	"slategray": 0x708090, "slategrey": 0x708090, "snow": 0xfffafa, "springgreen": 0x00ff7f,
	"steelblue": 0x4682b4, "tan": 0xd2b48c, "teal": 0x008080, "thistle": 0xd8bfd8, "tomato": 0xff6347,
	"turquoise": 0x40e0d0, "violet": 0xee82ee, "wheat": 0xf5deb3, "white": 0xffffff,
	"whitesmoke": 0xf5f5f5, "yellow": 0xffff00, "yellowgreen": 0x9acd32,
}
