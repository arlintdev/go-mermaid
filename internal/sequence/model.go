// Package sequence parses, lays out, and renders Mermaid sequence diagrams
// to SVG. It is self-contained: source text in, SVG bytes out.
package sequence

// Head is the arrowhead style at the end of a message.
type Head int

const (
	// HeadNone is a line with no arrowhead (-> / -->).
	HeadNone Head = iota
	// HeadArrow is a solid triangular arrowhead (->> / -->>).
	HeadArrow
	// HeadOpen is an open arrowhead for an asynchronous message (-) / --)).
	HeadOpen
	// HeadCross is an X at the end, denoting a lost message (-x / --x).
	HeadCross
)

// Arrow describes a message line's style.
type Arrow struct {
	Dashed bool
	Head   Head
	// Both draws the head at the sender's end too (<<->> / <<-->>).
	Both bool
}

// Kind is how a participant is drawn.
type Kind int

const (
	// KindParticipant is drawn as a box.
	KindParticipant Kind = iota
	// KindActor is drawn as a stick figure with its name below.
	KindActor
)

// Participant is an actor with a vertical lifeline.
type Participant struct {
	ID    string
	Label string
	Kind  Kind
	Box   *Box

	// Created is set by `create participant`: the header is drawn where the
	// creating message lands instead of at the top.
	Created bool
	// Destroyed is set by `destroy`: the lifeline ends at the destroying
	// message and there is no header at the bottom.
	Destroyed bool

	// Layout results.
	X         float64  // lifeline centre
	Width     float64  // header width
	Lines     []string // label lines
	TopY      float64  // top of the (first) header
	LifeStart float64  // where the lifeline starts
	LifeEnd   float64  // where the lifeline stops
}

// Box groups participants under a label and an optional background colour.
type Box struct {
	Label   string
	Color   string // validated colour, or "" for none
	Members []*Participant

	Lines  []string
	X0, X1 float64
}

// Message is a single arrow from one participant to another.
type Message struct {
	From  string
	To    string
	Text  string
	Arrow Arrow

	Num        int  // autonumber value (0 = unnumbered)
	Activate   bool // `+` before the receiver
	Deactivate bool // `-` before the receiver: the sender's activation ends
	Creates    string
	Destroys   []string

	// Layout results.
	Lines  []string
	Y      float64 // y of the arrow line
	X1, X2 float64 // start and end of the arrow line
}

// NotePos is where a note sits relative to its participant(s).
type NotePos int

const (
	// NoteRight places the note to the right of a participant.
	NoteRight NotePos = iota
	// NoteLeft places the note to the left of a participant.
	NoteLeft
	// NoteOver spans the note over one or two participants.
	NoteOver
)

// Note is an annotation box.
type Note struct {
	Pos  NotePos
	Of   []string
	Text string

	Lines      []string
	X, Y, W, H float64
}

// Bar is an activation on a participant's lifeline.
type Bar struct {
	Participant string
	Depth       int // 0 for the outermost activation
	Y1, Y2      float64
}

// Section is an else/and/option divider inside a frame.
type Section struct {
	Label string

	Lines []string
	Y     float64
}

// Frame is a grouping box (loop/alt/opt/par/critical/break/rect).
type Frame struct {
	Kind     string
	Label    string
	Sections []*Section

	// Color is the background of a `rect` frame, validated; empty otherwise.
	Color string

	Lines          []string // condition lines
	X0, X1, Y0, Y1 float64
	Depth          int
}

type itemKind int

const (
	itMessage itemKind = iota
	itNote
	itFrameStart
	itSection
	itFrameEnd
	itActivate
	itDeactivate
	itDestroy
)

// item is one statement in source order; the layout walks them top to bottom.
type item struct {
	kind    itemKind
	msg     *Message
	note    *Note
	frame   *Frame
	section *Section
	who     string
}

// Diagram is a parsed sequence diagram.
type Diagram struct {
	Title        string
	Participants []*Participant
	Boxes        []*Box
	Messages     []*Message
	Notes        []*Note
	Frames       []*Frame
	Bars         []*Bar

	items []item
	index map[string]int
}

// participant returns the participant with id, or nil.
func (d *Diagram) participant(id string) *Participant {
	if i, ok := d.index[id]; ok {
		return d.Participants[i]
	}
	return nil
}
