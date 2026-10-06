package goldentest

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// Box is the estimated extent of one line of text, in the picture's own
// coordinates (every transform applied).
type Box struct {
	Text           string
	X0, Y0, X1, Y1 float64
}

func (b Box) String() string {
	return fmt.Sprintf("%q [%.1f,%.1f]-[%.1f,%.1f]", b.Text, b.X0, b.Y0, b.X1, b.Y1)
}

// affine is the matrix [a c e; b d f; 0 0 1].
type affine struct{ a, b, c, d, e, f float64 }

var identity = affine{a: 1, d: 1}

func (m affine) mul(n affine) affine {
	return affine{
		a: m.a*n.a + m.c*n.b, b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d, d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e, f: m.b*n.e + m.d*n.f + m.f,
	}
}

func (m affine) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// parseTransform reads the translate, rotate and scale lists the renderers
// write; anything else is an error, so a new kind is noticed.
func parseTransform(s string) (affine, error) {
	m := identity
	s = strings.TrimSpace(s)
	for s != "" {
		open := strings.IndexByte(s, '(')
		end := strings.IndexByte(s, ')')
		if open < 0 || end < open {
			return m, fmt.Errorf("bad transform %q", s)
		}
		name := strings.TrimSpace(s[:open])
		var v []float64
		for _, f := range strings.FieldsFunc(s[open+1:end], func(r rune) bool { return r == ',' || r == ' ' }) {
			x, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return m, fmt.Errorf("bad transform %q", s)
			}
			v = append(v, x)
		}
		var t affine
		switch {
		case name == "translate" && len(v) == 1:
			t = affine{a: 1, d: 1, e: v[0]}
		case name == "translate" && len(v) == 2:
			t = affine{a: 1, d: 1, e: v[0], f: v[1]}
		case name == "scale" && len(v) == 1:
			t = affine{a: v[0], d: v[0]}
		case name == "scale" && len(v) == 2:
			t = affine{a: v[0], d: v[1]}
		case name == "rotate" && (len(v) == 1 || len(v) == 3):
			r := v[0] * math.Pi / 180
			t = affine{a: math.Cos(r), b: math.Sin(r), c: -math.Sin(r), d: math.Cos(r)}
			if len(v) == 3 {
				t = affine{a: 1, d: 1, e: v[1], f: v[2]}.mul(t).mul(affine{a: 1, d: 1, e: -v[1], f: -v[2]})
			}
		default:
			return m, fmt.Errorf("unsupported transform %q", s)
		}
		m = m.mul(t)
		s = strings.TrimLeft(s[end+1:], " ,")
	}
	return m, nil
}

type textState struct {
	m              affine
	size           float64
	bold           bool
	anchor, family string
	baseline       string
}

// TextBoxes estimates the box of every line of text in svg: each <tspan>,
// and the text written directly in a <text>. Widths use the library's own
// metrics (bold where the text is bold), heights the font's ascent and
// descent.
func TextBoxes(svg []byte) ([]Box, error) {
	d := xml.NewDecoder(strings.NewReader(string(svg)))
	stack := []textState{{m: identity, size: 16, anchor: "start", family: "sans-serif"}}
	var boxes []Box
	type pos struct{ x, y float64 }
	var cur pos     // current text position inside a <text>
	inText := 0
	var pending *struct {
		st   textState
		x, y float64
		text strings.Builder
	}
	flush := func() {
		if pending == nil {
			return
		}
		t := strings.TrimSpace(pending.text.String())
		if t != "" {
			boxes = append(boxes, lineBox(t, pending.x, pending.y, pending.st))
		}
		pending = nil
	}
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			st := stack[len(stack)-1]
			attr := map[string]string{}
			for _, a := range t.Attr {
				attr[a.Name.Local] = a.Value
			}
			if v, ok := attr["transform"]; ok {
				m, err := parseTransform(v)
				if err != nil {
					return nil, err
				}
				st.m = st.m.mul(m)
			}
			if v, ok := attr["font-size"]; ok {
				if f, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64); err == nil {
					st.size = f
				}
			}
			if v, ok := attr["font-weight"]; ok {
				n, _ := strconv.Atoi(v)
				st.bold = v == "bold" || v == "bolder" || n >= 600
			}
			if v, ok := attr["text-anchor"]; ok {
				st.anchor = v
			}
			if v, ok := attr["font-family"]; ok {
				st.family = v
			}
			if v, ok := attr["dominant-baseline"]; ok {
				st.baseline = v
			}
			num := func(k string, def float64) float64 {
				if v, ok := attr[k]; ok {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						return f
					}
				}
				return def
			}
			switch t.Name.Local {
			case "text":
				flush()
				inText++
				cur = pos{num("x", 0), num("y", 0)}
				pending = &struct {
					st   textState
					x, y float64
					text strings.Builder
				}{st: st, x: cur.x, y: cur.y}
			case "tspan":
				flush()
				cur = pos{num("x", cur.x), num("y", cur.y) + num("dy", 0)}
				pending = &struct {
					st   textState
					x, y float64
					text strings.Builder
				}{st: st, x: cur.x, y: cur.y}
			}
			stack = append(stack, st)
		case xml.EndElement:
			if t.Name.Local == "tspan" || t.Name.Local == "text" {
				flush()
				if t.Name.Local == "text" {
					inText--
				}
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if inText > 0 && pending != nil {
				pending.text.Write(t)
			}
		}
	}
	return boxes, nil
}

func lineBox(s string, x, y float64, st textState) Box {
	face := svgutil.FaceFor(st.family)
	w := face.Width(s, st.size)
	if st.bold {
		w = face.BoldWidth(s, st.size)
	}
	switch st.anchor {
	case "middle":
		x -= w / 2
	case "end":
		x -= w
	}
	asc, desc := 0.75*st.size, 0.2*st.size
	switch st.baseline {
	case "middle", "central":
		asc, desc = 0.5*st.size, 0.5*st.size
	case "hanging", "text-before-edge":
		asc, desc = 0, 0.95*st.size
	}
	b := Box{Text: s, X0: math.Inf(1), Y0: math.Inf(1), X1: math.Inf(-1), Y1: math.Inf(-1)}
	for _, c := range [4][2]float64{{x, y - asc}, {x + w, y - asc}, {x, y + desc}, {x + w, y + desc}} {
		px, py := st.m.apply(c[0], c[1])
		b.X0, b.Y0 = math.Min(b.X0, px), math.Min(b.Y0, py)
		b.X1, b.Y1 = math.Max(b.X1, px), math.Max(b.Y1, py)
	}
	return b
}

// Canvas returns the picture's viewBox, or its width and height when it has
// none.
func Canvas(svg []byte) (x, y, w, h float64, err error) {
	d := xml.NewDecoder(strings.NewReader(string(svg)))
	for {
		tok, err := d.Token()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "svg" {
			var wa, ha string
			for _, a := range se.Attr {
				switch a.Name.Local {
				case "viewBox":
					var v [4]float64
					f := strings.Fields(strings.ReplaceAll(a.Value, ",", " "))
					if len(f) == 4 {
						for i := range f {
							v[i], _ = strconv.ParseFloat(f[i], 64)
						}
						return v[0], v[1], v[2], v[3], nil
					}
				case "width":
					wa = a.Value
				case "height":
					ha = a.Value
				}
			}
			w, _ := strconv.ParseFloat(wa, 64)
			h, _ := strconv.ParseFloat(ha, 64)
			return 0, 0, w, h, nil
		}
	}
}

// OffCanvas returns the lines of text that reach more than tol outside the
// picture's canvas.
func OffCanvas(svg []byte, tol float64) ([]Box, error) {
	x, y, w, h, err := Canvas(svg)
	if err != nil {
		return nil, err
	}
	boxes, err := TextBoxes(svg)
	if err != nil {
		return nil, err
	}
	var out []Box
	for _, b := range boxes {
		if b.X0 < x-tol || b.Y0 < y-tol || b.X1 > x+w+tol || b.Y1 > y+h+tol {
			out = append(out, b)
		}
	}
	return out, nil
}

// TextOverlaps returns the pairs of text lines whose boxes overlap by more
// than tol in both directions.
func TextOverlaps(svg []byte, tol float64) ([][2]Box, error) {
	boxes, err := TextBoxes(svg)
	if err != nil {
		return nil, err
	}
	var out [][2]Box
	for i, a := range boxes {
		for _, b := range boxes[i+1:] {
			if math.Min(a.X1, b.X1)-math.Max(a.X0, b.X0) > tol && math.Min(a.Y1, b.Y1)-math.Max(a.Y0, b.Y0) > tol {
				out = append(out, [2]Box{a, b})
			}
		}
	}
	return out, nil
}
