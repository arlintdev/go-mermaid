package gantt

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls gantt appearance.
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

// chartW is the width of the time axis. Mermaid stretches the axis to the
// page; a fixed width keeps one day as wide as the next across charts.
const chartW = 720.0

const sectionTitleMaxW = 150.0

// Render parses and renders gantt source to SVG.
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

type layout struct {
	o                     RenderOptions
	face                  svgutil.Face
	fs, taskFs, tickFs    float64
	titleFs               float64
	lo, hi                time.Time
	chartLeft, chartRight float64
}

func (l *layout) x(t time.Time) float64 {
	span := l.hi.Sub(l.lo).Seconds()
	if span <= 0 {
		return l.chartLeft
	}
	return l.chartLeft + t.Sub(l.lo).Seconds()/span*(l.chartRight-l.chartLeft)
}

func svg(d *Diagram, o RenderOptions) []byte {
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	pal := theme.For(o.Theme).Escaped()
	gc := pal.Gantt
	l := &layout{o: o, face: svgutil.FaceFor(o.FontFace), fs: o.FontSize}
	l.taskFs = math.Round(o.FontSize * 0.86)
	l.tickFs = math.Round(o.FontSize * 0.79)
	l.titleFs = math.Round(o.FontSize * 1.3)
	pad := o.Padding
	barH := l.taskFs + 8
	rowH := barH + 6

	var rows []*Task
	var verts []*Task
	for _, t := range d.Tasks {
		if t.Vert {
			verts = append(verts, t)
		} else {
			rows = append(rows, t)
		}
	}
	l.lo, l.hi = d.Bounds()
	if !l.hi.After(l.lo) {
		l.hi = l.lo.Add(24 * time.Hour)
		if len(rows) > 0 {
			// A chart of milestones only: show a day either side.
			l.lo = l.lo.Add(-24 * time.Hour)
		}
	}

	// Section titles sit in a gutter left of the chart.
	titles := make([][]string, len(d.Sections))
	gutter := 0.0
	for i, s := range d.Sections {
		// A word too long to wrap may widen the gutter up to 2.5 times the
		// wrap width; past that it is cut.
		for _, ln := range l.face.Wrap(s, l.taskFs, sectionTitleMaxW) {
			titles[i] = append(titles[i], l.face.WrapWithin(ln, l.taskFs, sectionTitleMaxW*2.5)...)
		}
		for _, ln := range titles[i] {
			gutter = math.Max(gutter, l.face.Width(ln, l.taskFs)+20)
		}
	}

	ticks, labelEvery, format := l.pickTicks(d)
	labels := make([]string, len(ticks))
	firstW, lastW := 0.0, 0.0
	for i, t := range ticks {
		labels[i] = strftime(format, t)
	}
	if len(ticks) > 0 {
		firstW = l.face.Width(labels[0], l.tickFs)
		lastW = l.face.Width(labels[len(labels)-1], l.tickFs)
	}
	l.chartLeft = pad + math.Max(gutter, firstW/2)
	l.chartRight = l.chartLeft + chartW

	y := pad
	var b strings.Builder
	var body strings.Builder
	maxRight := l.chartRight + lastW/2

	if o.Title != "" {
		y += l.titleFs*1.4 + 6
	}
	topLabelY := 0.0
	if d.TopAxis && len(ticks) > 0 {
		topLabelY = y + l.tickFs
		y += l.tickFs + 8
	}
	vertLabelY := 0.0
	if len(verts) > 0 {
		vertLabelY = y + l.tickFs
		y += l.tickFs + 8
	}
	gridTop := y
	rowsTop := gridTop + 4
	// Each run of rows in one section is a block at least as tall as the
	// section's title; the rows sit in the middle of a taller block, and the
	// first and last row's band reach its edges.
	rowY := make([]float64, len(rows))
	bandY := make([]float64, len(rows))
	bandH := make([]float64, len(rows))
	blockMid := map[int]float64{}
	lh := l.taskFs * 1.25
	for i, by := 0, rowsTop; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].Section == rows[i].Section {
			j++
		}
		n := float64(j - i)
		extra := 0.0
		if si := rows[i].Section; si >= 0 && si < len(titles) {
			extra = math.Max(0, float64(len(titles[si]))*lh+8-n*rowH)
		}
		for k := i; k < j; k++ {
			rowY[k] = by + extra/2 + float64(k-i)*rowH
			bandY[k], bandH[k] = rowY[k], rowH
		}
		bandY[i] = by
		bandH[i] += extra / 2
		bandH[j-1] += extra / 2
		if _, seen := blockMid[rows[i].Section]; !seen {
			blockMid[rows[i].Section] = by + (n*rowH+extra)/2
		}
		by += n*rowH + extra
		i = j
	}
	rowsBottom := rowsTop
	if len(rows) > 0 {
		rowsBottom = bandY[len(rows)-1] + bandH[len(rows)-1]
	}
	gridBottom := rowsBottom + 4
	labelY := gridBottom + 6 + l.tickFs

	// Excluded days, behind everything else.
	if d.HasExcludes() && l.hi.Sub(l.lo) <= 3660*24*time.Hour {
		day := time.Date(l.lo.Year(), l.lo.Month(), l.lo.Day(), 0, 0, 0, 0, time.UTC)
		var runStart time.Time
		inRun := false
		flush := func(end time.Time) {
			x1, x2 := l.x(maxTime(runStart, l.lo)), l.x(minTime(end, l.hi))
			if x2 > x1 {
				fmt.Fprintf(&body, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n",
					svgutil.Num(x1), svgutil.Num(gridTop), svgutil.Num(x2-x1), svgutil.Num(gridBottom-gridTop), gc.ExcludeFill)
			}
		}
		for ; day.Before(l.hi); day = day.AddDate(0, 0, 1) {
			ex := d.Excluded(day)
			if ex && !inRun {
				runStart, inRun = day, true
			} else if !ex && inRun {
				flush(day)
				inRun = false
			}
		}
		if inRun {
			flush(day)
		}
	}

	// Grid lines and tick labels.
	for i, t := range ticks {
		x := l.x(t)
		fmt.Fprintf(&body, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-opacity="0.8"/>`+"\n",
			svgutil.Num(x), svgutil.Num(gridTop), svgutil.Num(x), svgutil.Num(gridBottom), gc.Grid)
		if i%labelEvery != 0 {
			continue
		}
		for _, ly := range []float64{labelY, topLabelY} {
			if ly == 0 {
				continue
			}
			fmt.Fprintf(&body, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
				svgutil.Num(x), svgutil.Num(ly), pal.Text, svgutil.Num(l.tickFs), svgutil.Esc(labels[i]))
		}
	}

	// Section bands go here, between the grid and the bars, once the
	// width is known.
	bandsAt := body.Len()

	// Bars, milestones and their labels.
	for i, t := range rows {
		cy := rowY[i] + rowH/2
		fill, stroke, inText := gc.TaskFill, gc.TaskStroke, gc.TaskText
		switch {
		case t.Done:
			fill, stroke, inText = gc.DoneFill, gc.DoneStroke, gc.DoneText
		case t.Active:
			fill, inText = gc.ActiveFill, gc.ActiveText
		case t.Crit:
			fill = gc.CritFill
		}
		if t.Crit {
			stroke = gc.CritStroke
		}
		tw := l.face.Width(t.Name, l.taskFs)
		var x1, x2 float64
		italic := ""
		if t.Milestone {
			cx := (l.x(t.Start) + l.x(t.RenderEnd)) / 2
			r := barH * 0.56
			fmt.Fprintf(&body, `<polygon points="%s,%s %s,%s %s,%s %s,%s" fill="%s" stroke="%s" stroke-width="2"/>`+"\n",
				svgutil.Num(cx), svgutil.Num(cy-r), svgutil.Num(cx+r), svgutil.Num(cy),
				svgutil.Num(cx), svgutil.Num(cy+r), svgutil.Num(cx-r), svgutil.Num(cy), fill, stroke)
			x1, x2 = cx-r, cx+r
			italic = ` font-style="italic"`
		} else {
			x1, x2 = l.x(t.Start), l.x(t.RenderEnd)
			if x2-x1 < 2 {
				x2 = x1 + 2
			}
			fmt.Fprintf(&body, `<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s" stroke="%s" stroke-width="2"/>`+"\n",
				svgutil.Num(x1), svgutil.Num(cy-barH/2), svgutil.Num(x2-x1), svgutil.Num(barH), fill, stroke)
		}
		ty := cy + l.taskFs*0.35
		switch {
		case !t.Milestone && tw+12 <= x2-x1:
			fmt.Fprintf(&body, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
				svgutil.Num((x1+x2)/2), svgutil.Num(ty), inText, svgutil.Num(l.taskFs), svgutil.Esc(t.Name))
		case x2+6+tw > l.chartRight && x1-6-tw >= l.chartLeft:
			fmt.Fprintf(&body, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="end"%s>%s</text>`+"\n",
				svgutil.Num(x1-6), svgutil.Num(ty), pal.Text, svgutil.Num(l.taskFs), italic, svgutil.Esc(t.Name))
		default:
			fmt.Fprintf(&body, `<text x="%s" y="%s" fill="%s" font-size="%s"%s>%s</text>`+"\n",
				svgutil.Num(x2+6), svgutil.Num(ty), pal.Text, svgutil.Num(l.taskFs), italic, svgutil.Esc(t.Name))
			maxRight = math.Max(maxRight, x2+6+tw)
		}
	}

	// Vertical markers.
	for _, t := range verts {
		x := l.x(t.Start)
		fmt.Fprintf(&body, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="2"/>`+"\n",
			svgutil.Num(x), svgutil.Num(gridTop), svgutil.Num(x), svgutil.Num(gridBottom), gc.Marker)
		fmt.Fprintf(&body, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(x), svgutil.Num(vertLabelY), gc.Marker, svgutil.Num(l.tickFs), svgutil.Esc(t.Name))
		maxRight = math.Max(maxRight, x+l.face.Width(t.Name, l.tickFs)/2)
	}

	// Section titles, centred on their rows.
	for si, lines := range titles {
		mid, ok := blockMid[si]
		if !ok {
			continue
		}
		top := mid - lh*float64(len(lines)-1)/2 + l.taskFs*0.35
		fmt.Fprintf(&body, `<text fill="%s" font-size="%s">`, pal.Text, svgutil.Num(l.taskFs))
		for k, ln := range lines {
			fmt.Fprintf(&body, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(pad+8), svgutil.Num(top+float64(k)*lh), svgutil.Esc(ln))
		}
		body.WriteString("</text>\n")
	}

	w := math.Ceil(maxRight + pad)
	var bands strings.Builder
	for i, t := range rows {
		band := gc.Bands[max(t.Section, 0)%len(gc.Bands)]
		fmt.Fprintf(&bands, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s" fill-opacity="%s"/>`+"\n",
			svgutil.Num(pad), svgutil.Num(bandY[i]), svgutil.Num(w-2*pad), svgutil.Num(bandH[i]),
			svgutil.Esc(band.Fill), svgutil.Num(band.Opacity))
	}
	h := math.Ceil(labelY + 4 + pad)
	if len(ticks) == 0 {
		h = math.Ceil(gridBottom + pad)
	}
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(o.FontSize))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	all := body.String()
	b.WriteString(all[:bandsAt])
	b.WriteString(bands.String())
	b.WriteString(all[bandsAt:])
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+l.titleFs), pal.Text, svgutil.Num(l.titleFs), svgutil.Esc(o.Title))
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// pickTicks chooses the tick step, how many ticks share one label (so
// labels never collide) and the label format.
func (l *layout) pickTicks(d *Diagram) ([]time.Time, int, string) {
	format := d.AxisFormat
	span := l.hi.Sub(l.lo)
	if format == "" {
		format = "%Y-%m-%d"
		if span <= 2*24*time.Hour {
			format = "%H:%M"
		}
	}
	fits := func(ts []time.Time, every int) bool {
		widest, prev := 0.0, ""
		for i := 0; i < len(ts); i += every {
			lbl := strftime(format, ts[i])
			if i > 0 && lbl == prev {
				return false // the format cannot tell these ticks apart
			}
			prev = lbl
			widest = math.Max(widest, l.face.Width(lbl, l.tickFs))
		}
		n := float64((len(ts) + every - 1) / every)
		return n*(widest+14) <= chartW+widest
	}
	if iv, ok := parseTickInterval(d.TickInterval); ok && span/iv.approx() <= maxTicks {
		ts := iv.ticks(l.lo, l.hi, d.weekStartsOn)
		for every := 1; every <= len(ts); every++ {
			if fits(ts, every) {
				return ts, every, format
			}
		}
	}
	for _, iv := range autoIntervals {
		if span/iv.approx() > 60 {
			continue
		}
		ts := iv.ticks(l.lo, l.hi, d.weekStartsOn)
		if fits(ts, 1) {
			return ts, 1, format
		}
		// d3 steps from weeks straight to months; label every other week
		// before giving up the weekly grid.
		if iv.unit == 'w' && fits(ts, 2) {
			return ts, 2, format
		}
	}
	return nil, 1, format
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
