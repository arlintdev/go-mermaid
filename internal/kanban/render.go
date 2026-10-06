package kanban

import (
	"fmt"
	"math"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls kanban appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

const (
	colW      = 200.0
	colGap    = 6.0
	cardInset = 7.5
	cardGap   = 6.0
)

// Render parses and renders kanban source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return svg(d, o), nil
}

type cardLayout struct {
	lines []string
	h     float64
	meta  bool
}

func svg(d *Diagram, o RenderOptions) []byte {
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	pal := theme.For(o.Theme).Escaped()
	kc := pal.Kanban
	priority := map[string]string{"Very High": kc.VeryHigh, "High": kc.High, "Low": kc.Low, "Very Low": kc.VeryLow}
	face := svgutil.FaceFor(o.FontFace)
	fs := o.FontSize
	metaFs := math.Round(fs * 0.86)
	lh := fs * 1.3
	pad := o.Padding
	cardW := colW - 2*cardInset
	textW := cardW - 20

	titleFs := math.Round(fs * 1.3)
	top := pad
	if o.Title != "" {
		top += titleFs*1.4 + 8
	}
	headH := lh + 12

	// Every column is as tall as the fullest one, as in Mermaid.
	layouts := make([][]cardLayout, len(d.Columns))
	colTitles := make([][]string, len(d.Columns))
	colH := 0.0
	for ci, col := range d.Columns {
		colTitles[ci] = face.Wrap(col.Title, fs, colW-16)
		h := headH + float64(len(colTitles[ci])-1)*lh
		for _, c := range col.Cards {
			cl := cardLayout{lines: face.Wrap(c.Text, fs, textW), meta: c.Ticket != "" || c.Assigned != ""}
			cl.h = float64(len(cl.lines))*lh + 20
			if cl.meta {
				cl.h += metaFs*1.3 + 2
			}
			cl.h = math.Max(cl.h, 44)
			layouts[ci] = append(layouts[ci], cl)
			h += cl.h + cardGap
		}
		colH = math.Max(colH, h+cardInset-cardGap+4)
	}
	headMax := 0.0
	for _, t := range colTitles {
		headMax = math.Max(headMax, float64(len(t)-1)*lh)
	}

	w := pad*2 + float64(len(d.Columns))*colW + float64(max(len(d.Columns)-1, 0))*colGap
	if tw := face.Width(o.Title, titleFs) + 2*pad; tw > w {
		w = tw
	}
	h := top + colH + pad

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+titleFs), pal.Text, svgutil.Num(titleFs), svgutil.Esc(o.Title))
	}

	for ci, col := range d.Columns {
		x := pad + float64(ci)*(colW+colGap)
		fill := kc.Columns[(ci+1)%len(kc.Columns)]
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="5" fill="%s" stroke="%s"/>`+"\n",
			svgutil.Num(x), svgutil.Num(top), svgutil.Num(colW), svgutil.Num(colH), fill, fill)
		svgutil.MultilineText(&b, colTitles[ci], x+colW/2, top+6+lh/2+fs*0.35+float64(len(colTitles[ci])-1)*lh/2, lh, kc.ColumnText, "")
		b.WriteByte('\n')
		cy := top + headH + headMax
		for k, c := range col.Cards {
			cl := layouts[ci][k]
			cx := x + cardInset
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="5" fill="%s" stroke="%s"/>`+"\n",
				svgutil.Num(cx), svgutil.Num(cy), svgutil.Num(cardW), svgutil.Num(cl.h), kc.CardFill, kc.CardStroke)
			if s, ok := priority[c.Priority]; ok {
				fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="4"/>`+"\n",
					svgutil.Num(cx+2), svgutil.Num(cy+2), svgutil.Num(cx+2), svgutil.Num(cy+cl.h-2), s)
			}
			textH := float64(len(cl.lines)) * lh
			if !cl.meta {
				textH = cl.h - 20
			}
			fmt.Fprintf(&b, `<text fill="%s">`, pal.Text)
			for i, ln := range cl.lines {
				ty := cy + 10 + (textH-float64(len(cl.lines))*lh)/2 + float64(i)*lh + lh/2 + fs*0.35
				fmt.Fprintf(&b, `<tspan x="%s" y="%s">%s</tspan>`, svgutil.Num(cx+10), svgutil.Num(ty), svgutil.Esc(ln))
			}
			b.WriteString("</text>\n")
			if cl.meta {
				my := cy + cl.h - 10
				if c.Ticket != "" {
					fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s">%s</text>`+"\n",
						svgutil.Num(cx+10), svgutil.Num(my), pal.Text, svgutil.Num(metaFs), svgutil.Esc(clip(face, c.Ticket, metaFs, cardW/2-14)))
				}
				if c.Assigned != "" {
					fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="end">%s</text>`+"\n",
						svgutil.Num(cx+cardW-10), svgutil.Num(my), pal.Text, svgutil.Num(metaFs), svgutil.Esc(clip(face, c.Assigned, metaFs, cardW/2-14)))
				}
			}
			cy += cl.h + cardGap
		}
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// clip shortens s with an ellipsis to fit maxW.
func clip(face svgutil.Face, s string, fs, maxW float64) string {
	if face.Width(s, fs) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && face.Width(string(r)+"…", fs) > maxW {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
