package naming

import (
	"fmt"
	"strings"
	"time"
)

// Generate builds a default name of the form
// YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN. The digits are caller-supplied
// (random in production) and zero-padded to 4. Word selection is derived from
// the digits so the function is pure and testable. Negative digits are
// normalized to a non-negative value so the function never panics on a negative
// slice index (the suffix is always rendered from 0..9999).
func Generate(ts time.Time, digits int) string {
	n := ((digits % 10000) + 10000) % 10000
	adj := adjectives[(n/len(nouns))%len(adjectives)]
	noun := nouns[n%len(nouns)]
	return fmt.Sprintf("%s-%s-%s-%04d",
		ts.Format("2006-01-02_15-04"), adj, noun, n)
}

// SanitizeDir converts a branch-style name into a filesystem-safe directory
// name: strips a leading "wt/" prefix and replaces remaining slashes with "-".
func SanitizeDir(name string) string {
	name = strings.TrimPrefix(name, "wt/")
	return strings.ReplaceAll(name, "/", "-")
}
