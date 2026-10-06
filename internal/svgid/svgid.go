// Package svgid derives element ids (markers, clip ids) that are unique per
// diagram, so two diagrams inlined on one page never share an id such as
// "arrow" and one picture's markers never restyle another's.
package svgid

import (
	"hash/fnv"
	"regexp"
	"strconv"
)

// Prefix returns a short id prefix derived from the diagram source, such as
// "m3f2a9c1b". The same source always yields the same prefix, so output
// stays deterministic.
func Prefix(src string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(src))
	return "m" + strconv.FormatUint(uint64(h.Sum32()), 36)
}

var validRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// Valid reports whether prefix can start an id: a letter, then up to 63
// letters, digits, '-' or '_'.
func Valid(prefix string) bool { return validRe.MatchString(prefix) }

// For returns the id prefix of a picture: the caller's prefix when it is
// valid, one derived from src when the caller gave none, and "m" for any
// other value.
func For(prefix, src string) string {
	switch {
	case prefix == "":
		return Prefix(src)
	case Valid(prefix):
		return prefix
	default:
		return "m"
	}
}
