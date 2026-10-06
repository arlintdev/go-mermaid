// Package timeline parses and renders Mermaid timeline diagrams to SVG.
//
// Syntax:
//
//	timeline
//	    title History
//	    section Early
//	      2002 : LinkedIn
//	      2004 : Facebook : Google
package timeline

import (
	"strings"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// Period is a point in time with one or more events.
type Period struct {
	Time   string
	Events []string
}

// Section groups consecutive periods.
type Section struct {
	Name    string
	Periods []*Period
}

// Diagram is a parsed timeline.
type Diagram struct {
	Title    string
	Sections []*Section
}

// Periods returns all periods across sections in order.
func (d *Diagram) Periods() []*Period {
	var all []*Period
	for _, s := range d.Sections {
		all = append(all, s.Periods...)
	}
	return all
}

// Parse builds a Diagram from timeline source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{}
	var cur *Section
	var last *Period

	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(syntax.StripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			if syntax.FirstWord(line) != "timeline" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'timeline' header")
			}
			headerSeen = true
			continue
		}
		key := syntax.FirstWord(line)
		rest := strings.TrimSpace(line[len(key):])
		switch {
		case key == "title":
			d.Title = rest
		case key == "section":
			cur = &Section{Name: rest}
			d.Sections = append(d.Sections, cur)
		case strings.HasPrefix(line, ":"):
			// A continuation line adds events to the period above it.
			if last == nil {
				return nil, syntax.Errorf(lineNo, 1, "event before any time period")
			}
			last.Events = append(last.Events, events(line[1:])...)
		default:
			parts := splitEvents(line)
			last = &Period{Time: strings.TrimSpace(parts[0]), Events: events(strings.Join(parts[1:], ": "))}
			if cur == nil {
				cur = &Section{}
				d.Sections = append(d.Sections, cur)
			}
			cur.Periods = append(cur.Periods, last)
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'timeline' header")
	}
	return d, nil
}

// events splits "a : b : c" into its non-empty events.
func events(s string) []string {
	var out []string
	for _, ev := range splitEvents(s) {
		if ev = strings.TrimSpace(ev); ev != "" {
			out = append(out, ev)
		}
	}
	return out
}

// splitEvents splits s at each colon followed by a space or the end of the
// line, as Mermaid does, so a colon inside a word (a URL, a time) stays.
func splitEvents(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ':' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t') {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
