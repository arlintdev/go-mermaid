// Package svgid derives element ids (markers, clip ids) that are unique per
// diagram, so two diagrams inlined on one page never share an id such as
// "arrow" and one picture's markers never restyle another's.
package svgid

import (
	"hash/fnv"
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
