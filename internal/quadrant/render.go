package quadrant

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls quadrant chart appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

const (
	chartSize     = 500.0 // Mermaid's default quadrant chart width and height
	defaultRadius = 5.0
	edge          = 5.0 // gap between the canvas edge and the axis text
)

// Render parses and renders quadrant chart source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	if o.Title == "" {
		o.Title = d.Title
	}
	return svg(d, o), nil
}

type drawer struct {
	b    strings.Builder
	face svgutil.Face
}

// text writes s as one <text>, wrapped onto further lines below the first
// when it is wider than maxW.
func (dr *drawer) text(s string, x, y, size, maxW float64, fill, anchor, extra string) {
	lines := dr.face.Wrap(s, size, maxW)
	lh := size * 1.2
	fmt.Fprintf(&dr.b, `  <text fill="%s" font-size="%s" text-anchor="%s"%s>`, fill, svgutil.Num(size), anchor, extra)
	for i, ln := range lines {
		fmt.Fprintf(&dr.b, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(x), svgutil.Num(y+float64(i)*lh), svgutil.Esc(ln))
	}
	dr.b.WriteString("</text>\n")
}

func svg(d *Diagram, o RenderOptions) []byte {
	pal := theme.For(o.Theme)
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	dr := &drawer{face: svgutil.FaceFor(o.FontFace)}
	pad := o.Padding

	titleSize := fs * 1.25
	pointSize := fs * 0.8
	axisSize := fs

	// Plot box: room on the left for the rotated y-axis labels, below for
	// the x-axis labels, above for the title.
	left := pad + edge + axisSize*1.4
	if d.YBottom == "" && d.YTop == "" {
		left = pad + edge
	}
	top := pad + edge
	if o.Title != "" {
		top += titleSize*1.2 + 2*edge
	}
	plotW := chartSize - (left - pad) - edge
	plotH := chartSize - (top - pad) - edge
	if d.XLeft != "" || d.XRight != "" {
		plotH -= axisSize * 1.6
	}
	w := chartSize + 2*pad
	h := chartSize + 2*pad
	halfW, halfH := plotW/2, plotH/2
	midX, midY := left+halfW, top+halfH

	b := &dr.b
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`,
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	b.WriteByte('\n')
	fmt.Fprintf(b, `  <rect width="100%%" height="100%%" fill="%s"/>`, svgutil.Esc(pal.Background))
	b.WriteByte('\n')
	text := svgutil.Esc(pal.Text)
	if o.Title != "" {
		dr.text(o.Title, w/2, pad+edge+titleSize*0.85, titleSize, chartSize-2*edge, text, "middle", "")
	}

	// Quadrant 1 (top right) takes the theme's node fill; the others are
	// progressively lighter, as Mermaid shades them.
	fill := svgutil.Esc(pal.NodeFill)
	bg := pal.Background
	fills := [4]string{fill, mix(pal.NodeFill, bg, 0.27, fill), mix(pal.NodeFill, bg, 0.53, fill), mix(pal.NodeFill, bg, 0.79, fill)}
	origin := [4][2]float64{{midX, top}, {left, top}, {left, midY}, {midX, midY}}
	for i, q := range origin {
		fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`,
			svgutil.Num(q[0]), svgutil.Num(q[1]), svgutil.Num(halfW), svgutil.Num(halfH), fills[i])
		b.WriteByte('\n')
	}
	border := mix(pal.NodeStroke, pal.NodeFill, 0.6, svgutil.Esc(pal.NodeStroke))
	fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" fill="none" stroke="%s" stroke-width="2"/>`,
		svgutil.Num(left), svgutil.Num(top), svgutil.Num(plotW), svgutil.Num(plotH), border)
	b.WriteByte('\n')
	fmt.Fprintf(b, `  <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/><line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`,
		svgutil.Num(midX), svgutil.Num(top), svgutil.Num(midX), svgutil.Num(top+plotH), border,
		svgutil.Num(left), svgutil.Num(midY), svgutil.Num(left+plotW), svgutil.Num(midY), border)
	b.WriteByte('\n')

	// Quadrant names sit at the top of their quadrant, or in its middle when
	// the chart has no points (as Mermaid does).
	for i, q := range origin {
		name := d.Quadrant[i]
		if name == "" {
			continue
		}
		y := q[1] + edge + fs*0.85
		if len(d.Points) == 0 {
			n := len(dr.face.Wrap(name, fs, halfW-2*edge))
			y = q[1] + halfH/2 - float64(n-1)*fs*0.6 + fs*0.35
		}
		dr.text(name, q[0]+halfW/2, y, fs, halfW-2*edge, text, "middle", "")
	}

	// Axis labels: each end centred under (or beside) its half of the plot;
	// a lone label spans the whole axis.
	ax := top + plotH + edge + axisSize*0.85
	switch {
	case d.XRight != "":
		dr.text(d.XLeft, left+halfW/2, ax, axisSize, halfW-edge, text, "middle", "")
		dr.text(d.XRight, midX+halfW/2, ax, axisSize, halfW-edge, text, "middle", "")
	case d.XLeft != "":
		dr.text(d.XLeft, midX, ax, axisSize, plotW, text, "middle", "")
	}
	ay := left - edge - axisSize*0.3
	axisY := func(s string, cy, maxW float64) {
		if s == "" {
			return
		}
		rot := fmt.Sprintf(` transform="rotate(-90 %s %s)"`, svgutil.Num(ay), svgutil.Num(cy))
		dr.text(oneLine(dr.face, s, axisSize, maxW), ay, cy, axisSize, maxW*2, text, "middle", rot)
	}
	if d.YTop == "" {
		axisY(d.YBottom, midY, plotH)
	} else {
		axisY(d.YTop, top+halfH/2, halfH-edge)
		axisY(d.YBottom, midY+halfH/2, halfH-edge)
	}

	// Points: X right, Y up. Labels go under the dot, kept inside the plot.
	defFill := svgutil.Esc(pal.NodeStroke)
	for _, p := range d.Points {
		st := p.Style.merge(d.Classes[p.Class])
		r := st.Radius
		if r == 0 {
			r = defaultRadius
		}
		cx := left + clampUnit(p.X)*plotW
		cy := top + (1-clampUnit(p.Y))*plotH
		pf := defFill
		if st.Color != "" {
			pf = svgutil.Esc(st.Color)
		}
		fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="%s" fill="%s"`, svgutil.Num(cx), svgutil.Num(cy), svgutil.Num(r), pf)
		if st.StrokeWidth > 0 {
			sc := st.StrokeColor
			if sc == "" {
				sc = pal.Text
			}
			fmt.Fprintf(b, ` stroke="%s" stroke-width="%s"`, svgutil.Esc(sc), svgutil.Num(st.StrokeWidth))
		}
		b.WriteString("/>\n")
		if p.Label == "" {
			continue
		}
		lw := min(dr.face.Width(p.Label, pointSize), plotW-4)
		lx := min(max(cx, left+lw/2+2), left+plotW-lw/2-2)
		ly := cy + r + 2 + pointSize*0.85
		if ly+pointSize*0.3 > top+plotH {
			ly = cy - r - 2 - pointSize*0.3
		}
		dr.text(p.Label, lx, ly, pointSize, plotW, text, "middle", "")
	}

	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// oneLine keeps a rotated axis label on one line, shortened with an ellipsis
// when it is longer than its half of the axis: a wrapped rotated label would
// run into the plot.
func oneLine(face svgutil.Face, s string, size, maxW float64) string {
	s = strings.Join(strings.Fields(strings.Join(svgutil.SplitLines(s), " ")), " ")
	if face.Width(s, size) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && face.Width(string(r)+"…", size) > maxW {
		r = r[:len(r)-1]
	}
	return strings.TrimSpace(string(r)) + "…"
}

// mix blends hex colour a toward hex colour b by t (0 keeps a). When either
// is not a #rgb or #rrggbb colour it returns fallback.
func mix(a, b string, t float64, fallback string) string {
	ra, ga, ba, ok1 := hexRGB(a)
	rb, gb, bb, ok2 := hexRGB(b)
	if !ok1 || !ok2 {
		return fallback
	}
	c := func(x, y int) int { return x + int(float64(y-x)*t+0.5) }
	return fmt.Sprintf("#%02x%02x%02x", c(ra, rb), c(ga, gb), c(ba, bb))
}

func hexRGB(s string) (r, g, b int, ok bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v >> 16), int(v >> 8 & 0xff), int(v & 0xff), true
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
