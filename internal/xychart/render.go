package xychart

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls xychart appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// seriesColors is Mermaid's default xychart plot palette.
var seriesColors = []string{"#ececff", "#8493a6", "#ffb6c1", "#c4a000", "#fcfc7f", "#f5deb3", "#87ceeb", "#ffe4e1", "#e6e6fa", "#90ee90"}

const (
	tickLen = 5.0
	gap     = 6.0
)

// Render parses and renders xychart source to SVG.
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

// scale is a linear value axis with nice ticks.
type scale struct {
	lo, hi float64
	ticks  []float64
	dec    int // decimals in tick labels
}

func niceScale(lo, hi float64, fixed bool, target int) scale {
	if hi <= lo {
		hi = lo + 1
	}
	raw := (hi - lo) / float64(target)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	step := mag
	for _, m := range []float64{1, 2, 5, 10} {
		if m*mag >= raw {
			step = m * mag
			break
		}
	}
	if !fixed {
		lo = math.Floor(lo/step) * step
		hi = math.Ceil(hi/step) * step
	}
	s := scale{lo: lo, hi: hi, dec: max(0, -int(math.Floor(math.Log10(step)+1e-9)))}
	for v, i := math.Ceil(lo/step-1e-9)*step, 0; v <= hi+step*1e-9 && i < 100; v, i = v+step, i+1 {
		s.ticks = append(s.ticks, v)
	}
	return s
}

func (s scale) label(v float64) string {
	if math.Abs(v) < 1e-12 {
		v = 0
	}
	return strconv.FormatFloat(v, 'f', s.dec, 64)
}

func (s scale) frac(v float64) float64 {
	f := (v - s.lo) / (s.hi - s.lo)
	return math.Max(0, math.Min(1, f))
}

func svg(d *Diagram, o RenderOptions) []byte {
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	pal := theme.For(o.Theme)
	face := svgutil.FaceFor(o.FontFace)
	fs := o.FontSize
	pad := o.Padding
	titleFs := math.Round(fs * 1.43)
	axisTitleFs := fs

	n := len(d.XCats)
	for _, s := range d.Series {
		n = max(n, len(s.Values))
	}
	n = max(n, 1)
	labels := make([]string, n)
	copy(labels, d.XCats)

	lo, hi := d.Bounds()
	plotW, plotH := 560.0, 320.0
	if d.Horizontal {
		plotH = math.Max(160, math.Min(480, float64(n)*52))
	}
	valLen := plotH
	if d.Horizontal {
		valLen = plotW
	}
	per := fs * 2.6 // vertical: one label's height and a gap
	if d.Horizontal {
		per = 70 // horizontal: one number's width and a gap
	}
	vs := niceScale(lo, hi, d.HasYRange, max(2, int(valLen/per)))
	var xs scale // numeric category axis, when x-axis is a range
	if d.HasXRange && len(d.XCats) == 0 {
		catLen := plotW
		if d.Horizontal {
			catLen = plotH
		}
		xs = niceScale(d.XMin, d.XMax, true, max(2, int(catLen/80)))
	}

	valLabelW := 0.0
	for _, t := range vs.ticks {
		valLabelW = math.Max(valLabelW, face.Width(vs.label(t), fs))
	}
	catLabelW := 0.0
	catLines := make([][]string, n)
	for i, l := range labels {
		if d.Horizontal {
			catLines[i] = face.Wrap(l, fs, 160)
		} else {
			catLines[i] = []string{l}
		}
		for _, ln := range catLines[i] {
			catLabelW = math.Max(catLabelW, face.Width(ln, fs))
		}
	}
	if xs.ticks != nil {
		for _, t := range xs.ticks {
			catLabelW = math.Max(catLabelW, face.Width(xs.label(t), fs))
		}
	}

	top := pad
	if o.Title != "" {
		top += titleFs*1.3 + 8
	}
	axisTitleH := axisTitleFs*1.3 + 4
	var left, plotTop, w, h float64
	rotate := false
	if !d.Horizontal {
		band := plotW / float64(n)
		rotate = xs.ticks == nil && catLabelW > band-6
		left = pad + valLabelW + gap + tickLen
		if d.YLabel != "" {
			left += axisTitleH
		}
		plotTop = top + fs/2
		bottom := plotTop + plotH + tickLen + gap
		if rotate {
			bottom += catLabelW*0.71 + fs*0.71
		} else {
			bottom += fs
		}
		if d.XLabel != "" {
			bottom += axisTitleH
		}
		w = left + plotW + pad + 4
		h = bottom + pad
	} else {
		left = pad + catLabelW + gap + tickLen
		if d.XLabel != "" {
			left += axisTitleH
		}
		plotTop = top + fs + gap + tickLen
		if d.YLabel != "" {
			plotTop += axisTitleH
		}
		w = left + plotW + valLabelW/2 + pad
		h = plotTop + plotH + pad
	}
	if tw := face.Width(o.Title, titleFs) + 2*pad; tw > w {
		w = tw
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+titleFs), pal.Text, svgutil.Num(titleFs), svgutil.Esc(o.Title))
	}

	// catPos is the centre of category i along its axis; valPos a value's
	// position along the value axis, both in page coordinates.
	catLen := plotW
	if d.Horizontal {
		catLen = plotH
	}
	band := catLen / float64(n)
	catPos := func(i int) float64 {
		off := band * (float64(i) + 0.5)
		if xs.ticks != nil {
			if n == 1 {
				off = catLen / 2
			} else {
				v := d.XMin + float64(i)*(d.XMax-d.XMin)/float64(n-1)
				off = band/2 + xs.frac(v)*(catLen-band)
			}
		}
		if d.Horizontal {
			return plotTop + off
		}
		return left + off
	}
	valPos := func(v float64) float64 {
		if d.Horizontal {
			return left + vs.frac(v)*plotW
		}
		return plotTop + plotH - vs.frac(v)*plotH
	}
	point := func(i int, v float64) (float64, float64) {
		if d.Horizontal {
			return valPos(v), catPos(i)
		}
		return catPos(i), valPos(v)
	}

	// Bars, grouped side by side within each category.
	var bars []*Series
	for _, s := range d.Series {
		if s.Kind == "bar" {
			bars = append(bars, s)
		}
	}
	base := math.Max(vs.lo, math.Min(vs.hi, 0))
	group := band * 0.6
	if xs.ticks != nil {
		group = math.Min(group, 40)
	}
	for bi, s := range bars {
		color := seriesColors[seriesIdx(d, s)%len(seriesColors)]
		bw := group / float64(len(bars))
		for i, v := range s.Values {
			c := catPos(i) - group/2 + float64(bi)*bw
			v0, v1 := valPos(base), valPos(v)
			if v1 < v0 {
				v0, v1 = v1, v0
			}
			x, y, rw, rh := c, v0, bw, v1-v0
			if d.Horizontal {
				x, y, rw, rh = v0, c, v1-v0, bw
			}
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"/>`+"\n",
				svgutil.Num(x), svgutil.Num(y), svgutil.Num(rw), svgutil.Num(rh), color, darken(color))
		}
	}
	for _, s := range d.Series {
		if s.Kind != "line" || len(s.Values) == 0 {
			continue
		}
		var p strings.Builder
		for i, v := range s.Values {
			x, y := point(i, v)
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&p, "%s%s,%s ", cmd, svgutil.Num(x), svgutil.Num(y))
		}
		fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`+"\n",
			strings.TrimSpace(p.String()), lineColor(seriesColors[seriesIdx(d, s)%len(seriesColors)]))
	}

	axis := func(x1, y1, x2, y2 float64) {
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="2" stroke-linecap="square"/>`+"\n",
			svgutil.Num(x1), svgutil.Num(y1), svgutil.Num(x2), svgutil.Num(y2), pal.Text)
	}
	text := func(x, y float64, anchor, s, extra string) {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" text-anchor="%s"%s>%s</text>`+"\n",
			svgutil.Num(x), svgutil.Num(y), pal.Text, anchor, extra, svgutil.Esc(s))
	}
	catTicks := func(f func(pos float64, label string)) {
		if xs.ticks != nil {
			for _, t := range xs.ticks {
				off := band/2 + xs.frac(t)*(catLen-band)
				if n == 1 {
					off = catLen / 2
				}
				f(off, xs.label(t))
			}
			return
		}
		for i := 0; i < n; i++ {
			f(band*(float64(i)+0.5), labels[i])
		}
	}

	if !d.Horizontal {
		bottom := plotTop + plotH
		axis(left, plotTop, left, bottom)
		axis(left, bottom, left+plotW, bottom)
		for _, t := range vs.ticks {
			y := valPos(t)
			axis(left-tickLen, y, left, y)
			text(left-tickLen-gap, y+fs*0.35, "end", vs.label(t), "")
		}
		catTicks(func(off float64, label string) {
			x := left + off
			axis(x, bottom, x, bottom+tickLen)
			if label == "" {
				return
			}
			ly := bottom + tickLen + gap
			if rotate {
				fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" text-anchor="end" transform="rotate(-45 %s %s)">%s</text>`+"\n",
					svgutil.Num(x), svgutil.Num(ly+fs*0.7), pal.Text, svgutil.Num(x), svgutil.Num(ly+fs*0.7), svgutil.Esc(label))
				return
			}
			text(x, ly+fs*0.8, "middle", label, "")
		})
		if d.YLabel != "" {
			cx, cy := pad+axisTitleFs, plotTop+plotH/2
			fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" text-anchor="middle" transform="rotate(-90 %s %s)">%s</text>`+"\n",
				svgutil.Num(cx), svgutil.Num(cy), pal.Text, svgutil.Num(cx), svgutil.Num(cy), svgutil.Esc(d.YLabel))
		}
		if d.XLabel != "" {
			text(left+plotW/2, h-pad-4, "middle", d.XLabel, "")
		}
	} else {
		right := left + plotW
		axis(left, plotTop, right, plotTop)
		axis(left, plotTop, left, plotTop+plotH)
		for _, t := range vs.ticks {
			x := valPos(t)
			axis(x, plotTop-tickLen, x, plotTop)
			text(x, plotTop-tickLen-gap, "middle", vs.label(t), "")
		}
		i := 0
		catTicks(func(off float64, label string) {
			y := plotTop + off
			axis(left-tickLen, y, left, y)
			lines := []string{label}
			if xs.ticks == nil {
				lines = catLines[i]
			}
			i++
			lh := fs * 1.25
			fmt.Fprintf(&b, `<text fill="%s" text-anchor="end">`, pal.Text)
			for k, ln := range lines {
				ty := y + fs*0.35 - lh*float64(len(lines)-1)/2 + float64(k)*lh
				fmt.Fprintf(&b, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(left-tickLen-gap), svgutil.Num(ty), svgutil.Esc(ln))
			}
			b.WriteString("</text>\n")
		})
		if d.YLabel != "" {
			text(left+plotW/2, plotTop-tickLen-gap-fs-gap, "middle", d.YLabel, "")
		}
		if d.XLabel != "" {
			cx, cy := pad+axisTitleFs, plotTop+plotH/2
			fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" text-anchor="middle" transform="rotate(-90 %s %s)">%s</text>`+"\n",
				svgutil.Num(cx), svgutil.Num(cy), pal.Text, svgutil.Num(cx), svgutil.Num(cy), svgutil.Esc(d.XLabel))
		}
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// darken returns a #rrggbb colour a third of the way to black, for a bar's
// outline, so pale bars still read on a white page.
func darken(c string) string {
	v, err := strconv.ParseUint(strings.TrimPrefix(c, "#"), 16, 32)
	if err != nil || len(c) != 7 {
		return c
	}
	r, g, bl := v>>16&0xff, v>>8&0xff, v&0xff
	f := func(x uint64) uint64 { return x * 2 / 3 }
	return fmt.Sprintf("#%02x%02x%02x", f(r), f(g), f(bl))
}

// lineColor keeps a palette colour for a line unless it is too pale to see
// as a thin stroke on a white page (Mermaid's first colour is), in which
// case it is darkened.
func lineColor(c string) string {
	v, err := strconv.ParseUint(strings.TrimPrefix(c, "#"), 16, 32)
	if err != nil {
		return c
	}
	lum := (0.2126*float64(v>>16&0xff) + 0.7152*float64(v>>8&0xff) + 0.0722*float64(v&0xff)) / 255
	if lum > 0.8 {
		return darken(c)
	}
	return c
}

func seriesIdx(d *Diagram, s *Series) int {
	for i, x := range d.Series {
		if x == s {
			return i
		}
	}
	return 0
}
