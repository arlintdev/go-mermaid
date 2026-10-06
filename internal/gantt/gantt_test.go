package gantt

import (
	"strings"
	"testing"
	"time"

	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

func day(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func days(t *Task) float64 { return t.End.Sub(t.Start).Hours() / 24 }

func mustParse(t *testing.T, src string) *Diagram {
	t.Helper()
	d, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return d
}

func TestParseBasics(t *testing.T) {
	d := mustParse(t, "gantt\ntitle P\ndateFormat YYYY-MM-DD\nsection Phase\nDesign : d1, 2024-01-01, 10d\nBuild : after d1, 2w")
	if d.Title != "P" || len(d.Tasks) != 2 || len(d.Sections) != 1 {
		t.Fatalf("got %+v", d)
	}
	if days(d.Tasks[0]) != 10 || days(d.Tasks[1]) != 14 {
		t.Errorf("lengths %v %v", days(d.Tasks[0]), days(d.Tasks[1]))
	}
	if !d.Tasks[1].Start.Equal(d.Tasks[0].End) {
		t.Error("after does not chain to the dependency's end")
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{
		"title X",
		"gantt\nsection S\nBad : 2024-01-01",            // a start and no end
		"gantt\nA : 5d",                                 // nothing to start from
		"gantt\nT : 2024-01-05, 2024-01-01",             // ends before it starts
		"gantt\nT : a, after b, 1d\nU : b, after a, 1d", // circular
		"gantt\nT : after nobody, 1d",
		"gantt\nT : 2024-01-01, 99999999y",
		"gantt\nT : 2024-01-01, 900000d",
		"gantt\nT : nonsense, 1d",
	} {
		if _, err := Parse(src); err == nil {
			t.Errorf("no error for %q", src)
		}
	}
}

func TestForwardAfterDependency(t *testing.T) {
	d := mustParse(t, "gantt\nTask A : a, after b, 5d\nTask B : b, 2024-01-01, 3d")
	if !d.Tasks[0].Start.Equal(d.Tasks[1].End) {
		t.Error("forward dependency not resolved")
	}
}

func TestTaskForms(t *testing.T) {
	d := mustParse(t, `gantt
A : a, 2024-01-01, 2d
B : 3d
C : c, 1d
D : after a c, 1d
E : 2024-01-02, until c
F : done, crit, f, 2024-01-01, 2024-01-05
G : milestone, 2024-01-04, 0d
H : 2024-01-01, 12h
I : 2024-01-01, 1M`)
	want := []struct{ start, end string }{
		{"2024-01-01", "2024-01-03"}, {"2024-01-03", "2024-01-06"}, {"2024-01-06", "2024-01-07"},
		{"2024-01-07", "2024-01-08"}, {"2024-01-02", "2024-01-06"}, {"2024-01-01", "2024-01-05"},
		{"2024-01-04", "2024-01-04"}, {"2024-01-01", "2024-01-01"}, {"2024-01-01", "2024-02-01"},
	}
	for i, w := range want {
		tk := d.Tasks[i]
		if tk.Start.Format("2006-01-02") != w.start || tk.End.Format("2006-01-02") != w.end {
			t.Errorf("%s: %s..%s, want %s..%s", tk.Name, tk.Start, tk.End, w.start, w.end)
		}
	}
	if f := d.Tasks[5]; !f.Done || !f.Crit || f.ID != "f" {
		t.Errorf("tags: %+v", f)
	}
	if !d.Tasks[6].Milestone {
		t.Error("milestone tag lost")
	}
	if got := d.Tasks[7].End.Sub(d.Tasks[7].Start); got != 12*time.Hour {
		t.Errorf("12h task lasts %v", got)
	}
}

// Mermaid pushes a task's end past excluded days and starts a dependant
// after them; the bar is drawn to the last working day.
func TestExcludesWeekends(t *testing.T) {
	d := mustParse(t, `gantt
excludes weekends
R : r, 2026-09-01, 5d
M : m, after r, 4d
F : after m, 8d`)
	r, m, f := d.Tasks[0], d.Tasks[1], d.Tasks[2]
	if !r.End.Equal(day("2026-09-08")) || !r.RenderEnd.Equal(day("2026-09-08")) {
		t.Errorf("R ends %v / %v", r.End, r.RenderEnd)
	}
	if !m.End.Equal(day("2026-09-14")) || !m.RenderEnd.Equal(day("2026-09-12")) {
		t.Errorf("M ends %v, drawn to %v", m.End, m.RenderEnd)
	}
	if !f.Start.Equal(day("2026-09-14")) {
		t.Errorf("F starts %v", f.Start)
	}
}

func TestExcludesDatesAndIncludes(t *testing.T) {
	d := mustParse(t, "gantt\nexcludes 2026-01-02, sunday\nincludes 2026-01-04\nA : 2026-01-01, 3d")
	// Jan 2 is excluded; Jan 4 is a Sunday but included.
	if got := d.Tasks[0].End; !got.Equal(day("2026-01-05")) {
		t.Errorf("end %v", got)
	}
}

func TestDateFormats(t *testing.T) {
	d := mustParse(t, "gantt\ndateFormat DD/MM/YYYY HH:mm\nA : 05/02/2024 13:30, 2h")
	if got := d.Tasks[0].Start.Format(time.RFC3339); got != "2024-02-05T13:30:00Z" {
		t.Errorf("start %s", got)
	}
	d = mustParse(t, "gantt\ndateFormat X\nA : 86400, 1d")
	if !d.Tasks[0].Start.Equal(day("1970-01-02")) {
		t.Errorf("unix start %v", d.Tasks[0].Start)
	}
	// Like Mermaid, a date that does not match dateFormat is read as ISO.
	d = mustParse(t, "gantt\ndateFormat YYYY\nA : 2024-03-01, 1d")
	if !d.Tasks[0].Start.Equal(day("2024-03-01")) {
		t.Errorf("fallback start %v", d.Tasks[0].Start)
	}
}

func TestStrftime(t *testing.T) {
	ts := time.Date(2026, 9, 5, 14, 7, 0, 0, time.UTC)
	for f, want := range map[string]string{
		"%Y-%m-%d": "2026-09-05", "%b %d": "Sep 05", "%e %B": " 5 September", "%-d/%-m": "5/9",
		"%H:%M %p": "14:07 PM", "%a %j": "Sat 248", "100%%": "100%", "%y %I": "26 02",
	} {
		if got := strftime(f, ts); got != want {
			t.Errorf("%q: %q, want %q", f, got, want)
		}
	}
}

func TestTicksDoNotRepeat(t *testing.T) {
	out, err := Render("gantt\nA : 2026-01-01, 3d", RenderOptions{FontSize: 14, Padding: 16})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(out), ">2026-01-02<") != 1 {
		t.Errorf("a day label repeats or is missing:\n%s", out)
	}
}

func TestNoTodayMarker(t *testing.T) {
	a, _ := Render("gantt\ntodayMarker on\nA : 2026-01-01, 3d", RenderOptions{FontSize: 14})
	if strings.Contains(string(a), "#ff0000") || strings.Contains(string(a), "red") {
		t.Error("a today marker was drawn; output must not depend on the clock")
	}
}

func opts() RenderOptions {
	return RenderOptions{Theme: "default", FontFace: "sans-serif", FontSize: 14, Padding: 16}
}

func TestGolden(t *testing.T) {
	goldentest.Golden(t, "testdata", func(src string) ([]byte, error) { return Render(src, opts()) })
}

func TestHostile(t *testing.T) {
	inj := goldentest.Injection
	src := "gantt\ntitle " + inj + "\ndateFormat " + inj + "\naxisFormat %Y " + inj + "\nexcludes " + inj +
		"\ntickInterval " + inj + "\nsection " + inj + "\n" + inj + " : " + inj + ", 2026-01-01, 3d\n" +
		inj + " : milestone, 2026-01-02, 0d\n" + inj + " : vert, 2026-01-03, 0d\nx : after " + inj + " a, 1d"
	goldentest.Hostile(t, func(s string) ([]byte, error) { return Render(s, opts()) }, src)
	o := opts()
	o.Title = inj
	goldentest.Hostile(t, func(s string) ([]byte, error) { return Render(s, o) }, "gantt\nA : 2026-01-01, 3d")
}
