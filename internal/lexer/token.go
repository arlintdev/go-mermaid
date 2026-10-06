package lexer

// Kind enumerates token categories produced by the lexer.
type Kind int

const (
	// EOF marks the end of input.
	EOF Kind = iota
	// Newline separates statements.
	Newline
	// Keyword is a reserved word (graph, flowchart, subgraph, end,
	// direction).
	Keyword
	// Ident is a node identifier or bare word.
	Ident
	// Text is the contents of a shape or edge label, or the rest of a
	// subgraph line.
	Text
	// Arrow is an edge connector (-->, ---, -.->, ==>, <-->, --o, ~~~, and
	// the middle-label forms such as "-- text -->"). Link describes it.
	Arrow
	// ShapeOpen is an opening shape delimiter ([ ( ([ (( ((( { {{ >).
	ShapeOpen
	// ShapeClose is a closing shape delimiter (] ) ]) )) ))) } }}).
	ShapeClose
	// Pipe is the | bracketing an inline edge label.
	Pipe
	// Amp is the & joining several nodes on one side of a link.
	Amp
	// ClassRef is an inline class assignment, ":::name"; Val is the name.
	ClassRef
	// Meta is a node or edge metadata block, "@{ ... }"; Val is its body.
	Meta
	// EdgeID names the link that follows it, "e1@-->"; Val is the id.
	EdgeID
)

// Link describes an Arrow token.
type Link struct {
	// Start and End are the end markers: "" for none, or "arrow",
	// "circle", "cross".
	Start, End string
	// Line is "solid", "dotted", "thick" or "invisible".
	Line string
	// Len is how many ranks the link spans at least (1 for -->, 2 for --->).
	Len int
	// Label is the text written between the halves of the link, as in
	// "A -- text --> B", and HasLabel reports whether there was one.
	Label    string
	HasLabel bool
}

// Token is a lexical unit with its source position.
type Token struct {
	Kind Kind
	Val  string
	Line int // 1-based
	Col  int // 1-based, rune offset within the line
	// Link is set on Arrow tokens.
	Link *Link
}
