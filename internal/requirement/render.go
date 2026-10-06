package requirement

import (
	"fmt"
	"math"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/curve"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/svgid"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// RenderOptions controls requirement diagram appearance.
type RenderOptions struct {
	Theme    string
	FontFace string
	FontSize float64
	Padding  float64
	Title    string
}

const maxTextW = 220.0 // field values wrap near this width

type metrics struct {
	face          svgutil.Face
	fs, lh, padX  float64
	padY, headGap float64
}

// Render parses and renders requirement diagram source to SVG.
func Render(src string, o RenderOptions) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	fs := o.FontSize
	if fs <= 0 {
		fs = 14
	}
	m := metrics{face: svgutil.FaceFor(o.FontFace), fs: fs, lh: fs * 1.4, padX: fs * 0.8, padY: fs * 0.55, headGap: fs * 0.4}

	g := &domain.Graph{Direction: directionOf(d.Direction)}
	for _, n := range d.Nodes {
		w, h := nodeSize(n, m)
		g.Nodes = append(g.Nodes, &domain.Node{ID: n.ID, Label: " ", Shape: domain.ShapeRect, Size: domain.Size{W: w, H: h}})
	}
	for _, r := range d.Rels {
		g.Edges = append(g.Edges, &domain.Edge{From: r.From, To: r.To, Label: "«" + r.Type + "»"})
	}
	res, err := layout.Compute(g, layout.Options{NodeSep: 50, RankSep: 80, FontSize: fs * 0.9, FontFace: o.FontFace})
	if err != nil {
		return nil, err
	}
	return svg(d, g, res, o, m, svgid.Prefix(src)), nil
}

func directionOf(dir string) domain.Direction {
	switch dir {
	case "LR":
		return domain.LeftRight
	case "RL":
		return domain.RightLeft
	case "BT":
		return domain.BottomTop
	default:
		return domain.TopBottom
	}
}

// header is the stereotype and name lines of a node.
func header(n *Node) (stereo, name string) {
	kind := n.Kind
	if kind == "" {
		return "", n.ID
	}
	return "«" + kinds[kind] + "»", n.ID
}

// fields are a node's labelled rows, in Mermaid's order and wording.
func fields(n *Node) [][2]string {
	var out [][2]string
	add := func(label, key string, title bool) {
		if v := n.Fields[key]; v != "" {
			if title {
				v = titleCase(v)
			}
			out = append(out, [2]string{label, v})
		}
	}
	if n.IsElement {
		add("Type", "type", false)
		add("Doc Ref", "docref", false)
		return out
	}
	add("ID", "id", false)
	add("Text", "text", false)
	add("Risk", "risk", true)
	add("Verification", "verifymethod", true)
	return out
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(strings.ToLower(s))
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}

// fieldLines wraps one "Label: value" row into lines.
func fieldLines(f [2]string, m metrics) []string {
	return m.face.WrapHard(f[0]+": "+f[1], m.fs, maxTextW)
}

func nodeSize(n *Node, m metrics) (float64, float64) {
	stereo, name := header(n)
	w := m.face.Width(name, m.fs) * 1.08
	h := m.padY*2 + m.lh
	if stereo != "" {
		w = max(w, m.face.Width(stereo, m.fs*0.9))
		h += m.lh
	}
	rows := 0
	for _, f := range fields(n) {
		for _, l := range fieldLines(f, m) {
			w = max(w, m.face.Width(l, m.fs))
			rows++
		}
	}
	if rows > 0 {
		h += float64(rows)*m.lh + m.padY*2
	}
	return math.Ceil(max(w+2*m.padX, m.fs*8)), math.Ceil(h)
}

func svg(d *Diagram, g *domain.Graph, res *layout.Result, o RenderOptions, m metrics, id string) []byte {
	pal := theme.For(o.Theme)
	pad := o.Padding
	titleH := svgutil.TitleHeight(o.Title, m.fs)
	vert := g.Direction == domain.TopBottom || g.Direction == domain.BottomTop
	lfs := m.fs * 0.9

	type drawn struct {
		r      *Rel
		sh     curve.Shape
		lx, ly float64
	}
	var rels []drawn
	var bd svgutil.Bounds
	bd.AddRect(0, 0, res.Width, res.Height)
	for i, r := range d.Rels {
		e := g.Edges[i]
		if len(e.Points) < 2 {
			continue
		}
		var obs []curve.Box
		for _, n := range g.Nodes {
			if n.ID != r.From && n.ID != r.To {
				obs = append(obs, curve.Box{X: n.Pos.X, Y: n.Pos.Y, W: n.Size.W, H: n.Size.H})
			}
		}
		sh := curve.Edge(e.Points, vert, obs, 0, 0)
		lx, ly := e.LabelPos.X, e.LabelPos.Y-lfs*0.3
		if sh.Curved {
			lx, ly = sh.Mid.X, sh.Mid.Y
		}
		tw := m.face.Width("«"+r.Type+"»", lfs)
		bd.AddRect(lx-tw/2-4, ly-lfs, tw+8, lfs*2)
		for _, p := range e.Points {
			bd.Add(p.X, p.Y)
		}
		rels = append(rels, drawn{r, sh, lx, ly})
	}
	// Nudge labels apart where two would overlap.
	type lb struct{ x0, y0, x1, y1 float64 }
	boxOf := func(de drawn) lb {
		tw := m.face.Width("«"+de.r.Type+"»", lfs)
		return lb{de.lx - tw/2 - 4, de.ly - lfs*0.65 - 2, de.lx + tw/2 + 4, de.ly + lfs*0.65 + 2}
	}
	for i := range rels {
		for pass := 0; pass < 4; pass++ {
			moved := false
			a := boxOf(rels[i])
			for j := 0; j < i; j++ {
				o := boxOf(rels[j])
				if a.x0 < o.x1 && a.x1 > o.x0 && a.y0 < o.y1 && a.y1 > o.y0 {
					if rels[i].ly >= rels[j].ly {
						rels[i].ly += o.y1 - a.y0 + 2
					} else {
						rels[i].ly -= a.y1 - o.y0 + 2
					}
					moved = true
					a = boxOf(rels[i])
				}
			}
			if !moved {
				break
			}
		}
		a := boxOf(rels[i])
		bd.AddRect(a.x0, a.y0, a.x1-a.x0, a.y1-a.y0)
	}
	shiftX, shiftY := bd.Offset()
	cw, ch := bd.Size()
	w := cw + pad*2
	h := ch + titleH + pad*2
	if o.Title != "" {
		w = max(w, m.face.Width(o.Title, m.fs)+2*pad)
	}

	var b strings.Builder
	edge := svgutil.Esc(pal.Edge)
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" font-family="%s" font-size="%s">`+"\n",
		svgutil.Num(w), svgutil.Num(h), svgutil.Num(w), svgutil.Num(h), svgutil.Esc(o.FontFace), svgutil.Num(m.fs))
	fmt.Fprintf(&b, `  <rect width="100%%" height="100%%" fill="%s"/>`+"\n", svgutil.Esc(pal.Background))
	// An open arrowhead for the relationship's target, and a circled plus
	// for the container end of "contains", as Mermaid draws them.
	fmt.Fprintf(&b, `  <defs><marker id="%s-arrow" viewBox="0 0 12 12" refX="11" refY="6" markerWidth="12" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto"><path d="M1,1 L11,6 L1,11" fill="none" stroke="%s" stroke-width="1.3"/></marker>`+
		`<marker id="%s-contains" viewBox="0 0 20 20" refX="1" refY="10" markerWidth="18" markerHeight="18" markerUnits="userSpaceOnUse" orient="auto"><circle cx="10" cy="10" r="8.5" fill="%s" stroke="%s" stroke-width="1.3"/><path d="M10,2 V18 M2,10 H18" stroke="%s" stroke-width="1.3"/></marker></defs>`+"\n",
		id, edge, id, svgutil.Esc(pal.Background), edge, edge)
	if o.Title != "" {
		fmt.Fprintf(&b, `  <text x="%s" y="%s" fill="%s" text-anchor="middle" font-weight="bold">%s</text>`+"\n",
			svgutil.Num(w/2), svgutil.Num(pad+m.fs), svgutil.Esc(pal.Text), svgutil.Esc(o.Title))
	}
	fmt.Fprintf(&b, `  <g transform="translate(%s,%s)">`+"\n", svgutil.Num(pad+shiftX), svgutil.Num(pad+titleH+shiftY))
	for _, de := range rels {
		markers := fmt.Sprintf(` marker-end="url(#%s-arrow)"`, id)
		dash := ` stroke-dasharray="8 5"`
		if de.r.Type == "contains" {
			markers = fmt.Sprintf(` marker-start="url(#%s-contains)"`, id)
			dash = ""
		}
		fmt.Fprintf(&b, `    <path d="%s" fill="none" stroke="%s" stroke-width="1.3"%s%s/>`+"\n", de.sh.D, edge, dash, markers)
	}
	for _, n := range d.Nodes {
		writeNode(&b, d, n, g.NodeByID(n.ID), pal, m)
	}
	for _, de := range rels {
		label := "«" + de.r.Type + "»"
		tw := m.face.Width(label, lfs)
		fmt.Fprintf(&b, `    <rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="#e8e8e8" fill-opacity="0.85"/>`+"\n",
			svgutil.Num(de.lx-tw/2-4), svgutil.Num(de.ly-lfs*0.65-2), svgutil.Num(tw+8), svgutil.Num(lfs*1.3+4))
		fmt.Fprintf(&b, `    <text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			svgutil.Num(de.lx), svgutil.Num(de.ly+lfs*0.35), svgutil.Esc(pal.Text), svgutil.Num(lfs), svgutil.Esc(label))
	}
	b.WriteString("  </g>\n</svg>\n")
	return []byte(b.String())
}

func writeNode(b *strings.Builder, d *Diagram, n *Node, dn *domain.Node, pal theme.Palette, m metrics) {
	if dn == nil {
		return
	}
	var st Style
	for _, c := range n.Classes {
		cd := d.ClassDefs[c]
		for _, f := range [][2]*string{{&st.Fill, &cd.Fill}, {&st.Stroke, &cd.Stroke}, {&st.StrokeWidth, &cd.StrokeWidth}, {&st.Dash, &cd.Dash}, {&st.Color, &cd.Color}} {
			if *f[1] != "" {
				*f[0] = *f[1]
			}
		}
	}
	fill, stroke, text := svgutil.Esc(pal.NodeFill), svgutil.Esc(pal.NodeStroke), svgutil.Esc(pal.Text)
	if st.Fill != "" {
		fill = svgutil.Esc(st.Fill)
	}
	if st.Stroke != "" {
		stroke = svgutil.Esc(st.Stroke)
	}
	if st.Color != "" {
		text = svgutil.Esc(st.Color)
	}
	extra := ""
	if st.StrokeWidth != "" {
		extra += ` stroke-width="` + svgutil.Esc(st.StrokeWidth) + `"`
	}
	if st.Dash != "" {
		extra += ` stroke-dasharray="` + svgutil.Esc(st.Dash) + `"`
	}
	num := svgutil.Num
	x, y, w, h := dn.Pos.X, dn.Pos.Y, dn.Size.W, dn.Size.H
	fmt.Fprintf(b, `    <rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s"%s/>`+"\n", num(x), num(y), num(w), num(h), fill, stroke, extra)
	stereo, name := header(n)
	cy := y + m.padY
	if stereo != "" {
		cy += m.lh
		fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="middle">%s</text>`+"\n",
			num(x+w/2), num(cy-m.lh*0.3), text, num(m.fs*0.9), svgutil.Esc(stereo))
	}
	cy += m.lh
	fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s" text-anchor="middle" font-weight="bold">%s</text>`+"\n",
		num(x+w/2), num(cy-m.lh*0.3), text, svgutil.Esc(name))
	cy += m.padY
	fs := fields(n)
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(b, `    <line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`+"\n", num(x), num(cy), num(x+w), num(cy), stroke)
	cy += m.padY
	for _, f := range fs {
		for _, l := range fieldLines(f, m) {
			cy += m.lh
			fmt.Fprintf(b, `    <text x="%s" y="%s" fill="%s">%s</text>`+"\n", num(x+m.padX), num(cy-m.lh*0.3), text, svgutil.Esc(l))
		}
	}
}
