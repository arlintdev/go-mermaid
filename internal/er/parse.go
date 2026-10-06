// Package er parses and renders Mermaid entity-relationship diagrams
// (erDiagram) to SVG, reusing the shared layered layout engine. Entities
// are tables of their attributes (type, name, keys, comment); relationships
// carry crow's-foot cardinality glyphs at each end and a label.
package er

import (
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Attribute is one row of an entity: "type name PK, FK "comment"".
type Attribute struct {
	Type, Name string
	Keys       []string
	Comment    string
}

// Entity is a table-like box with attribute rows.
type Entity struct {
	Name       string
	Alias      string // display name from NAME[Alias], or empty
	Attributes []Attribute
	Classes    []string
}

// Label is the name drawn in the entity's header.
func (e *Entity) Label() string {
	if e.Alias != "" {
		return e.Alias
	}
	return e.Name
}

// Card is a crow's-foot cardinality kind.
type Card int

const (
	// CardOne is exactly one (||).
	CardOne Card = iota
	// CardZeroOne is zero or one (|o / o|).
	CardZeroOne
	// CardOneMany is one or more (}| / |{).
	CardOneMany
	// CardZeroMany is zero or more (}o / o{).
	CardZeroMany
)

// Relationship connects two entities with cardinality at each end.
type Relationship struct {
	From      string
	To        string
	Label     string
	LeftKind  Card // crow's-foot kind at the From end
	RightKind Card // ... at the To end
	Dashed    bool // non-identifying relationship
}

// Style is a validated look from a style or classDef line; an empty field
// means "not set".
type Style struct {
	Fill, Stroke, StrokeWidth, Dash, Color string
}

// Diagram is a parsed ER diagram.
type Diagram struct {
	Entities      []*Entity
	Relationships []*Relationship
	ClassDefs     map[string]Style
	Direction     string
}

func (d *Diagram) entity(name string) *Entity {
	for _, e := range d.Entities {
		if e.Name == name {
			return e
		}
	}
	return nil
}

func (d *Diagram) ensureEntity(name string) *Entity {
	if e := d.entity(name); e != nil {
		return e
	}
	e := &Entity{Name: name}
	d.Entities = append(d.Entities, e)
	return e
}

const maxEntities = 5000

// Parse builds a Diagram from ER diagram source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{ClassDefs: map[string]Style{}}
	lines := strings.Split(src, "\n")

	headerSeen := false
	var classLines [][2]string
	for i := 0; i < len(lines); i++ {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(lines[i]))
		if line == "" {
			continue
		}
		if !headerSeen {
			if firstWord(line) != "erDiagram" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'erDiagram' header")
			}
			headerSeen = true
			continue
		}
		if len(d.Entities) > maxEntities {
			return nil, syntax.Errorf(lineNo, 1, "too many entities")
		}
		kw := firstWord(line)
		rest := strings.TrimSpace(line[len(kw):])
		switch kw {
		case "direction":
			d.Direction = strings.ToUpper(rest)
			continue
		case "classDef":
			name := firstWord(rest)
			st := parseCSS(strings.TrimSpace(rest[len(name):]))
			for _, n := range strings.Split(name, ",") {
				if n = strings.TrimSpace(n); n != "" {
					d.ClassDefs[n] = st
				}
			}
			continue
		case "class":
			ids := firstWord(rest)
			classLines = append(classLines, [2]string{ids, strings.TrimSpace(rest[len(ids):])})
			continue
		case "style":
			id := firstWord(rest)
			name := "\x00style:" + id
			d.ClassDefs[name] = parseCSS(strings.TrimSpace(rest[len(id):]))
			classLines = append(classLines, [2]string{id, name})
			continue
		}

		e, n, err := d.entityToken(line, lineNo)
		if err != nil {
			return nil, err
		}
		after := strings.TrimSpace(line[n:])
		switch {
		case after == "{":
			i = d.consumeBlock(e, lines, i+1)
		case after == "":
			// A bare entity declaration.
		default:
			if err := d.parseRelation(e, after, lineNo); err != nil {
				return nil, err
			}
		}
	}

	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'erDiagram' header")
	}
	for _, cl := range classLines {
		for _, id := range strings.Split(cl[0], ",") {
			if e := d.entity(strings.TrimSpace(id)); e != nil && cl[1] != "" {
				e.Classes = append(e.Classes, cl[1])
			}
		}
	}
	return d, nil
}

// entityToken reads an entity reference at the start of s: NAME, "Quoted
// name", NAME[Alias] or NAME["Alias"], with an optional :::class. It
// returns the entity and the bytes used.
func (d *Diagram) entityToken(s string, lineNo int) (*Entity, int, error) {
	var name string
	n := 0
	if strings.HasPrefix(s, `"`) {
		end := strings.IndexByte(s[1:], '"')
		if end < 0 {
			return nil, 0, syntax.Errorf(lineNo, 1, "unterminated name")
		}
		name, n = s[1:end+1], end+2
	} else {
		for n < len(s) && isNameByte(s[n]) {
			n++
		}
		name = s[:n]
	}
	if name == "" {
		return nil, 0, syntax.Errorf(lineNo, 1, "expected an entity name at %q", clip(s))
	}
	e := d.ensureEntity(name)
	if strings.HasPrefix(s[n:], "[") {
		end := strings.IndexByte(s[n:], ']')
		if end < 0 {
			return nil, 0, syntax.Errorf(lineNo, 1, "unterminated alias")
		}
		e.Alias = strings.Trim(strings.TrimSpace(s[n+1:n+end]), `"`)
		n += end + 1
	}
	if strings.HasPrefix(s[n:], ":::") {
		m := n + 3
		for m < len(s) && isNameByte(s[m]) {
			m++
		}
		if cls := s[n+3 : m]; cls != "" {
			e.Classes = append(e.Classes, cls)
		}
		n = m
	}
	return e, n, nil
}

func isNameByte(c byte) bool {
	return c == '_' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func (d *Diagram) consumeBlock(e *Entity, lines []string, start int) int {
	for j := start; j < len(lines); j++ {
		line := strings.TrimSpace(stripComment(lines[j]))
		if line == "" {
			continue
		}
		if line == "}" {
			return j
		}
		e.Attributes = append(e.Attributes, parseAttribute(line))
	}
	return len(lines) - 1
}

// parseAttribute reads `type name PK, FK "comment"`.
func parseAttribute(line string) Attribute {
	var a Attribute
	if q := strings.IndexByte(line, '"'); q >= 0 {
		a.Comment = strings.TrimSuffix(line[q+1:], `"`)
		line = strings.TrimSpace(line[:q])
	}
	f := strings.Fields(strings.ReplaceAll(line, ",", " , "))
	if len(f) > 0 {
		a.Type = generics(f[0])
	}
	if len(f) > 1 {
		a.Name = f[1]
	}
	for _, k := range f[min(2, len(f)):] {
		switch u := strings.ToUpper(k); u {
		case "PK", "FK", "UK":
			a.Keys = append(a.Keys, u)
		}
	}
	return a
}

// generics writes ~T~ type parameters in angle brackets.
func generics(s string) string {
	if strings.Count(s, "~") < 2 {
		return s
	}
	var b strings.Builder
	open := true
	for _, r := range s {
		if r != '~' {
			b.WriteRune(r)
			continue
		}
		if open {
			b.WriteByte('<')
		} else {
			b.WriteByte('>')
		}
		open = !open
	}
	return b.String()
}

var (
	leftCards  = map[string]Card{"||": CardOne, "|o": CardZeroOne, "}|": CardOneMany, "}o": CardZeroMany}
	rightCards = map[string]Card{"||": CardOne, "o|": CardZeroOne, "|{": CardOneMany, "o{": CardZeroMany}
	wordCards  = map[string]Card{
		"only one": CardOne, "1": CardOne, "exactly one": CardOne,
		"zero or one": CardZeroOne, "one or zero": CardZeroOne,
		"one or more": CardOneMany, "one or many": CardOneMany, "many(1)": CardOneMany, "1+": CardOneMany,
		"zero or more": CardZeroMany, "zero or many": CardZeroMany, "many(0)": CardZeroMany, "0+": CardZeroMany,
	}
)

// parseRelation reads `||--o{ OTHER : label` (or the word form
// `only one to zero or more OTHER : label`) after the first entity.
func (d *Diagram) parseRelation(from *Entity, s string, lineNo int) error {
	r := &Relationship{From: from.Name}
	if c := labelColon(s); c >= 0 {
		r.Label = strings.TrimSpace(s[c+1:])
		if len(r.Label) >= 2 && r.Label[0] == '"' && r.Label[len(r.Label)-1] == '"' {
			r.Label = r.Label[1 : len(r.Label)-1]
		}
		s = strings.TrimSpace(s[:c])
	}
	var rest string
	if op := firstWord(s); len(op) == 6 && (op[2:4] == "--" || op[2:4] == "..") {
		l, okL := leftCards[op[:2]]
		rc, okR := rightCards[op[4:]]
		if !okL || !okR {
			return syntax.Errorf(lineNo, 1, "unknown cardinality %q", op)
		}
		r.LeftKind, r.RightKind, r.Dashed = l, rc, op[2:4] == ".."
		rest = strings.TrimSpace(s[len(op):])
	} else {
		lower := strings.ToLower(s)
		sep, dashed := " optionally to ", true
		at := strings.Index(lower, sep)
		if at < 0 {
			sep, dashed = " to ", false
			at = strings.Index(lower, sep)
		}
		if at < 0 {
			return syntax.Errorf(lineNo, 1, "invalid relationship %q", clip(s))
		}
		l, ok := wordCards[strings.Join(strings.Fields(lower[:at]), " ")]
		if !ok {
			return syntax.Errorf(lineNo, 1, "unknown cardinality %q", clip(s[:at]))
		}
		tail := strings.TrimSpace(s[at+len(sep):])
		var rc Card
		best, lt := "", strings.ToLower(tail)
		for phrase, k := range wordCards {
			if strings.HasPrefix(lt, phrase+" ") && len(phrase) > len(best) {
				best, rc = phrase, k
			}
		}
		found := best != ""
		rest = strings.TrimSpace(tail[len(best):])
		if !found {
			return syntax.Errorf(lineNo, 1, "unknown cardinality in %q", clip(tail))
		}
		r.LeftKind, r.RightKind, r.Dashed = l, rc, dashed
	}
	to, n, err := d.entityToken(rest, lineNo)
	if err != nil {
		return err
	}
	if strings.TrimSpace(rest[n:]) != "" {
		return syntax.Errorf(lineNo, 1, "unexpected %q", clip(rest[n:]))
	}
	r.To = to.Name
	d.Relationships = append(d.Relationships, r)
	return nil
}

// labelColon is the index of the colon that starts a relationship label,
// skipping colons inside quotes, brackets and :::class; -1 if none.
func labelColon(s string) int {
	quoted, depth := false, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '[':
			depth++
		case c == ']':
			depth--
		case c == ':' && depth <= 0:
			if strings.HasPrefix(s[i:], ":::") {
				i += 2
				continue
			}
			return i
		}
	}
	return -1
}

// parseCSS reads "fill:#f9f,stroke:#333,stroke-width:4px" keeping only
// values that validate.
func parseCSS(s string) Style {
	var st Style
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "fill":
			if c, ok := cssval.Color(v); ok {
				st.Fill = c
			}
		case "stroke":
			if c, ok := cssval.Color(v); ok {
				st.Stroke = c
			}
		case "color":
			if c, ok := cssval.Color(v); ok {
				st.Color = c
			}
		case "stroke-width":
			if w, ok := cssval.Pixels(v, 20); ok {
				st.StrokeWidth = strconv.FormatFloat(w, 'f', -1, 64)
			}
		case "stroke-dasharray":
			if dsh, ok := cssval.Dash(v); ok {
				st.Dash = dsh
			}
		}
	}
	return st
}

func clip(s string) string {
	r := []rune(s)
	if len(r) > 30 {
		return string(r[:30]) + "…"
	}
	return s
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

func stripComment(s string) string {
	if i := strings.Index(s, "%%"); i >= 0 {
		return s[:i]
	}
	return s
}
