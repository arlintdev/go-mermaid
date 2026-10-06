package packet

import (
	"fmt"
	"math"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls packet diagram appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// Mermaid's packet look: grey blocks with a black outline, a small gap
// between blocks, and the first and last bit numbers above each block.
const (
	bitsPerRow = 32
	bitW       = 22.0
	blockH     = 32.0
	blockGap   = 5.0
)

// Render parses and renders packet-beta source to SVG.
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
	pal := theme.For(o.Theme).Escaped()
	pc := pal.Packet
	face := svgutil.FaceFor(o.FontFace)
	labelFs := math.Round(o.FontSize * 0.86)
	byteFs := math.Round(o.FontSize * 0.72)
	titleFs := o.FontSize
	pad := o.Padding

	maxBit := 0
	for _, f := range d.Fields {
		maxBit = max(maxBit, f.End)
	}
	rows := maxBit/bitsPerRow + 1
	rowH := byteFs + 3 + blockH + blockGap
	left := pad
	top := pad
	w := left + bitsPerRow*bitW + pad
	h := top + float64(rows)*rowH + pad
	if o.Title != "" {
		h += titleFs * 1.6
		w = math.Max(w, face.Width(o.Title, titleFs)+2*pad)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(o.FontSize))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))

	for _, f := range d.Fields {
		for bit := f.Start; bit <= f.End; {
			row := bit / bitsPerRow
			segEnd := min(f.End, (row+1)*bitsPerRow-1)
			x := left + float64(bit%bitsPerRow)*bitW
			y := top + float64(row)*rowH + byteFs + 3
			bw := float64(segEnd-bit+1)*bitW - blockGap
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"/>`+"\n",
				svgutil.Num(x), svgutil.Num(y), svgutil.Num(bw), svgutil.Num(blockH), pc.Fill, pc.Stroke)
			writeLabel(&b, face, f.Label, x+bw/2, y+blockH/2, bw-6, labelFs, pc.Text)
			if bit == segEnd {
				fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%d</text>`+"\n",
					svgutil.Num(x+bw/2), svgutil.Num(y-3), pc.Text, svgutil.Num(byteFs), bit)
			} else {
				fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s">%d</text>`+"\n",
					svgutil.Num(x), svgutil.Num(y-3), pc.Text, svgutil.Num(byteFs), bit)
				fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="end">%d</text>`+"\n",
					svgutil.Num(x+bw), svgutil.Num(y-3), pc.Text, svgutil.Num(byteFs), segEnd)
			}
			bit = segEnd + 1
		}
	}
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(h-pad-titleFs*0.3), pal.Text, svgutil.Num(titleFs), svgutil.Esc(o.Title))
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// writeLabel fits a field's label in its block: on one line, else wrapped
// onto two in a smaller size, else turned upright in a narrow block (a
// one-bit flag), else cut short with an ellipsis.
func writeLabel(b *strings.Builder, face svgutil.Face, label string, cx, cy, maxW, fs float64, text string) {
	if label == "" {
		return
	}
	lines := []string{label}
	if face.Width(label, fs) > maxW {
		small := math.Max(9, fs-2)
		if l := face.Wrap(label, small, maxW); len(l) <= 2 && fits(face, l, small, maxW) {
			lines, fs = l, small
		} else if face.Width(label, small) <= blockH-6 && small <= maxW+4 {
			fmt.Fprintf(b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle" transform="rotate(-90 %s %s)">%s</text>`+"\n",
				svgutil.Num(cx), svgutil.Num(cy+small*0.35), text, svgutil.Num(small), svgutil.Num(cx), svgutil.Num(cy), svgutil.Esc(label))
			return
		} else {
			lines, fs = []string{clip(face, label, small, maxW)}, small
		}
	}
	lh := fs * 1.15
	fmt.Fprintf(b, `<text fill="%s" font-size="%s" text-anchor="middle">`, text, svgutil.Num(fs))
	for i, ln := range lines {
		y := cy + fs*0.35 - lh*float64(len(lines)-1)/2 + float64(i)*lh
		fmt.Fprintf(b, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(cx), svgutil.Num(y), svgutil.Esc(ln))
	}
	b.WriteString("</text>\n")
}

func fits(face svgutil.Face, lines []string, fs, maxW float64) bool {
	for _, l := range lines {
		if face.Width(l, fs) > maxW {
			return false
		}
	}
	return true
}

// clip shortens s with an ellipsis to fit maxW.
func clip(face svgutil.Face, s string, fs, maxW float64) string {
	r := []rune(s)
	for len(r) > 1 && face.Width(string(r)+"…", fs) > maxW {
		r = r[:len(r)-1]
	}
	if len(r) == len([]rune(s)) {
		return s
	}
	return string(r) + "…"
}
