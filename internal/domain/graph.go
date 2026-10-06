// Package domain holds the pure diagram model: the types every other
// stage produces or consumes. It has no I/O and no third-party
// dependencies, so it can be reasoned about and tested in isolation.
package domain

import "strings"

// Direction is the flow direction of a flowchart.
type Direction string

// ParseDirection reads a direction as a source writes it: TB, TD, BT, LR
// or RL, in any case.
func ParseDirection(s string) (Direction, bool) {
	switch strings.ToUpper(s) {
	case "TD", "TB":
		return TopBottom, true
	case "BT":
		return BottomTop, true
	case "LR":
		return LeftRight, true
	case "RL":
		return RightLeft, true
	}
	return "", false
}

// DirectionOf is ParseDirection, with TopBottom for anything it does not
// read.
func DirectionOf(s string) Direction {
	if d, ok := ParseDirection(s); ok {
		return d
	}
	return TopBottom
}

const (
	// TopBottom lays ranks out top to bottom (graph TD / graph TB).
	TopBottom Direction = "TB"
	// BottomTop lays ranks out bottom to top (graph BT).
	BottomTop Direction = "BT"
	// LeftRight lays ranks out left to right (graph LR).
	LeftRight Direction = "LR"
	// RightLeft lays ranks out right to left (graph RL).
	RightLeft Direction = "RL"
)

// Subgraph groups a set of nodes under an optional title. The renderer draws
// a cluster box around the members' bounding region.
type Subgraph struct {
	ID      string
	Title   string
	NodeIDs []string

	// Parent is the ID of the enclosing subgraph, empty at the top level.
	Parent string
	// Direction is the flow inside the subgraph when the source sets one
	// with "direction"; empty means the enclosing direction.
	Direction Direction
	// Style holds optional overrides from style/class lines; nil means use
	// the theme.
	Style *Style

	// Box is the laid-out cluster box and TitleLines the title as drawn.
	// Set by layouts that place subgraphs; zero otherwise.
	Box        Rect
	TitleLines []string
	// TitleX is the centre of the title across the box, when a layout has
	// moved it off the middle to keep it clear of the edges entering the
	// box; zero means the middle.
	TitleX float64
}

// Graph is a parsed flowchart, independent of layout or rendering.
// Coordinates are not set until the layout stage populates them.
type Graph struct {
	Direction Direction
	Nodes     []*Node
	Edges     []*Edge
	Subgraphs []*Subgraph
}

// SubgraphByID returns the subgraph with the given id, or nil if absent.
func (g *Graph) SubgraphByID(id string) *Subgraph {
	for _, sg := range g.Subgraphs {
		if sg.ID == id {
			return sg
		}
	}
	return nil
}

// NodeByID returns the node with the given id, or nil if absent.
func (g *Graph) NodeByID(id string) *Node {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}
