// Package git parses and renders Mermaid gitGraph diagrams to SVG.
//
// Syntax:
//
//	gitGraph
//	   commit
//	   commit id: "feat" tag: "v1" type: HIGHLIGHT
//	   branch develop order: 2
//	   checkout develop
//	   commit type: REVERSE
//	   checkout main
//	   merge develop tag: "v1.1"
//	   cherry-pick id: "feat"
//
// The header may name a direction: gitGraph LR:, gitGraph TB: or
// gitGraph BT:.
package git

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/syntax"
)

// CommitType is how a commit is drawn.
type CommitType int

// Commit types, as Mermaid's type: NORMAL, REVERSE and HIGHLIGHT, plus the
// kinds a statement makes.
const (
	Normal CommitType = iota
	Reverse
	Highlight
	MergeCommit
	CherryPick
)

// Commit is one commit. Seq is its position along the time axis.
type Commit struct {
	Seq      int
	ID       string
	CustomID bool
	Branch   string
	Tags     []string
	Type     CommitType
	Parents  []*Commit
}

// Branch is a lane in the graph.
type Branch struct {
	Name  string
	Order float64
	Lane  int
	head  *Commit
}

// Diagram is a parsed gitGraph.
type Diagram struct {
	Direction string // LR, TB or BT
	Branches  []*Branch
	Commits   []*Commit
}

// branch returns the named branch, or nil.
func (d *Diagram) branch(name string) *Branch {
	for _, b := range d.Branches {
		if b.Name == name {
			return b
		}
	}
	return nil
}

const maxCommits = 10000

var (
	headerRe = regexp.MustCompile(`(?i)^gitgraph\s*(LR|TB|BT)?\s*:?$`)
	attrRe   = regexp.MustCompile(`^\s*(id|tag|type|msg|order|parent)\s*:\s*("([^"]*)"|[^\s"]+)`)
)

// Parse builds a Diagram from gitGraph source.
func Parse(src string) (*Diagram, error) {
	d := &Diagram{Direction: "LR"}
	d.Branches = append(d.Branches, &Branch{Name: "main"})
	cur := d.Branches[0]
	ids := map[string]*Commit{}

	headerSeen := false
	for i, raw := range strings.Split(src, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if !headerSeen {
			m := headerRe.FindStringSubmatch(line)
			if m == nil {
				return nil, syntax.Errorf(lineNo, 1, "expected 'gitGraph' header")
			}
			if m[1] != "" {
				d.Direction = strings.ToUpper(m[1])
			}
			headerSeen = true
			continue
		}
		kw := firstWord(line)
		rest := strings.TrimSpace(line[len(kw):])
		if len(d.Commits) >= maxCommits {
			return nil, syntax.Errorf(lineNo, 1, "too many commits")
		}
		newCommit := func(a attrs, typ CommitType, parents ...*Commit) (*Commit, error) {
			c := &Commit{Seq: len(d.Commits), Branch: cur.Name, Tags: a.tags, Type: typ}
			for _, p := range parents {
				if p != nil {
					c.Parents = append(c.Parents, p)
				}
			}
			if a.typ != "" && typ == Normal || a.typ != "" && typ == MergeCommit {
				switch strings.ToUpper(a.typ) {
				case "REVERSE":
					c.Type = Reverse
				case "HIGHLIGHT":
					c.Type = Highlight
				case "NORMAL":
				default:
					return nil, syntax.Errorf(lineNo, 1, "unknown commit type %q", a.typ)
				}
			}
			if a.id != "" {
				if ids[a.id] != nil {
					return nil, syntax.Errorf(lineNo, 1, "commit id %q is already used", a.id)
				}
				c.ID, c.CustomID = a.id, true
			} else {
				c.ID = autoID(src, c.Seq)
			}
			ids[c.ID] = c
			d.Commits = append(d.Commits, c)
			cur.head = c
			return c, nil
		}
		switch kw {
		case "commit":
			a, err := parseAttrs(rest, lineNo)
			if err != nil {
				return nil, err
			}
			if _, err := newCommit(a, Normal, cur.head); err != nil {
				return nil, err
			}
		case "branch":
			name, after := branchName(rest)
			if name == "" {
				return nil, syntax.Errorf(lineNo, 1, "branch needs a name")
			}
			if d.branch(name) != nil {
				return nil, syntax.Errorf(lineNo, 1, "branch %q already exists", name)
			}
			a, err := parseAttrs(after, lineNo)
			if err != nil {
				return nil, err
			}
			b := &Branch{Name: name, Order: float64(len(d.Branches)), head: cur.head}
			if a.order != "" {
				if v, err := strconv.ParseFloat(a.order, 64); err == nil && v == v {
					b.Order = v
				}
			}
			d.Branches = append(d.Branches, b)
			cur = b
		case "checkout", "switch":
			name, _ := branchName(rest)
			b := d.branch(name)
			if b == nil {
				return nil, syntax.Errorf(lineNo, 1, "cannot check out %q: no such branch", name)
			}
			cur = b
		case "merge":
			name, after := branchName(rest)
			from := d.branch(name)
			switch {
			case from == nil:
				return nil, syntax.Errorf(lineNo, 1, "cannot merge %q: no such branch", name)
			case from == cur:
				return nil, syntax.Errorf(lineNo, 1, "cannot merge a branch into itself")
			case from.head == nil:
				return nil, syntax.Errorf(lineNo, 1, "cannot merge %q: it has no commits", name)
			case from.head == cur.head:
				return nil, syntax.Errorf(lineNo, 1, "cannot merge %q: nothing new on it", name)
			}
			a, err := parseAttrs(after, lineNo)
			if err != nil {
				return nil, err
			}
			if _, err := newCommit(a, MergeCommit, cur.head, from.head); err != nil {
				return nil, err
			}
		case "cherry-pick":
			a, err := parseAttrs(rest, lineNo)
			if err != nil {
				return nil, err
			}
			srcC := ids[a.id]
			if srcC == nil {
				return nil, syntax.Errorf(lineNo, 1, "cherry-pick: no commit with id %q", a.id)
			}
			if !a.tagSet {
				a.tags = []string{"cherry-pick:" + a.id}
			}
			a.id, a.typ = "", ""
			if _, err := newCommit(a, CherryPick, cur.head, srcC); err != nil {
				return nil, err
			}
		default:
			return nil, syntax.Errorf(lineNo, 1, "unrecognized statement %q", line)
		}
	}
	if !headerSeen {
		return nil, syntax.Errorf(1, 1, "expected 'gitGraph' header")
	}
	order := append([]*Branch(nil), d.Branches...)
	sort.SliceStable(order, func(i, j int) bool { return order[i].Order < order[j].Order })
	for i, b := range order {
		b.Lane = i
	}
	return d, nil
}

type attrs struct {
	id, typ, order string
	tags           []string
	tagSet         bool
}

// parseAttrs reads `id: "x" tag: "t" type: HIGHLIGHT order: 2 …` in any
// order.
func parseAttrs(s string, lineNo int) (attrs, error) {
	var a attrs
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			return a, nil
		}
		m := attrRe.FindStringSubmatch(s)
		if m == nil {
			return a, syntax.Errorf(lineNo, 1, "unexpected %q", clip(s))
		}
		v := m[2]
		if strings.HasPrefix(v, `"`) {
			v = m[3]
		}
		switch m[1] {
		case "id":
			a.id = v
		case "tag":
			a.tags = append(a.tags, v)
			a.tagSet = true
		case "type":
			a.typ = v
		case "order":
			a.order = v
		}
		s = s[len(m[0]):]
	}
}

// branchName reads a branch name (bare or quoted) and returns the rest.
func branchName(s string) (name, rest string) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, `"`) {
		if e := strings.IndexByte(s[1:], '"'); e >= 0 {
			return s[1 : e+1], s[e+2:]
		}
	}
	name = firstWord(s)
	return name, s[len(name):]
}

// autoID makes the id Mermaid would give a commit without one, "3-a1b2c3d",
// from a hash of the source so the output stays deterministic.
func autoID(src string, seq int) string {
	h := fnv.New32a()
	_, _ = fmt.Fprintf(h, "%d\x00%s", seq, src)
	return fmt.Sprintf("%d-%07x", seq, h.Sum32()&0xfffffff)
}

func clip(s string) string {
	r := []rune(s)
	if len(r) > 20 {
		return string(r[:20]) + "…"
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
