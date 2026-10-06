package domain

// Arrow is the head/line style of an edge.
type Arrow string

const (
	// ArrowNormal is a solid line with an arrowhead (-->).
	ArrowNormal Arrow = "normal"
	// ArrowOpen is a solid line with no arrowhead (---).
	ArrowOpen Arrow = "open"
	// ArrowDotted is a dotted line with an arrowhead (-.->).
	ArrowDotted Arrow = "dotted"
	// ArrowThick is a thick line with an arrowhead (==>).
	ArrowThick Arrow = "thick"
)

// Marker is what an edge end is drawn with.
type Marker string

const (
	// MarkerNone draws the line end plain.
	MarkerNone Marker = ""
	// MarkerArrow is a filled arrowhead (--> or <--).
	MarkerArrow Marker = "arrow"
	// MarkerCircle is a filled circle (--o or o--).
	MarkerCircle Marker = "circle"
	// MarkerCross is a cross (--x or x--).
	MarkerCross Marker = "cross"
)

// Line is the stroke of an edge.
type Line string

const (
	// LineSolid is a plain line (--).
	LineSolid Line = ""
	// LineDotted is a dotted line (-.-).
	LineDotted Line = "dotted"
	// LineThick is a heavy line (==).
	LineThick Line = "thick"
	// LineInvisible takes part in layout but is not drawn (~~~).
	LineInvisible Line = "invisible"
)

// Edge connects two nodes by ID.
type Edge struct {
	From  string
	To    string
	Label string
	Arrow Arrow

	// Start and End are the markers drawn at each end. Renderers that only
	// know Arrow may ignore them; the flowchart parser sets both.
	Start, End Marker
	// Line is the stroke kind.
	Line Line
	// MinLen is the least number of ranks the edge spans, from the length of
	// the link in the source (---> spans two). Zero means one.
	MinLen int

	// Style holds optional per-edge overrides from a linkStyle directive.
	// nil means use the theme and the arrow kind.
	Style *Style

	// Points is the laid-out polyline from source to target, including
	// any bend points. Empty until layout runs.
	Points []Point

	// LabelPos is the anchor for the edge label on the routed path. Layout
	// sets it to the path midpoint, or to a staggered position when several
	// edges join the same node pair and their labels would otherwise overlap.
	LabelPos Point

	// LabelLines is the label broken into the lines it is drawn on, and
	// LabelSize the box they need. Set by layouts that wrap labels; empty
	// otherwise.
	LabelLines []string
	LabelSize  Size
	// LabelCenter is true when LabelPos is the centre of the label box rather
	// than the baseline of its last line.
	LabelCenter bool
}
