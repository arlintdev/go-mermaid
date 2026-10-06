// Package requirement parses and renders Mermaid requirement diagrams to SVG,
// reusing the shared layered layout engine.
//
// Syntax:
//
//	requirementDiagram
//	    direction LR
//	    requirement test_req {
//	        id: 1
//	        text: "the test text"
//	        risk: high
//	        verifymethod: test
//	    }
//	    functionalRequirement "Login" { … }
//	    element test_entity {
//	        type: simulation
//	        docref: reqs/test_entity
//	    }
//	    test_entity - satisfies -> test_req
//	    test_req <- contains - parent_req
//	    classDef hot fill:#f96
//	    class test_req hot
package requirement

import (
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Node is a requirement or an element.
type Node struct {
	ID        string
	Kind      string // requirement type or "element"
	Fields    map[string]string
	IsElement bool
	Defined   bool // declared with a body, not only named by a relationship
	Classes   []string
}

// Rel is a typed relationship (satisfies, traces, derives, …), always
// stored source to target.
type Rel struct {
	From string
	To   string
	Type string
}

// Style is a validated look from a classDef or style line; an empty field
// means "not set".
type Style = cssval.Style

// Diagram is a parsed requirement diagram.
type Diagram struct {
	Nodes     []*Node
	Rels      []*Rel
	Direction string
	ClassDefs map[string]Style
}

func (d *Diagram) node(id string) *Node {
	for _, n := range d.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// kinds are the requirement types Mermaid knows and the stereotype each
// is drawn with.
var kinds = map[string]string{
	"requirement":            "Requirement",
	"functionalrequirement":  "Functional Requirement",
	"interfacerequirement":   "Interface Requirement",
	"performancerequirement": "Performance Requirement",
	"physicalrequirement":    "Physical Requirement",
	"designconstraint":       "Design Constraint",
	"element":                "Element",
}

var (
	blockRe   = regexp.MustCompile(`^(\w+)\s+("[^"]*"|[^\s{"]+)\s*(?::::([\w-]+))?\s*\{$`)
	forwardRe = regexp.MustCompile(`^("[^"]*"|[^\s"]+)\s*-\s*(\w+)\s*->\s*("[^"]*"|[^\s"]+)$`)
	backRe    = regexp.MustCompile(`^("[^"]*"|[^\s"]+)\s*<-\s*(\w+)\s*-\s*("[^"]*"|[^\s"]+)$`)
)

const maxNodes = 5000

// Parse builds a Diagram from requirement diagram source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{ClassDefs: map[string]Style{}}
	lines := strings.Split(src, "\n")

	headerSeen := false
	var classLines [][2]string
	for i := 0; i < len(lines); i++ {
		lineNo := i + 1
		line := strings.TrimSpace(syntax.StripComment(lines[i]))
		if line == "" {
			continue
		}
		if !headerSeen {
			if syntax.FirstWord(line) != "requirementDiagram" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'requirementDiagram' header")
			}
			headerSeen = true
			continue
		}
		if len(d.Nodes) > maxNodes {
			return nil, syntax.Errorf(lineNo, 1, "too many requirements")
		}
		kw := syntax.FirstWord(line)
		rest := strings.TrimSpace(line[len(kw):])
		switch {
		case kw == "direction":
			d.Direction = strings.ToUpper(rest)
		case kw == "classDef":
			name := syntax.FirstWord(rest)
			st := cssval.Parse(strings.TrimSpace(rest[len(name):]))
			for _, n := range strings.Split(name, ",") {
				if n = strings.TrimSpace(n); n != "" {
					d.ClassDefs[n] = st
				}
			}
		case kw == "class":
			ids := syntax.FirstWord(rest)
			classLines = append(classLines, [2]string{ids, strings.TrimSpace(rest[len(ids):])})
		case kw == "style":
			id := syntax.FirstWord(rest)
			name := "\x00style:" + id
			d.ClassDefs[name] = cssval.Parse(strings.TrimSpace(rest[len(id):]))
			classLines = append(classLines, [2]string{id, name})
		case strings.HasSuffix(line, "{"):
			m := blockRe.FindStringSubmatch(line)
			if m == nil {
				return nil, syntax.Errorf(lineNo, 1, "invalid block %q", clip(line))
			}
			kind := strings.ToLower(m[1])
			if _, ok := kinds[kind]; !ok {
				return nil, syntax.Errorf(lineNo, 1, "unknown requirement type %q", m[1])
			}
			n := d.ensure(unquote(m[2]))
			n.Kind, n.IsElement, n.Defined = kind, kind == "element", true
			if m[3] != "" {
				n.Classes = append(n.Classes, m[3])
			}
			i = d.consumeBlock(n, lines, i+1)
		default:
			if m := forwardRe.FindStringSubmatch(line); m != nil {
				d.addRel(unquote(m[1]), unquote(m[3]), m[2])
			} else if m := backRe.FindStringSubmatch(line); m != nil {
				d.addRel(unquote(m[3]), unquote(m[1]), m[2])
			} else {
				return nil, syntax.Errorf(lineNo, 1, "unrecognized statement %q", clip(line))
			}
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'requirementDiagram' header")
	}
	for _, cl := range classLines {
		for _, id := range strings.Split(cl[0], ",") {
			if n := d.node(strings.TrimSpace(id)); n != nil && cl[1] != "" {
				n.Classes = append(n.Classes, cl[1])
			}
		}
	}
	return d, nil
}

func (d *Diagram) consumeBlock(n *Node, lines []string, start int) int {
	for j := start; j < len(lines); j++ {
		line := strings.TrimSpace(syntax.StripComment(lines[j]))
		if line == "" {
			continue
		}
		if line == "}" {
			return j
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			n.Fields[strings.ToLower(strings.TrimSpace(k))] = unquote(strings.TrimSpace(v))
		}
	}
	return len(lines) - 1
}

func (d *Diagram) addRel(from, to, typ string) {
	d.ensure(from)
	d.ensure(to)
	d.Rels = append(d.Rels, &Rel{From: from, To: to, Type: strings.ToLower(typ)})
}

func (d *Diagram) ensure(id string) *Node {
	if n := d.node(id); n != nil {
		return n
	}
	n := &Node{ID: id, Fields: map[string]string{}}
	d.Nodes = append(d.Nodes, n)
	return n
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func clip(s string) string {
	r := []rune(s)
	if len(r) > 30 {
		return string(r[:30]) + "…"
	}
	return s
}
