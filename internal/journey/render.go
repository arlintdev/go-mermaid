package journey

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgid"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls journey diagram appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// Mermaid's default journey colours.
var (
	sectionFills = []string{"#ececff", "#ffffde", "#ffecfe", "#deffe0", "#ecfffe", "#ffdee0", "#ffefec", "#defbff"}
	actorFills   = []string{"#8fbc8f", "#7cfc00", "#00ffff", "#20b2aa", "#b0e0e6", "#ffffe0"}
)

const (
	boxStroke  = "#666666"
	faceFill   = "#fff8dc"
	faceStroke = "#999999"
	taskW      = 150.0
	taskGap    = 50.0
	minBoxH    = 50.0
	faceR      = 15.0
	scoreStep  = 30.0
)

// Render parses and renders journey source to SVG.
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

func svg(d *Diagram, o RenderOptions, id string) []byte {
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	pal := theme.For(o.Theme)
	face := svgutil.FaceFor(o.FontFace)
	fs := o.FontSize
	pad := o.Padding
	lh := fs * 1.25
	titleFs := math.Round(fs * 1.6)
	tasks := d.Tasks()

	// Actors, coloured in name order as Mermaid does.
	seen := map[string]bool{}
	var actors []string
	for _, t := range tasks {
		for _, a := range t.Actors {
			if !seen[a] {
				seen[a] = true
				actors = append(actors, a)
			}
		}
	}
	sort.Strings(actors)
	colour := map[string]string{}
	legendW := 0.0
	for i, a := range actors {
		colour[a] = actorFills[i%len(actorFills)]
		legendW = math.Max(legendW, 24+face.Width(a, fs))
	}
	if legendW > 0 {
		legendW += 24
	}

	// Box heights fit the longest wrapped label.
	inner := taskW - 16
	boxH := minBoxH
	taskLines := make([][]string, len(tasks))
	for i, t := range tasks {
		taskLines[i] = wrap(face, t.Name, fs, inner)
		boxH = math.Max(boxH, float64(len(taskLines[i]))*lh+26)
	}
	secLines := make([][]string, len(d.Sections))
	secH := minBoxH
	x := pad + legendW
	for i, s := range d.Sections {
		n := float64(len(s.Tasks))
		w := n*taskW + (n-1)*taskGap
		secLines[i] = wrap(face, s.Name, fs, w-16)
		secH = math.Max(secH, float64(len(secLines[i]))*lh+16)
	}

	top := pad
	if o.Title != "" {
		top += titleFs*1.2 + 12
	}
	secY := top
	taskY := secY + secH + 10
	arrowY := taskY + boxH + 40
	faceTop := arrowY + 60 + faceR // centre of a score-5 face
	lineEnd := faceTop + 4*scoreStep + faceR + 15
	lastRight := x + float64(len(tasks))*(taskW+taskGap) - taskGap
	arrowEnd := lastRight + 46
	w := math.Max(arrowEnd+8+pad, pad+face.Width(o.Title, titleFs)+pad)
	h := lineEnd + pad
	if lh := secY + float64(len(actors))*20 + pad; lh > h {
		h = lh
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	marker := id + "-arrowhead"
	fmt.Fprintf(&b, `<defs><marker id="%s" refX="5" refY="2" markerWidth="6" markerHeight="4" orient="auto"><path d="M0,0 V4 L6,2 Z" fill="%s"/></marker></defs>`+"\n",
		marker, pal.Text)
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" font-weight="bold">%s</text>`+"\n",
			svgutil.Num(pad+legendW), svgutil.Num(pad+titleFs), pal.Text, svgutil.Num(titleFs), svgutil.Esc(o.Title))
	}

	// Actor legend.
	for i, a := range actors {
		cy := secY + 10 + float64(i)*20
		fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="7" fill="%s" stroke="#000000"/>`+"\n",
			svgutil.Num(pad+7), svgutil.Num(cy), colour[a])
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s">%s</text>`+"\n",
			svgutil.Num(pad+24), svgutil.Num(cy+fs*0.35), pal.Text, svgutil.Esc(a))
	}

	label := func(lines []string, cx, cy float64) {
		svgutil.MultilineText(&b, lines, cx, cy+fs*0.35, lh, pal.Text, "")
		b.WriteByte('\n')
	}

	i := 0
	for si, s := range d.Sections {
		fill := sectionFills[si%len(sectionFills)]
		if len(s.Tasks) == 0 {
			continue
		}
		n := float64(len(s.Tasks))
		sw := n*taskW + (n-1)*taskGap
		if s.Name != "" {
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s" stroke="%s"/>`+"\n",
				svgutil.Num(x), svgutil.Num(secY), svgutil.Num(sw), svgutil.Num(secH), fill, boxStroke)
			label(secLines[si], x+sw/2, secY+secH/2)
		}
		for _, t := range s.Tasks {
			cx := x + taskW/2
			score := min(max(t.Score, 1), 5)
			fy := faceTop + float64(5-score)*scoreStep
			fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-dasharray="4 2"/>`+"\n",
				svgutil.Num(cx), svgutil.Num(taskY+boxH), svgutil.Num(cx), svgutil.Num(lineEnd), boxStroke)
			writeFace(&b, cx, fy, score)
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s" stroke="%s"/>`+"\n",
				svgutil.Num(x), svgutil.Num(taskY), svgutil.Num(taskW), svgutil.Num(boxH), fill, boxStroke)
			for k, a := range t.Actors {
				fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="7" fill="%s" stroke="#000000"><title>%s</title></circle>`+"\n",
					svgutil.Num(x+14+float64(k)*10), svgutil.Num(taskY), colour[a], svgutil.Esc(a))
			}
			label(taskLines[i], cx, taskY+boxH/2+3)
			x += taskW + taskGap
			i++
		}
	}

	if len(tasks) > 0 {
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="4" marker-end="url(#%s)"/>`+"\n",
			svgutil.Num(pad+legendW), svgutil.Num(arrowY), svgutil.Num(arrowEnd), svgutil.Num(arrowY), pal.Text, marker)
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// writeFace draws Mermaid's score face: a smile above 3, a frown below,
// a straight mouth at 3.
func writeFace(b *strings.Builder, cx, cy float64, score int) {
	fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="2"/>`+"\n",
		svgutil.Num(cx), svgutil.Num(cy), svgutil.Num(faceR), faceFill, faceStroke)
	for _, dx := range []float64{-5, 5} {
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="1.5" fill="#666666" stroke="#666666" stroke-width="2"/>`+"\n",
			svgutil.Num(cx+dx), svgutil.Num(cy-5))
	}
	switch {
	case score > 3:
		my := cy + 2
		fmt.Fprintf(b, `<path d="M%s,%s A7.5,7.5 0 0 0 %s,%s" fill="none" stroke="#666666" stroke-width="1.2"/>`+"\n",
			svgutil.Num(cx-6.5), svgutil.Num(my), svgutil.Num(cx+6.5), svgutil.Num(my))
	case score < 3:
		my := cy + 9
		fmt.Fprintf(b, `<path d="M%s,%s A7.5,7.5 0 0 1 %s,%s" fill="none" stroke="#666666" stroke-width="1.2"/>`+"\n",
			svgutil.Num(cx-6.5), svgutil.Num(my), svgutil.Num(cx+6.5), svgutil.Num(my))
	default:
		fmt.Fprintf(b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="#666666"/>`+"\n",
			svgutil.Num(cx-5), svgutil.Num(cy+7), svgutil.Num(cx+5), svgutil.Num(cy+7))
	}
}
