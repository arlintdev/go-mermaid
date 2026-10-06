package timeline

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgid"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls timeline appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// hues of Mermaid's default colour scale; each section (or, without
// sections, each period) takes the next one.
var hues = []float64{240, 60, 80, 270, 300, 330, 0, 30, 90, 150, 180, 210}

const (
	colW     = 180.0 // a period column, box included
	colGap   = 12.0
	boxPadX  = 12.0
	boxPadY  = 9.0
	rowGap   = 14.0 // between section row, period row, axis and events
	eventGap = 8.0
	axisLead = 24.0 // axis run-in before the first column
)

// Render parses and renders timeline source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	if o.Title == "" {
		o.Title = d.Title
	}
	return svg(d, o, svgid.Prefix(src)), nil
}

type colour struct{ fill, accent, text, light, lightAccent string }

func colourFor(i int) colour {
	h := hues[i%len(hues)]
	l := 0.7627
	if h == 60 {
		l = 0.7353
	}
	r, g, b := hsl(h, 1, l)
	return colour{
		fill:        hex(r, g, b),
		accent:      hex(shade(r, -0.28), shade(g, -0.28), shade(b, -0.28)),
		text:        "#1f1f1f",
		light:       hex(shade(r, 0.45), shade(g, 0.45), shade(b, 0.45)),
		lightAccent: hex(r, g, b),
	}
}

func svg(d *Diagram, o RenderOptions, id string) []byte {
	pal := theme.For(o.Theme)
	pad := o.Padding
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	face := svgutil.FaceFor(o.FontFace)
	lh := fs * 1.25
	textW := colW - 2*boxPadX
	boxH := func(lines int) float64 { return float64(lines)*lh + 2*boxPadY }

	periods := d.Periods()
	sectioned := false
	for _, s := range d.Sections {
		if s.Name != "" {
			sectioned = true
		}
	}

	// Row heights: every box in a row is as tall as the tallest, as Mermaid
	// draws them.
	secLines, perLines := 1, 1
	for _, s := range d.Sections {
		if s.Name != "" {
			span := float64(len(s.Periods))*(colW+colGap) - colGap
			secLines = max(secLines, len(wrap(face, s.Name, fs, max(span, colW)-2*boxPadX)))
		}
	}
	for _, p := range periods {
		perLines = max(perLines, len(wrap(face, p.Time, fs, textW)))
	}

	titleSize := fs * 1.6
	y := pad
	if o.Title != "" {
		y += titleSize*1.3 + rowGap
	}
	secY := y
	if sectioned {
		y += boxH(secLines) + rowGap
	}
	perY := y
	axisY := perY + boxH(perLines) + rowGap*2
	evTop := axisY + rowGap*2

	n := max(len(periods), 1)
	left := pad + axisLead
	w := left + float64(n)*(colW+colGap) - colGap + axisLead + pad
	if o.Title != "" {
		w = max(w, pad*2+face.Width(o.Title, titleSize)*1.1)
	}

	var b strings.Builder
	ff := svgutil.Esc(fontFamily(o.FontFace))

	// Event columns, to size the canvas before writing it.
	bottom := axisY + rowGap*2
	evLines := make([][][]string, len(periods))
	for i, p := range periods {
		yy := evTop
		for _, ev := range p.Events {
			ls := wrap(face, ev, fs, textW)
			evLines[i] = append(evLines[i], ls)
			yy += boxH(len(ls)) + eventGap
		}
		bottom = max(bottom, yy+rowGap)
	}
	h := bottom + 10 + pad

	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), ff, svgutil.Num(fs))
	fmt.Fprintf(&b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	edge := svgutil.Esc(pal.Edge)
	fmt.Fprintf(&b, `  <defs><marker id="%s-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" markerUnits="strokeWidth" orient="auto"><path d="M0,0 L10,5 L0,10 z" fill="%s"/></marker>`+
		`<marker id="%s-arrow-sm" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="5" markerHeight="5" orient="auto"><path d="M0,0 L10,5 L0,10 z" fill="%s"/></marker></defs>`+"\n",
		id, edge, id, edge)
	if o.Title != "" {
		fmt.Fprintf(&b, `  <text x="%s" y="%s" fill="%s" font-size="%s" font-weight="bold">%s</text>`+"\n",
			svgutil.Num(pad), svgutil.Num(pad+titleSize), svgutil.Esc(pal.Text), svgutil.Num(titleSize), svgutil.Esc(o.Title))
	}

	colX := func(i int) float64 { return left + float64(i)*(colW+colGap) }

	// Dashed drop lines first, so the boxes sit on top of them.
	for i := range periods {
		cx := colX(i) + colW/2
		end := bottom - rowGap + 6
		fmt.Fprintf(&b, `  <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.5" stroke-dasharray="5 5" marker-end="url(#%s-arrow-sm)"/>`+"\n",
			svgutil.Num(cx), svgutil.Num(perY+boxH(perLines)), svgutil.Num(cx), svgutil.Num(end), edge, id)
	}
	fmt.Fprintf(&b, `  <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="3" marker-end="url(#%s-arrow)"/>`+"\n",
		svgutil.Num(pad), svgutil.Num(axisY), svgutil.Num(w-pad), svgutil.Num(axisY), edge, id)

	// Sections, periods, events.
	idx := 0
	for si, sec := range d.Sections {
		for pi, p := range sec.Periods {
			c := colourFor(si)
			if !sectioned {
				c = colourFor(idx)
			}
			x := colX(idx)
			if sectioned && pi == 0 && sec.Name != "" {
				span := float64(len(sec.Periods))*(colW+colGap) - colGap
				drawBox(&b, x, secY, span, boxH(secLines), c.fill, c.accent, c.text, wrap(face, sec.Name, fs, span-2*boxPadX), true, lh, fs)
			}
			drawBox(&b, x, perY, colW, boxH(perLines), c.fill, c.accent, c.text, wrap(face, p.Time, fs, textW), !sectioned, lh, fs)
			ey := evTop
			for _, ls := range evLines[idx] {
				drawBox(&b, x, ey, colW, boxH(len(ls)), c.light, c.lightAccent, c.text, ls, false, lh, fs)
				ey += boxH(len(ls)) + eventGap
			}
			idx++
		}
	}

	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// drawBox writes a rounded box with a darker rule along its bottom edge and
// its lines of text centred in it, as Mermaid draws timeline nodes.
func drawBox(b *strings.Builder, x, y, w, h float64, fill, accent, text string, lines []string, bold bool, lh, fs float64) {
	fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" rx="5" fill="%s"/>`+"\n",
		svgutil.Num(x), svgutil.Num(y), svgutil.Num(w), svgutil.Num(h), fill)
	fmt.Fprintf(b, `  <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="3"/>`+"\n",
		svgutil.Num(x+1), svgutil.Num(y+h-1.5), svgutil.Num(x+w-1), svgutil.Num(y+h-1.5), accent)
	weight := ""
	if bold {
		weight = ` font-weight="bold"`
	}
	ty := y + h/2 - float64(len(lines)-1)*lh/2 + fs*0.35
	fmt.Fprintf(b, `  <text fill="%s" text-anchor="middle"%s>`, text, weight)
	for i, ln := range lines {
		fmt.Fprintf(b, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(x+w/2), svgutil.Num(ty+float64(i)*lh), svgutil.Esc(ln))
	}
	b.WriteString("</text>\n")
}

// wrap breaks s into lines no wider than maxW, at spaces where it can and
// inside a word only when the word alone is too wide.
func wrap(face svgutil.Face, s string, size, maxW float64) []string {
	var lines []string
	for _, para := range svgutil.SplitLines(s) {
		cur := ""
		for _, wd := range strings.Fields(para) {
			for face.Width(wd, size) > maxW && len([]rune(wd)) > 1 {
				if cur != "" {
					lines = append(lines, cur)
					cur = ""
				}
				// The longest prefix that fits, measured rune by rune so a
				// very long word costs linear time.
				r := []rune(wd)
				k, w := 0, 0.0
				for k < len(r) {
					cw := face.Width(string(r[k]), size)
					if k > 0 && w+cw > maxW {
						break
					}
					w += cw
					k++
				}
				lines = append(lines, string(r[:k]))
				wd = string(r[k:])
			}
			try := wd
			if cur != "" {
				try = cur + " " + wd
			}
			if cur != "" && face.Width(try, size) > maxW {
				lines = append(lines, cur)
				cur = wd
				continue
			}
			cur = try
		}
		lines = append(lines, cur)
	}
	return lines
}

var plainFont = regexp.MustCompile(`^[A-Za-z0-9 ,'"_-]{1,200}$`)

// fontFamily returns face when it is a plain font list, else sans-serif:
// a font option is written into an attribute, so it must carry nothing else.
func fontFamily(face string) string {
	l := strings.ToLower(face)
	if !plainFont.MatchString(face) || strings.Contains(l, "javascript") || strings.Contains(l, "expression") {
		return "sans-serif"
	}
	return face
}

// hsl converts a hue (degrees), saturation and lightness (0..1) to RGB.
func hsl(h, s, l float64) (r, g, b float64) {
	c := (1 - math.Abs(2*l-1)) * s
	hp := h / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	switch {
	case hp < 1:
		r, g = c, x
	case hp < 2:
		r, g = x, c
	case hp < 3:
		g, b = c, x
	case hp < 4:
		g, b = x, c
	case hp < 5:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := l - c/2
	return r + m, g + m, b + m
}

// shade moves a channel toward white (t > 0) or black (t < 0).
func shade(v, t float64) float64 {
	if t > 0 {
		return v + (1-v)*t
	}
	return v * (1 + t)
}

func hex(r, g, b float64) string {
	c := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", c(r), c(g), c(b))
}
