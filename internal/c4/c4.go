// Package c4 parses and renders Mermaid C4 diagrams (C4Context,
// C4Container, C4Component, C4Dynamic, C4Deployment) to SVG. Each boundary
// is laid out on its own with the shared layered layout and placed in its
// parent as one box, and relationships are drawn between the elements.
//
// Syntax:
//
//	C4Context
//	    title System Context
//	    Person(custA, "Customer", "A bank customer")
//	    Enterprise_Boundary(b0, "Bank") {
//	        System(sysA, "Internet Banking", "Lets customers …")
//	        SystemDb(db, "Mainframe", "Stores …")
//	    }
//	    Rel(custA, sysA, "Uses", "HTTPS")
//	    UpdateElementStyle(custA, $bgColor="grey")
package c4

import (
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Element is a C4 element (Person, System, Container, Component, …).
type Element struct {
	ID    string
	Kind  string
	Label string
	Techn string
	Descr string
	Style Style
}

// Boundary groups elements and nested boundaries: Enterprise_Boundary,
// System_Boundary, Container_Boundary, Boundary and the deployment nodes.
type Boundary struct {
	ID       string
	Kind     string
	Label    string
	Type     string // the [type] shown under the label
	Descr    string
	Children []any // *Element or *Boundary, in source order
	parent   *Boundary
}

// Rel is a relationship between two elements.
type Rel struct {
	From  string
	To    string
	Label string

	// Tech is the optional technology written as the fourth argument, such
	// as the protocol in Rel(a, b, "reads", "HTTPS").
	Tech  string
	Kind  string // Rel, BiRel, Rel_Back, Rel_U, Rel_D, Rel_L, Rel_R …
	Style Style
}

// Style holds validated colours from UpdateElementStyle / UpdateRelStyle.
type Style struct {
	Fill, Text, Stroke string
}

// Diagram is a parsed C4 diagram.
type Diagram struct {
	Title      string
	Root       *Boundary
	Elements   []*Element
	Boundaries []*Boundary
	Rels       []*Rel
}

func (d *Diagram) element(id string) *Element {
	for _, e := range d.Elements {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (d *Diagram) boundary(id string) *Boundary {
	for _, b := range d.Boundaries {
		if b.ID == id {
			return b
		}
	}
	return nil
}

var (
	elementKinds = map[string]bool{}
	boundaryKind = map[string]bool{
		"Boundary": true, "Enterprise_Boundary": true, "System_Boundary": true, "Container_Boundary": true,
		"Deployment_Node": true, "Node": true, "Node_L": true, "Node_R": true,
	}
	relKinds = map[string]bool{
		"Rel": true, "BiRel": true, "Rel_Back": true, "Rel_U": true, "Rel_Up": true, "Rel_D": true, "Rel_Down": true,
		"Rel_L": true, "Rel_Left": true, "Rel_R": true, "Rel_Right": true,
	}
	ignored = map[string]bool{
		"UpdateLayoutConfig": true, "UpdateBoundaryStyle": true, "AddElementTag": true, "AddRelTag": true,
		"SHOW_LEGEND": true, "LAYOUT_TOP_DOWN": true, "LAYOUT_LEFT_RIGHT": true, "LAYOUT_WITH_LEGEND": true,
	}
	callRe = regexp.MustCompile(`^([A-Za-z_]+)\s*\((.*)\)\s*(\{)?$`)
)

func init() {
	for _, base := range []string{"Person", "System", "SystemDb", "SystemQueue", "Container", "ContainerDb", "ContainerQueue",
		"Component", "ComponentDb", "ComponentQueue"} {
		elementKinds[base] = true
		elementKinds[base+"_Ext"] = true
	}
}

const (
	maxItems = 5000
	maxDepth = 64
)

// Parse builds a Diagram from C4 source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{Root: &Boundary{}}
	stack := []*Boundary{d.Root}
	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(syntax.StripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			switch firstWord(line) {
			case "C4Context", "C4Container", "C4Component", "C4Dynamic", "C4Deployment":
			default:
				return nil, syntax.Errorf(lineNo, 1, "expected a C4 header")
			}
			headerSeen = true
			continue
		}
		if len(d.Elements)+len(d.Boundaries) > maxItems {
			return nil, syntax.Errorf(lineNo, 1, "too many elements")
		}
		cur := stack[len(stack)-1]
		switch {
		case line == "}":
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		case line == "{":
			continue
		case firstWord(line) == "title":
			d.Title = strings.TrimSpace(line[len("title"):])
			continue
		case strings.HasPrefix(line, "accTitle") || strings.HasPrefix(line, "accDescr"):
			continue
		}
		m := callRe.FindStringSubmatch(line)
		if m == nil {
			return nil, syntax.Errorf(lineNo, 1, "unrecognized statement %q", clip(line))
		}
		kind, named := m[1], map[string]string{}
		var args []string
		for _, a := range splitArgs(m[2]) {
			if strings.HasPrefix(a, "$") {
				if k, v, ok := strings.Cut(a[1:], "="); ok {
					named[strings.TrimSpace(k)] = unquote(strings.TrimSpace(v))
				}
				continue
			}
			args = append(args, unquote(a))
		}
		arg := func(i int) string {
			if i < len(args) {
				return args[i]
			}
			return ""
		}
		switch {
		case ignored[kind]:
		case kind == "UpdateElementStyle":
			if e := d.element(arg(0)); e != nil {
				e.Style = styleFrom(named, e.Style)
			}
		case kind == "UpdateRelStyle":
			for _, r := range d.Rels {
				if r.From == arg(0) && r.To == arg(1) {
					r.Style = styleFrom(named, r.Style)
				}
			}
		case relKinds[kind]:
			if arg(0) == "" || arg(1) == "" {
				return nil, syntax.Errorf(lineNo, 1, "%s needs two elements", kind)
			}
			d.Rels = append(d.Rels, &Rel{From: arg(0), To: arg(1), Label: arg(2), Tech: arg(3), Kind: kind})
		case boundaryKind[kind]:
			if arg(0) == "" {
				return nil, syntax.Errorf(lineNo, 1, "%s needs an alias", kind)
			}
			if len(stack) >= maxDepth {
				return nil, syntax.Errorf(lineNo, 1, "boundaries nested too deeply")
			}
			b := &Boundary{ID: arg(0), Kind: kind, Label: arg(1), parent: cur}
			switch kind {
			case "Boundary":
				b.Type = arg(2)
			case "Enterprise_Boundary":
				b.Type = "Enterprise"
			case "System_Boundary":
				b.Type = "System"
			case "Container_Boundary":
				b.Type = "Container"
			default:
				b.Type, b.Descr = arg(2), arg(3)
			}
			if named["type"] != "" {
				b.Type = named["type"]
			}
			cur.Children = append(cur.Children, b)
			d.Boundaries = append(d.Boundaries, b)
			if m[3] == "{" {
				stack = append(stack, b)
			}
		case elementKinds[kind]:
			if arg(0) == "" {
				return nil, syntax.Errorf(lineNo, 1, "%s needs an alias", kind)
			}
			e := &Element{ID: arg(0), Kind: kind, Label: arg(1)}
			if strings.HasPrefix(kind, "Container") || strings.HasPrefix(kind, "Component") {
				e.Techn, e.Descr = arg(2), arg(3)
			} else {
				e.Descr = arg(2)
			}
			if named["techn"] != "" {
				e.Techn = named["techn"]
			}
			if named["descr"] != "" {
				e.Descr = named["descr"]
			}
			if e.Label == "" {
				e.Label = e.ID
			}
			cur.Children = append(cur.Children, e)
			d.Elements = append(d.Elements, e)
		default:
			return nil, syntax.Errorf(lineNo, 1, "unknown C4 statement %q", kind)
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected a C4 header")
	}
	return d, nil
}

func styleFrom(named map[string]string, st Style) Style {
	if c, ok := cssval.Color(named["bgColor"]); ok {
		st.Fill = c
	}
	if c, ok := cssval.Color(named["fontColor"]); ok {
		st.Text = c
	}
	if c, ok := cssval.Color(named["textColor"]); ok {
		st.Text = c
	}
	if c, ok := cssval.Color(named["borderColor"]); ok {
		st.Stroke = c
	}
	if c, ok := cssval.Color(named["lineColor"]); ok {
		st.Stroke = c
	}
	return st
}

// splitArgs splits the inside of a call at commas outside quotes.
func splitArgs(inner string) []string {
	var args []string
	var cur strings.Builder
	inQuote := false
	for _, r := range inner {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case r == ',' && !inQuote:
			args = append(args, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" || len(args) > 0 {
		args = append(args, s)
	}
	return args
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
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

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t("); i >= 0 {
		return s[:i]
	}
	return s
}
