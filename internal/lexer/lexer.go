// Package lexer turns Mermaid flowchart source into a flat token stream.
// It captures shape and edge-label text as single Text tokens, and reads
// every link form (markers, lengths, labels written between its halves) into
// one Arrow token, so the parser can stay grammar-focused.
package lexer

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Lex scans src and returns its tokens, always terminated by an EOF token.
func Lex(src string) ([]Token, error) {
	l := &lexer{src: []rune(src), line: 1, col: 1, identEnd: -1}
	return l.run()
}

type lexer struct {
	src  []rune
	pos  int
	line int
	col  int
	toks []Token
	// identEnd is the position just past the last identifier, so a shape
	// opener written straight after it (A>flag]) can be told apart from a
	// link.
	identEnd int
}

var keywords = map[string]bool{
	"graph": true, "flowchart": true, "subgraph": true, "end": true,
}

func (l *lexer) run() ([]Token, error) {
	for l.pos < len(l.src) {
		r := l.src[l.pos]
		var err error
		switch {
		case r == '\n' || r == ';':
			l.toks = append(l.toks, l.emit(Newline, string(r)))
			l.advance()
		case r == '\r' || r == ' ' || r == '\t' || r == ' ':
			l.advance()
		case r == '%' && l.peek(1) == '%':
			l.skipLine()
		case r == '&':
			l.toks = append(l.toks, l.emit(Amp, "&"))
			l.advance()
		case r == '|':
			err = l.lexPipeLabel()
		case r == ':' && l.peek(1) == ':' && l.peek(2) == ':':
			l.lexClassRef()
		case r == '>' && l.identEnd == l.pos:
			err = l.lexShape()
		case isShapeOpen(r):
			err = l.lexShape()
		case r == '<' || isConnectorRune(r):
			err = l.lexLink("")
		case isIdentRune(r):
			err = l.lexIdent()
		default:
			return nil, syntax.Errorf(l.line, l.col, "unexpected character %q", string(r))
		}
		if err != nil {
			return nil, err
		}
	}
	l.toks = append(l.toks, Token{Kind: EOF, Line: l.line, Col: l.col})
	return l.toks, nil
}

// prevEndsNode reports whether the last token closes a node reference, so
// what follows may be a link.
func (l *lexer) prevEndsNode() bool {
	if len(l.toks) == 0 {
		return false
	}
	switch l.toks[len(l.toks)-1].Kind {
	case Ident, ShapeClose, ClassRef, Meta:
		return true
	}
	return false
}

func (l *lexer) lexIdent() error {
	start, line, col := l.pos, l.line, l.col
	// "o" or "x" written against a link after a node is the link's start
	// marker, as in A o--o B.
	if (l.src[l.pos] == 'o' || l.src[l.pos] == 'x') && l.prevEndsNode() && l.pos > 0 &&
		unicode.IsSpace(l.src[l.pos-1]) && isLinkBody(l.peek(1)) && isLinkBody(l.peek(2)) {
		marker := "circle"
		if l.src[l.pos] == 'x' {
			marker = "cross"
		}
		l.advance()
		return l.lexLinkFrom(marker, line, col)
	}
	for l.pos < len(l.src) {
		r := l.src[l.pos]
		if isIdentRune(r) {
			l.advance()
			continue
		}
		// A single hyphen or dot joins an identifier, as in "node-1" and
		// "a.b", but only when a word character follows. That keeps the
		// connectors "-->" and "-.->" out of the identifier.
		if (r == '-' || r == '.') && l.pos+1 < len(l.src) && isIdentRune(l.src[l.pos+1]) {
			l.advance()
			continue
		}
		break
	}
	val := string(l.src[start:l.pos])
	if strings.HasPrefix(val, "flowchart-") && !l.prevEndsNode() {
		val = "flowchart" // flowchart-elk, flowchart-v2
	}
	k := Ident
	switch {
	case keywords[val]:
		k = Keyword
	case val == "direction" && l.directionFollows():
		k = Keyword
	}
	l.toks = append(l.toks, Token{Kind: k, Val: val, Line: line, Col: col})
	if k == Keyword && val == "subgraph" {
		l.lexRestOfLine()
		return nil
	}
	if k != Ident {
		return nil
	}
	l.identEnd = l.pos
	if l.peek(0) == '@' {
		switch {
		case l.peek(1) == '{':
			return l.lexMeta()
		case l.peek(1) == '<' || isConnectorRune(l.peek(1)):
			// e1@--> names the link that follows.
			l.toks[len(l.toks)-1].Kind = EdgeID
			l.advance()
		}
	}
	return nil
}

// directionFollows reports whether the word after the current position is a
// flow direction, so "direction TB" is read as a statement.
func (l *lexer) directionFollows() bool {
	i := l.pos
	for i < len(l.src) && (l.src[i] == ' ' || l.src[i] == '\t') {
		i++
	}
	j := i
	for j < len(l.src) && isIdentRune(l.src[j]) {
		j++
	}
	switch strings.ToUpper(string(l.src[i:j])) {
	case "TB", "TD", "BT", "LR", "RL":
		return true
	}
	return false
}

// lexRestOfLine captures what follows "subgraph" up to the end of the line
// as one Text token.
func (l *lexer) lexRestOfLine() {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.advance()
	}
	start, line, col := l.pos, l.line, l.col
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
	text := strings.TrimSpace(string(l.src[start:l.pos]))
	text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
	if text != "" {
		l.toks = append(l.toks, Token{Kind: Text, Val: text, Line: line, Col: col})
	}
}

// lexMeta reads "@{ ... }" after a node or edge id. The body may hold quoted
// strings and run over several lines.
func (l *lexer) lexMeta() error {
	line, col := l.line, l.col
	l.advance() // @
	l.advance() // {
	start := l.pos
	var quote rune
	for l.pos < len(l.src) {
		r := l.src[l.pos]
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '}':
			body := string(l.src[start:l.pos])
			l.advance()
			l.toks = append(l.toks, Token{Kind: Meta, Val: body, Line: line, Col: col})
			return nil
		}
		l.advance()
	}
	return syntax.Errorf(line, col, "unterminated metadata block")
}

func (l *lexer) lexClassRef() {
	line, col := l.line, l.col
	for range 3 {
		l.advance()
	}
	start := l.pos
	for l.pos < len(l.src) && (isIdentRune(l.src[l.pos]) || l.src[l.pos] == '-') {
		l.advance()
	}
	l.toks = append(l.toks, Token{Kind: ClassRef, Val: string(l.src[start:l.pos]), Line: line, Col: col})
}

// lexLink reads a link at the current position.
func (l *lexer) lexLink(start string) error {
	return l.lexLinkFrom(start, l.line, l.col)
}

// Link halves that close a label written inside a link, by the kind of
// opening half.
var (
	solidClose  = regexp.MustCompile(`^(-{2,})(>|o|x|-)`)
	thickClose  = regexp.MustCompile(`^(={2,})(>|o|x|=)`)
	dottedClose = regexp.MustCompile(`^(\.+)-(>|o|x)?`)
)

func (l *lexer) lexLinkFrom(start string, line, col int) error {
	if l.peek(0) == '<' {
		start = "arrow"
		l.advance()
	}
	bodyStart := l.pos
	for l.pos < len(l.src) && isLinkBody(l.src[l.pos]) {
		l.advance()
	}
	body := string(l.src[bodyStart:l.pos])
	if body == "" {
		return syntax.Errorf(line, col, "unexpected character %q", "<")
	}
	end := l.readEndMarker()
	link := classify(body, start, end)

	// A label written between the halves: A -- text --> B.
	if end == "" && (body == "--" || body == "==" || body == "-.") {
		if ok := l.lexMiddleLabel(body, link); !ok {
			// No closing half on the line: a plain open link, A -- B.
			link = classify(body+"-", start, "")
		}
	}
	l.toks = append(l.toks, Token{Kind: Arrow, Val: string(l.src[bodyStart:l.pos]), Line: line, Col: col, Link: link})
	return nil
}

// readEndMarker consumes an arrowhead, circle or cross at the end of a link.
func (l *lexer) readEndMarker() string {
	switch r := l.peek(0); {
	case r == '>':
		l.advance()
		return "arrow"
	case (r == 'o' || r == 'x') && !isIdentRune(l.peek(1)):
		l.advance()
		if r == 'o' {
			return "circle"
		}
		return "cross"
	}
	return ""
}

// lexMiddleLabel looks past an opening half (--, ==, -.) for the label and
// the closing half on the same line. On success it fills link and leaves the
// position after the closing half.
func (l *lexer) lexMiddleLabel(open string, link *Link) bool {
	i := l.pos
	for i < len(l.src) && (l.src[i] == ' ' || l.src[i] == '\t') {
		i++
	}
	lineEnd := i
	for lineEnd < len(l.src) && l.src[lineEnd] != '\n' {
		lineEnd++
	}
	if i == l.pos && i < lineEnd && l.src[i] != '"' {
		return false // "--x" style links have no space before the label
	}
	textStart, textEnd := i, -1
	searchFrom := i
	if i < lineEnd && l.src[i] == '"' {
		q := i + 1
		for q < lineEnd && l.src[q] != '"' {
			q++
		}
		if q >= lineEnd {
			return false
		}
		textStart, textEnd, searchFrom = i+1, q, q+1
	}
	closeRe := solidClose
	switch open {
	case "==":
		closeRe = thickClose
	case "-.":
		closeRe = dottedClose
	}
	for j := searchFrom; j < lineEnd; j++ {
		if j > searchFrom && !(l.src[j-1] == ' ' || l.src[j-1] == '\t') && textEnd < 0 {
			// The closing half starts a new word after the label.
			continue
		}
		rest := string(l.src[j:lineEnd])
		m := closeRe.FindStringSubmatch(rest)
		if m == nil {
			continue
		}
		closer := m[0]
		end := ""
		switch m[len(m)-1] {
		case ">":
			end = "arrow"
		case "o":
			end = "circle"
		case "x":
			end = "cross"
		}
		if (end == "circle" || end == "cross") && j+utf8.RuneCountInString(closer) < lineEnd &&
			isIdentRune(l.src[j+utf8.RuneCountInString(closer)]) {
			continue
		}
		label := ""
		if textEnd >= 0 {
			if strings.TrimSpace(string(l.src[searchFrom:j])) != "" {
				return false
			}
			label = string(l.src[textStart:textEnd])
		} else {
			label = strings.TrimSpace(string(l.src[textStart:j]))
		}
		if label == "" && textEnd < 0 {
			return false
		}
		body := m[1]
		if open == "-." {
			body = "-" + m[1] + "-"
		} else if end == "" {
			body = m[0]
		}
		c := classify(body, link.Start, end)
		*link = *c
		link.Label, link.HasLabel = label, true
		for l.pos < j+utf8.RuneCountInString(closer) {
			l.advance()
		}
		return true
	}
	return false
}

// classify works out a link's line, length and markers from its body (the
// run of - = . ~ characters) and end markers.
func classify(body, start, end string) *Link {
	link := &Link{Start: start, End: end, Line: "solid"}
	count := func(r rune) int { return strings.Count(body, string(r)) }
	switch {
	case strings.Contains(body, "~"):
		link.Line, link.Len = "invisible", count('~')-2
	case strings.Contains(body, "="):
		link.Line = "thick"
		link.Len = count('=') - 1
		if end == "" {
			link.Len = count('=') - 2
		}
	case strings.Contains(body, "."):
		link.Line, link.Len = "dotted", count('.')
	default:
		link.Len = count('-') - 1
		if end == "" {
			link.Len = count('-') - 2
		}
	}
	if link.Len < 1 {
		link.Len = 1
	}
	return link
}

// lexShape captures a node shape: opener, inner text, closer. Some openers
// admit more than one closer (e.g. "[/" closes with "/]" for a parallelogram
// or "\]" for a trapezoid). Text in double quotes may hold any closer.
func (l *lexer) lexShape() error {
	line, col := l.line, l.col
	save := *l
	open := l.readOpener()
	if err := l.lexShapeBody(open, shapeClosers[open], line, col); err == nil {
		return nil
	} else if len([]rune(open)) == 2 && (open == "[/" || open == "[\\") {
		// "[/api: x]" is a plain box whose text starts with a slash.
		*l = save
		l.advance()
		return l.lexShapeBody("[", shapeClosers["["], line, col)
	} else {
		return err
	}
}

func (l *lexer) lexShapeBody(open string, candidates []string, line, col int) error {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.advance()
	}
	textLine, textCol := l.line, l.col
	var text string
	if l.peek(0) == '"' {
		l.advance()
		start := l.pos
		for l.pos < len(l.src) && l.src[l.pos] != '"' {
			l.advance()
		}
		if l.pos >= len(l.src) {
			return syntax.Errorf(line, col, "unterminated string in shape %q", open)
		}
		text = string(l.src[start:l.pos])
		l.advance()
		for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
			l.advance()
		}
		if l.matchCloser(candidates) == "" {
			return syntax.Errorf(l.line, l.col, "expected the end of shape %q", open)
		}
	} else {
		start := l.pos
		for l.pos < len(l.src) && l.matchCloser(candidates) == "" {
			if l.src[l.pos] == '\n' {
				return syntax.Errorf(line, col, "unterminated shape %q", open)
			}
			l.advance()
		}
		if l.pos >= len(l.src) {
			return syntax.Errorf(line, col, "unterminated shape %q", open)
		}
		text = strings.TrimSpace(string(l.src[start:l.pos]))
	}
	closer := l.matchCloser(candidates)
	closeLine, closeCol := l.line, l.col
	for range []rune(closer) {
		l.advance()
	}
	l.toks = append(l.toks,
		Token{Kind: ShapeOpen, Val: open, Line: line, Col: col},
		Token{Kind: Text, Val: text, Line: textLine, Col: textCol},
		Token{Kind: ShapeClose, Val: closer, Line: closeLine, Col: closeCol},
	)
	return nil
}

// matchCloser returns the candidate closer present at the current position.
func (l *lexer) matchCloser(candidates []string) string {
	for _, c := range candidates {
		if l.hasPrefix(c) {
			return c
		}
	}
	return ""
}

// lexPipeLabel captures |text| used for inline edge labels.
func (l *lexer) lexPipeLabel() error {
	line, col := l.line, l.col
	l.advance() // opening |
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.advance()
	}
	textLine, textCol := l.line, l.col
	var text string
	if l.peek(0) == '"' {
		l.advance()
		start := l.pos
		for l.pos < len(l.src) && l.src[l.pos] != '"' {
			l.advance()
		}
		if l.pos >= len(l.src) {
			return syntax.Errorf(line, col, "unterminated label")
		}
		text = string(l.src[start:l.pos])
		l.advance()
		for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
			l.advance()
		}
		if l.peek(0) != '|' {
			return syntax.Errorf(l.line, l.col, "unterminated label")
		}
	} else {
		start := l.pos
		for l.pos < len(l.src) && l.src[l.pos] != '|' {
			if l.src[l.pos] == '\n' {
				return syntax.Errorf(line, col, "unterminated label")
			}
			l.advance()
		}
		if l.pos >= len(l.src) {
			return syntax.Errorf(line, col, "unterminated label")
		}
		text = strings.TrimSpace(string(l.src[start:l.pos]))
	}
	closeCol := l.col
	l.advance() // closing |
	l.toks = append(l.toks,
		Token{Kind: Pipe, Val: "|", Line: line, Col: col},
		Token{Kind: Text, Val: text, Line: textLine, Col: textCol},
		Token{Kind: Pipe, Val: "|", Line: l.line, Col: closeCol},
	)
	return nil
}

// shapeOpeners lists opening delimiters, longest first so the longest match
// at a position wins (e.g. "[[" before "[").
var shapeOpeners = []string{"(((", "[[", "[(", "[/", "[\\", "([", "((", "{{", "[", "(", "{", ">"}

// shapeClosers maps each opener to its candidate closing delimiters.
var shapeClosers = map[string][]string{
	"(((": {")))"},
	"[[":  {"]]"},
	"[(":  {")]"},
	"[/":  {"/]", "\\]"}, // parallelogram or trapezoid
	"[\\": {"\\]", "/]"}, // parallelogram-alt or trapezoid-alt
	"([":  {"])"},
	"((":  {"))"},
	"{{":  {"}}"},
	"[":   {"]"},
	"(":   {")"},
	"{":   {"}"},
	">":   {"]"},
}

// readOpener consumes the longest valid opening delimiter at pos.
func (l *lexer) readOpener() string {
	for _, o := range shapeOpeners {
		if l.hasPrefix(o) {
			for range []rune(o) {
				l.advance()
			}
			return o
		}
	}
	o := string(l.src[l.pos])
	l.advance()
	return o
}

func (l *lexer) skipLine() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
}

func (l *lexer) hasPrefix(s string) bool {
	rs := []rune(s)
	if l.pos+len(rs) > len(l.src) {
		return false
	}
	for i, r := range rs {
		if l.src[l.pos+i] != r {
			return false
		}
	}
	return true
}

func (l *lexer) peek(n int) rune {
	if l.pos+n >= len(l.src) {
		return 0
	}
	return l.src[l.pos+n]
}

func (l *lexer) advance() {
	if l.pos < len(l.src) && l.src[l.pos] == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	l.pos++
}

func (l *lexer) emit(k Kind, v string) Token {
	return Token{Kind: k, Val: v, Line: l.line, Col: l.col}
}

func isShapeOpen(r rune) bool { return r == '[' || r == '(' || r == '{' }

func isConnectorRune(r rune) bool {
	switch r {
	case '-', '.', '=', '~':
		return true
	}
	return false
}

func isLinkBody(r rune) bool { return isConnectorRune(r) }

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || unicode.Is(unicode.Mn, r)
}
