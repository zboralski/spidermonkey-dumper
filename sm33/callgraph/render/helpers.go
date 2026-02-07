package render

import (
	"fmt"
	"strings"
)

// dotEscape escapes a string for use in DOT HTML labels.
func dotEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// dotID creates a safe DOT identifier from a function name.
func dotID(name string) string {
	var b strings.Builder
	b.WriteString("n_")
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		} else {
			fmt.Fprintf(&b, "_%04x", c)
		}
	}
	return b.String()
}

// isAllCaps returns true if the name looks like a constant (all uppercase + underscores).
func isAllCaps(s string) bool {
	if len(s) < 2 {
		return false
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}
