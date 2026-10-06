// Package theme holds the color palettes shared by all diagram renderers.
// It is a dependency-free leaf so every render stage can use one source of
// truth for colors.
package theme

import "sync"

// Palette holds the colors used to render a diagram.
type Palette struct {
	Background string
	NodeFill   string
	NodeStroke string
	Text       string
	Edge       string

	// ClusterFill and ClusterStroke color a flowchart subgraph's box, and
	// LabelBackground the box behind an edge label. Empty values fall back
	// to colors derived from the ones above (see Flow).
	ClusterFill     string
	ClusterStroke   string
	LabelBackground string
}

// Over returns p with every non-empty color of o put over it.
func (p Palette) Over(o Palette) Palette {
	set := func(d *string, s string) {
		if s != "" {
			*d = s
		}
	}
	set(&p.Background, o.Background)
	set(&p.NodeFill, o.NodeFill)
	set(&p.NodeStroke, o.NodeStroke)
	set(&p.Text, o.Text)
	set(&p.Edge, o.Edge)
	set(&p.ClusterFill, o.ClusterFill)
	set(&p.ClusterStroke, o.ClusterStroke)
	set(&p.LabelBackground, o.LabelBackground)
	return p
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

// palettes maps theme names to palettes. Unknown names fall back to default.
var palettes = map[string]Palette{
	"default": {
		Background:      "#ffffff",
		NodeFill:        "#ECECFF",
		NodeStroke:      "#9370DB",
		Text:            "#333333",
		Edge:            "#333333",
		ClusterFill:     "#ffffde",
		ClusterStroke:   "#aaaa33",
		LabelBackground: "#e8e8e8",
	},
	"dark": {
		Background:      "#1e1e1e",
		NodeFill:        "#2b2b40",
		NodeStroke:      "#8888bb",
		Text:            "#e6e6e6",
		Edge:            "#bbbbbb",
		ClusterFill:     "#2a2a33",
		ClusterStroke:   "#77775a",
		LabelBackground: "#3a3a3a",
	},
	"neutral": {
		Background:      "#ffffff",
		NodeFill:        "#eeeeee",
		NodeStroke:      "#999999",
		Text:            "#222222",
		Edge:            "#555555",
		ClusterFill:     "#f4f4f4",
		ClusterStroke:   "#999999",
		LabelBackground: "#ececec",
	},
	"forest": {
		Background:      "#ffffff",
		NodeFill:        "#cde498",
		NodeStroke:      "#13540c",
		Text:            "#13540c",
		Edge:            "#3a7a2a",
		ClusterFill:     "#f2f9e8",
		ClusterStroke:   "#6eaa49",
		LabelBackground: "#e8f2df",
	},
	"base": {
		Background:      "#ffffff",
		NodeFill:        "#e8e8e8",
		NodeStroke:      "#666666",
		Text:            "#1a1a1a",
		Edge:            "#444444",
		ClusterFill:     "#f4f4f4",
		ClusterStroke:   "#888888",
		LabelBackground: "#ececec",
	},
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
// built-in, otherwise the default palette.
func For(name string) Palette {
	mu.RLock()
	p, ok := custom[name]
	mu.RUnlock()
	if ok {
		return p
	}
	if p, ok := palettes[name]; ok {
		return p
	}
	return palettes["default"]
}
