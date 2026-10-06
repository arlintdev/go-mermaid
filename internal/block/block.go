// Package block parses and renders Mermaid block diagrams (block-beta) to
// SVG: blocks flow left to right into a column grid, composite blocks hold
// their own grid, and edges are drawn as arrows between blocks.
//
// Syntax:
//
//	block-beta
//	    columns 3
//	    a["Frontend"]:3
//	    b c(("Workers")) d[("DB")]
//	    block:group:2
//	      columns 2
//	      e f
//	    end
//	    arrow<["send"]>(right)
//	    b --> d
//	    c -- "reads" --> d
//	    style b fill:#f9f,stroke:#333,stroke-width:2px
//	    classDef hot fill:#f96
//	    class c hot
package block

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Shape is the outline of a block.
type Shape int

// The block shapes Mermaid knows, written id["…"], id("…"), id(["…"]) and
// so on.
const (
	ShapeRect Shape = iota
	ShapeRound
	ShapeStadium
	ShapeSubroutine
	ShapeCylinder
	ShapeCircle
	ShapeDoubleCircle
	ShapeAsymmetric
	ShapeRhombus
	ShapeHexagon
	ShapeParallelogram
	ShapeParallelogramAlt
	ShapeTrapezoid
	ShapeTrapezoidAlt
	ShapeArrow // a block arrow, id<["…"]>(right)
)

// Style is the validated look given by style, classDef and class lines.
// An empty field means "not set".
type Style = cssval.Style

// Block is one cell of a grid: a drawn block, a gap, or a composite block
// holding a grid of its own.
type Block struct {
	ID    string
	Label string
	Shape Shape
	Span  int

	// Space marks a gap in the grid (`space` or `space:2`).
	Space bool

	// Composite blocks (block … end) hold Children laid out in Columns
	// columns; Columns 0 means all on one row.
	Composite bool
	Columns   int
	Children  []*Block

	// ArrowDir is the direction of a block arrow: right, left, up, down,
	// x (both ways across) or y (both ways up and down).
	ArrowDir string

	Style   Style
	Classes []string
}

// Edge is a link drawn between two blocks.
type Edge struct {
	From, To  string
	Label     string
	ArrowEnd  bool
	ArrowBack bool
	Thick     bool
	Dotted    bool
	Invisible bool
}

// Diagram is a parsed block diagram. Root is the top-level composite.
type Diagram struct {
	Root    *Block
	Edges   []Edge
	Classes map[string]Style
	byID    map[string]*Block
}

// Columns is the column count of the top-level grid (0: one row).
func (d *Diagram) Columns() int { return d.Root.Columns }

// Block returns the block with the given id, or nil.
func (d *Diagram) Block(id string) *Block { return d.byID[id] }

const maxDepth = 64

// Parse builds a Diagram from block-beta source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{Root: &Block{Composite: true, Span: 1}, Classes: map[string]Style{}, byID: map[string]*Block{}}
	stack := []*Block{d.Root}
	headerSeen := false
	var classLines, styleLines [][2]string
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(syntax.StripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			if w := strings.ToLower(syntax.FirstWord(line)); w != "block-beta" && w != "block" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'block-beta' header")
			}
			headerSeen = true
			continue
		}
		cur := stack[len(stack)-1]
		key := syntax.FirstWord(line)
		rest := strings.TrimSpace(line[len(key):])
		switch {
		case key == "columns":
			cur.Columns = 0
			if n, err := strconv.Atoi(rest); err == nil && n > 0 {
				cur.Columns = min(n, 1000)
			}
			continue
		case key == "end":
			if len(stack) == 1 {
				return nil, syntax.Errorf(lineNo, 1, "'end' without an open block")
			}
			stack = stack[:len(stack)-1]
			continue
		case key == "block" || strings.HasPrefix(key, "block:"):
			if len(stack) >= maxDepth {
				return nil, syntax.Errorf(lineNo, 1, "blocks nested too deeply")
			}
			c := &Block{Composite: true, Span: 1}
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				c.ID = idRe.FindString(strings.TrimSpace(parts[1]))
			}
			if len(parts) > 2 {
				if n, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil && n > 0 {
					c.Span = min(n, 1000)
				}
			}
			cur.Children = append(cur.Children, c)
			if c.ID != "" {
				d.byID[c.ID] = c
			}
			stack = append(stack, c)
			continue
		case key == "style":
			id := syntax.FirstWord(rest)
			styleLines = append(styleLines, [2]string{id, strings.TrimSpace(rest[len(id):])})
			continue
		case key == "classDef":
			name := syntax.FirstWord(rest)
			css := cssval.Parse(strings.TrimSpace(rest[len(name):]))
			for _, n := range strings.Split(name, ",") {
				if n = strings.TrimSpace(n); n != "" {
					d.Classes[n] = css
				}
			}
			continue
		case key == "class":
			ids := syntax.FirstWord(rest)
			classLines = append(classLines, [2]string{ids, strings.TrimSpace(rest[len(ids):])})
			continue
		}
		if err := d.parseStatements(cur, line, lineNo); err != nil {
			return nil, err
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'block-beta' header")
	}
	for _, s := range styleLines {
		if b := d.byID[s[0]]; b != nil {
			b.Style = cssval.Parse(s[1]).Over(b.Style)
		}
	}
	for _, c := range classLines {
		for _, id := range strings.Split(c[0], ",") {
			if b := d.byID[strings.TrimSpace(id)]; b != nil && c[1] != "" {
				b.Classes = append(b.Classes, c[1])
			}
		}
	}
	return d, nil
}

var (
	idRe        = regexp.MustCompile(`^[\p{L}\p{N}_]+(?:[-.][\p{L}\p{N}_]+)*`)
	labelEdgeRe = regexp.MustCompile(`^(<?)(--|==|-\.)\s*(?:"([^"]*)"|([^"<>=.\-\s][^<>=]*?))\s*(-->|---|==>|===|\.->|\.-|--[xo]|--)`)
	plainEdgeRe = regexp.MustCompile(`^(<?)(-{2,}>|-{3,}|={2,}>|={3,}|-\.+->|-\.+-|~{3,}|--[xo])(?:\|([^|]*)\|)?`)
	spanRe      = regexp.MustCompile(`^:(\d+)`)
	arrowDirRe  = regexp.MustCompile(`^\(\s*(right|left|up|down|x|y)\s*\)`)
)

// shapes maps opening delimiters to their closer and shape, longest first.
var shapes = []struct {
	open, close string
	shape       Shape
}{
	{"(((", ")))", ShapeDoubleCircle},
	{"((", "))", ShapeCircle},
	{"([", "])", ShapeStadium},
	{"[[", "]]", ShapeSubroutine},
	{"[(", ")]", ShapeCylinder},
	{"[/", "/]", ShapeParallelogram},
	{"[/", `\]`, ShapeTrapezoid},
	{`[\`, `\]`, ShapeParallelogramAlt},
	{`[\`, "/]", ShapeTrapezoidAlt},
	{"{{", "}}", ShapeHexagon},
	{"<[", "]>", ShapeArrow},
	{"(", ")", ShapeRound},
	{"[", "]", ShapeRect},
	{"{", "}", ShapeRhombus},
	{">", "]", ShapeAsymmetric},
}

// parseStatements reads one line of blocks and edges into cur.
func (d *Diagram) parseStatements(cur *Block, line string, lineNo int) error {
	s := line
	var last *Block
	var pending *Edge
	for {
		s = strings.TrimLeft(s, " \t;")
		if s == "" {
			break
		}
		if last != nil && pending == nil {
			if m := labelEdgeRe.FindStringSubmatch(s); m != nil {
				label := m[3]
				if label == "" {
					label = strings.TrimSpace(m[4])
				}
				pending = edgeFrom(last.ID, m[1], m[2]+m[5], label)
				s = s[len(m[0]):]
				continue
			}
			if m := plainEdgeRe.FindStringSubmatch(s); m != nil {
				pending = edgeFrom(last.ID, m[1], m[2], strings.TrimSpace(strings.Trim(m[3], `"`)))
				s = s[len(m[0]):]
				continue
			}
		}
		b, n, err := parseBlock(s, lineNo)
		if err != nil {
			return err
		}
		s = s[n:]
		if pending != nil {
			if b.Space {
				return syntax.Errorf(lineNo, 1, "an edge cannot end at a space")
			}
			pending.To = b.ID
			d.Edges = append(d.Edges, *pending)
			pending = nil
			last = d.place(cur, b, true)
			continue
		}
		last = d.place(cur, b, false)
		if last.Space {
			last = nil
		}
	}
	if pending != nil {
		return syntax.Errorf(lineNo, 1, "edge has no target block")
	}
	return nil
}

// place adds b to cur and returns the block its id names. A block already
// defined is not placed again when an edge mentions it; an edge to an id
// never seen before defines that block, as Mermaid does.
func (d *Diagram) place(cur *Block, b *Block, inEdge bool) *Block {
	if b.Space {
		cur.Children = append(cur.Children, b)
		return b
	}
	old := d.byID[b.ID]
	if old == nil {
		d.byID[b.ID] = b
		cur.Children = append(cur.Children, b)
		return b
	}
	if b.Label != b.ID || b.Shape != ShapeRect {
		old.Label, old.Shape, old.ArrowDir = b.Label, b.Shape, b.ArrowDir
	}
	return old
}

func edgeFrom(from, back, op, label string) *Edge {
	e := &Edge{From: from, Label: label, ArrowBack: back == "<"}
	switch {
	case strings.HasPrefix(op, "~"):
		e.Invisible = true
	case strings.HasPrefix(op, "="):
		e.Thick = true
	case strings.Contains(op, "."):
		e.Dotted = true
	}
	e.ArrowEnd = strings.HasSuffix(op, ">") || strings.HasSuffix(op, "x") || strings.HasSuffix(op, "o")
	return e
}

// parseBlock reads one block token at the start of s and returns it with
// the number of bytes it used.
func parseBlock(s string, lineNo int) (*Block, int, error) {
	if strings.HasPrefix(s, "space") && (len(s) == 5 || strings.ContainsRune(" \t:;", rune(s[5]))) {
		b := &Block{Space: true, Span: 1}
		n := 5
		if m := spanRe.FindStringSubmatch(s[n:]); m != nil {
			v, _ := strconv.Atoi(m[1])
			b.Span = max(1, min(v, 1000))
			n += len(m[0])
		}
		return b, n, nil
	}
	id := idRe.FindString(s)
	if id == "" {
		return nil, 0, syntax.Errorf(lineNo, 1, "expected a block id at %q", clip(s))
	}
	b := &Block{ID: id, Label: id, Span: 1}
	n := len(id)
	rest := s[n:]
	for _, sh := range shapes {
		if !strings.HasPrefix(rest, sh.open) {
			continue
		}
		body := rest[len(sh.open):]
		var label string
		var used int
		if strings.HasPrefix(body, `"`) {
			end := strings.IndexByte(body[1:], '"')
			if end < 0 {
				return nil, 0, syntax.Errorf(lineNo, 1, "unterminated label")
			}
			label = body[1 : end+1]
			used = end + 2
			if !strings.HasPrefix(body[used:], sh.close) {
				continue
			}
		} else {
			end := strings.Index(body, sh.close)
			if end < 0 {
				continue
			}
			label = strings.TrimSpace(body[:end])
			used = end
		}
		b.Label, b.Shape = label, sh.shape
		n += len(sh.open) + used + len(sh.close)
		if sh.shape == ShapeArrow {
			b.ArrowDir = "right"
			if m := arrowDirRe.FindStringSubmatch(s[n:]); m != nil {
				b.ArrowDir = m[1]
				n += len(m[0])
			}
		}
		break
	}
	if m := spanRe.FindStringSubmatch(s[n:]); m != nil {
		v, _ := strconv.Atoi(m[1])
		b.Span = max(1, min(v, 1000))
		n += len(m[0])
	}
	return b, n, nil
}

func clip(s string) string {
	r := []rune(s)
	if len(r) > 20 {
		return string(r[:20]) + "…"
	}
	return s
}
