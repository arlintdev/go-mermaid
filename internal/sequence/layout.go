package sequence

import (
	"math"
	"strconv"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// Options tunes sequence diagram spacing and metrics.
type Options struct {
	FontSize float64
	Padding  float64
	FontFace string // CSS font-family the SVG will ask for; picks the metrics
}

// metrics are the sizes the layout works in, scaled with the font size so a
// larger font gets proportionally roomier boxes and gaps.
type metrics struct {
	fs, k, lineH float64
	face         svgutil.Face

	actorMinW, actorPadX, actorH, actorGap float64
	figH                                   float64 // stick figure height
	wrapW                                  float64 // width labels wrap at when there is no more room
	msgPad                                 float64 // room either side of a message label
	numR                                   float64 // autonumber badge radius
	loopW, loopH                           float64 // self-message loop
	barW                                   float64 // activation bar width
	noteMargin, notePadX, notePadY         float64
	noteMinW                               float64
	boxPad                                 float64
	frameMargin, tabH                      float64
}

func newMetrics(o Options) metrics {
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	k := fs / 14
	return metrics{
		fs: fs, k: k, lineH: fs * 1.3, face: svgutil.FaceFor(o.FontFace),
		actorMinW: 130 * k, actorPadX: 14 * k, actorH: 52 * k, actorGap: 44 * k,
		figH:  40 * k,
		wrapW: 210 * k, msgPad: 16 * k, numR: 8 * k,
		loopW: 34 * k, loopH: 20 * k,
		barW:       10 * k,
		noteMargin: 10 * k, notePadX: 10 * k, notePadY: 7 * k, noteMinW: 80 * k,
		boxPad:      10 * k,
		frameMargin: 12 * k, tabH: fs*1.3 + 6*k,
	}
}

// Layout holds computed geometry for rendering.
type Layout struct {
	Diagram *Diagram
	Width   float64
	Height  float64
	// OffsetX shifts the drawing right so content left of the first
	// lifeline (a left note, a frame) stays on the canvas.
	OffsetX float64

	HeadH   float64 // participant header height
	HeadTop float64 // y of the top headers
	BottomY float64 // y of the mirrored bottom headers

	m metrics
}

type constraint struct {
	i, j int
	d    float64
}

// Compute assigns positions to everything in the diagram.
func Compute(d *Diagram, opts Options) *Layout {
	m := newMetrics(opts)
	lay := &Layout{Diagram: d, m: m}
	ps := d.Participants

	lay.HeadH = m.actorH
	for _, p := range ps {
		p.Lines = wrap(p.Label, m.wrapW, m.face, m.fs)
		if len(p.Lines) == 0 {
			p.Lines = []string{""}
		}
		p.Width = max(m.actorMinW, widest(p.Lines, m.face, m.fs)+2*m.actorPadX)
		n := float64(len(p.Lines))
		h := n*m.lineH + 24*m.k
		if p.Kind == KindActor {
			h = m.figH + n*m.lineH + 8*m.k
		}
		lay.HeadH = max(lay.HeadH, h)
	}

	xs := place(d, m, nil)
	cons := measure(d, m, xs)
	for _, b := range d.Boxes {
		if len(b.Members) > 0 {
			b.X0, b.X1 = boxSpan(d, b, m, xs)
			b.grow = boxGrow(b, m)
		}
	}
	xs = place(d, m, cons)
	measure(d, m, xs)
	for i, p := range ps {
		p.X = xs[i]
	}

	boxLabelH := 0.0
	for _, b := range d.Boxes {
		if len(b.Members) == 0 {
			continue
		}
		b.X0, b.X1 = boxSpan(d, b, m, xs)
		g := boxGrow(b, m)
		b.X0, b.X1 = b.X0-g, b.X1+g
		boxLabelH = max(boxLabelH, m.boxPad, float64(len(b.Lines))*m.lineH+10*m.k)
	}
	lay.HeadTop = boxLabelH
	vertical(lay)
	lay.bounds()
	return lay
}

// boxSpan returns the left and right edges of a box around its members at
// the lifeline positions xs.
func boxSpan(d *Diagram, b *Box, m metrics, xs []float64) (x0, x1 float64) {
	x0, x1 = math.Inf(1), math.Inf(-1)
	for _, p := range b.Members {
		x := xs[d.index[p.ID]]
		x0 = min(x0, x-p.Width/2-m.boxPad)
		x1 = max(x1, x+p.Width/2+m.boxPad)
	}
	return x0, x1
}

// boxGrow wraps a box's label to the box's width and returns how far the box
// must grow on each side to hold it: a word wider than the box stays whole.
func boxGrow(b *Box, m metrics) float64 {
	inner := b.X1 - b.X0 - 16*m.k
	b.Lines = wrap(b.Label, inner, m.face, m.fs)
	return max(0, widest(b.Lines, m.face, m.fs)-inner) / 2
}

// place returns lifeline x positions, left to right, honouring the header
// widths and every constraint (a minimum distance between two lifelines).
func place(d *Diagram, m metrics, cons []constraint) []float64 {
	ps := d.Participants
	xs := make([]float64, len(ps))
	byJ := map[int][]constraint{}
	for _, c := range cons {
		byJ[c.j] = append(byJ[c.j], c)
	}
	for j, p := range ps {
		if j == 0 {
			xs[0] = p.Width / 2
			if p.Box != nil {
				xs[0] += m.boxPad + p.Box.grow
			}
			continue
		}
		prev := ps[j-1]
		gap := prev.Width/2 + p.Width/2 + m.actorGap
		if prev.Box != p.Box {
			if prev.Box != nil {
				gap += m.boxPad + prev.Box.grow
			}
			if p.Box != nil {
				gap += m.boxPad + p.Box.grow
			}
		}
		x := xs[j-1] + gap
		for _, c := range byJ[j] {
			x = max(x, xs[c.i]+c.d)
		}
		xs[j] = x
	}
	return xs
}

// measure wraps every message and note label for the lifeline positions xs
// and returns the distances the labels need between lifelines.
func measure(d *Diagram, m metrics, xs []float64) []constraint {
	var cons []constraint
	n := len(xs)
	idx := d.index
	for _, msg := range d.Messages {
		i, j := idx[msg.From], idx[msg.To]
		label := msg.Text
		numW := 0.0
		if msg.Num > 0 {
			numW = numRadius(m, msg.Num) + 4*m.k
		}
		if i == j {
			off := selfLabelOffset(m, msg)
			avail := 0.0
			if i+1 < n {
				avail = xs[i+1] - xs[i] - off - m.msgPad
			}
			msg.Lines = wrap(label, max(avail, m.wrapW), m.face, m.fs)
			if i+1 < n {
				need := max(off+widest(msg.Lines, m.face, m.fs)+m.msgPad, m.loopW+m.barW+m.msgPad)
				cons = append(cons, constraint{i, i + 1, need})
			}
			continue
		}
		lo, hi := min(i, j), max(i, j)
		avail := xs[hi] - xs[lo] - 2*m.msgPad - 2*numW - m.barW
		msg.Lines = wrap(label, max(avail, m.wrapW), m.face, m.fs)
		need := widest(msg.Lines, m.face, m.fs) + 2*m.msgPad + 2*numW + m.barW
		if msg.Creates != "" {
			if c := d.participant(msg.Creates); c != nil {
				need += c.Width / 2
			}
		}
		cons = append(cons, constraint{lo, hi, need})
	}
	for _, note := range d.Notes {
		i := idx[note.Of[0]]
		j := idx[note.Of[len(note.Of)-1]]
		pad := 2 * m.notePadX
		switch {
		case note.Pos == NoteRight:
			avail := 0.0
			if i+1 < n {
				avail = xs[i+1] - xs[i] - 2*m.noteMargin - m.barW
			}
			note.Lines = wrap(note.Text, max(avail, m.wrapW+pad)-pad, m.face, m.fs)
			note.W = max(widest(note.Lines, m.face, m.fs)+pad, m.noteMinW)
			if i+1 < n {
				cons = append(cons, constraint{i, i + 1, note.W + 2*m.noteMargin + m.barW})
			}
		case note.Pos == NoteLeft:
			avail := 0.0
			if i > 0 {
				avail = xs[i] - xs[i-1] - 2*m.noteMargin - m.barW
			}
			note.Lines = wrap(note.Text, max(avail, m.wrapW+pad)-pad, m.face, m.fs)
			note.W = max(widest(note.Lines, m.face, m.fs)+pad, m.noteMinW)
			if i > 0 {
				cons = append(cons, constraint{i - 1, i, note.W + 2*m.noteMargin + m.barW})
			}
		case i == j:
			note.Lines = wrap(note.Text, m.wrapW, m.face, m.fs)
			note.W = max(widest(note.Lines, m.face, m.fs)+pad, m.noteMinW)
			if i > 0 {
				cons = append(cons, constraint{i - 1, i, note.W/2 + m.noteMargin})
			}
			if i+1 < n {
				cons = append(cons, constraint{i, i + 1, note.W/2 + m.noteMargin})
			}
		default:
			lo, hi := min(i, j), max(i, j)
			span := xs[hi] - xs[lo] + 2*overhang(m)
			note.Lines = wrap(note.Text, max(span, 1.5*m.wrapW+pad)-pad, m.face, m.fs)
			note.W = max(widest(note.Lines, m.face, m.fs)+pad, span)
		}
	}
	return append(cons, frameConstraints(d, m)...)
}

// frameLines wraps a frame's condition or a section's label. A word too long
// to wrap may widen the frame up to frameWordMax; past that it is cut.
func frameLines(label string, m metrics) []string {
	if label == "" {
		return nil
	}
	var out []string
	for _, l := range wrap("["+label+"]", m.wrapW*1.4, m.face, m.fs) {
		out = append(out, m.face.WrapWithin(l, m.fs, frameWordMax*m.k)...)
	}
	return out
}

// frameWordMax is the widest a frame's label line may be, at 14px.
const frameWordMax = 520.0

// frameConstraints keeps every frame wide enough for its labels by moving
// lifelines apart: a frame around several lifelines spreads them, and one
// around a single lifeline pushes the next one clear of its right edge, so
// a frame never reaches over a lifeline it does not enclose.
func frameConstraints(d *Diagram, m metrics) []constraint {
	type span struct {
		f      *Frame
		lo, hi int
	}
	var stack []span
	var cons []constraint
	n := len(d.Participants)
	cover := func(i, j int) {
		if len(stack) == 0 {
			return
		}
		top := &stack[len(stack)-1]
		if top.lo < 0 {
			top.lo, top.hi = i, j
			return
		}
		top.lo, top.hi = min(top.lo, i), max(top.hi, j)
	}
	for _, it := range d.items {
		switch it.kind {
		case itMessage:
			i, j := d.index[it.msg.From], d.index[it.msg.To]
			cover(min(i, j), max(i, j))
		case itNote:
			i, j := d.index[it.note.Of[0]], d.index[it.note.Of[len(it.note.Of)-1]]
			cover(min(i, j), max(i, j))
		case itFrameStart:
			if it.frame.Kind != "rect" {
				it.frame.Lines = frameLines(it.frame.Label, m)
			}
			stack = append(stack, span{f: it.frame, lo: -1, hi: -1})
		case itSection:
			it.section.Lines = frameLines(it.section.Label, m)
		case itFrameEnd:
			if len(stack) == 0 || stack[len(stack)-1].f != it.frame {
				continue
			}
			sp := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if sp.lo < 0 {
				sp.lo, sp.hi = 0, n-1
			}
			if w := frameMinWidth(sp.f, m) - 2*m.frameMargin; sp.lo < sp.hi {
				cons = append(cons, constraint{sp.lo, sp.hi, w})
			} else if sp.hi+1 < n {
				cons = append(cons, constraint{sp.hi, sp.hi + 1, w + m.frameMargin + m.barW + m.frameMargin})
			}
			cover(sp.lo, sp.hi)
		}
	}
	return cons
}

// overhang is how far a note over several participants reaches past the
// outer lifelines.
func overhang(m metrics) float64 { return 24 * m.k }

// selfLabelOffset is where a self-message's label starts, right of the
// lifeline, clear of the autonumber badge.
func selfLabelOffset(m metrics, msg *Message) float64 {
	off := m.barW + 6*m.k
	if msg.Num > 0 {
		off = max(off, numRadius(m, msg.Num)+6*m.k)
	}
	return off
}

// numRadius is the autonumber badge radius, wide enough for its digits.
func numRadius(m metrics, n int) float64 {
	w := m.face.Width(strconv.Itoa(n), m.fs*0.8)
	return max(m.numR, w/2+4*m.k)
}

// frameExt accumulates the horizontal extent of what a frame encloses.
type frameExt struct {
	lo, hi float64
	set    bool
}

func (e *frameExt) add(lo, hi float64) {
	if !e.set {
		e.lo, e.hi, e.set = lo, hi, true
		return
	}
	e.lo, e.hi = min(e.lo, lo), max(e.hi, hi)
}

// vertical walks the statements top to bottom, placing each below the last.
func vertical(lay *Layout) {
	d, m := lay.Diagram, lay.m
	k := m.k
	for _, p := range d.Participants {
		p.TopY = lay.HeadTop
		p.LifeStart = lay.headBottom(p)
		p.LifeEnd = -1
	}
	cursor := lay.HeadTop + lay.HeadH + 8*k
	lastY := cursor
	stacks := map[string][]float64{}
	var frames []*Frame
	var exts []frameExt

	addExt := func(lo, hi float64) {
		if n := len(exts); n > 0 {
			exts[n-1].add(lo, hi)
		}
	}
	// edge is where a line meets participant p's lifeline or its top
	// activation bar, on the side facing x.
	edge := func(p *Participant, x float64) float64 {
		depth := len(stacks[p.ID])
		if depth == 0 {
			return p.X
		}
		left := p.X - m.barW/2 + float64(depth-1)*m.barW/2
		if x < p.X {
			return left
		}
		return left + m.barW
	}
	push := func(id string, y float64) {
		stacks[id] = append(stacks[id], y)
	}
	pop := func(id string, y float64) {
		st := stacks[id]
		if len(st) == 0 {
			return
		}
		y1 := st[len(st)-1]
		stacks[id] = st[:len(st)-1]
		d.Bars = append(d.Bars, &Bar{Participant: id, Depth: len(st) - 1, Y1: y1, Y2: max(y, y1+m.lineH)})
	}

	for _, it := range d.items {
		switch it.kind {
		case itMessage:
			msg := it.msg
			from, to := d.participant(msg.From), d.participant(msg.To)
			textH := float64(len(msg.Lines)) * m.lineH
			y := cursor + 10*k + textH + 6*k
			if msg.Num > 0 {
				y = max(y, cursor+10*k+numRadius(m, msg.Num))
			}
			created := d.participant(msg.Creates)
			if created != nil {
				y = max(y, cursor+lay.HeadH/2+6*k)
				created.TopY = y - lay.HeadH/2
				created.LifeStart = lay.headBottom(created)
			}
			msg.Y = y
			if from == to {
				msg.X1 = edge(from, from.X+1)
				if msg.Activate {
					push(to.ID, y)
				}
				msg.X2 = edge(to, to.X+1)
				bottom := y + m.loopH
				labelR := from.X + selfLabelOffset(m, msg) + widest(msg.Lines, m.face, m.fs)
				lo := from.X - m.barW/2
				if msg.Num > 0 {
					lo = min(lo, msg.X1-numRadius(m, msg.Num))
				}
				addExt(lo, max(msg.X1+m.loopW, labelR))
				if msg.Deactivate {
					pop(from.ID, bottom)
				}
				for _, id := range msg.Destroys {
					d.participant(id).LifeEnd = bottom
				}
				lastY = bottom
				cursor = bottom + 10*k
				continue
			}
			if msg.Activate {
				push(to.ID, y)
			}
			msg.X1 = edge(from, to.X)
			msg.X2 = edge(to, from.X)
			if created == to {
				msg.X2 = to.X - sign(to.X-from.X)*to.Width/2
			}
			if created == from {
				msg.X1 = from.X + sign(to.X-from.X)*from.Width/2
			}
			if msg.Deactivate {
				pop(from.ID, y)
			}
			for _, id := range msg.Destroys {
				d.participant(id).LifeEnd = y
			}
			lo, hi := min(from.X, to.X)-m.barW/2, max(from.X, to.X)+m.barW/2
			if msg.Num > 0 {
				r := numRadius(m, msg.Num)
				lo, hi = min(lo, msg.X1-r), max(hi, msg.X1+r)
			}
			addExt(lo, hi)
			lastY = y
			cursor = y + 10*k
			if created != nil {
				cursor = max(cursor, y+lay.HeadH/2+10*k)
			}
		case itNote:
			n := it.note
			n.H = float64(len(n.Lines))*m.lineH + 2*m.notePadY
			if len(n.Lines) == 0 {
				n.H = m.lineH + 2*m.notePadY
			}
			n.Y = cursor + 10*k
			p := d.participant(n.Of[0])
			switch n.Pos {
			case NoteRight:
				n.X = edge(p, p.X+1) + m.noteMargin
			case NoteLeft:
				n.X = edge(p, p.X-1) - m.noteMargin - n.W
			default:
				q := d.participant(n.Of[len(n.Of)-1])
				n.X = (p.X+q.X)/2 - n.W/2
			}
			addExt(n.X, n.X+n.W)
			cursor = n.Y + n.H
			lastY = cursor
		case itFrameStart:
			f := it.frame
			f.Depth = len(frames)
			frames = append(frames, f)
			exts = append(exts, frameExt{})
			if f.Kind == "rect" {
				f.Y0 = cursor + 6*k
				cursor = f.Y0 + 2*k
				continue
			}
			f.Y0 = cursor + 10*k
			f.Lines = frameLines(f.Label, m)
			cursor = f.Y0 + max(m.tabH, float64(len(f.Lines))*m.lineH+8*k)
		case itSection:
			s := it.section
			s.Y = cursor + 8*k
			s.Lines = frameLines(s.Label, m)
			cursor = s.Y + float64(len(s.Lines))*m.lineH + 4*k
		case itFrameEnd:
			n := len(frames)
			if n == 0 || frames[n-1] != it.frame {
				continue
			}
			f := it.frame
			e := exts[n-1]
			frames, exts = frames[:n-1], exts[:n-1]
			if !e.set {
				e = allSpan(d)
			}
			margin := m.frameMargin
			if f.Kind == "rect" {
				margin = 8 * k
			}
			f.X0, f.X1 = e.lo-margin, e.hi+margin
			if w := frameMinWidth(f, m); f.X1-f.X0 < w {
				f.X1 = f.X0 + w
			}
			if f.Kind == "rect" {
				f.Y1 = cursor + 6*k
			} else {
				f.Y1 = cursor + 10*k
			}
			cursor = f.Y1
			addExt(f.X0, f.X1)
		case itActivate:
			if p := d.participant(it.who); p != nil {
				push(p.ID, lastY)
			}
		case itDeactivate:
			pop(it.who, lastY)
		case itDestroy:
			if p := d.participant(it.who); p != nil {
				p.LifeEnd = cursor + 6*k
				cursor += 14 * k
			}
		}
	}

	end := cursor + 12*k
	for _, p := range d.Participants {
		if p.LifeEnd < 0 {
			p.LifeEnd = end
		}
	}
	for _, p := range d.Participants {
		for len(stacks[p.ID]) > 0 {
			pop(p.ID, end-6*k)
		}
	}
	lay.BottomY = end
	lay.Height = end + lay.HeadH
	if len(d.Boxes) > 0 {
		lay.Height += m.boxPad
	}
}

// actorInset is how far below the slot top an actor's figure starts in a
// top header: the figure and its name are centred in the header slot.
func (lay *Layout) actorInset(p *Participant) float64 {
	return (lay.HeadH - actorBlockH(p, lay.m)) / 2
}

func actorBlockH(p *Participant, m metrics) float64 {
	return m.figH + float64(len(p.Lines))*m.lineH + 2*m.k
}

// headBottom is where p's lifeline starts below its top header.
func (lay *Layout) headBottom(p *Participant) float64 {
	if p.Kind == KindActor {
		return p.TopY + lay.actorInset(p) + actorBlockH(p, lay.m)
	}
	return p.TopY + lay.HeadH
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// allSpan is the extent of an empty frame: every lifeline.
func allSpan(d *Diagram) frameExt {
	var e frameExt
	for _, p := range d.Participants {
		e.add(p.X, p.X)
	}
	return e
}

// tabWidth is the width of a frame's keyword tab.
func tabWidth(f *Frame, m metrics) float64 {
	return m.face.Width(f.Kind, m.fs) + 20*m.k
}

// frameMinWidth is the narrowest a frame may be and still hold its tab,
// condition and section labels.
func frameMinWidth(f *Frame, m metrics) float64 {
	if f.Kind == "rect" {
		return 0
	}
	textX := tabWidth(f, m) + 8*m.k
	w := textX + widest(f.Lines, m.face, m.fs) + 12*m.k
	for _, s := range f.Sections {
		w = max(w, textX+widest(s.Lines, m.face, m.fs)+12*m.k)
	}
	return w
}

// bounds sizes the canvas to everything drawn and shifts content that
// reaches left of x=0 back onto it.
func (lay *Layout) bounds() {
	d, m := lay.Diagram, lay.m
	var b svgutil.Bounds
	b.Add(0, 0)
	for _, p := range d.Participants {
		b.Add(p.X-p.Width/2, 0)
		b.Add(p.X+p.Width/2, 0)
	}
	for _, x := range d.Boxes {
		if len(x.Members) > 0 {
			b.Add(x.X0, 0)
			b.Add(x.X1, 0)
		}
	}
	for _, n := range d.Notes {
		b.Add(n.X, 0)
		b.Add(n.X+n.W, 0)
	}
	for _, f := range d.Frames {
		if f.X1 > f.X0 {
			b.Add(f.X0, 0)
			b.Add(f.X1, 0)
		}
	}
	for _, msg := range d.Messages {
		w := widest(msg.Lines, m.face, m.fs)
		if msg.From == msg.To {
			p := d.participant(msg.From)
			b.Add(msg.X1+m.loopW+2*m.k, 0)
			b.Add(p.X+selfLabelOffset(m, msg)+w, 0)
			continue
		}
		mid := (msg.X1 + msg.X2) / 2
		b.Add(mid-w/2, 0)
		b.Add(mid+w/2, 0)
		if msg.Num > 0 {
			r := numRadius(m, msg.Num)
			b.Add(msg.X1-r, 0)
			b.Add(msg.X1+r, 0)
		}
	}
	lay.OffsetX, _ = b.Offset()
	lay.Width, _ = b.Size()
}
