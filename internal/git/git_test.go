package git

import (
	"strings"
	"testing"
)

func TestParseBranchesAndMerges(t *testing.T) {
	d, err := Parse("gitGraph\ncommit\ncommit id: \"a\"\nbranch develop\ncommit\ncheckout main\nmerge develop tag: \"v1\" type: REVERSE")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Branches) != 2 || d.Branches[1].Name != "develop" {
		t.Fatalf("branches: %+v", d.Branches)
	}
	if len(d.Commits) != 4 {
		t.Fatalf("got %d commits", len(d.Commits))
	}
	c := d.Commits
	if c[1].ID != "a" || !c[1].CustomID || c[1].Parents[0] != c[0] {
		t.Errorf("commit a: %+v", c[1])
	}
	if c[2].Branch != "develop" || c[2].Parents[0] != c[1] {
		t.Errorf("develop's first commit must branch from a: %+v", c[2])
	}
	m := c[3]
	if m.Type != Reverse || len(m.Parents) != 2 || m.Parents[0] != c[1] || m.Parents[1] != c[2] || m.Tags[0] != "v1" {
		t.Errorf("merge: %+v", m)
	}
	if !strings.HasPrefix(c[0].ID, "0-") || c[0].CustomID {
		t.Errorf("generated id: %q", c[0].ID)
	}
}

func TestParseOptions(t *testing.T) {
	d, err := Parse("gitGraph TB:\ncommit id: \"x\" tag: \"v1\" tag: \"v2\" type: HIGHLIGHT\nbranch d order: 2\nbranch e order: 1\ncommit\ncheckout main\ncherry-pick id: \"x\"")
	if err != nil {
		t.Fatal(err)
	}
	if d.Direction != "TB" {
		t.Errorf("direction %q", d.Direction)
	}
	if d.branch("d").Lane != 2 || d.branch("e").Lane != 1 {
		t.Errorf("order: d=%d e=%d", d.branch("d").Lane, d.branch("e").Lane)
	}
	x := d.Commits[0]
	if x.Type != Highlight || len(x.Tags) != 2 {
		t.Errorf("x: %+v", x)
	}
	cp := d.Commits[2]
	if cp.Type != CherryPick || cp.Tags[0] != "cherry-pick:x" || cp.Parents[1] != x {
		t.Errorf("cherry-pick: %+v", cp)
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{
		"commit",
		"gitGraph\ncheckout nowhere",
		"gitGraph\nmerge nothing",
		"gitGraph\ncommit\nmerge main",
		"gitGraph\ncommit\nbranch b\ncheckout main\nmerge b",
		"gitGraph\ncommit id: \"a\"\ncommit id: \"a\"",
		"gitGraph\ncommit type: SIDEWAYS",
		"gitGraph\ncherry-pick id: \"none\"",
		"gitGraph\nbranch main",
	} {
		if _, err := Parse(src); err == nil {
			t.Errorf("expected an error for %q", src)
		}
	}
	if _, err := Parse("gitGraph:\ncommit"); err != nil {
		t.Errorf("trailing colon: %v", err)
	}
}
