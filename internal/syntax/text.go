package syntax

import "strings"

// StripComment returns s without a %% comment and what follows it.
func StripComment(s string) string {
	if i := strings.Index(s, "%%"); i >= 0 {
		return s[:i]
	}
	return s
}

// FirstWord returns s up to its first space or tab.
func FirstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}
