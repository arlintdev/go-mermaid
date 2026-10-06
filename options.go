package mermaid

import (
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/render"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

// Theme selects a built-in color palette for rendering.
type Theme string

const (
	// Default is the light theme used when no theme is set.
	Default Theme = "default"
	// Dark is a dark-background palette.
	Dark Theme = "dark"
	// Neutral is a grayscale palette suitable for print.
	Neutral Theme = "neutral"
	// Forest is a green palette.
	Forest Theme = "forest"
	// Base is a muted neutral palette.
	Base Theme = "base"
)

// Themes returns the names of all built-in themes.
func Themes() []string { return theme.Names() }

// Palette is a custom color set for WithCustomTheme.
type Palette = theme.Palette

// WithCustomTheme registers a palette under name and selects it for this
// render. Subsequent renders can also reference the name via WithTheme.
func WithCustomTheme(name string, p Palette) Option {
	return func(c *config) {
		theme.Register(name, p)
		c.theme, c.themeSet = Theme(name), true
	}
}

// DiagramTypes returns the diagram header keywords this library can render.
func DiagramTypes() []string {
	return []string{
		"graph / flowchart",
		"sequenceDiagram",
		"classDiagram",
		"stateDiagram-v2",
		"erDiagram",
		"pie",
		"journey",
		"quadrantChart",
		"gitGraph",
		"timeline",
		"mindmap",
		"gantt",
		"C4Context / C4Container",
		"requirementDiagram",
		"sankey-beta",
		"xychart-beta",
		"block-beta",
		"kanban",
		"packet-beta",
		"radar-beta",
	}
}

// Option configures a Render call. Options are applied in order; later
// options override earlier ones.
type Option func(*config)

// config holds resolved rendering settings. It is unexported; callers
// mutate it only through Option values.
type config struct {
	theme    Theme
	fontFace string
	fontSize float64
	padding  float64
	nodeSep  float64 // horizontal gap between nodes in a rank
	rankSep  float64 // gap between ranks
	curved   bool    // render flowchart edges as smooth curves

	bgColor       string // override background color
	bgTransparent bool   // omit the background rect entirely

	fontSet  bool // WithFont was given
	themeSet bool // WithTheme or WithCustomTheme was given

	themeVars theme.Palette // colours from the diagram's init directive
	idPrefix  string        // WithIDPrefix, empty for one derived from the source
}

// fontFamilyRe accepts a plain CSS font-family list: names, generic
// families, commas, spaces, hyphens and quotes. Anything else could carry
// markup into the SVG, so it falls back to the default.
var fontFamilyRe = regexp.MustCompile(`^[A-Za-z0-9 ,'"-]{1,200}$`)

// safeFontFace returns face when it is a plain font-family list, otherwise
// the default family.
func safeFontFace(face string) string {
	if fontFamilyRe.MatchString(face) && strings.TrimSpace(face) != "" {
		return face
	}
	return defaultConfig().fontFace
}

func defaultConfig() config {
	return config{
		theme:    Default,
		fontFace: "sans-serif",
		fontSize: 14,
		padding:  16,
		nodeSep:  50,
		rankSep:  50,
	}
}

func (c config) layout() layout.Options {
	return layout.Options{
		NodeSep:  c.nodeSep,
		RankSep:  c.rankSep,
		FontSize: c.fontSize,
		FontFace: c.fontFace,
	}
}

func (c config) render() render.Options {
	return render.Options{
		Theme:    string(c.theme),
		FontFace: c.fontFace,
		FontSize: c.fontSize,
		Padding:  c.padding,
		Curved:   c.curved,
	}
}

// WithTheme sets the color palette.
func WithTheme(t Theme) Option {
	return func(c *config) { c.theme, c.themeSet = t, true }
}

// WithFont sets the font family and base size (in pixels) for labels.
func WithFont(face string, size float64) Option {
	return func(c *config) {
		c.fontFace = face
		c.fontSize = size
		c.fontSet = true
	}
}

// WithPadding sets the outer padding (in pixels) around the diagram.
func WithPadding(px float64) Option {
	return func(c *config) { c.padding = px }
}

// WithSpacing sets the gap between sibling nodes (nodeSep) and between
// ranks (rankSep), in pixels.
func WithSpacing(nodeSep, rankSep float64) Option {
	return func(c *config) {
		c.nodeSep = nodeSep
		c.rankSep = rankSep
	}
}

// WithCurvedEdges renders flowchart edges as smooth curves instead of
// straight orthogonal segments.
func WithCurvedEdges(on bool) Option {
	return func(c *config) { c.curved = on }
}

// WithBackground overrides the diagram background color (any CSS color).
func WithBackground(color string) Option {
	return func(c *config) { c.bgColor = color; c.bgTransparent = false }
}

// WithTransparentBackground removes the background rect so the diagram blends
// with whatever it is embedded in.
func WithTransparentBackground() Option {
	return func(c *config) { c.bgTransparent = true; c.bgColor = "" }
}

// WithIDPrefix sets the prefix of every id in a flowchart's SVG (its
// markers), so pictures inlined on one page never share an id. Without it
// the prefix is derived from a hash of the source. The prefix must start
// with a letter and hold only letters, digits, '-' and '_'; any other value
// is replaced by "m".
func WithIDPrefix(prefix string) Option {
	return func(c *config) { c.idPrefix = prefix }
}
