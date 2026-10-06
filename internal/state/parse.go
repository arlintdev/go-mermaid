package state

import (
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

const (
	maxStates = 5000
	maxDepth  = 64
)

type scope struct {
	id     string
	region int
}

var classSuffix = regexp.MustCompile(`:::([\w-]+)\s*$`)

// Parse builds a Diagram from state diagram source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{ClassDefs: map[string]Style{}}
	lines := strings.Split(src, "\n")

	headerSeen := false
	// scopes is the stack of enclosing composite states and the region of
	// each that is open. The bottom entry is the diagram itself.
	scopes := []scope{{}}
	var classLines [][2]string
	for i := 0; i < len(lines); i++ {
		lineNo := i + 1
		line := strings.TrimSpace(syntax.StripComment(lines[i]))
		if line == "" {
			continue
		}
		if !headerSeen {
			w := strings.ToLower(syntax.FirstWord(line))
			if w != "statediagram-v2" && w != "statediagram" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'stateDiagram-v2' header")
			}
			headerSeen = true
			continue
		}
		if len(d.States) > maxStates {
			return nil, syntax.Errorf(lineNo, 1, "too many states")
		}

		sc := scopes[len(scopes)-1]
		kw := syntax.FirstWord(line)

		switch {
		case line == "}":
			if len(scopes) > 1 {
				scopes = scopes[:len(scopes)-1]
			}

		case isCompositeOpen(line):
			if len(scopes) >= maxDepth {
				return nil, syntax.Errorf(lineNo, 1, "states nested too deeply")
			}
			id, label := declaredState(strings.TrimSuffix(line, "{"))
			if id == "" {
				return nil, syntax.Errorf(lineNo, 1, "composite state needs a name")
			}
			s := d.place(id, sc)
			s.Label = label
			if d.composite(id) == nil {
				d.Composites = append(d.Composites, &Composite{ID: id, Label: label, Regions: 1})
			}
			scopes = append(scopes, scope{id: id})

		case isDirection(line):
			dir := strings.ToUpper(strings.TrimSpace(line[len("direction"):]))
			if len(scopes) == 1 {
				d.Direction = dir
			}

		case kw == "hide" || kw == "scale" || kw == "click":
			// Display hints with nothing to draw here.

		case kw == "classDef":
			rest := strings.TrimSpace(line[len(kw):])
			name := syntax.FirstWord(rest)
			st := cssval.Parse(strings.TrimSpace(rest[len(name):]))
			for _, n := range strings.Split(name, ",") {
				if n = strings.TrimSpace(n); n != "" {
					d.ClassDefs[n] = st
				}
			}

		case kw == "class":
			rest := strings.TrimSpace(line[len(kw):])
			ids := syntax.FirstWord(rest)
			classLines = append(classLines, [2]string{ids, strings.TrimSpace(rest[len(ids):])})

		case kw == "style":
			// style S fill:… applies like an anonymous class.
			rest := strings.TrimSpace(line[len(kw):])
			id := syntax.FirstWord(rest)
			name := "\x00style:" + id
			d.ClassDefs[name] = cssval.Parse(strings.TrimSpace(rest[len(id):]))
			classLines = append(classLines, [2]string{id, name})

		case isNoteOpen(line):
			note, body, ok := parseNote(line)
			if !ok {
				return nil, syntax.Errorf(lineNo, 1, "invalid note %q", clip(line))
			}
			if body != "" {
				note.Text = body
				d.Notes = append(d.Notes, note)
				break
			}
			// A note with no inline text runs until `end note`.
			var text []string
			for i+1 < len(lines) {
				i++
				n := strings.TrimSpace(syntax.StripComment(lines[i]))
				if strings.EqualFold(n, "end note") {
					break
				}
				text = append(text, n)
			}
			note.Text = strings.Join(text, "\n")
			d.Notes = append(d.Notes, note)

		case line == "--":
			// A concurrency separator: what follows is the composite's next
			// region, with its own [*].
			if len(scopes) > 1 {
				top := &scopes[len(scopes)-1]
				top.region++
				if c := d.composite(top.id); c != nil {
					c.Regions = max(c.Regions, top.region+1)
				}
			}

		// A transition has "-->"; but a description like `S : text --> more`
		// also contains it, so only treat the line as a transition when the
		// arrow comes before any ':' (which would start a description).
		case isTransition(line):
			if err := d.parseTransition(line, sc, lineNo); err != nil {
				return nil, err
			}

		case isStereotype(line):
			id, kind := parseStereotype(line)
			s := d.place(id, sc)
			s.Kind = kind
			s.Label = ""

		case descColon(line) >= 0:
			d.parseDescription(line, sc)

		case strings.HasPrefix(line, "state "):
			id, label := declaredState(line)
			s := d.place(id, sc)
			s.Label = label

		default:
			id, _ := declaredState(line)
			d.place(id, sc)
		}
	}

	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'stateDiagram-v2' header")
	}
	for _, cl := range classLines {
		for _, id := range strings.Split(cl[0], ",") {
			if s := d.state(strings.TrimSpace(id)); s != nil && cl[1] != "" {
				s.Classes = append(s.Classes, cl[1])
			}
		}
	}
	return d, nil
}

// place returns the state id, creating it in scope sc when it is new. A
// state keeps the scope it was first seen in.
func (d *Diagram) place(id string, sc scope) *State {
	id, cls := splitClass(id)
	s := d.state(id)
	if s == nil {
		s = &State{ID: id, Label: id, Parent: sc.id, Region: sc.region}
		d.States = append(d.States, s)
		d.addMember(sc.id, id)
	}
	if cls != "" {
		s.Classes = append(s.Classes, cls)
	}
	return s
}

// splitClass separates a trailing :::class from a state token.
func splitClass(tok string) (id, cls string) {
	if m := classSuffix.FindStringSubmatchIndex(tok); m != nil {
		return strings.TrimSpace(tok[:m[0]]), tok[m[2]:m[3]]
	}
	return strings.TrimSpace(tok), ""
}

// addMember records that id belongs to the composite parent. Top-level states
// have no parent and are not recorded.
func (d *Diagram) addMember(parent, id string) {
	if parent == "" {
		return
	}
	c := d.composite(parent)
	if c == nil {
		return
	}
	for _, m := range c.Members {
		if m == id {
			return
		}
	}
	c.Members = append(c.Members, id)
}

func (d *Diagram) parseTransition(line string, sc scope, lineNo int) error {
	idx := strings.Index(line, "-->")
	from := strings.TrimSpace(line[:idx])
	rest := line[idx+3:]
	label := ""
	if c := descColon(rest); c >= 0 {
		label = strings.TrimSpace(rest[c+1:])
		rest = rest[:c]
	}
	to := strings.TrimSpace(rest)
	if from == "" || to == "" {
		return syntax.Errorf(lineNo, 1, "a transition needs two states")
	}
	fromID := d.resolve(from, sc, false)
	toID := d.resolve(to, sc, true)
	d.Transitions = append(d.Transitions, &Transition{From: fromID, To: toID, Label: label})
	return nil
}

func (d *Diagram) parseDescription(line string, sc scope) {
	c := descColon(line)
	name, desc := line[:c], line[c+1:]
	id, _ := declaredState(strings.TrimSpace(name))
	s := d.place(id, sc)
	desc = strings.TrimSpace(desc)
	if s.Label != s.ID && s.Label != "" {
		s.Label += "\n" + desc
	} else {
		s.Label = desc
	}
}

// descColon is the index of the colon that starts a state description,
// skipping the colons of a :::class; -1 if there is none.
func descColon(line string) int {
	for i := 0; i < len(line); i++ {
		if line[i] != ':' {
			continue
		}
		if strings.HasPrefix(line[i:], ":::") {
			i += 2
			continue
		}
		return i
	}
	return -1
}

// resolve maps a token to a state ID, turning [*] into the start or end
// pseudostate of the enclosing scope depending on whether it is a target.
func (d *Diagram) resolve(token string, sc scope, asTarget bool) string {
	token = strings.TrimSpace(token)
	if token == "[*]" {
		p := d.ensurePseudo(sc.id, sc.region, asTarget)
		// A composite's own entry and exit belong inside its box.
		d.addMember(sc.id, p.ID)
		return p.ID
	}
	id, _ := declaredState(token)
	return d.place(id, sc).ID
}

// declaredState splits a state declaration into its ID and its display label.
// It understands the bare form `S`, the `state S` prefix, and the aliased
// form `state "Some description" as S`, where the ID is the alias and the
// quoted text is the label.
func declaredState(s string) (id, label string) {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimPrefix(s, "state "))
	if i := strings.Index(s, " as "); i >= 0 {
		label = strings.Trim(strings.TrimSpace(s[:i]), `"`)
		id = strings.TrimSpace(s[i+4:])
		return id, label
	}
	s = strings.TrimSpace(s)
	return s, s
}

// isCompositeOpen reports whether the line opens a composite state body.
func isCompositeOpen(line string) bool {
	return strings.HasSuffix(line, "{") && strings.HasPrefix(line, "state ")
}

// isDirection reports whether the line sets the layout direction.
func isDirection(line string) bool {
	return strings.HasPrefix(line, "direction ") || line == "direction"
}

// isNoteOpen reports whether the line starts a note.
func isNoteOpen(line string) bool {
	return strings.HasPrefix(strings.ToLower(line), "note ")
}

// parseNote reads `note right of S : text` or `note left of S`. The returned
// body is empty when the note continues on following lines until `end note`.
func parseNote(line string) (n *Note, body string, ok bool) {
	rest := strings.TrimSpace(line[len("note"):])
	side := SideRight
	switch {
	case strings.HasPrefix(strings.ToLower(rest), "left of "):
		side = SideLeft
		rest = rest[len("left of "):]
	case strings.HasPrefix(strings.ToLower(rest), "right of "):
		rest = rest[len("right of "):]
	default:
		return nil, "", false
	}
	target, text, _ := strings.Cut(rest, ":")
	return &Note{Target: strings.TrimSpace(target), Side: side}, strings.TrimSpace(text), true
}

// isStereotype reports whether the line declares a pseudostate such as
// `state fork_state <<fork>>`.
func isStereotype(line string) bool {
	return strings.HasPrefix(line, "state ") &&
		strings.Contains(line, "<<") && strings.HasSuffix(line, ">>")
}

// parseStereotype reads the ID and kind out of `state ID <<kind>>`.
func parseStereotype(line string) (id string, kind Kind) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "state "))
	i := strings.Index(rest, "<<")
	id = strings.TrimSpace(rest[:i])
	switch strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rest[i+2:]), ">>")) {
	case "fork":
		kind = KindFork
	case "join":
		kind = KindJoin
	case "choice":
		kind = KindChoice
	}
	return id, kind
}

// isTransition reports whether a line is a transition (`A --> B`) rather than a
// description (`A : text`). When both delimiters appear, the one that comes
// first wins, so `S : note about --> arrows` stays a description.
func isTransition(line string) bool {
	a := strings.Index(line, "-->")
	if a < 0 {
		return false
	}
	c := strings.Index(line, ":")
	return c < 0 || a < c || strings.HasPrefix(line[c:], ":::") && a > c
}

func clip(s string) string {
	r := []rune(s)
	if len(r) > 30 {
		return string(r[:30]) + "…"
	}
	return s
}
