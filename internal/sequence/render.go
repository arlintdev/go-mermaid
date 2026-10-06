package sequence

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgid"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls sequence diagram appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// Render parses, lays out, and renders sequence diagram source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	if o.FontSize <= 0 {
		o.FontSize = 14
	}
	if o.Title == "" {
		o.Title = d.Title
	}
	lay := Compute(d, Options{FontSize: o.FontSize, Padding: o.Padding, FontFace: o.FontFace})
	return svg(lay, o, svgid.Prefix(src)), nil
}

// colours are the fixed colours of a sequence diagram beyond the palette.
type colours struct {
	theme.Palette
	noteFill, noteStroke, noteText string
	barFill, barStroke             string
	lifeline                       string
	badgeText                      string
}

func coloursFor(name string) colours {
	pal := theme.For(name)
	c := colours{
		Palette:  pal,
		noteFill: "#fff5ad", noteStroke: "#aaaa33", noteText: "#333333",
		barFill: "#f4f4f4", barStroke: "#666666",
		lifeline:  pal.NodeStroke,
		badgeText: "#ffffff",
	}
	if name == "dark" {
		c.barFill, c.barStroke = pal.NodeFill, pal.NodeStroke
		c.badgeText = pal.Background
	}
	return c
}

type writer struct {
	b   strings.Builder
	lay *Layout
	m   metrics
	c   colours
	id  string
}

func (w *writer) f(format string, args ...any) {
	w.b.WriteString("    ")
	fmt.Fprintf(&w.b, format, args...)
	w.b.WriteByte('\n')
}

func num(f float64) string { return svgutil.Num(f) }

func svg(lay *Layout, o RenderOptions, prefix string) []byte {
	w := &writer{lay: lay, m: lay.m, c: coloursFor(o.Theme), id: prefix}
	m := lay.m
	pad := o.Padding
	titleH := svgutil.TitleHeight(o.Title, o.FontSize)
	contentW := lay.Width
	if tw := m.face.Width(o.Title, o.FontSize); tw > contentW {
		contentW = tw
	}
	width := contentW + pad*2
	height := lay.Height + titleH + pad*2
	b := &w.b

	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`,
		num(width), num(height), num(width), num(height), svgutil.Esc(o.FontFace), num(o.FontSize))
	b.WriteByte('\n')
	w.defs()
	fmt.Fprintf(b, `  <rect width="100%%" height="100%%" fill="%s"/>`, w.c.Background)
	b.WriteByte('\n')
	if o.Title != "" {
		fmt.Fprintf(b, `  <text x="%s" y="%s" fill="%s" text-anchor="middle" font-weight="bold">%s</text>`,
			num(width/2), num(pad+o.FontSize), w.c.Text, svgutil.Esc(o.Title))
		b.WriteByte('\n')
	}
	dx := pad + lay.OffsetX + (contentW-lay.Width)/2
	fmt.Fprintf(b, `  <g transform="translate(%s,%s)">`, num(dx), num(pad+titleH))
	b.WriteByte('\n')

	d := lay.Diagram
	for _, x := range d.Boxes {
		w.box(x)
	}
	for _, f := range d.Frames {
		if f.Kind == "rect" {
			w.rect(f)
		}
	}
	for _, p := range d.Participants {
		w.f(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`,
			num(p.X), num(p.LifeStart), num(p.X), num(p.LifeEnd), w.c.lifeline)
	}
	for _, bar := range d.Bars {
		w.bar(bar)
	}
	for _, f := range d.Frames {
		if f.Kind != "rect" {
			w.frame(f)
		}
	}
	for _, msg := range d.Messages {
		w.message(msg)
	}
	for _, n := range d.Notes {
		w.note(n)
	}
	for _, p := range d.Participants {
		w.header(p, p.TopY, lay.actorInset(p))
		if !p.Destroyed {
			w.header(p, lay.BottomY, 0)
		} else {
			w.destroyMark(p)
		}
	}
	b.WriteString("  </g>\n</svg>\n")
	return []byte(b.String())
}

func (w *writer) defs() {
	k := w.m.k
	fmt.Fprintf(&w.b, `  <defs><marker id="%s-seq-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerUnits="userSpaceOnUse" markerWidth="%s" markerHeight="%s" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="%s"/></marker>`,
		w.id, num(10*k), num(10*k), w.c.Edge)
	fmt.Fprintf(&w.b, `<marker id="%s-seq-open" viewBox="0 0 10 10" refX="9" refY="5" markerUnits="userSpaceOnUse" markerWidth="%s" markerHeight="%s" orient="auto-start-reverse"><path d="M1,1 L9,5 L1,9" fill="none" stroke="%s" stroke-width="1.5"/></marker></defs>`,
		w.id, num(11*k), num(11*k), w.c.Edge)
	w.b.WriteByte('\n')
}

// lines writes a block of text lines, the first baseline at y.
func (w *writer) lines(ls []string, x, y float64, anchor, fill, extra string) {
	if len(ls) == 0 {
		return
	}
	if len(ls) == 1 {
		w.f(`<text x="%s" y="%s" fill="%s" text-anchor="%s"%s>%s</text>`,
			num(x), num(y), fill, anchor, extra, svgutil.Esc(ls[0]))
		return
	}
	var t strings.Builder
	for i, l := range ls {
		fmt.Fprintf(&t, `<tspan x="%s" y="%s">%s</tspan>`, num(x), num(y+float64(i)*w.m.lineH), svgutil.Esc(l))
	}
	w.f(`<text fill="%s" text-anchor="%s"%s>%s</text>`, fill, anchor, extra, t.String())
}

// centred writes lines centred vertically on cy.
func (w *writer) centred(ls []string, x, cy float64, anchor, fill, extra string) {
	first := cy - float64(len(ls)-1)*w.m.lineH/2 + w.m.fs*0.35
	w.lines(ls, x, first, anchor, fill, extra)
}

func (w *writer) header(p *Participant, top, inset float64) {
	m, c := w.m, w.c
	h := w.lay.HeadH
	if p.Kind == KindActor {
		k := m.k
		top += inset
		cx := p.X
		headR := 8 * k
		cy := top + 4*k + headR
		neck := cy + headR
		hip := neck + 13*k
		w.f(`<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="1.5"/>`,
			num(cx), num(cy), num(headR), c.NodeFill, c.NodeStroke)
		w.f(`<path d="M%s,%s L%s,%s M%s,%s L%s,%s M%s,%s L%s,%s L%s,%s" fill="none" stroke="%s" stroke-width="1.5"/>`,
			num(cx), num(neck), num(cx), num(hip),
			num(cx-11*k), num(neck+5*k), num(cx+11*k), num(neck+5*k),
			num(cx-10*k), num(hip+11*k), num(cx), num(hip), num(cx+10*k), num(hip+11*k),
			c.NodeStroke)
		labelTop := top + m.figH
		w.lines(p.Lines, cx, labelTop+m.fs*0.95, "middle", c.Text, "")
		return
	}
	w.f(`<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s" stroke="%s"/>`,
		num(p.X-p.Width/2), num(top), num(p.Width), num(h), c.NodeFill, c.NodeStroke)
	w.centred(p.Lines, p.X, top+h/2, "middle", c.Text, "")
}

func (w *writer) destroyMark(p *Participant) {
	s := 8 * w.m.k
	x, y := p.X, p.LifeEnd
	w.f(`<path d="M%s,%s L%s,%s M%s,%s L%s,%s" stroke="%s" stroke-width="2"/>`,
		num(x-s), num(y-s), num(x+s), num(y+s), num(x-s), num(y+s), num(x+s), num(y-s), w.c.Edge)
}

func (w *writer) box(x *Box) {
	if len(x.Members) == 0 {
		return
	}
	fill := "none"
	if x.Color != "" {
		fill = svgutil.Esc(x.Color)
	}
	y1 := w.lay.BottomY + w.lay.HeadH + w.m.boxPad
	w.f(`<rect x="%s" y="0" width="%s" height="%s" fill="%s" stroke="%s" stroke-opacity="0.6"/>`,
		num(x.X0), num(x.X1-x.X0), num(y1), fill, w.c.NodeStroke)
	w.lines(x.Lines, (x.X0+x.X1)/2, 4*w.m.k+w.m.fs, "middle", w.c.Text, "")
}

func (w *writer) rect(f *Frame) {
	if f.X1 <= f.X0 {
		return
	}
	fill, op := w.c.NodeFill, ` fill-opacity="0.5"`
	if f.Color != "" {
		fill, op = svgutil.Esc(f.Color), ""
	}
	w.f(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s"%s/>`,
		num(f.X0), num(f.Y0), num(f.X1-f.X0), num(f.Y1-f.Y0), fill, op)
}

func (w *writer) frame(f *Frame) {
	if f.X1 <= f.X0 {
		return
	}
	m, c := w.m, w.c
	k := m.k
	w.f(`<rect x="%s" y="%s" width="%s" height="%s" fill="none" stroke="%s" stroke-width="1.5" stroke-dasharray="3,3"/>`,
		num(f.X0), num(f.Y0), num(f.X1-f.X0), num(f.Y1-f.Y0), c.NodeStroke)
	tw := tabWidth(f, m)
	th := m.tabH
	cut := 7 * k
	w.f(`<polygon points="%s,%s %s,%s %s,%s %s,%s %s,%s" fill="%s" stroke="%s"/>`,
		num(f.X0), num(f.Y0), num(f.X0+tw), num(f.Y0), num(f.X0+tw), num(f.Y0+th-cut),
		num(f.X0+tw-cut), num(f.Y0+th), num(f.X0), num(f.Y0+th), c.NodeFill, c.NodeStroke)
	base := f.Y0 + th/2 + m.fs*0.35
	w.lines([]string{f.Kind}, f.X0+tw/2-2*k, base, "middle", c.Text, ` font-weight="bold"`)
	textX := f.X0 + tw + 8*k
	w.lines(f.Lines, textX, base, "start", c.Text, "")
	for _, s := range f.Sections {
		w.f(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.5" stroke-dasharray="3,3"/>`,
			num(f.X0), num(s.Y), num(f.X1), num(s.Y), c.NodeStroke)
		w.lines(s.Lines, textX, s.Y+m.fs+2*k, "start", c.Text, "")
	}
}

func (w *writer) bar(bar *Bar) {
	p := w.lay.Diagram.participant(bar.Participant)
	if p == nil {
		return
	}
	bw := w.m.barW
	x := p.X - bw/2 + float64(bar.Depth)*bw/2
	w.f(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"/>`,
		num(x), num(bar.Y1), num(bw), num(bar.Y2-bar.Y1), w.c.barFill, w.c.barStroke)
}

func (w *writer) message(msg *Message) {
	m, c := w.m, w.c
	k := m.k
	attrs := ` stroke-width="1.5"`
	if msg.Arrow.Dashed {
		attrs += ` stroke-dasharray="3,3"`
	}
	head := ""
	switch msg.Arrow.Head {
	case HeadArrow:
		head = w.id + "-seq-arrow"
	case HeadOpen:
		head = w.id + "-seq-open"
	}
	if head != "" {
		attrs += fmt.Sprintf(` marker-end="url(#%s)"`, head)
		if msg.Arrow.Both {
			attrs += fmt.Sprintf(` marker-start="url(#%s)"`, head)
		}
	}
	y := msg.Y
	textBottom := y - 6*k - m.fs*0.25
	first := textBottom - float64(len(msg.Lines)-1)*m.lineH

	if msg.From == msg.To {
		x1, x2 := msg.X1, msg.X2
		bottom := y + m.loopH
		reach := x1 + m.loopW
		w.f(`<path d="M%s,%s C%s,%s %s,%s %s,%s" fill="none" stroke="%s"%s/>`,
			num(x1), num(y), num(reach), num(y-4*k), num(reach), num(bottom+4*k), num(x2), num(bottom), c.Edge, attrs)
		if msg.Arrow.Head == HeadCross {
			w.cross(x2+5*k, bottom)
		}
		p := w.lay.Diagram.participant(msg.From)
		w.lines(msg.Lines, p.X+selfLabelOffset(m, msg), first, "start", c.Text, "")
		w.badge(msg)
		return
	}
	x2 := msg.X2
	if msg.Arrow.Head == HeadCross {
		x2 -= sign(msg.X2-msg.X1) * 1 * k
	}
	w.f(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"%s/>`,
		num(msg.X1), num(y), num(x2), num(y), c.Edge, attrs)
	if msg.Arrow.Head == HeadCross {
		w.cross(msg.X2-sign(msg.X2-msg.X1)*5*k, y)
	}
	w.lines(msg.Lines, (msg.X1+msg.X2)/2, first, "middle", c.Text, "")
	w.badge(msg)
}

// badge draws the autonumber circle at the start of a message.
func (w *writer) badge(msg *Message) {
	if msg.Num <= 0 {
		return
	}
	r := numRadius(w.m, msg.Num)
	w.f(`<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, num(msg.X1), num(msg.Y), num(r), w.c.Edge)
	w.f(`<text x="%s" y="%s" fill="%s" text-anchor="middle" font-size="%s">%s</text>`,
		num(msg.X1), num(msg.Y+w.m.fs*0.8*0.35), w.c.badgeText, num(w.m.fs*0.8), strconv.Itoa(msg.Num))
}

func (w *writer) cross(x, y float64) {
	s := 4 * w.m.k
	w.f(`<path d="M%s,%s L%s,%s M%s,%s L%s,%s" stroke="%s" stroke-width="1.5"/>`,
		num(x-s), num(y-s), num(x+s), num(y+s), num(x-s), num(y+s), num(x+s), num(y-s), w.c.Edge)
}

func (w *writer) note(n *Note) {
	c := w.c
	w.f(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"/>`,
		num(n.X), num(n.Y), num(n.W), num(n.H), c.noteFill, c.noteStroke)
	w.centred(n.Lines, n.X+n.W/2, n.Y+n.H/2, "middle", c.noteText, "")
}

var plainFont = regexp.MustCompile(`^[A-Za-z0-9 ,'"_-]{1,200}$`)
