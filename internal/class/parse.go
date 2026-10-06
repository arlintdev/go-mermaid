package class

import (
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

const maxClasses = 5000

var (
	name      = "(`[^`]+`|[\\p{L}\\p{N}_~.,\\-]+?)"
	relRe     = regexp.MustCompile(`^` + name + `\s*(?:"([^"]*)")?\s*(<\||\*|o|<|\(\))?(--|\.\.)(\|>|\*|o|>|\(\))?\s*(?:"([^"]*)")?\s*` + name + `\s*$`)
	classRe   = regexp.MustCompile(`^class\s+` + name + `\s*(?:\["([^"]*)"\])?\s*(?::::([\w-]+))?\s*(\{)?\s*(\})?$`)
	annotRe   = regexp.MustCompile(`^<<\s*([^<>]+?)\s*>>\s*` + name + `$`)
	noteForRe = regexp.MustCompile(`^note\s+for\s+` + name + `\s+"(.*)"$`)
	noteRe    = regexp.MustCompile(`^note\s+"(.*)"$`)
	nsRe      = regexp.MustCompile(`^namespace\s+([\p{L}\p{N}_.\-]+)\s*\{$`)
)

// Parse builds a Diagram from class diagram source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{ClassDefs: map[string]Style{}}
	lines := strings.Split(src, "\n")

	headerSeen := false
	ns := "" // name of the enclosing namespace block, empty at the top level
	var styleLines, classLines [][2]string
	for i := 0; i < len(lines); i++ {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(lines[i]))
		if line == "" {
			continue
		}
		if !headerSeen {
			if firstWord(line) != "classDiagram" && firstWord(line) != "classDiagram-v2" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'classDiagram' header")
			}
			headerSeen = true
			continue
		}
		if len(d.Classes) > maxClasses {
			return nil, syntax.Errorf(lineNo, 1, "too many classes")
		}
		kw := firstWord(line)
		rest := strings.TrimSpace(line[len(kw):])

		switch {
		case line == "}":
			ns = ""

		case nsRe.MatchString(line):
			ns = nsRe.FindStringSubmatch(line)[1]
			if d.namespace(ns) == nil {
				d.Namespaces = append(d.Namespaces, &Namespace{Name: ns})
			}

		case kw == "direction":
			d.Direction = strings.ToUpper(rest)

		case kw == "class":
			m := classRe.FindStringSubmatch(line)
			if m == nil {
				return nil, syntax.Errorf(lineNo, 1, "invalid class declaration %q", clip(line))
			}
			c := d.declare(m[1], ns)
			if m[2] != "" {
				c.Display = m[2]
			}
			if m[3] != "" {
				c.Classes = append(c.Classes, m[3])
			}
			if m[4] != "" && m[5] == "" {
				i = d.consumeBlock(c, lines, i+1)
			}

		case strings.HasPrefix(line, "<<"):
			m := annotRe.FindStringSubmatch(line)
			if m == nil {
				return nil, syntax.Errorf(lineNo, 1, "invalid annotation %q", clip(line))
			}
			d.declare(m[2], ns).Annotation = m[1]

		case kw == "note":
			if m := noteForRe.FindStringSubmatch(line); m != nil {
				c := d.declare(m[1], ns)
				d.Notes = append(d.Notes, &Note{For: c.Name, Text: m[2]})
			} else if m := noteRe.FindStringSubmatch(line); m != nil {
				d.Notes = append(d.Notes, &Note{Text: m[1]})
			} else {
				return nil, syntax.Errorf(lineNo, 1, "invalid note %q", clip(line))
			}

		case kw == "style":
			id := firstWord(rest)
			styleLines = append(styleLines, [2]string{id, strings.TrimSpace(rest[len(id):])})

		case kw == "classDef":
			n := firstWord(rest)
			st := cssval.Parse(strings.TrimSpace(rest[len(n):]))
			for _, one := range strings.Split(n, ",") {
				if one = strings.TrimSpace(one); one != "" {
					d.ClassDefs[one] = st
				}
			}

		case kw == "cssClass":
			// cssClass "A,B" name
			q := strings.Trim(firstWord(rest), `"`)
			classLines = append(classLines, [2]string{q, strings.TrimSpace(rest[len(firstWord(rest)):])})

		case kw == "click" || kw == "link" || kw == "callback":
			// Interactive bindings: a static picture has nothing to bind.

		default:
			core, label := line, ""
			if c := labelColon(line); c >= 0 {
				core, label = strings.TrimSpace(line[:c]), strings.TrimSpace(line[c+1:])
			}
			if m := relRe.FindStringSubmatch(core); m != nil {
				d.addRelation(m, label)
				continue
			}
			if label != "" && !strings.ContainsAny(core, " \t") {
				d.declare(core, ns).addMember(label)
				continue
			}
			return nil, syntax.Errorf(lineNo, 1, "unrecognized statement %q", clip(line))
		}
	}

	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'classDiagram' header")
	}
	for _, s := range styleLines {
		if c := d.class(className(s[0])); c != nil {
			c.Style = cssval.Parse(s[1]).Over(c.Style)
		}
	}
	for _, cl := range classLines {
		for _, id := range strings.Split(cl[0], ",") {
			if c := d.class(className(id)); c != nil && cl[1] != "" {
				c.Classes = append(c.Classes, cl[1])
			}
		}
	}
	return d, nil
}

// labelColon is the index of the colon that starts a relation's label or
// a member, skipping colons inside quoted multiplicities; -1 if none.
func labelColon(line string) int {
	quoted := false
	for i, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ':' && !quoted:
			return i
		}
	}
	return -1
}

func (d *Diagram) addRelation(m []string, label string) {
	leftName, leftCard, lh, line, rh, rightCard, rightName := m[1], m[2], m[3], m[4], m[5], m[6], m[7]
	from := d.declare(leftName, "").Name
	to := d.declare(rightName, "").Name
	d.Relations = append(d.Relations, &Relation{
		From: from, To: to, Label: label,
		Dashed:    line == "..",
		Left:      head(lh),
		Right:     head(rh),
		LeftCard:  leftCard,
		RightCard: rightCard,
	})
}

func head(s string) headKind {
	switch s {
	case "<|", "|>":
		return headTriangle
	case "*":
		return headDiamondFilled
	case "o":
		return headDiamondHollow
	case "<", ">":
		return headArrow
	case "()":
		return headLollipop
	}
	return headNone
}

// consumeBlock reads member lines until a closing "}" and returns the index of
// that closing line so the caller's loop continues after it.
func (d *Diagram) consumeBlock(c *Class, lines []string, start int) int {
	for j := start; j < len(lines); j++ {
		line := strings.TrimSpace(stripComment(lines[j]))
		if line == "" {
			continue
		}
		if line == "}" {
			return j
		}
		c.addMember(line)
	}
	return len(lines) - 1
}

// declare registers a class written as `Box~T~`, keeping the generic
// parameters for display while using the bare name as the identity.
func (d *Diagram) declare(raw, ns string) *Class {
	raw = strings.Trim(strings.TrimSpace(raw), "`")
	c := d.ensureClass(className(raw))
	if strings.ContainsRune(raw, '~') && c.Display == "" {
		c.Display = raw
	}
	if ns != "" && c.Namespace == "" {
		c.Namespace = ns
		if n := d.namespace(ns); n != nil {
			n.Members = append(n.Members, c.Name)
		}
	}
	return c
}

// addMember classifies a member as a method (contains "("), an annotation
// written as <<interface>>, or an attribute.
func (c *Class) addMember(m string) {
	if m == "" {
		return
	}
	if strings.HasPrefix(m, "<<") && strings.HasSuffix(m, ">>") {
		c.Annotation = strings.TrimSpace(m[2 : len(m)-2])
		return
	}
	if strings.Contains(m, "(") {
		c.Methods = append(c.Methods, m)
	} else {
		c.Attributes = append(c.Attributes, m)
	}
}

// className strips a generic suffix like "List~T~" down to "List".
func className(s string) string {
	s = strings.Trim(strings.TrimSpace(s), "`")
	if i := strings.IndexByte(s, '~'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// generics writes Mermaid's ~T~ generic markers as angle brackets:
// List~List~int~~ becomes List<List<int>>. A ~ between two word
// characters opens a parameter list; any other closes one.
func generics(s string) string {
	if !strings.ContainsRune(s, '~') {
		return s
	}
	r := []rune(s)
	word := func(i int) bool {
		if i < 0 || i >= len(r) {
			return false
		}
		c := r[i]
		return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c > 127
	}
	var b strings.Builder
	depth := 0
	for i, c := range r {
		if c != '~' {
			b.WriteRune(c)
			continue
		}
		if word(i-1) && (word(i+1) || i+1 < len(r) && r[i+1] == '[') || depth == 0 {
			b.WriteByte('<')
			depth++
		} else {
			b.WriteByte('>')
			depth--
		}
	}
	return b.String()
}

// member is a class member formatted the way Mermaid draws it.
type member struct {
	text      string
	italic    bool // abstract (*)
	underline bool // static ($)
}

// formatMember turns "+move(int d)* bool" into "+move(int d) : bool",
// marked italic, and "count$" into an underlined "count".
func formatMember(m string, method bool) member {
	var out member
	m = strings.TrimSpace(m)
	if !method {
		if strings.HasSuffix(m, "$") {
			m, out.underline = strings.TrimSuffix(m, "$"), true
		} else if strings.HasSuffix(m, "*") {
			m, out.italic = strings.TrimSuffix(m, "*"), true
		}
		out.text = generics(m)
		return out
	}
	close := strings.LastIndexByte(m, ')')
	if close < 0 {
		out.text = generics(m)
		return out
	}
	sig, ret := m[:close+1], strings.TrimSpace(m[close+1:])
	for _, mark := range []string{"*", "$"} {
		if strings.HasPrefix(ret, mark) || strings.HasSuffix(ret, mark) {
			ret = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(ret, mark), mark))
			if mark == "*" {
				out.italic = true
			} else {
				out.underline = true
			}
		}
	}
	out.text = generics(sig)
	if ret != "" {
		out.text += " : " + generics(ret)
	}
	return out
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
