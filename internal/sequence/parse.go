package sequence

import (
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// arrowTokens are message operators, longest first so the longest match at a
// position wins (e.g. "-->>" before "->>").
var arrowTokens = []struct {
	tok   string
	arrow Arrow
}{
	{"<<-->>", Arrow{Dashed: true, Head: HeadArrow, Both: true}},
	{"<<->>", Arrow{Head: HeadArrow, Both: true}},
	{"-->>", Arrow{Dashed: true, Head: HeadArrow}},
	{"-->", Arrow{Dashed: true, Head: HeadNone}},
	{"--x", Arrow{Dashed: true, Head: HeadCross}},
	{"--)", Arrow{Dashed: true, Head: HeadOpen}},
	{"->>", Arrow{Head: HeadArrow}},
	{"->", Arrow{Head: HeadNone}},
	{"-x", Arrow{Head: HeadCross}},
	{"-)", Arrow{Head: HeadOpen}},
}

// frameKeywords open a drawn frame; the value is the word on its tab.
var frameKeywords = map[string]string{
	"loop": "loop", "alt": "alt", "opt": "opt", "par": "par", "par_over": "par",
	"rect": "rect", "critical": "critical", "break": "break",
}

// sectionKeywords divide the innermost frame.
var sectionKeywords = map[string]bool{"else": true, "and": true, "option": true}

// ignoredKeywords are statements that carry nothing a static picture can
// show: links and menus need a script or an anchor, which the output may
// not contain.
var ignoredKeywords = map[string]bool{
	"link": true, "links": true, "properties": true, "details": true,
}

// openBlock is an entry on the block stack. Frames and boxes are both closed
// by `end`, so both are tracked or the `end` that closes a box would close an
// enclosing frame instead.
type openBlock struct {
	frame *Frame
	box   *Box
}

type parser struct {
	d        *Diagram
	open     []openBlock
	autonum  bool
	msgNum   int
	numStep  int
	creating map[string]bool // created participants waiting for their message
	dying    map[string]bool // destroyed participants waiting for their message
	dyingOrd []string
}

// Parse builds a Diagram from sequence diagram source.
func Parse(src string) (*Diagram, error) {
	p := &parser{
		d:        &Diagram{index: map[string]int{}},
		creating: map[string]bool{},
		dying:    map[string]bool{},
	}
	lines := strings.Split(src, "\n")

	headerSeen := false
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			if syntax.FirstWord(line) != "sequenceDiagram" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'sequenceDiagram' header")
			}
			headerSeen = true
			continue
		}
		if err := p.statement(line, lineNo); err != nil {
			return nil, err
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'sequenceDiagram' header")
	}
	p.finish()
	return p.d, nil
}

func (p *parser) statement(line string, lineNo int) error {
	word := syntax.FirstWord(line)
	kw := strings.ToLower(word)
	rest := strings.TrimSpace(line[len(word):])
	if kw == "title:" {
		kw, rest = "title", strings.TrimSpace(line[len("title:"):])
	}
	switch {
	case kw == "participant" || kw == "actor":
		return p.parseParticipant(kw, rest, lineNo, false)
	case kw == "create":
		w := syntax.FirstWord(rest)
		k := strings.ToLower(w)
		if k != "participant" && k != "actor" {
			return syntax.Errorf(lineNo, 1, "create needs 'participant' or 'actor'")
		}
		return p.parseParticipant(k, strings.TrimSpace(rest[len(w):]), lineNo, true)
	case kw == "destroy":
		if rest == "" {
			return syntax.Errorf(lineNo, 1, "destroy needs a participant")
		}
		p.ensureParticipant(rest, rest)
		if !p.dying[rest] {
			p.dying[rest] = true
			p.dyingOrd = append(p.dyingOrd, rest)
		}
	case kw == "title":
		p.d.Title = decodeEntities(rest)
	case kw == "note":
		return p.parseNote(rest, lineNo)
	case kw == "autonumber":
		p.parseAutonumber(rest)
	case kw == "activate":
		if rest != "" {
			p.ensureParticipant(rest, rest)
			p.d.items = append(p.d.items, item{kind: itActivate, who: rest})
		}
	case kw == "deactivate":
		if rest != "" {
			p.d.items = append(p.d.items, item{kind: itDeactivate, who: rest})
		}
	case kw == "box":
		p.openBox(rest)
	case frameKeywords[kw] != "":
		f := &Frame{Kind: frameKeywords[kw]}
		if kw == "rect" {
			if c, ok := cssval.Color(rest); ok {
				f.Color = c
			} else if !isColorish(rest) {
				f.Label = decodeEntities(rest)
			}
		} else {
			f.Label = decodeEntities(rest)
		}
		p.d.Frames = append(p.d.Frames, f)
		p.open = append(p.open, openBlock{frame: f})
		p.d.items = append(p.d.items, item{kind: itFrameStart, frame: f})
	case sectionKeywords[kw]:
		if top := p.topFrame(); top != nil {
			s := &Section{Label: decodeEntities(rest)}
			top.Sections = append(top.Sections, s)
			p.d.items = append(p.d.items, item{kind: itSection, frame: top, section: s})
		}
	case kw == "end":
		p.closeBlock()
	case ignoredKeywords[kw] && !strings.Contains(line, "->") && !strings.Contains(line, "-x") && !strings.Contains(line, "-)"):
	default:
		return p.parseMessage(line, lineNo)
	}
	return nil
}

func (p *parser) closeBlock() {
	n := len(p.open)
	if n == 0 {
		return
	}
	top := p.open[n-1]
	p.open = p.open[:n-1]
	if top.frame != nil {
		p.d.items = append(p.d.items, item{kind: itFrameEnd, frame: top.frame})
	}
}

// finish closes what the source left open.
func (p *parser) finish() {
	for len(p.open) > 0 {
		p.closeBlock()
	}
	for _, id := range p.dyingOrd {
		if p.dying[id] {
			p.d.items = append(p.d.items, item{kind: itDestroy, who: id})
			if pp := p.d.participant(id); pp != nil {
				pp.Destroyed = true
			}
		}
	}
	for id := range p.creating {
		if pp := p.d.participant(id); pp != nil {
			pp.Created = false
		}
	}
}

func (p *parser) openBox(rest string) {
	b := &Box{}
	color, label := splitColor(rest)
	if color != "" {
		if strings.EqualFold(color, "transparent") {
			b.Color = ""
		} else if c, ok := cssval.Color(color); ok {
			b.Color = c
		}
		b.Label = label
	} else {
		b.Label = rest
	}
	b.Label = decodeEntities(b.Label)
	p.d.Boxes = append(p.d.Boxes, b)
	p.open = append(p.open, openBlock{box: b})
}

// splitColor splits a leading colour token off a box operand. It returns an
// empty colour when the first token is not one.
func splitColor(s string) (color, rest string) {
	l := strings.ToLower(s)
	for _, fn := range []string{"rgb(", "rgba(", "hsl(", "hsla("} {
		if strings.HasPrefix(l, fn) {
			if end := strings.IndexByte(s, ')'); end > 0 {
				return s[:end+1], strings.TrimSpace(s[end+1:])
			}
			return "", s
		}
	}
	w := syntax.FirstWord(s)
	if strings.EqualFold(w, "transparent") {
		return w, strings.TrimSpace(s[len(w):])
	}
	if _, ok := cssval.Color(w); ok && w != "" {
		return w, strings.TrimSpace(s[len(w):])
	}
	return "", s
}

func (p *parser) currentBox() *Box {
	for i := len(p.open) - 1; i >= 0; i-- {
		if p.open[i].box != nil {
			return p.open[i].box
		}
	}
	return nil
}

func (p *parser) parseParticipant(kw, rest string, lineNo int, create bool) error {
	kind := KindParticipant
	if kw == "actor" {
		kind = KindActor
	}
	// Mermaid's `A@{ "type": "actor" }` metadata: read the type, drop the rest.
	if at := strings.Index(rest, "@{"); at >= 0 {
		meta := rest[at:]
		rest = strings.TrimSpace(rest[:at])
		if end := strings.IndexByte(meta, '}'); end >= 0 {
			tail := strings.TrimSpace(meta[end+1:])
			if strings.Contains(strings.ToLower(meta[:end]), `"actor"`) {
				kind = KindActor
			}
			if tail != "" {
				rest += " " + tail
			}
		}
	}
	if rest == "" {
		return syntax.Errorf(lineNo, 1, "participant requires a name")
	}
	id, label := rest, rest
	if idx := strings.Index(rest, " as "); idx >= 0 {
		id = strings.TrimSpace(rest[:idx])
		label = strings.TrimSpace(rest[idx+4:])
	}
	if id == "" {
		return syntax.Errorf(lineNo, 1, "participant requires a name")
	}
	existed := p.d.participant(id) != nil
	pp := p.ensureParticipant(id, decodeEntities(label))
	pp.Label = decodeEntities(label)
	pp.Kind = kind
	if b := p.currentBox(); b != nil && pp.Box == nil {
		pp.Box = b
		b.Members = append(b.Members, pp)
	}
	if create && !existed {
		pp.Created = true
		p.creating[id] = true
	}
	return nil
}

func (p *parser) parseMessage(line string, lineNo int) error {
	idx, tok, arrow := findArrow(line)
	if idx < 0 {
		return syntax.Errorf(lineNo, 1, "unrecognized statement %q", line)
	}
	from := strings.TrimSpace(line[:idx])
	rest := line[idx+len(tok):]

	toRaw, text := rest, ""
	if c := strings.IndexByte(rest, ':'); c >= 0 {
		toRaw = rest[:c]
		text = strings.TrimSpace(rest[c+1:])
	}
	toRaw = strings.TrimSpace(toRaw)

	m := &Message{Arrow: arrow, Text: decodeEntities(text)}
	switch {
	case strings.HasPrefix(toRaw, "+"):
		m.Activate, toRaw = true, strings.TrimSpace(toRaw[1:])
	case strings.HasPrefix(toRaw, "-"):
		m.Deactivate, toRaw = true, strings.TrimSpace(toRaw[1:])
	}
	m.From, m.To = from, toRaw
	if m.From == "" || m.To == "" {
		return syntax.Errorf(lineNo, 1, "message needs a sender and receiver")
	}
	p.ensureParticipant(m.From, m.From)
	p.ensureParticipant(m.To, m.To)

	for _, id := range []string{m.To, m.From} {
		if p.creating[id] && m.Creates == "" {
			m.Creates = id
			delete(p.creating, id)
		}
	}
	for _, id := range []string{m.From, m.To} {
		if p.dying[id] {
			m.Destroys = append(m.Destroys, id)
			p.dying[id] = false
			p.d.participant(id).Destroyed = true
		}
	}

	if p.autonum {
		m.Num = p.msgNum
		p.msgNum += p.numStep
	}
	p.d.Messages = append(p.d.Messages, m)
	p.d.items = append(p.d.items, item{kind: itMessage, msg: m})
	return nil
}

func (p *parser) parseNote(rest string, lineNo int) error {
	l := strings.ToLower(rest)
	var pos NotePos
	switch {
	case strings.HasPrefix(l, "right of "):
		pos, rest = NoteRight, rest[len("right of "):]
	case strings.HasPrefix(l, "left of "):
		pos, rest = NoteLeft, rest[len("left of "):]
	case strings.HasPrefix(l, "over "):
		pos, rest = NoteOver, rest[len("over "):]
	default:
		return syntax.Errorf(lineNo, 1, "note needs 'right of', 'left of', or 'over'")
	}
	who, text := rest, ""
	if c := strings.IndexByte(rest, ':'); c >= 0 {
		who = rest[:c]
		text = strings.TrimSpace(rest[c+1:])
	}
	var of []string
	for _, w := range strings.Split(who, ",") {
		if w = strings.TrimSpace(w); w != "" {
			of = append(of, w)
			p.ensureParticipant(w, w)
		}
	}
	if len(of) == 0 {
		return syntax.Errorf(lineNo, 1, "note needs a participant")
	}
	if len(of) > 2 {
		of = []string{of[0], of[len(of)-1]}
	}
	n := &Note{Pos: pos, Of: of, Text: decodeEntities(text)}
	p.d.Notes = append(p.d.Notes, n)
	p.d.items = append(p.d.items, item{kind: itNote, note: n})
	return nil
}

func (p *parser) ensureParticipant(id, label string) *Participant {
	if pp := p.d.participant(id); pp != nil {
		return pp
	}
	pp := &Participant{ID: id, Label: label}
	p.d.index[id] = len(p.d.Participants)
	p.d.Participants = append(p.d.Participants, pp)
	return pp
}

// findArrow returns the earliest arrow operator in s (longest match wins).
func findArrow(s string) (idx int, tok string, arrow Arrow) {
	for i := 0; i < len(s); i++ {
		if s[i] != '-' && s[i] != '<' {
			continue
		}
		for _, at := range arrowTokens {
			if strings.HasPrefix(s[i:], at.tok) {
				return i, at.tok, at.arrow
			}
		}
	}
	return -1, "", Arrow{}
}

func stripComment(s string) string {
	if strings.HasPrefix(strings.TrimSpace(s), "%%") {
		return ""
	}
	return s
}

// topFrame returns the innermost open frame, skipping boxes, or nil.
func (p *parser) topFrame() *Frame {
	for i := len(p.open) - 1; i >= 0; i-- {
		if p.open[i].frame != nil {
			return p.open[i].frame
		}
	}
	return nil
}

// parseAutonumber reads the optional start and step operands. `autonumber`
// alone numbers from 1 in steps of 1; `autonumber 10 10` starts at 10 and
// steps by 10; `autonumber off` stops numbering.
func (p *parser) parseAutonumber(operand string) {
	p.autonum = true
	p.msgNum = 1
	p.numStep = 1
	fields := strings.Fields(operand)
	if len(fields) > 0 && strings.EqualFold(fields[0], "off") {
		p.autonum = false
		return
	}
	if len(fields) > 0 {
		if n, err := strconv.Atoi(fields[0]); err == nil && n > 0 && n < 1e9 {
			p.msgNum = n
		}
	}
	if len(fields) > 1 {
		if n, err := strconv.Atoi(fields[1]); err == nil && n > 0 && n < 1e9 {
			p.numStep = n
		}
	}
}

// isColorish reports whether a rect operand was meant as a colour, even an
// invalid one, so it is dropped rather than drawn as a label.
func isColorish(s string) bool {
	l := strings.ToLower(s)
	return strings.HasPrefix(l, "rgb") || strings.HasPrefix(l, "hsl") || strings.HasPrefix(l, "#")
}

var namedEntities = map[string]string{
	"quot": `"`, "amp": "&", "lt": "<", "gt": ">", "nbsp": " ", "colon": ":",
	"semi": ";", "num": "#", "apos": "'", "lpar": "(", "rpar": ")",
}

// decodeEntities turns Mermaid's escapes (#59; or #quot;) into characters.
// The renderer escapes everything again on output.
func decodeEntities(s string) string {
	if !strings.Contains(s, "#") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '#' {
			if end := strings.IndexByte(s[i+1:], ';'); end > 0 && end <= 8 {
				code := s[i+1 : i+1+end]
				if n, err := strconv.Atoi(code); err == nil && n > 31 && n < 0x110000 && (n < 0xD800 || n > 0xDFFF) {
					b.WriteRune(rune(n))
					i += end + 1
					continue
				}
				if r, ok := namedEntities[strings.ToLower(code)]; ok {
					b.WriteString(r)
					i += end + 1
					continue
				}
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
