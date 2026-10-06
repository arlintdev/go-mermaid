package git

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls gitGraph appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

// laneColor is a branch's colour on Mermaid's default git scale, with the
// text colour for its label and the colour of a highlighted commit on it.
type laneColor struct{ fill, label, highlight string }

var laneColors = []laneColor{
	{"#0000ec", "#ffffff", "#131300"},
	{"#dede00", "#000000", "#0000a1"},
	{"#9eec00", "#000000", "#310093"},
	{"#0076ec", "#ffffff", "#934900"},
	{"#00ecec", "#000000", "#930000"},
	{"#00ec76", "#000000", "#930049"},
	{"#ec00ec", "#000000", "#009300"},
	{"#ec0000", "#000000", "#009393"},
}

const (
	step     = 50.0 // between commits along the time axis
	laneGap  = 90.0 // between lanes (across)
	dotR     = 10.0
	arcR     = 20.0
	arrowW   = 8.0
	labelGap = 19.0 // branch label to the first commit
)

// Render parses and renders gitGraph source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return svg(d, o), nil
}

type pt struct{ t, l float64 } // along the time axis, across the lanes

type renderer struct {
	d        *Diagram
	o        RenderOptions
	pal      theme.Palette
	face     svgutil.Face
	fs, sfs  float64 // label and small (commit id, tag) font sizes
	vertical bool
	flip     bool // BT: time runs upward
	tmax     float64
	gap      float64
	pos      map[*Commit]pt
	laneAt   map[string]float64
	color    map[string]laneColor
	ox, oy   float64 // where the time/lane origin sits on the canvas
}

// xy maps a (time, lane) point to the canvas.
func (r *renderer) xy(p pt) (float64, float64) {
	if !r.vertical {
		return r.ox + p.t, r.oy + p.l
	}
	t := p.t
	if r.flip {
		t = r.tmax - p.t
	}
	return r.ox + p.l, r.oy + t
}

func (r *renderer) xys(p pt) string {
	x, y := r.xy(p)
	return svgutil.Num(x) + "," + svgutil.Num(y)
}

func svg(d *Diagram, o RenderOptions) []byte {
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	r := &renderer{d: d, o: o, pal: theme.For(o.Theme), face: svgutil.FaceFor(o.FontFace), fs: fs, sfs: fs * 0.75,
		vertical: d.Direction != "LR", flip: d.Direction == "BT",
		pos: map[*Commit]pt{}, laneAt: map[string]float64{}, color: map[string]laneColor{}}

	// Lanes. Vertical graphs set commit ids beside the commit, so a lane
	// is as wide as its longest id.
	gap := laneGap
	if r.vertical {
		// Ids and tags sit before their commit, between it and the lane
		// before; the lane gap makes room for the widest pair.
		for _, c := range d.Commits {
			need := dotR + 30 + r.tagWidth(c)
			if r.showID(c) {
				need += r.face.Width(c.ID, r.sfs) + 10
			}
			gap = max(gap, need)
		}
	}
	r.gap = gap
	for _, b := range d.Branches {
		r.laneAt[b.Name] = float64(b.Lane) * gap
		r.color[b.Name] = laneColors[b.Lane%len(laneColors)]
	}

	// Commits along the time axis; a tagged commit gets room for its tag
	// next to the previous tagged commit on the same lane.
	lastTag := map[string]float64{}
	t := 0.0
	for i, c := range d.Commits {
		if i > 0 {
			t += step
		}
		if len(c.Tags) > 0 && !r.vertical {
			w := r.tagWidth(c) / 2
			if prev, ok := lastTag[c.Branch]; ok {
				t = max(t, prev+w+8)
			}
			lastTag[c.Branch] = t + w
		}
		r.pos[c] = pt{t, r.laneAt[c.Branch]}
	}
	r.tmax = t

	// Canvas: branch labels before the first commit (left, or above), tags
	// above (or to the left), ids below (or to the right).
	pad := o.Padding
	maxLabel := 0.0
	for _, b := range d.Branches {
		maxLabel = max(maxLabel, r.face.Width(b.Name, fs)+18)
	}
	lastLane := float64(len(d.Branches)-1) * gap
	titleH := 0.0
	if o.Title != "" {
		titleH = fs*1.4 + 12
	}
	var w, h float64
	if !r.vertical {
		r.ox = pad + maxLabel + labelGap + dotR
		r.oy = pad + titleH + max(dotR, r.fs*0.75) + 4
		if r.hasTagOn(0) {
			r.oy += r.sfs + 12
		}
		w = r.ox + r.tmax + dotR + 40 + pad
		h = r.oy + lastLane + dotR + r.idDrop() + pad
	} else {
		r.ox = pad + dotR + 12
		for _, c := range d.Commits {
			if r.d.branch(c.Branch).Lane != 0 {
				continue
			}
			need := r.tagWidth(c) + dotR + 12
			if r.showID(c) {
				need += r.face.Width(c.ID, r.sfs) + 14
			}
			r.ox = max(r.ox, pad+need)
		}
		r.oy = pad + titleH + 21 + labelGap + dotR
		lastName := d.Branches[0].Name
		for _, br := range d.Branches {
			if br.Lane == len(d.Branches)-1 {
				lastName = br.Name
			}
			if br.Lane == 0 {
				r.ox = max(r.ox, pad+(r.face.Width(br.Name, fs)+18)/2)
			}
		}
		w = r.ox + lastLane + max(dotR+30, (r.face.Width(lastName, fs)+18)/2+4) + pad
		h = r.oy + r.tmax + dotR + 30 + pad
	}
	if o.Title != "" {
		w = max(w, r.face.Width(o.Title, fs*1.15)+2*pad)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(fs))
	fmt.Fprintf(&b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(r.pal.Background))
	if o.Title != "" {
		fmt.Fprintf(&b, `  <text x="%s" y="%s" fill="%s" font-size="%s" font-weight="bold" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+fs*1.15), svgutil.Esc(r.pal.Text), svgutil.Num(fs*1.15), svgutil.Esc(o.Title))
	}
	r.lanes(&b)
	r.arrows(&b)
	r.bullets(&b)
	r.labels(&b)
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

func (r *renderer) hasTagOn(lane int) bool {
	for _, c := range r.d.Commits {
		if len(c.Tags) > 0 && r.d.branch(c.Branch).Lane == lane {
			return true
		}
	}
	return false
}

// idDrop is how far below a lane the rotated commit ids reach.
func (r *renderer) idDrop() float64 {
	drop := 10.0
	for _, c := range r.d.Commits {
		if r.showID(c) {
			drop = max(drop, 16+(r.face.Width(c.ID, r.sfs)+6)*0.71)
		}
	}
	return drop
}

// showID reports whether a commit's id is written: always for an id the
// source gave, and for generated ids except on merges, as Mermaid does.
func (r *renderer) showID(c *Commit) bool {
	return c.CustomID || c.Type != MergeCommit && len(c.Parents) < 2
}

func (r *renderer) tagWidth(c *Commit) float64 {
	w := 0.0
	for _, t := range c.Tags {
		w = max(w, r.face.Width(t, r.sfs)+22)
	}
	return w
}

// lanes draws each branch's dotted guide line and its name.
func (r *renderer) lanes(b *strings.Builder) {
	end := r.tmax + 40
	for _, br := range r.d.Branches {
		l := r.laneAt[br.Name]
		x1, y1 := r.xy(pt{-dotR, l})
		x2, y2 := r.xy(pt{end, l})
		if r.flip {
			x1, y1 = r.xy(pt{r.tmax + dotR, l})
			x2, y2 = r.xy(pt{-40, l})
		}
		fmt.Fprintf(b, `  <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1" stroke-dasharray="2"/>`+"\n",
			svgutil.Num(x1), svgutil.Num(y1), svgutil.Num(x2), svgutil.Num(y2), svgutil.Esc(r.pal.Edge))
		c := r.color[br.Name]
		tw := r.face.Width(br.Name, r.fs)
		bw, bh := tw+18, r.fs+7
		var bx, by float64
		if !r.vertical {
			bx, by = x1-labelGap-bw, y1-bh/2
		} else {
			bx, by = x1-bw/2, y1-labelGap-bh
			if r.flip {
				by = y1 + labelGap
			}
		}
		fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" rx="4" fill="%s"/>`+"\n",
			svgutil.Num(bx), svgutil.Num(by), svgutil.Num(bw), svgutil.Num(bh), c.fill)
		fmt.Fprintf(b, `  <text x="%s" y="%s" fill="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(bx+bw/2), svgutil.Num(by+bh/2+r.fs*0.35), c.label, svgutil.Esc(br.Name))
	}
}

// arrows draws the thick coloured paths from each commit's parents to it:
// straight along a lane, else a rounded elbow. A branch's first commit
// leaves its parent across the lanes then runs along its own lane, in the
// branch's colour; a merge runs along the merged lane then across into the
// merge commit, in the merged branch's colour.
func (r *renderer) arrows(b *strings.Builder) {
	for _, c := range r.d.Commits {
		for i, p := range c.Parents {
			from, to := r.pos[p], r.pos[c]
			col := r.color[c.Branch].fill
			var d string
			switch {
			case from.l == to.l:
				d = "M" + r.xys(from) + " L" + r.xys(to)
			case i == 0:
				// Branch-off: across first, then along.
				s := sign(to.l - from.l)
				d = "M" + r.xys(from) + " L" + r.xys(pt{from.t, to.l - s*arcR}) +
					" " + r.arc(pt{from.t, to.l - s*arcR}, pt{from.t, to.l}, pt{from.t + arcR, to.l}) + " L" + r.xys(to)
			case r.blocked(from, to):
				// A cherry-pick whose source lane has later commits on it
				// leaves along its lane, then runs between the lanes, on the
				// side the commit ids do not use, so it crosses no commit.
				col = r.color[p.Branch].fill
				s := sign(to.l - from.l)
				m := max(from.l, to.l) - 30
				if r.vertical {
					m = min(from.l, to.l) + 30
				}
				t1 := from.t + step/2
				const k = 12.0
				d = "M" + r.xys(from) + " L" + r.xys(pt{t1 - k, from.l}) +
					" " + r.arcR(pt{t1 - k, from.l}, pt{t1, from.l}, pt{t1, from.l + s*k}, k) +
					" L" + r.xys(pt{t1, m - s*k}) +
					" " + r.arcR(pt{t1, m - s*k}, pt{t1, m}, pt{t1 + k, m}, k) +
					" L" + r.xys(pt{to.t - k, m}) +
					" " + r.arcR(pt{to.t - k, m}, pt{to.t, m}, pt{to.t, m + s*k}, k) + " L" + r.xys(to)
			default:
				// Merge or cherry-pick: along the source lane, then across.
				col = r.color[p.Branch].fill
				s := sign(to.l - from.l)
				d = "M" + r.xys(from) + " L" + r.xys(pt{to.t - arcR, from.l}) +
					" " + r.arc(pt{to.t - arcR, from.l}, pt{to.t, from.l}, pt{to.t, from.l + s*arcR}) + " L" + r.xys(to)
			}
			fmt.Fprintf(b, `  <path d="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round"/>`+"\n",
				d, col, svgutil.Num(arrowW))
		}
	}
}

// blocked reports whether a commit sits on from's lane between from and to.
func (r *renderer) blocked(from, to pt) bool {
	for _, q := range r.pos {
		if q.l == from.l && q.t > from.t && q.t < to.t {
			return true
		}
	}
	return false
}

// arc is the SVG arc command for a quarter turn from a to b around the
// corner where their straight legs would meet; the sweep follows the turn.
func (r *renderer) arc(a, corner, b pt) string { return r.arcR(a, corner, b, arcR) }

func (r *renderer) arcR(a, corner, b pt, radius float64) string {
	ax, ay := r.xy(a)
	cx, cy := r.xy(corner)
	bx, by := r.xy(b)
	sweep := "0"
	if (cx-ax)*(by-cy)-(cy-ay)*(bx-cx) > 0 {
		sweep = "1"
	}
	return fmt.Sprintf("A%s,%s 0 0 %s %s,%s", svgutil.Num(radius), svgutil.Num(radius), sweep, svgutil.Num(bx), svgutil.Num(by))
}

func (r *renderer) bullets(b *strings.Builder) {
	inner := svgutil.Esc(r.pal.NodeFill)
	for _, c := range r.d.Commits {
		x, y := r.xy(r.pos[c])
		col := r.color[c.Branch]
		n := svgutil.Num
		switch {
		case c.Type == Highlight:
			fmt.Fprintf(b, `  <rect x="%s" y="%s" width="20" height="20" fill="%s"/>`+"\n", n(x-10), n(y-10), col.highlight)
			fmt.Fprintf(b, `  <rect x="%s" y="%s" width="12" height="12" fill="%s"/>`+"\n", n(x-6), n(y-6), inner)
		case c.Type == Reverse:
			fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", n(x), n(y), n(dotR), col.fill)
			fmt.Fprintf(b, `  <path d="M%s,%s L%s,%s M%s,%s L%s,%s" stroke="%s" stroke-width="3" stroke-linecap="round"/>`+"\n",
				n(x-5), n(y-5), n(x+5), n(y+5), n(x-5), n(y+5), n(x+5), n(y-5), inner)
		case c.Type == CherryPick:
			fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", n(x), n(y), n(dotR), col.fill)
			fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="2.75" fill="#ffffff"/><circle cx="%s" cy="%s" r="2.75" fill="#ffffff"/>`+"\n",
				n(x-3), n(y+2), n(x+3), n(y+2))
			fmt.Fprintf(b, `  <path d="M%s,%s L%s,%s M%s,%s L%s,%s" stroke="#ffffff" stroke-width="1.5" fill="none"/>`+"\n",
				n(x+3), n(y+1), n(x), n(y-5), n(x-3), n(y+1), n(x), n(y-5))
		case c.Type == MergeCommit:
			fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", n(x), n(y), n(dotR), col.fill)
			fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="6" fill="%s"/>`+"\n", n(x), n(y), inner)
		default:
			fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", n(x), n(y), n(dotR), col.fill)
		}
	}
}

// labels writes commit ids (rotated below the commit, or beside it in a
// vertical graph) and tags (above, or before it).
func (r *renderer) labels(b *strings.Builder) {
	n := svgutil.Num
	text := svgutil.Esc(r.pal.Text)
	for _, c := range r.d.Commits {
		x, y := r.xy(r.pos[c])
		if r.showID(c) {
			tw := r.face.Width(c.ID, r.sfs)
			if !r.vertical {
				ax, ay := x-6, y+dotR+5
				rot := fmt.Sprintf(` transform="rotate(-45 %s %s)"`, n(ax), n(ay))
				fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" fill="#ffffde" fill-opacity="0.6"%s/>`+"\n",
					n(ax-tw-3), n(ay-r.sfs*0.85), n(tw+6), n(r.sfs*1.25), rot)
				fmt.Fprintf(b, `  <text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="end"%s>%s</text>`+"\n",
					n(ax), n(ay+r.sfs*0.1), text, n(r.sfs), rot, svgutil.Esc(c.ID))
			} else {
				ax := x - dotR - 8
				fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" fill="#ffffde" fill-opacity="0.6"/>`+"\n",
					n(ax-tw-3), n(y-r.sfs*0.75), n(tw+6), n(r.sfs*1.4))
				fmt.Fprintf(b, `  <text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="end">%s</text>`+"\n",
					n(ax), n(y+r.sfs*0.35), text, n(r.sfs), svgutil.Esc(c.ID))
			}
		}
		off := 0.0
		if r.vertical && r.showID(c) {
			off = r.face.Width(c.ID, r.sfs) + 14
		}
		for i, tag := range c.Tags {
			r.tag(b, tag, x-off, y, i)
		}
	}
}

// tag draws a luggage-tag label pointing at the commit at (x, y); the i-th
// tag of a commit stacks further out.
func (r *renderer) tag(b *strings.Builder, tag string, x, y float64, i int) {
	n := svgutil.Num
	tw := r.face.Width(tag, r.sfs)
	th := r.sfs + 5
	fill := svgutil.Esc(r.pal.NodeFill)
	stroke := mix(r.pal.NodeStroke, r.pal.NodeFill, 0.55, svgutil.Esc(r.pal.NodeStroke))
	text := svgutil.Esc(r.pal.Text)
	if !r.vertical {
		cy := y - dotR - 8 - th/2 - float64(i)*(th+4)
		l, rr := x-tw/2-10, x+tw/2+6
		fmt.Fprintf(b, `  <polygon points="%s,%s %s,%s %s,%s %s,%s %s,%s %s,%s" fill="%s" stroke="%s"/>`+"\n",
			n(l), n(cy+2), n(l), n(cy-2), n(l+7), n(cy-th/2), n(rr), n(cy-th/2), n(rr), n(cy+th/2), n(l+7), n(cy+th/2), fill, stroke)
		fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="1.5" fill="%s"/>`+"\n", n(l+4), n(cy), text)
		fmt.Fprintf(b, `  <text x="%s" y="%s" fill="%s" font-size="%s">%s</text>`+"\n", n(l+10), n(cy+r.sfs*0.35), text, n(r.sfs), svgutil.Esc(tag))
		return
	}
	cy := y - float64(i)*(th+4)
	rr := x - dotR - 6
	l := rr - tw - 16
	fmt.Fprintf(b, `  <polygon points="%s,%s %s,%s %s,%s %s,%s %s,%s %s,%s" fill="%s" stroke="%s"/>`+"\n",
		n(rr), n(cy+2), n(rr), n(cy-2), n(rr-7), n(cy-th/2), n(l), n(cy-th/2), n(l), n(cy+th/2), n(rr-7), n(cy+th/2), fill, stroke)
	fmt.Fprintf(b, `  <circle cx="%s" cy="%s" r="1.5" fill="%s"/>`+"\n", n(rr-4), n(cy), text)
	fmt.Fprintf(b, `  <text x="%s" y="%s" fill="%s" font-size="%s">%s</text>`+"\n", n(l+5), n(cy+r.sfs*0.35), text, n(r.sfs), svgutil.Esc(tag))
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

var plainFont = regexp.MustCompile(`^[A-Za-z0-9 ,'"_-]{1,200}$`)

// mix blends hex colour a toward hex colour b by t (0 keeps a). When either
// is not a #rrggbb colour it returns fallback.
func mix(a, b string, t float64, fallback string) string {
	var ra, ga, ba, rb, gb, bb int
	if _, err := fmt.Sscanf(strings.ToLower(a), "#%02x%02x%02x", &ra, &ga, &ba); err != nil || len(a) != 7 {
		return fallback
	}
	if _, err := fmt.Sscanf(strings.ToLower(b), "#%02x%02x%02x", &rb, &gb, &bb); err != nil || len(b) != 7 {
		return fallback
	}
	c := func(x, y int) int { return x + int(float64(y-x)*t+0.5) }
	return fmt.Sprintf("#%02x%02x%02x", c(ra, rb), c(ga, gb), c(ba, bb))
}
