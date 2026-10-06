package radar

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls radar appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// curveColors are Mermaid's default radar curve colours.
var curveColors = []string{"#8686ff", "#ffff78", "#d7ff86", "#c286ff", "#ff86ff", "#ff86c2", "#ff8686", "#ffc286", "#c2ff86", "#86ffc2", "#86ffff", "#86c2ff"}

const (
	radius      = 170.0
	graticule   = "#dedede"
	axisStroke  = "#333333"
	tension     = 0.17 // Mermaid's curveTension
	labelOffset = 14.0
)

// Render parses and renders radar-beta source to SVG.
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

func svg(d *Diagram, o RenderOptions) []byte {
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	o.FontFace = fontFamily(o.FontFace)
	pal := theme.For(o.Theme)
	face := svgutil.FaceFor(o.FontFace)
	fs := o.FontSize
	labelFs := math.Round(fs * 0.93)
	legendFs := labelFs
	titleFs := math.Round(fs * 1.15)
	pad := o.Padding
	n := len(d.Axes)
	lo, hi := d.Min(), d.Max()

	angle := func(i int) float64 { return -math.Pi/2 + 2*math.Pi*float64(i)/float64(n) }
	// Axis labels set how much room the plot needs around it.
	var left, right, up, down float64
	for i, ax := range d.Axes {
		a := angle(i)
		c, s := math.Cos(a), math.Sin(a)
		w := face.Width(ax, labelFs)
		x := (radius + labelOffset) * c
		y := (radius + labelOffset) * s
		switch {
		case c > 0.1:
			right = math.Max(right, x+w)
		case c < -0.1:
			left = math.Max(left, -x+w)
		default:
			right = math.Max(right, w/2)
			left = math.Max(left, w/2)
		}
		if s < 0 {
			up = math.Max(up, -y+labelFs)
		} else {
			down = math.Max(down, y+labelFs)
		}
	}
	left, right = math.Max(left, radius+2), math.Max(right, radius+2)
	up, down = math.Max(up, radius+2), math.Max(down, radius+2)

	// Legend at the top right, beside the plot.
	legendW := 0.0
	if !d.HideLegend {
		for _, c := range d.Curves {
			legendW = math.Max(legendW, 12+6+face.Width(c.Name, legendFs))
		}
	}
	top := pad
	if o.Title != "" {
		top += titleFs*1.4 + 8
	}
	cx := pad + left
	cy := top + up
	w := cx + right + pad
	if legendW > 0 {
		w += legendW + 20
	}
	h := cy + down + pad
	if lh := top + float64(len(d.Curves))*(legendFs+8) + pad; !d.HideLegend && lh > h {
		h = lh
	}
	if tw := face.Width(o.Title, titleFs) + 2*pad; tw > w {
		w = tw
	}

	point := func(i int, v float64) (float64, float64) {
		f := math.Max(0, math.Min(1, (v-lo)/(hi-lo)))
		a := angle(i)
		return cx + radius*f*math.Cos(a), cy + radius*f*math.Sin(a)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(cx), svgutil.Num(pad+titleFs), pal.Text, svgutil.Num(titleFs), svgutil.Esc(o.Title))
	}

	// Graticule: circles or polygons, outermost first.
	for k := d.Ticks; k >= 1; k-- {
		r := radius * float64(k) / float64(d.Ticks)
		if !d.Polygon || n < 3 {
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s" fill="%s" fill-opacity="0.3" stroke="%s"/>`+"\n",
				svgutil.Num(cx), svgutil.Num(cy), svgutil.Num(r), graticule, graticule)
			continue
		}
		var p strings.Builder
		for i := 0; i < n; i++ {
			a := angle(i)
			if i > 0 {
				p.WriteByte(' ')
			}
			fmt.Fprintf(&p, "%s,%s", svgutil.Num(cx+r*math.Cos(a)), svgutil.Num(cy+r*math.Sin(a)))
		}
		fmt.Fprintf(&b, `<polygon points="%s" fill="%s" fill-opacity="0.3" stroke="%s"/>`+"\n", p.String(), graticule, graticule)
	}

	// Axes and their labels.
	for i, ax := range d.Axes {
		a := angle(i)
		c, s := math.Cos(a), math.Sin(a)
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="2"/>`+"\n",
			svgutil.Num(cx), svgutil.Num(cy), svgutil.Num(cx+radius*c), svgutil.Num(cy+radius*s), axisStroke)
		lx, ly := cx+(radius+labelOffset)*c, cy+(radius+labelOffset)*s
		anchor := "middle"
		if c > 0.1 {
			anchor = "start"
		} else if c < -0.1 {
			anchor = "end"
		}
		if s >= 0 {
			ly += labelFs * 0.8 // below the plot, hang the text
		}
		if math.Abs(s) < 0.1 {
			ly = cy + (radius+labelOffset)*s + labelFs*0.35
		}
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="%s">%s</text>`+"\n",
			svgutil.Num(lx), svgutil.Num(ly), pal.Text, svgutil.Num(labelFs), anchor, svgutil.Esc(ax))
	}

	// Curves: Mermaid's closed cardinal curve through each value.
	for ci, c := range d.Curves {
		color := curveColors[ci%len(curveColors)]
		pts := make([][2]float64, n)
		for i := range pts {
			v := lo
			if i < len(c.Values) {
				v = c.Values[i]
			}
			pts[i][0], pts[i][1] = point(i, v)
		}
		fmt.Fprintf(&b, `<path d="%s" fill="%s" fill-opacity="0.5" stroke="%s" stroke-width="2"/>`+"\n", closedCurve(pts), color, outline(color))
	}

	if !d.HideLegend {
		lx := cx + right + 20
		for ci, c := range d.Curves {
			y := top + float64(ci)*(legendFs+8)
			color := curveColors[ci%len(curveColors)]
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="12" height="12" fill="%s" fill-opacity="0.5" stroke="%s"/>`+"\n",
				svgutil.Num(lx), svgutil.Num(y), color, outline(color))
			fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s">%s</text>`+"\n",
				svgutil.Num(lx+18), svgutil.Num(y+6+legendFs*0.35), pal.Text, svgutil.Num(legendFs), svgutil.Esc(c.Name))
		}
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// outline is the stroke for a curve colour: the colour itself, or a darker
// shade when it is too pale (Mermaid's yellows) to see as a line.
func outline(c string) string {
	v, err := strconv.ParseUint(strings.TrimPrefix(c, "#"), 16, 32)
	if err != nil {
		return c
	}
	r, g, b := v>>16&0xff, v>>8&0xff, v&0xff
	if (0.2126*float64(r)+0.7152*float64(g)+0.0722*float64(b))/255 <= 0.8 {
		return c
	}
	return fmt.Sprintf("#%02x%02x%02x", r*2/3, g*2/3, b*2/3)
}

// closedCurve draws a smooth closed path through pts as Mermaid's radar
// does: each segment a cubic whose control points follow the neighbours.
func closedCurve(pts [][2]float64) string {
	n := len(pts)
	if n == 0 {
		return "M0,0"
	}
	var p strings.Builder
	fmt.Fprintf(&p, "M%s,%s", svgutil.Num(pts[0][0]), svgutil.Num(pts[0][1]))
	for i := 0; i < n; i++ {
		p0, p1, p2, p3 := pts[(i-1+n)%n], pts[i], pts[(i+1)%n], pts[(i+2)%n]
		c1x, c1y := p1[0]+(p2[0]-p0[0])*tension, p1[1]+(p2[1]-p0[1])*tension
		c2x, c2y := p2[0]-(p3[0]-p1[0])*tension, p2[1]-(p3[1]-p1[1])*tension
		fmt.Fprintf(&p, " C%s,%s %s,%s %s,%s", svgutil.Num(c1x), svgutil.Num(c1y), svgutil.Num(c2x), svgutil.Num(c2y),
			svgutil.Num(p2[0]), svgutil.Num(p2[1]))
	}
	p.WriteString(" Z")
	return p.String()
}

// fontFamily keeps a font-family option only when it is a plain list of
// family names, so the option can never carry markup into the picture.
func fontFamily(s string) string {
	if strings.TrimSpace(s) == "" || len(s) > 200 {
		return "sans-serif"
	}
	for _, r := range s {
		if !(r == ' ' || r == ',' || r == '-' || r == '_' || r == '\'' || r == '"' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return "sans-serif"
		}
	}
	return s
}
