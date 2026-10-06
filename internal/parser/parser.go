// Package parser turns a token stream into a domain.Graph. It targets the
// flowchart grammar: a header (graph/flowchart + direction) followed by node,
// link and subgraph statements. Styling lines are read by Preprocess.
package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/lexer"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Flowchart parses a whole flowchart source: its styling lines, then its
// statements, and applies the styles to the nodes, subgraphs and links they
// name.
func Flowchart(src string) (*domain.Graph, error) {
	clean, st := preprocess(src)
	toks, err := lexer.Lex(clean)
	if err != nil {
		return nil, err
	}
	p := newParser(toks)
	g, err := p.parse()
	if err != nil {
		return nil, err
	}
	st.apply(g, p.classRefs)
	return g, nil
}

// Parse builds a Graph from tokens produced by the lexer. Inline ":::class"
// references are read but not resolved; Flowchart resolves them.
func Parse(toks []lexer.Token) (*domain.Graph, error) {
	return newParser(toks).parse()
}

type parser struct {
	toks  []lexer.Token
	pos   int
	graph *domain.Graph
	seen  map[string]*domain.Node
	// open holds the subgraphs whose "end" has not been read yet, innermost
	// last, with the ids referenced directly inside each.
	open []*openSubgraph
	// claimed marks the nodes and subgraphs a finished subgraph has taken.
	claimed  map[string]bool
	edgeIDs  map[string]bool
	sgSerial int
	// classRefs are the ":::name" references in source order.
	classRefs []classAssign
}

type openSubgraph struct {
	sg   *domain.Subgraph
	refs []string
}

func newParser(toks []lexer.Token) *parser {
	return &parser{
		toks:    toks,
		seen:    map[string]*domain.Node{},
		claimed: map[string]bool{},
		edgeIDs: map[string]bool{},
	}
}

func (p *parser) parse() (*domain.Graph, error) {
	p.graph = &domain.Graph{Direction: domain.TopBottom}

	p.skipNewlines()
	if err := p.parseHeader(); err != nil {
		return nil, err
	}

	for !p.at(lexer.EOF) {
		p.skipNewlines()
		if p.at(lexer.EOF) {
			break
		}
		t := p.cur()
		switch {
		case t.Kind == lexer.Keyword && t.Val == "subgraph":
			p.parseSubgraph()
			continue
		case t.Kind == lexer.Keyword && t.Val == "end":
			p.next()
			p.closeSubgraph()
			continue
		case t.Kind == lexer.Keyword && t.Val == "direction":
			p.next()
			if d, ok := direction(p.cur().Val); ok && p.at(lexer.Ident) {
				if n := len(p.open); n > 0 {
					p.open[n-1].sg.Direction = d
				} else {
					p.graph.Direction = d
				}
				p.next()
			}
			continue
		case t.Kind == lexer.Ident && p.peekKind(1) == lexer.Meta && p.edgeIDs[t.Val]:
			// e1@{ animate: true } sets how a link moves; a static picture
			// has nothing to do with it.
			p.next()
			p.next()
			continue
		}
		if err := p.parseStatement(); err != nil {
			return nil, err
		}
		if !p.at(lexer.Newline) && !p.at(lexer.EOF) {
			return nil, p.errAt(p.cur(), "unexpected %q", p.cur().Val)
		}
	}
	for len(p.open) > 0 {
		p.closeSubgraph()
	}
	p.dropSubgraphNodes()
	return p.graph, nil
}

// subgraphHeader splits "id[title]", "id [\"title\"]" and a bare title.
var subgraphHeader = regexp.MustCompile(`^([^\s\[\]"]+)\s*\[(.*)\]$`)

// parseSubgraph reads "subgraph [id] [\[title\]]" and opens a subgraph.
func (p *parser) parseSubgraph() {
	p.next() // subgraph
	sg := &domain.Subgraph{}
	if p.at(lexer.Text) {
		text := p.cur().Val
		p.next()
		if m := subgraphHeader.FindStringSubmatch(text); m != nil {
			sg.ID = m[1]
			sg.Title = cleanLabel(unquote(strings.TrimSpace(m[2])))
		} else {
			sg.ID, sg.Title = unquote(text), cleanLabel(unquote(text))
		}
	}
	if sg.ID == "" {
		p.sgSerial++
		sg.ID = "subGraph" + strconv.Itoa(p.sgSerial)
	}
	if n := len(p.open); n > 0 {
		parent := p.open[n-1]
		parent.refs = append(parent.refs, sg.ID)
	}
	p.graph.Subgraphs = append(p.graph.Subgraphs, sg)
	p.open = append(p.open, &openSubgraph{sg: sg})
}

// closeSubgraph finishes the innermost open subgraph. As in mermaid.js, it
// takes every node and subgraph referenced directly inside it that no
// subgraph finished earlier has taken: an inner subgraph ends first, so its
// members stay its own, and a node first written outside still joins the
// subgraph that names it.
func (p *parser) closeSubgraph() {
	n := len(p.open)
	if n == 0 {
		return
	}
	o := p.open[n-1]
	p.open = p.open[:n-1]
	for _, id := range o.refs {
		if p.claimed[id] || id == o.sg.ID {
			continue
		}
		p.claimed[id] = true
		if child := p.graph.SubgraphByID(id); child != nil && child != o.sg {
			child.Parent = o.sg.ID
			continue
		}
		o.sg.NodeIDs = append(o.sg.NodeIDs, id)
	}
}

// dropSubgraphNodes removes nodes that only stand for a subgraph: a link to a
// subgraph's id names the subgraph, not a new node.
func (p *parser) dropSubgraphNodes() {
	if len(p.graph.Subgraphs) == 0 {
		return
	}
	isSG := map[string]bool{}
	for _, sg := range p.graph.Subgraphs {
		isSG[sg.ID] = true
	}
	kept := p.graph.Nodes[:0]
	for _, n := range p.graph.Nodes {
		if isSG[n.ID] && n.Label == n.ID && n.Shape == domain.ShapeRect {
			continue
		}
		kept = append(kept, n)
	}
	p.graph.Nodes = kept
	for _, sg := range p.graph.Subgraphs {
		ids := sg.NodeIDs[:0]
		for _, id := range sg.NodeIDs {
			if isSG[id] && p.graph.NodeByID(id) == nil {
				if child := p.graph.SubgraphByID(id); child != nil && child != sg && child.Parent == "" {
					child.Parent = sg.ID
				}
				continue
			}
			ids = append(ids, id)
		}
		sg.NodeIDs = ids
	}
}

func (p *parser) parseHeader() error {
	t := p.cur()
	if t.Kind != lexer.Keyword || (t.Val != "graph" && t.Val != "flowchart") {
		return p.errAt(t, "expected 'graph' or 'flowchart'")
	}
	p.next()
	if dir := p.cur(); dir.Kind == lexer.Ident {
		d, ok := direction(dir.Val)
		if !ok {
			return p.errAt(dir, "unknown direction %q", dir.Val)
		}
		p.graph.Direction = d
		p.next()
	}
	if !p.at(lexer.Newline) && !p.at(lexer.EOF) {
		return p.errAt(p.cur(), "expected end of header line")
	}
	return nil
}

// parseStatement parses a chain like A[x] --> B -->|label| C.
func (p *parser) parseStatement() error {
	from, err := p.parseNodeList()
	if err != nil {
		return err
	}
	for p.at(lexer.Arrow) || p.at(lexer.EdgeID) {
		if p.at(lexer.EdgeID) {
			id := p.cur().Val
			p.edgeIDs[id] = true
			p.next()
			if !p.at(lexer.Arrow) {
				return p.errAt(p.cur(), "expected a link after %q", id+"@")
			}
		}
		link := p.cur().Link
		p.next()

		label := ""
		if link.HasLabel {
			label = cleanLabel(link.Label)
		}
		if p.at(lexer.Pipe) {
			p.next()
			if !p.at(lexer.Text) {
				return p.errAt(p.cur(), "expected edge label text")
			}
			label = cleanLabel(p.cur().Val)
			p.next()
			if !p.at(lexer.Pipe) {
				return p.errAt(p.cur(), "expected closing '|'")
			}
			p.next()
		}

		to, err := p.parseNodeList()
		if err != nil {
			return err
		}
		// `A & B --> C & D` links every node on the left to every node on
		// the right, so the lists form a small cross product.
		for _, f := range from {
			for _, t := range to {
				p.graph.Edges = append(p.graph.Edges, newEdge(f, t, label, link))
			}
		}
		from = to
	}
	return nil
}

func newEdge(from, to, label string, link *lexer.Link) *domain.Edge {
	e := &domain.Edge{From: from, To: to, Label: label, MinLen: link.Len}
	e.Start, e.End = marker(link.Start), marker(link.End)
	switch link.Line {
	case "dotted":
		e.Line = domain.LineDotted
	case "thick":
		e.Line = domain.LineThick
	case "invisible":
		e.Line = domain.LineInvisible
	}
	switch {
	case e.Line == domain.LineDotted:
		e.Arrow = domain.ArrowDotted
	case e.Line == domain.LineThick:
		e.Arrow = domain.ArrowThick
	case e.End == domain.MarkerNone:
		e.Arrow = domain.ArrowOpen
	default:
		e.Arrow = domain.ArrowNormal
	}
	return e
}

func marker(s string) domain.Marker {
	switch s {
	case "arrow":
		return domain.MarkerArrow
	case "circle":
		return domain.MarkerCircle
	case "cross":
		return domain.MarkerCross
	}
	return domain.MarkerNone
}

// parseNodeList reads one or more node references joined by "&". Mermaid uses
// it to link several nodes at once, as in `A --> B & C`. It returns the ids.
func (p *parser) parseNodeList() ([]string, error) {
	first, err := p.parseNodeRef()
	if err != nil {
		return nil, err
	}
	ids := []string{first}
	for p.at(lexer.Amp) {
		p.next()
		id, err := p.parseNodeRef()
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parseNodeRef parses an identifier with an optional shape and label, or a
// metadata block, and any ":::class" after it. It registers the node, or
// updates one seen before: the last shape written wins, as in mermaid.js.
func (p *parser) parseNodeRef() (string, error) {
	idTok := p.cur()
	if idTok.Kind != lexer.Ident {
		return "", p.errAt(idTok, "expected node identifier")
	}
	p.next()
	id := idTok.Val

	node := p.seen[id]
	if node == nil {
		node = &domain.Node{ID: id, Label: id, Shape: domain.ShapeRect}
		p.seen[id] = node
		p.graph.Nodes = append(p.graph.Nodes, node)
	}
	if n := len(p.open); n > 0 {
		o := p.open[n-1]
		o.refs = append(o.refs, id)
	}

	switch {
	case p.at(lexer.ShapeOpen):
		open := p.cur().Val
		p.next()
		if !p.at(lexer.Text) {
			return "", p.errAt(p.cur(), "expected shape label")
		}
		node.Label = cleanLabel(p.cur().Val)
		p.next()
		if !p.at(lexer.ShapeClose) {
			return "", p.errAt(p.cur(), "expected closing shape delimiter")
		}
		node.Shape = shapeKind(open, p.cur().Val)
		p.next()
	case p.at(lexer.Meta):
		applyMeta(node, p.cur().Val)
		p.next()
	}
	for p.at(lexer.ClassRef) {
		if name := p.cur().Val; name != "" {
			p.classRefs = append(p.classRefs, classAssign{ids: []string{id}, names: []string{name}})
		}
		p.next()
	}
	return id, nil
}

// --- token cursor helpers ---

func (p *parser) cur() lexer.Token     { return p.toks[p.pos] }
func (p *parser) at(k lexer.Kind) bool { return p.toks[p.pos].Kind == k }

func (p *parser) peekKind(n int) lexer.Kind {
	if p.pos+n < len(p.toks) {
		return p.toks[p.pos+n].Kind
	}
	return lexer.EOF
}

func (p *parser) next() {
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
}

func (p *parser) skipNewlines() {
	for p.at(lexer.Newline) {
		p.next()
	}
}

func (p *parser) errAt(t lexer.Token, format string, args ...any) error {
	return syntax.Errorf(t.Line, t.Col, format, args...)
}

// --- mappings ---

func direction(s string) (domain.Direction, bool) {
	switch strings.ToUpper(s) {
	case "TD", "TB":
		return domain.TopBottom, true
	case "BT":
		return domain.BottomTop, true
	case "LR":
		return domain.LeftRight, true
	case "RL":
		return domain.RightLeft, true
	}
	return "", false
}

func shapeKind(open, closer string) domain.Shape {
	switch open {
	case "(":
		return domain.ShapeRound
	case "([":
		return domain.ShapeStadium
	case "((":
		return domain.ShapeCircle
	case "(((":
		return domain.ShapeDoubleCircle
	case "{":
		return domain.ShapeDiamond
	case "{{":
		return domain.ShapeHexagon
	case "[[":
		return domain.ShapeSubroutine
	case "[(":
		return domain.ShapeCylinder
	case ">":
		return domain.ShapeAsymmetric
	case "[/":
		if closer == "\\]" {
			return domain.ShapeTrapezoid
		}
		return domain.ShapeParallelogram
	case "[\\":
		if closer == "/]" {
			return domain.ShapeTrapezoidAlt
		}
		return domain.ShapeParallelogramAlt
	default:
		return domain.ShapeRect
	}
}

// metaShapes maps the Mermaid 11 shape names (@{ shape: ... }) onto the
// shapes this renderer draws. Rarer shapes take the closest one.
var metaShapes = map[string]domain.Shape{}

func init() {
	for shape, names := range map[domain.Shape]string{
		domain.ShapeRect: "rect rectangle proc process notch-rect card notched-rectangle lin-rect lined-rectangle " +
			"lined-process lin-proc shaded-process div-rect div-proc divided-rectangle divided-process " +
			"st-rect procs processes stacked-rectangle win-pane internal-storage window-pane tag-rect " +
			"tag-proc tagged-rectangle tagged-process bolt com-link lightning-bolt hourglass collate " +
			"tri triangle extract fork join",
		domain.ShapeRound:            "rounded round event delay half-rounded-rectangle curv-trap curved-trapezoid display bow-rect stored-data bow-tie-rectangle",
		domain.ShapeStadium:          "stadium pill terminal",
		domain.ShapeCircle:           "circle circ cross-circ summary crossed-circle",
		domain.ShapeDoubleCircle:     "dbl-circ double-circle",
		domain.ShapeDiamond:          "diamond diam decision question",
		domain.ShapeHexagon:          "hex hexagon prepare",
		domain.ShapeCylinder:         "cyl cylinder database db h-cyl das horizontal-cylinder lin-cyl disk lined-cylinder",
		domain.ShapeSubroutine:       "subproc subprocess subroutine fr-rect framed-rectangle",
		domain.ShapeParallelogram:    "lean-r lean-right in-out",
		domain.ShapeParallelogramAlt: "lean-l lean-left out-in sl-rect manual-input sloped-rectangle",
		domain.ShapeTrapezoid:        "trap-b trapezoid-bottom priority trapezoid",
		domain.ShapeTrapezoidAlt:     "trap-t trapezoid-top manual inv-trapezoid",
		domain.ShapeAsymmetric:       "odd flag paper-tape wave-rect",
		domain.ShapeDocument:         "doc document docs documents st-doc stacked-document tag-doc tagged-document lin-doc lined-document",
		domain.ShapeSmallCircle:      "sm-circ small-circle start f-circ filled-circle junction",
		domain.ShapeFramedCircle:     "fr-circ framed-circle stop",
		domain.ShapeText:             "text brace comment brace-l brace-r braces",
	} {
		for _, n := range strings.Fields(names) {
			metaShapes[n] = shape
		}
	}
}

// applyMeta reads `shape: name, label: "text"` from an @{ } block.
func applyMeta(n *domain.Node, body string) {
	for _, part := range splitMeta(body) {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "shape":
			if s, ok := metaShapes[strings.ToLower(unquote(v))]; ok {
				n.Shape = s
			} else {
				n.Shape = domain.ShapeRect
			}
		case "label":
			n.Label = cleanLabel(unquote(v))
		}
	}
}

// splitMeta splits metadata on commas and newlines that sit outside quotes,
// so a label may contain a comma.
func splitMeta(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote != 0 && c == inQuote:
			inQuote = 0
			cur.WriteByte(c)
		case inQuote == 0 && (c == '"' || c == '\''):
			inQuote = c
			cur.WriteByte(c)
		case inQuote == 0 && (c == ',' || c == '\n'):
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}
