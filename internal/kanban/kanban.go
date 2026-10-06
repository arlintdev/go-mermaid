// Package kanban parses and renders Mermaid kanban diagrams to SVG as columns
// of cards, derived from indentation.
//
// Syntax (subset):
//
//	kanban
//	  Todo
//	    [Task 1]
//	    [Task 2]
//	  Done
//	    [Task 3]
package kanban

import (
	"strings"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Card is a single item within a column, with the metadata Mermaid draws
// on it: a ticket reference, the person assigned and a priority.
type Card struct {
	Text     string
	Ticket   string
	Assigned string
	Priority string // "Very High", "High", "Low" or "Very Low"; other values are dropped
}

// Column is a labeled list of cards.
type Column struct {
	Title string
	Cards []*Card
}

// Diagram is a parsed kanban board.
type Diagram struct {
	Columns []*Column
}

// Parse builds a Diagram from kanban source using indentation: the shallowest
// item lines are column titles, deeper lines are cards.
func Parse(src string) (*Diagram, error) {
	type item struct {
		indent int
		text   string
	}
	var items []item

	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		if strings.TrimSpace(syntax.StripComment(raw)) == "" {
			continue
		}
		if !headerSeen {
			if strings.TrimSpace(raw) != "kanban" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'kanban' header")
			}
			headerSeen = true
			continue
		}
		items = append(items, item{indent: leadingSpaces(raw), text: strings.TrimSpace(syntax.StripComment(raw))})
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'kanban' header")
	}

	minIndent := -1
	for _, it := range items {
		if minIndent < 0 || it.indent < minIndent {
			minIndent = it.indent
		}
	}

	d := &Diagram{}
	var cur *Column
	for _, it := range items {
		if it.indent == minIndent {
			title, _ := splitMeta(it.text)
			cur = &Column{Title: cardText(title)}
			d.Columns = append(d.Columns, cur)
			continue
		}
		if cur == nil {
			cur = &Column{}
			d.Columns = append(d.Columns, cur)
		}
		text, meta := splitMeta(it.text)
		cur.Cards = append(cur.Cards, newCard(cardText(text), meta))
	}
	return d, nil
}

// cardText strips an "id[text]" wrapper down to its text.
func cardText(s string) string {
	if o := strings.IndexByte(s, '['); o >= 0 && strings.HasSuffix(s, "]") {
		s = strings.TrimSuffix(s[o+1:], "]")
	}
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

// splitMeta separates "text@{ key: value, ... }" into the text and its
// metadata.
func splitMeta(s string) (string, map[string]string) {
	i := strings.Index(s, "@{")
	if i < 0 {
		return s, nil
	}
	body := strings.TrimSuffix(strings.TrimSpace(s[i+2:]), "}")
	meta := map[string]string{}
	for _, kv := range strings.Split(body, ",") {
		k, v, ok := strings.Cut(kv, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '\'' || v[0] == '"') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		meta[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return strings.TrimSpace(s[:i]), meta
}

func newCard(text string, meta map[string]string) *Card {
	c := &Card{Text: text, Ticket: meta["ticket"], Assigned: meta["assigned"]}
	for _, p := range []string{"Very High", "High", "Low", "Very Low"} {
		if strings.EqualFold(strings.TrimSpace(meta["priority"]), p) {
			c.Priority = p
		}
	}
	return c
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
