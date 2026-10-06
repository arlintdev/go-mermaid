package mermaid_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	mermaid "github.com/arlintdev/go-mermaid"
)

var (
	idAttrRe = regexp.MustCompile(`\sid="([^"]*)"`)
	urlRefRe = regexp.MustCompile(`url\(#([^)]*)\)`)
)

// TestIDPrefixEveryType draws every test diagram of every type twice, with
// the prefixes "p1" and "p2", as a page holding a light and a dark copy of
// one diagram would. The two pictures must share no id, every id must start
// with its picture's prefix, and every url(#…) must point at an id of its
// own picture: a browser resolves url(#x) to the first x in the document,
// so a shared id would let a hidden copy steal the visible copy's markers.
func TestIDPrefixEveryType(t *testing.T) {
	inputs, err := filepath.Glob(filepath.Join("testdata", "golden", "*.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob(filepath.Join("internal", "*", "testdata", "*.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	inputs = append(inputs, more...)
	if len(inputs) == 0 {
		t.Fatal("no test diagrams found")
	}
	withIDs := 0
	for _, in := range inputs {
		src, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		var sets [2]map[string]bool
		for i, prefix := range []string{"p1", "p2"} {
			out, err := mermaid.Render(string(src), mermaid.WithIDPrefix(prefix))
			if err != nil {
				t.Errorf("%s: %v", in, err)
				break
			}
			ids := map[string]bool{}
			for _, m := range idAttrRe.FindAllStringSubmatch(string(out), -1) {
				id := m[1]
				if ids[id] {
					t.Errorf("%s (%s): id %q appears twice", in, prefix, id)
				}
				ids[id] = true
				if !strings.HasPrefix(id, prefix+"-") {
					t.Errorf("%s (%s): id %q does not start with %q", in, prefix, id, prefix+"-")
				}
			}
			for _, m := range urlRefRe.FindAllStringSubmatch(string(out), -1) {
				if !ids[m[1]] {
					t.Errorf("%s (%s): url(#%s) has no target in its own picture", in, prefix, m[1])
				}
			}
			sets[i] = ids
		}
		for id := range sets[0] {
			if sets[1][id] {
				t.Errorf("%s: both pictures hold id %q", in, id)
			}
		}
		if len(sets[0]) > 0 {
			withIDs++
		}
	}
	// Flowchart, sequence, state, journey, timeline, C4, requirement and
	// block diagrams draw markers; if none had ids the check above proved
	// nothing.
	if withIDs < 8 {
		t.Errorf("only %d test diagrams emit ids", withIDs)
	}
}

// TestIDPrefixDefault checks that without the option the ids are derived
// from the source: the same source gives the same ids, and two different
// sources give different ones.
func TestIDPrefixDefault(t *testing.T) {
	ids := func(src string) []string {
		out, err := mermaid.Render(src)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, m := range idAttrRe.FindAllStringSubmatch(string(out), -1) {
			got = append(got, m[1])
		}
		return got
	}
	a, b := ids("sequenceDiagram\n  A->>B: hi"), ids("sequenceDiagram\n  A->>B: hello")
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("sequence diagram emitted no ids")
	}
	if strings.Join(a, " ") != strings.Join(ids("sequenceDiagram\n  A->>B: hi"), " ") {
		t.Error("ids differ between two renders of one source")
	}
	for _, x := range a {
		for _, y := range b {
			if x == y {
				t.Errorf("different sources share id %q", x)
			}
		}
	}
}
