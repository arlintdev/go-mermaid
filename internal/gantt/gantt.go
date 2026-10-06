// Package gantt parses and renders Mermaid gantt charts to SVG.
//
// Syntax:
//
//	gantt
//	    title Project
//	    dateFormat YYYY-MM-DD
//	    axisFormat %b %d
//	    excludes weekends
//	    section Phase 1
//	      Design : done, d1, 2024-01-01, 10d
//	      Build  : crit, after d1, 2w
//	      Launch : milestone, 2024-02-01, 0d
//
// A task's data is a comma-separated list. Tags (done, active, crit,
// milestone, vert) come first; then, as in Mermaid, one field is the end
// (the task starts where the previous one ended), two are start and end,
// and three are id, start and end. A start is a date or "after id ...";
// an end is a date, a length (1.5d, 2w, 3h, 1M ...) or "until id ...".
package gantt

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// maxSpan bounds how far a chart may reach, so a hostile length cannot make
// the day-by-day exclusion walk or the axis run for ever.
const maxSpan = 200 * 366 * 24 * time.Hour

// Task is one bar (or milestone) with resolved start and end times.
type Task struct {
	ID      string
	Name    string
	Section int // index into Diagram.Sections, -1 before any section
	Line    int // source line, for error reporting

	Done, Active, Crit, Milestone, Vert bool

	startSpec, endSpec string
	manualEnd          bool // the end was given as a date

	Start time.Time
	End   time.Time
	// RenderEnd is where the bar is drawn to: End less the excluded days
	// at its tail, as Mermaid draws it.
	RenderEnd time.Time
	resolved  bool
}

// Diagram is a parsed gantt chart.
type Diagram struct {
	Title        string
	DateFormat   string // Mermaid (dayjs) date format
	AxisFormat   string // d3 strftime format, empty for the default
	TickInterval string // e.g. "1week", empty for automatic
	TopAxis      bool
	Inclusive    bool // inclusiveEndDates
	Sections     []string
	Tasks        []*Task

	excl, incl    dayRules
	weekendFriday bool
	weekStartsOn  time.Weekday
	excludesSrc   string
	includesSrc   string
}

type dayRules struct {
	weekends bool
	days     map[time.Weekday]bool
	dates    map[string]bool // YYYY-MM-DD
}

func (r dayRules) empty() bool { return !r.weekends && len(r.days) == 0 && len(r.dates) == 0 }

var keywordRe = regexp.MustCompile(`^(?i)(title|dateformat|axisformat|tickinterval|excludes|includes|todaymarker|weekday|weekend|section|displaymode|click|call)(\s|$)`)

// Parse builds a Diagram from gantt source and resolves task dates.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{DateFormat: "YYYY-MM-DD"}
	section := -1
	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			if firstWord(line) != "gantt" {
				return nil, syntax.Errorf(lineNo, 1, "expected 'gantt' header")
			}
			headerSeen = true
			continue
		}
		low := strings.ToLower(line)
		switch {
		case low == "topaxis":
			d.TopAxis = true
			continue
		case low == "inclusiveenddates":
			d.Inclusive = true
			continue
		case strings.HasPrefix(low, "acctitle"), strings.HasPrefix(low, "accdescr"):
			continue
		}
		if m := keywordRe.FindStringSubmatch(line); m != nil {
			arg := strings.TrimSpace(line[len(m[1]):])
			switch strings.ToLower(m[1]) {
			case "title":
				d.Title = arg
			case "dateformat":
				if arg != "" {
					d.DateFormat = arg
				}
			case "axisformat":
				d.AxisFormat = arg
			case "tickinterval":
				d.TickInterval = strings.ToLower(arg)
			case "excludes":
				d.excludesSrc = arg
			case "includes":
				d.includesSrc = arg
			case "weekday":
				if wd, ok := weekdays[strings.ToLower(arg)]; ok {
					d.weekStartsOn = wd
				}
			case "weekend":
				d.weekendFriday = strings.EqualFold(arg, "friday")
			case "section":
				d.Sections = append(d.Sections, arg)
				section = len(d.Sections) - 1
			}
			continue
		}
		t, err := parseTask(line, section, lineNo)
		if err != nil {
			return nil, err
		}
		d.Tasks = append(d.Tasks, t)
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'gantt' header")
	}
	d.excl = parseDayRules(d, d.excludesSrc)
	d.incl = parseDayRules(d, d.includesSrc)
	if err := d.resolve(); err != nil {
		return nil, err
	}
	return d, nil
}

func parseTask(line string, section, lineNo int) (*Task, error) {
	name, rest, ok := strings.Cut(line, ":")
	if !ok {
		return nil, syntax.Errorf(lineNo, 1, "expected 'name: data'")
	}
	t := &Task{Name: strings.TrimSpace(name), Section: section, Line: lineNo}
	var fields []string
	for _, f := range strings.Split(rest, ",") {
		f = strings.TrimSpace(f)
		switch strings.ToLower(f) {
		case "":
		case "done":
			t.Done = true
		case "active":
			t.Active = true
		case "crit":
			t.Crit = true
		case "milestone":
			t.Milestone = true
		case "vert":
			t.Vert = true
		default:
			fields = append(fields, f)
		}
	}
	switch len(fields) {
	case 0:
		return nil, syntax.Errorf(lineNo, 1, "task %q has no start or end", t.Name)
	case 1:
		t.endSpec = fields[0]
	case 2:
		// Mermaid reads two fields as start and end. Writers also give an
		// id and a length ("a1, 3d"); that one starts where the previous
		// task ended.
		if isStartSpec(fields[0]) || !isEndSpec(fields[1]) {
			t.startSpec, t.endSpec = fields[0], fields[1]
		} else {
			t.ID, t.endSpec = fields[0], fields[1]
		}
	default:
		t.ID, t.startSpec, t.endSpec = fields[0], fields[1], fields[2]
	}
	return t, nil
}

func isStartSpec(f string) bool {
	return strings.HasPrefix(strings.ToLower(f), "after ") || looksLikeDate(f)
}

func isEndSpec(f string) bool {
	_, ok := parseDuration(f)
	return ok || strings.HasPrefix(strings.ToLower(f), "until ") || looksLikeDate(f)
}

func looksLikeDate(f string) bool {
	digits := 0
	for _, r := range f {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 4 && (strings.ContainsAny(f, "-/.: ") || digits == len(f))
}

// resolve computes every task's start and end. A task may refer to tasks
// written after it, so references resolve to a fixpoint; unknown or
// circular references, bad dates and absurd spans are errors rather than
// silently misplaced bars.
func (d *Diagram) resolve() error {
	byID := map[string]*Task{}
	for _, t := range d.Tasks {
		if t.ID != "" {
			byID[t.ID] = t
		}
	}
	for {
		progress := false
		var pending *Task
		for i, t := range d.Tasks {
			if t.resolved {
				continue
			}
			ok, err := d.resolveTask(t, i, byID)
			if err != nil {
				return err
			}
			if ok {
				progress = true
			} else if pending == nil {
				pending = t
			}
		}
		if pending == nil {
			break
		}
		if !progress {
			return syntax.Errorf(pending.Line, 1, "task %q depends on tasks that never resolve", pending.Name)
		}
	}
	lo, hi := d.Bounds()
	if span := hi.Sub(lo); span > maxSpan || span < 0 {
		return syntax.Errorf(1, 1, "chart spans too long a time")
	}
	return nil
}

// refs resolves the tasks named after "after" or "until" and reports
// whether all of them that exist are resolved, and whether any exists.
func refs(spec string, byID map[string]*Task) (deps []*Task, ready bool) {
	for _, id := range strings.Fields(spec) {
		if dep, ok := byID[id]; ok {
			if !dep.resolved {
				return nil, false
			}
			deps = append(deps, dep)
		}
	}
	return deps, true
}

// resolveTask resolves t when every task it depends on is resolved and
// reports whether it did.
func (d *Diagram) resolveTask(t *Task, idx int, byID map[string]*Task) (bool, error) {
	var start time.Time
	lowStart := strings.ToLower(t.startSpec)
	switch {
	case t.startSpec == "":
		if idx == 0 {
			return false, syntax.Errorf(t.Line, 1, "task %q has no start date", t.Name)
		}
		prev := d.Tasks[idx-1]
		if !prev.resolved {
			return false, nil
		}
		start = prev.End
	case strings.HasPrefix(lowStart, "after "):
		deps, ready := refs(t.startSpec[len("after "):], byID)
		if !ready {
			return false, nil
		}
		if len(deps) == 0 {
			return false, syntax.Errorf(t.Line, 1, "task %q starts after an unknown task", t.Name)
		}
		for i, dep := range deps {
			if i == 0 || dep.End.After(start) {
				start = dep.End
			}
		}
	default:
		v, ok := parseDate(t.startSpec, d.DateFormat)
		if !ok {
			return false, syntax.Errorf(t.Line, 1, "task %q has invalid start date %q", t.Name, t.startSpec)
		}
		start = v
	}

	var end time.Time
	if strings.HasPrefix(strings.ToLower(t.endSpec), "until ") {
		deps, ready := refs(t.endSpec[len("until "):], byID)
		if !ready {
			return false, nil
		}
		if len(deps) == 0 {
			return false, syntax.Errorf(t.Line, 1, "task %q runs until an unknown task", t.Name)
		}
		for i, dep := range deps {
			if i == 0 || dep.Start.Before(end) {
				end = dep.Start
			}
		}
	} else if dur, ok := parseDuration(t.endSpec); ok {
		e, ok := dur.addTo(start)
		if !ok {
			return false, syntax.Errorf(t.Line, 1, "task %q is too long", t.Name)
		}
		end = e
	} else if v, ok := parseDate(t.endSpec, d.DateFormat); ok {
		end = v
		if d.Inclusive {
			end = end.AddDate(0, 0, 1)
		}
		t.manualEnd = true
	} else {
		return false, syntax.Errorf(t.Line, 1, "task %q has no valid end or length (%q)", t.Name, t.endSpec)
	}
	if end.Before(start) {
		return false, syntax.Errorf(t.Line, 1, "task %q ends before it starts", t.Name)
	}
	if end.Sub(start) > maxSpan {
		return false, syntax.Errorf(t.Line, 1, "task %q is too long", t.Name)
	}
	t.Start, t.End, t.RenderEnd = start, end, end
	if !t.manualEnd && !d.excl.empty() {
		t.End, t.RenderEnd = d.skipExcluded(start, end)
	}
	t.resolved = true
	return true, nil
}

// skipExcluded pushes end one day later for every excluded day the task
// covers, as Mermaid does, and returns the new end and the end to draw
// (which leaves out excluded days at the tail).
func (d *Diagram) skipExcluded(start, end time.Time) (time.Time, time.Time) {
	renderEnd := end
	invalid := false
	for steps := 0; !start.After(end) && steps < 200*366; steps++ {
		if !invalid {
			renderEnd = end
		}
		invalid = d.Excluded(start)
		if invalid {
			end = end.AddDate(0, 0, 1)
		}
		start = start.AddDate(0, 0, 1)
	}
	return end, renderEnd
}

// HasExcludes reports whether the chart leaves any days out.
func (d *Diagram) HasExcludes() bool { return !d.excl.empty() }

// Excluded reports whether the day of t is left out of working time.
func (d *Diagram) Excluded(t time.Time) bool {
	if d.incl.matches(d, t) {
		return false
	}
	return d.excl.matches(d, t)
}

func (r dayRules) matches(d *Diagram, t time.Time) bool {
	wd := t.Weekday()
	if r.weekends {
		if d.weekendFriday {
			if wd == time.Friday || wd == time.Saturday {
				return true
			}
		} else if wd == time.Saturday || wd == time.Sunday {
			return true
		}
	}
	return r.days[wd] || r.dates[t.Format("2006-01-02")]
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday,
	"thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
}

func parseDayRules(d *Diagram, s string) dayRules {
	r := dayRules{days: map[time.Weekday]bool{}, dates: map[string]bool{}}
	for _, f := range strings.FieldsFunc(s, func(c rune) bool { return c == ',' || c == ' ' || c == '\t' }) {
		l := strings.ToLower(f)
		if l == "weekends" {
			r.weekends = true
		} else if wd, ok := weekdays[l]; ok {
			r.days[wd] = true
		} else if v, ok := parseDate(f, d.DateFormat); ok {
			r.dates[v.Format("2006-01-02")] = true
		}
	}
	return r
}

// Bounds returns the earliest start and the latest end across tasks.
func (d *Diagram) Bounds() (lo, hi time.Time) {
	first := true
	for _, t := range d.Tasks {
		if !t.resolved {
			continue
		}
		if first {
			lo, hi, first = t.Start, t.End, false
			continue
		}
		if t.Start.Before(lo) {
			lo = t.Start
		}
		if t.End.After(hi) {
			hi = t.End
		}
	}
	return lo, hi
}

// duration is a Mermaid task length: a number and a unit.
type duration struct {
	n    float64
	unit string
}

var durationRe = regexp.MustCompile(`^(\d+(?:\.\d+)?)(ms|[Mdhmswy])$`)

func parseDuration(f string) (duration, bool) {
	m := durationRe.FindStringSubmatch(strings.TrimSpace(f))
	if m == nil {
		return duration{}, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil || n > 1e9 {
		return duration{}, false
	}
	return duration{n, m[2]}, true
}

func (u duration) addTo(t time.Time) (time.Time, bool) {
	clock := func(unit time.Duration) (time.Time, bool) {
		v := u.n * float64(unit)
		if v > float64(maxSpan) {
			return t, false
		}
		return t.Add(time.Duration(v)), true
	}
	switch u.unit {
	case "ms":
		return clock(time.Millisecond)
	case "s":
		return clock(time.Second)
	case "m":
		return clock(time.Minute)
	case "h":
		return clock(time.Hour)
	case "d", "w":
		days := u.n
		if u.unit == "w" {
			days *= 7
		}
		if days > 200*366 {
			return t, false
		}
		i, frac := math.Modf(days)
		return t.AddDate(0, 0, int(i)).Add(time.Duration(frac * float64(24*time.Hour))), true
	case "M":
		if u.n > 200*12 {
			return t, false
		}
		return t.AddDate(0, int(u.n), 0), true
	default: // y
		if u.n > 200 {
			return t, false
		}
		return t.AddDate(int(u.n), 0, 0), true
	}
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
