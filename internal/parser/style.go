package parser

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/domain"
	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// classAssign records "apply these class names to these ids".
type classAssign struct {
	ids   []string
	names []string
}

type directStyle struct {
	id    string
	style *domain.Style
}

// styles holds what the styling lines of a flowchart said.
type styles struct {
	classDefs map[string]*domain.Style
	assigns   []classAssign
	direct    []directStyle
	links     LinkStyles
}

// preprocess pulls the styling statements (classDef, class, style,
// linkStyle) and the interaction ones a static picture has no use for
// (click, callbacks) out of the source. It returns the remaining statements,
// one per line, for the lexer. Statements may be separated by newlines or by
// semicolons.
func preprocess(src string) (string, *styles) {
	st := &styles{classDefs: map[string]*domain.Style{}, links: LinkStyles{ByIndex: map[int]*domain.Style{}}}
	var kept []string
	inAccBlock := false
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if inAccBlock {
			if strings.Contains(t, "}") {
				inAccBlock = false
			}
			kept = append(kept, "")
			continue
		}
		if strings.HasPrefix(t, "accDescr") && strings.HasSuffix(t, "{") {
			inAccBlock = true
			kept = append(kept, "")
			continue
		}
		var out []string
		for _, stmt := range splitStatements(line) {
			if !st.read(strings.TrimSpace(stmt)) {
				out = append(out, stmt)
			}
		}
		// Keep the line count so error positions still point at the source.
		kept = append(kept, strings.Join(out, ";"))
	}
	return strings.Join(kept, "\n"), st
}

// read takes one statement and reports whether it was a styling or
// interaction statement it consumed.
func (st *styles) read(t string) bool {
	word, rest, _ := strings.Cut(t, " ")
	rest = strings.TrimSpace(rest)
	switch word {
	case "classDef":
		names, props, _ := strings.Cut(rest, " ")
		s := parseProps(props)
		for _, n := range strings.Split(names, ",") {
			if n = strings.TrimSpace(n); n != "" {
				st.classDefs[n] = s
			}
		}
	case "class":
		f := strings.Fields(rest)
		if len(f) >= 2 {
			st.assigns = append(st.assigns, classAssign{ids: splitList(f[0]), names: splitList(strings.Join(f[1:], ""))})
		}
	case "style":
		id, props, _ := strings.Cut(rest, " ")
		if id != "" {
			st.direct = append(st.direct, directStyle{id: id, style: parseProps(props)})
		}
	case "linkStyle":
		parseLinkStyle(rest, &st.links)
	case "click", "callback":
	case "accTitle:", "accDescr:":
	default:
		return strings.HasPrefix(t, "accTitle:") || strings.HasPrefix(t, "accDescr:")
	}
	return true
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// entityTail matches the end of an entity written before a semicolon, as in
// "#quot;" or "&amp;", so that semicolon does not end the statement.
var entityTail = regexp.MustCompile(`[#&]#?[A-Za-z0-9]+$`)

// splitStatements splits a line at the semicolons that end statements,
// leaving those inside quotes, shapes, pipes and entities alone.
func splitStatements(line string) []string {
	var out []string
	depth, inQuote, inPipe := 0, false, false
	start := 0
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inQuote:
			if c == '"' {
				inQuote = false
			}
		case c == '"':
			inQuote = true
		case c == '[' || c == '(' || c == '{':
			depth++
		case (c == ']' || c == ')' || c == '}') && depth > 0:
			depth--
		case c == '|' && depth == 0:
			inPipe = !inPipe
		case c == ';' && depth == 0 && !inPipe:
			if entityTail.MatchString(line[start:i]) && !isStyling(line[start:i]) {
				continue
			}
			out = append(out, line[start:i])
			start = i + 1
		}
	}
	return append(out, line[start:])
}

// isStyling reports whether a statement is a styling line, whose "#333;"
// is a colour and not an entity.
func isStyling(stmt string) bool {
	word, _, _ := strings.Cut(strings.TrimSpace(stmt), " ")
	switch word {
	case "classDef", "class", "style", "linkStyle":
		return true
	}
	return false
}

// apply resolves the styles onto the graph. The default class goes first,
// then classes in the order they were assigned (inline ones first), then
// direct style lines, so a later, more specific line wins.
func (st *styles) apply(g *domain.Graph, inline []classAssign) {
	targets := map[string]**domain.Style{}
	for _, n := range g.Nodes {
		targets[n.ID] = &n.Style
	}
	for _, sg := range g.Subgraphs {
		if _, ok := targets[sg.ID]; !ok {
			targets[sg.ID] = &sg.Style
		}
	}
	merge := func(id string, s *domain.Style) {
		if p, ok := targets[id]; ok && s != nil {
			if *p == nil {
				*p = &domain.Style{}
			}
			mergeStyle(*p, s)
		}
	}
	if def, ok := st.classDefs["default"]; ok {
		for _, n := range g.Nodes {
			merge(n.ID, def)
		}
	}
	for _, a := range append(append([]classAssign{}, inline...), st.assigns...) {
		for _, name := range a.names {
			for _, id := range a.ids {
				merge(id, st.classDefs[name])
			}
		}
	}
	for _, d := range st.direct {
		merge(d.id, d.style)
	}
	for i, e := range g.Edges {
		if s := st.links.For(i); s != nil {
			e.Style = s
		}
	}
}

// propSplit splits "fill:#f9f,stroke:rgb(1,2,3)" at the commas outside
// parentheses.
func propSplit(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// parseProps parses "fill:#f9f,stroke:#333,color:#fff" into a Style. Each
// value is checked by cssval; one that is not a plain colour, length, dash
// pattern or keyword is dropped, so a style line can never carry markup into
// the picture.
func parseProps(s string) *domain.Style {
	st := &domain.Style{}
	for _, kv := range propSplit(s) {
		k, v, ok := strings.Cut(kv, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimRight(v, "; ")), "!important"))
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "fill", "background", "background-color":
			st.Fill, _ = cssval.Color(v)
		case "stroke":
			st.Stroke, _ = cssval.Color(v)
		case "color":
			st.Color, _ = cssval.Color(v)
		case "stroke-width":
			if px, ok := cssval.Pixels(v, 40); ok {
				st.StrokeWidth = svgutil.Num(px)
			}
		case "stroke-dasharray":
			st.StrokeDash, _ = cssval.Dash(v)
		case "font-weight":
			st.FontWeight, _ = cssval.FontWeight(v)
		case "font-style":
			st.FontStyle, _ = cssval.FontStyle(v)
		}
	}
	return st
}

// mergeStyle overlays the non-empty fields of src onto dst.
func mergeStyle(dst, src *domain.Style) {
	set := func(d *string, s string) {
		if s != "" {
			*d = s
		}
	}
	set(&dst.Fill, src.Fill)
	set(&dst.Stroke, src.Stroke)
	set(&dst.Color, src.Color)
	set(&dst.StrokeWidth, src.StrokeWidth)
	set(&dst.StrokeDash, src.StrokeDash)
	set(&dst.FontWeight, src.FontWeight)
	set(&dst.FontStyle, src.FontStyle)
}

// LinkStyles holds the per-edge overrides from linkStyle directives. Default
// applies to every edge that has no index-specific entry.
type LinkStyles struct {
	ByIndex map[int]*domain.Style
	Default *domain.Style
}

// For returns the style for the edge at index i, or nil when none applies.
func (l *LinkStyles) For(i int) *domain.Style {
	if l == nil {
		return nil
	}
	if st, ok := l.ByIndex[i]; ok {
		if l.Default != nil {
			merged := *l.Default
			mergeStyle(&merged, st)
			return &merged
		}
		return st
	}
	return l.Default
}

var interpolate = regexp.MustCompile(`^interpolate\s+\S+\s*`)

// parseLinkStyle reads `0,2 stroke:#f00,stroke-width:4px` and `default ...`
// (the text after "linkStyle"). The selector is a comma-separated list of
// edge indexes in source order, matching Mermaid.
func parseLinkStyle(rest string, into *LinkStyles) {
	sel, props, ok := strings.Cut(rest, " ")
	if !ok {
		return
	}
	props = interpolate.ReplaceAllString(strings.TrimSpace(props), "")
	st := parseProps(props)
	if strings.EqualFold(strings.TrimSpace(sel), "default") {
		into.Default = st
		return
	}
	for _, part := range strings.Split(sel, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 0 {
			continue
		}
		into.ByIndex[n] = st
	}
}

var (
	htmlTag      = regexp.MustCompile(`</?[A-Za-z][^<>]*>`)
	hashEntity   = regexp.MustCompile(`#([A-Za-z]+|[0-9]+);`)
	mdStrong     = regexp.MustCompile(`(\*\*|__)(\S(?:.*?\S)?)(\*\*|__)`)
	mdEmphasis   = regexp.MustCompile(`(^|[^\w*])[*_](\S(?:[^*_]*?\S)?)[*_]($|[^\w*])`)
	spaceAroundN = regexp.MustCompile(`[ \t]*\n[ \t]*`)
)

// cleanLabel turns label source into the plain text drawn: a markdown string
// ("`**bold** text`") loses its markers and keeps its line breaks, HTML tags
// other than <br> are dropped, and entities written as "#quot;" or "&amp;"
// become their characters. The result is plain text; the renderer escapes
// it.
func cleanLabel(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		s = s[1 : len(s)-1]
		s = mdStrong.ReplaceAllString(s, "$2")
		s = mdEmphasis.ReplaceAllString(s, "$1$2$3")
		s = spaceAroundN.ReplaceAllString(strings.TrimSpace(s), "\n")
	}
	s = svgutil.JoinBreaks(s)
	s = htmlTag.ReplaceAllString(s, "")
	s = hashEntity.ReplaceAllStringFunc(s, func(m string) string {
		name := m[1 : len(m)-1]
		if name[0] >= '0' && name[0] <= '9' {
			return "&#" + name + ";"
		}
		return "&" + name + ";"
	})
	return strings.TrimSpace(html.UnescapeString(s))
}
