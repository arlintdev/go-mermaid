// Package theme holds the color palettes shared by all diagram renderers.
// It is a leaf of the render stages, so every diagram type takes its colors
// from one source of truth, and a dark theme is one palette, not a branch in
// each renderer.
package theme

import (
	"reflect"
	"sync"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
)

// Palette holds the colors used to render a diagram. The first group is
// what every diagram type uses; the rest are roles that only some types
// have, grouped by type. A palette may leave any field empty: For fills it
// from the built-in palette that suits its background.
type Palette struct {
	Background string
	NodeFill   string
	NodeStroke string
	Text       string
	Edge       string

	// ClusterFill and ClusterStroke color a group's box (a flowchart
	// subgraph, a class namespace), and LabelBackground the box behind a
	// flowchart edge label.
	ClusterFill     string
	ClusterStroke   string
	LabelBackground string

	// NoteFill, NoteStroke and NoteText draw a note (sequence, class,
	// state).
	NoteFill, NoteStroke, NoteText string
	// RelationLabel is the box, drawn part transparent, behind the label of
	// a relationship line (class, ER, state, requirement, block).
	RelationLabel string

	Sequence SequenceColors
	Gantt    GanttColors
	Git      GitColors
	Journey  JourneyColors
	Mindmap  MindmapColors
	Pie      PieColors
	Timeline []TimelineColor
	XYChart  XYChartColors
	Kanban   KanbanColors
	Packet   PacketColors
	Radar    RadarColors
	Sankey   SankeyColors
	C4       C4Colors
}

// SequenceColors are a sequence diagram's own colors.
type SequenceColors struct {
	ActivationFill, ActivationStroke string
	// NumberText is the text of an autonumber badge, drawn on Edge.
	NumberText string
}

// GanttColors are a gantt chart's own colors: a fill, stroke and text
// color per task state, and the chart's grid.
type GanttColors struct {
	TaskFill, TaskStroke, TaskText string
	ActiveFill, ActiveText         string
	DoneFill, DoneStroke, DoneText string
	CritFill, CritStroke           string
	ExcludeFill, Grid, Marker      string
	Bands                          []Band // the alternating section rows
}

// Band is a translucent row background.
type Band struct {
	Fill    string
	Opacity float64
}

// GitColors are a gitGraph's own colors.
type GitColors struct {
	Lanes []GitLane
	// Mark draws the cross or dots inside a reverse or cherry-pick commit.
	Mark string
	// TagFill is the background of a tag label.
	TagFill string
}

// GitLane is one branch's color, the color of its name drawn on it, and
// the color of a highlighted commit on it.
type GitLane struct{ Fill, Label, Highlight string }

// JourneyColors are a user journey's own colors.
type JourneyColors struct {
	Sections    []string // one per section, for its title and task boxes
	Actors      []string // one per actor, for the dots
	ActorStroke string
	Stroke      string // box outlines and the dashed drop lines
	FaceFill    string
	FaceStroke  string
	FaceFeature string // eyes and mouth
}

// MindmapColors are a mindmap's own colors.
type MindmapColors struct {
	RootFill, RootText string
	Fills              []string // one per top-level branch
	Lines              []string // the underline of each branch's nodes
	Text               string
}

// PieColors are a pie chart's own colors.
type PieColors struct {
	Slices []string
	Stroke string
}

// TimelineColor is the look of one timeline section: the fill and bottom
// rule of its title and period boxes, the fill of its event boxes, and the
// text on all of them.
type TimelineColor struct{ Fill, Accent, Light, Text string }

// XYChartColors are an xychart's own colors.
type XYChartColors struct {
	Series []string
}

// KanbanColors are a kanban board's own colors.
type KanbanColors struct {
	Columns              []string // the column backgrounds, in turn
	ColumnText           string
	CardFill, CardStroke string
	// VeryHigh to VeryLow color a card's left edge by its priority.
	VeryHigh, High, Low, VeryLow string
}

// PacketColors are a packet diagram's own colors.
type PacketColors struct {
	Fill, Stroke, Text string
}

// RadarColors are a radar chart's own colors.
type RadarColors struct {
	Curves []string
	Grid   string // the graticule
	Axis   string
}

// SankeyColors are a sankey diagram's own colors.
type SankeyColors struct {
	Nodes []string
}

// C4Colors are a C4 diagram's own colors: one look per kind of element, and
// the color of relationship lines and boundaries.
type C4Colors struct {
	Person, ExternalPerson       C4Look
	System, ExternalSystem       C4Look
	Container, ExternalContainer C4Look
	Component, ExternalComponent C4Look
	Line                         string
}

// C4Look is the fill, stroke and text color of one kind of C4 element.
type C4Look struct{ Fill, Stroke, Text string }

// Over returns p with every non-empty color of o put over it.
func (p Palette) Over(o Palette) Palette {
	merge(reflect.ValueOf(&p).Elem(), reflect.ValueOf(o), false)
	return p
}

// complete returns p with every empty color taken from base.
func (p Palette) complete(base Palette) Palette {
	merge(reflect.ValueOf(&p).Elem(), reflect.ValueOf(base), true)
	return p
}

// merge copies src's non-empty strings and slices into dst, field by
// field; with onlyEmpty set it fills only the fields dst leaves empty.
func merge(dst, src reflect.Value, onlyEmpty bool) {
	for i := 0; i < dst.NumField(); i++ {
		d, s := dst.Field(i), src.Field(i)
		switch d.Kind() {
		case reflect.Struct:
			merge(d, s, onlyEmpty)
		case reflect.String:
			if s.String() != "" && (!onlyEmpty || d.String() == "") {
				d.Set(s)
			}
		case reflect.Slice:
			if s.Len() > 0 && (!onlyEmpty || d.Len() == 0) {
				d.Set(s)
			}
		}
	}
}

// Flow returns p with the flowchart-only colors filled in.
func (p Palette) Flow() Palette {
	if p.ClusterFill == "" {
		p.ClusterFill = p.Background
	}
	if p.ClusterStroke == "" {
		p.ClusterStroke = p.NodeStroke
	}
	if p.LabelBackground == "" {
		p.LabelBackground = p.Background
	}
	return p
}

// Names returns the available theme names in a stable order.
func Names() []string {
	return []string{"default", "dark", "neutral", "forest", "base"}
}

var (
	mu     sync.RWMutex
	custom = map[string]Palette{}
)

// Register adds or replaces a custom palette under name.
func Register(name string, p Palette) {
	mu.Lock()
	custom[name] = p
	mu.Unlock()
}

// For returns the palette for name: a registered custom palette, then a
// built-in, otherwise the default palette. The type-specific colors a
// palette leaves empty come from the dark palette when its background is
// dark, else from the default one.
func For(name string) Palette {
	mu.RLock()
	p, ok := custom[name]
	mu.RUnlock()
	if ok {
		// A custom palette's group colors follow its own background and
		// node colors, as they always have.
		p = p.Flow()
	} else if p, ok = palettes[name]; !ok {
		p = palettes["default"]
	}
	base := palettes["default"]
	if IsDark(p.Background) {
		base = palettes["dark"]
	}
	return p.complete(base)
}

// Escaped returns p with every color escaped for an SVG attribute value, so
// a renderer can write the fields as they are. A built-in color needs no
// escaping; a palette a caller registered might.
func (p Palette) Escaped() Palette {
	escape(reflect.ValueOf(&p).Elem())
	return p
}

func escape(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			escape(v.Field(i))
		}
	case reflect.String:
		v.SetString(svgutil.Esc(v.String()))
	case reflect.Slice:
		if v.Len() == 0 {
			return
		}
		c := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(c, v)
		for i := 0; i < c.Len(); i++ {
			escape(c.Index(i))
		}
		v.Set(c)
	}
}
