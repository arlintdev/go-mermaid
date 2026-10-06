// Package mindmap parses and renders Mermaid mindmaps to SVG using an
// indentation-based hierarchy drawn as a left-to-right tree.
//
// Syntax:
//
//	mindmap
//	  root((Root))
//	    Origins
//	      Long history
//	    Tools
package mindmap

import (
	"strings"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Shape is a mindmap node's outline.
type Shape int

// The mindmap node shapes, by their delimiters.
const (
	ShapeDefault Shape = iota // Text
	ShapeSquare               // [Text]
	ShapeRounded              // (Text)
	ShapeCircle               // ((Text))
	ShapeBang                 // ))Text((
	ShapeCloud                // )Text(
	ShapeHexagon              // {{Text}}
)

// Node is a mindmap node with children.
type Node struct {
	Text     string
	Shape    Shape
	Depth    int
	Children []*Node

	// Layout, filled in by the renderer: centre, size, wrapped lines, the
	// branch (child of the root) it belongs to and its side of the root.
	X, Y, W, H float64
	lines      []string
	section    int
	side       float64
	span       float64
}

// Diagram is a parsed mindmap.
type Diagram struct {
	Root *Node
}

// Parse builds a Diagram from mindmap source using leading-space indentation.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{}
	type frame struct {
		node   *Node
		indent int
	}
	var stack []frame

	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		if strings.TrimSpace(syntax.StripComment(raw)) == "" {
			continue
		}
		if !headerSeen {
			if strings.TrimSpace(raw) != "mindmap" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'mindmap' header")
			}
			headerSeen = true
			continue
		}
		indent := leadingSpaces(raw)
		trimmed := strings.TrimSpace(syntax.StripComment(raw))
		// ::icon(...) and :::className decorate the node above them. Without
		// this they became child nodes labelled with the decoration text.
		if strings.HasPrefix(trimmed, "::") {
			continue
		}
		text, shape := parseNode(trimmed)
		n := &Node{Text: text, Shape: shape}

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			if d.Root == nil {
				d.Root = n
			} else {
				// Additional roots attach under the first root.
				d.Root.Children = append(d.Root.Children, n)
			}
		} else {
			parent := stack[len(stack)-1].node
			n.Depth = parent.Depth + 1
			parent.Children = append(parent.Children, n)
		}
		stack = append(stack, frame{node: n, indent: indent})
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'mindmap' header")
	}
	if d.Root == nil {
		return nil, syntax.Errorf(1, 1, "mindmap has no root node")
	}
	return d, nil
}

// shapes lists each shape's delimiters, longest first so "((" wins over "(".
var shapes = []struct {
	open, close string
	shape       Shape
}{
	{"((", "))", ShapeCircle}, {"))", "((", ShapeBang}, {"{{", "}}", ShapeHexagon},
	{"[", "]", ShapeSquare}, {"(", ")", ShapeRounded}, {")", "(", ShapeCloud},
}

// parseNode reads a node line: an optional id, then the text inside a
// shape's delimiters, or plain text. A trailing ":::class" is dropped:
// classes carry styles a static picture cannot apply.
func parseNode(s string) (string, Shape) {
	if i := strings.Index(s, ":::"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	for _, sh := range shapes {
		if !strings.HasSuffix(s, sh.close) {
			continue
		}
		i := strings.Index(s, sh.open)
		if i >= 0 && i+len(sh.open) <= len(s)-len(sh.close) {
			return unquote(s[i+len(sh.open) : len(s)-len(sh.close)]), sh.shape
		}
	}
	return unquote(s), ShapeDefault
}

// unquote strips "..." and "`...`" (Mermaid's markdown string) wrappers.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 2
		default:
			return n
		}
	}
	return n
}
