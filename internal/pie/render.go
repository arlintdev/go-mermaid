package pie

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls pie chart appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
	// IDPrefix starts every id in the picture; empty derives one from the
	// source (see svgid.For).
	IDPrefix string
}

// Render parses and renders pie chart source to SVG.
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

// label is a slice's percentage, drawn inside the slice or, when the slice
// is too thin for it, outside with a leader line.
type label struct {
	text    string
	mid     float64 // angle of the slice's middle
	w       float64
	outside bool
	y       float64 // outside labels: final baseline after spreading
}

func svg(d *Diagram, o RenderOptions) []byte {
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	pal := theme.For(o.Theme).Escaped()
	slices, strokeColor := pal.Pie.Slices, pal.Pie.Stroke
	face := svgutil.FaceFor(o.FontFace)
	fs := o.FontSize
	pad := o.Padding
	r := math.Round(fs * 9.5)
	titleFs := math.Round(fs * 1.3)
	total := d.Total()

	// Percentages, inside when they fit.
	var labels []*label
	angle := -math.Pi / 2
	for _, s := range d.Slices {
		if total <= 0 || s.Value <= 0 {
			labels = append(labels, nil)
			continue
		}
		sweep := s.Value / total * 2 * math.Pi
		l := &label{text: fmt.Sprintf("%d%%", int(math.Round(s.Value/total*100))), mid: angle + sweep/2}
		l.w = face.Width(l.text, fs)
		room := 2 * 0.72 * r * math.Sin(math.Min(sweep, math.Pi)/2)
		l.outside = room < l.w+8 || 0.72*r*sweep < fs*1.6
		labels = append(labels, l)
		angle += sweep
	}

	// Outside labels left and right of the pie, spread so none overlap.
	leftW, rightW := 0.0, 0.0
	var left, right []*label
	for _, l := range labels {
		if l == nil || !l.outside {
			continue
		}
		l.y = (r + 14) * math.Sin(l.mid)
		if math.Cos(l.mid) < 0 {
			left = append(left, l)
			leftW = math.Max(leftW, l.w+30)
		} else {
			right = append(right, l)
			rightW = math.Max(rightW, l.w+30)
		}
	}
	spread := func(ls []*label) (top, bottom float64) {
		sort.SliceStable(ls, func(i, j int) bool { return ls[i].y < ls[j].y })
		gap := fs + 4
		for i := 1; i < len(ls); i++ {
			ls[i].y = math.Max(ls[i].y, ls[i-1].y+gap)
		}
		// Pull the column back up when it ran off the bottom.
		if n := len(ls); n > 0 && ls[n-1].y > r+fs {
			shift := ls[n-1].y - (r + fs)
			for _, l := range ls {
				l.y -= shift
			}
			for i := n - 2; i >= 0; i-- {
				ls[i].y = math.Min(ls[i].y, ls[i+1].y-gap)
			}
		}
		if len(ls) == 0 {
			return 0, 0
		}
		return ls[0].y - fs, ls[len(ls)-1].y + fs/2
	}
	lt, lb := spread(left)
	rt, rb := spread(right)

	// Legend.
	sw := fs + 4
	rowH := sw + 4
	legendW := 0.0
	texts := make([]string, len(d.Slices))
	for i, s := range d.Slices {
		texts[i] = s.Label
		if d.ShowData {
			texts[i] += " [" + trimNum(s.Value) + "]"
		}
		legendW = math.Max(legendW, sw+6+face.Width(texts[i], fs))
	}
	legendH := float64(len(d.Slices))*rowH - 4

	top := pad
	if o.Title != "" {
		top += titleFs*1.4 + 8
	}
	half := math.Max(r+2, math.Max(legendH/2, math.Max(-math.Min(lt, rt), math.Max(lb, rb))))
	cx := pad + leftW + r + 2
	cy := top + half
	legendX := cx + r + 2 + rightW + 28
	w := legendX + legendW + pad
	if len(d.Slices) == 0 {
		w = cx + r + 2 + pad
	}
	h := cy + half + pad
	titleW := face.Width(o.Title, titleFs)
	shift := 0.0
	if titleW+2*pad > w {
		shift = (titleW + 2*pad - w) / 2
		w = titleW + 2*pad
		cx += shift
		legendX += shift
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+titleFs), pal.Text, svgutil.Num(titleFs), svgutil.Esc(o.Title))
	}

	writeSlices(&b, d, total, cx, cy, r, pal.Pie)
	fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s" fill="none" stroke="%s" stroke-width="2"/>`+"\n",
		svgutil.Num(cx), svgutil.Num(cy), svgutil.Num(r+1), strokeColor)

	for _, l := range labels {
		if l == nil {
			continue
		}
		if !l.outside {
			fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" text-anchor="middle">%s</text>`+"\n",
				svgutil.Num(cx+0.72*r*math.Cos(l.mid)), svgutil.Num(cy+0.72*r*math.Sin(l.mid)+fs*0.35),
				pal.Text, svgutil.Esc(l.text))
			continue
		}
		side := 1.0
		anchor := "start"
		if math.Cos(l.mid) < 0 {
			side, anchor = -1, "end"
		}
		ex, ey := cx+r*math.Cos(l.mid), cy+r*math.Sin(l.mid)
		kx := cx + side*(r+12)
		if side*(ex-kx) > 0 {
			kx = ex
		}
		ky := cy + l.y
		fmt.Fprintf(&b, `<polyline points="%s,%s %s,%s %s,%s" fill="none" stroke="%s" stroke-width="1"/>`+"\n",
			svgutil.Num(ex), svgutil.Num(ey), svgutil.Num(kx), svgutil.Num(ky-fs*0.35), svgutil.Num(kx+side*6), svgutil.Num(ky-fs*0.35), pal.Text)
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" text-anchor="%s">%s</text>`+"\n",
			svgutil.Num(kx+side*9), svgutil.Num(ky), pal.Text, anchor, svgutil.Esc(l.text))
	}

	ly := cy - legendH/2
	for i := range d.Slices {
		y := ly + float64(i)*rowH
		c := slices[i%len(slices)]
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"/>`+"\n",
			svgutil.Num(legendX), svgutil.Num(y), svgutil.Num(sw), svgutil.Num(sw), c, c)
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s">%s</text>`+"\n",
			svgutil.Num(legendX+sw+6), svgutil.Num(y+sw/2+fs*0.35), pal.Text, svgutil.Esc(texts[i]))
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

func writeSlices(b *strings.Builder, d *Diagram, total, cx, cy, r float64, pc theme.PieColors) {
	slices, strokeColor := pc.Slices, pc.Stroke
	if total <= 0 {
		return
	}
	angle := -math.Pi / 2 // start at the top, clockwise, in source order
	for i, s := range d.Slices {
		if s.Value <= 0 {
			continue
		}
		sweep := s.Value / total * 2 * math.Pi
		color := slices[i%len(slices)]
		if sweep >= 2*math.Pi-1e-9 {
			fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="2" opacity="0.7"/>`+"\n",
				svgutil.Num(cx), svgutil.Num(cy), svgutil.Num(r), color, strokeColor)
			return
		}
		x1, y1 := cx+r*math.Cos(angle), cy+r*math.Sin(angle)
		angle += sweep
		x2, y2 := cx+r*math.Cos(angle), cy+r*math.Sin(angle)
		largeArc := 0
		if sweep > math.Pi {
			largeArc = 1
		}
		fmt.Fprintf(b, `<path d="M%s,%s L%s,%s A%s,%s 0 %d 1 %s,%s Z" fill="%s" stroke="%s" stroke-width="2" stroke-linejoin="round" opacity="0.7"/>`+"\n",
			svgutil.Num(cx), svgutil.Num(cy), svgutil.Num(x1), svgutil.Num(y1),
			svgutil.Num(r), svgutil.Num(r), largeArc, svgutil.Num(x2), svgutil.Num(y2), color, strokeColor)
	}
}

func trimNum(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return fmt.Sprintf("%d", int64(f))
	}
	return svgutil.Num(f)
}
