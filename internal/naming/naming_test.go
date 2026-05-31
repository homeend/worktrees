package naming

import (
	"math"
	"regexp"
	"testing"
	"time"
)

func TestGenerate_DateFirstFormat(t *testing.T) {
	ts := time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC)
	name := Generate(ts, 4821)
	re := regexp.MustCompile(`^2026-05-31_14-30-[a-z]+-[a-z]+-4821$`)
	if !re.MatchString(name) {
		t.Errorf("name %q does not match expected pattern", name)
	}
}

func TestGenerate_IsDeterministicForSeed(t *testing.T) {
	ts := time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC)
	a := Generate(ts, 1)
	if a[:16] != "2026-05-31_14-30" {
		t.Errorf("date prefix wrong: %q", a)
	}
	if a[len(a)-4:] != "0001" {
		t.Errorf("digit suffix should be zero-padded: %q", a)
	}
}

func TestGenerate_NonNegativeAndWellFormedForEdgeDigits(t *testing.T) {
	ts := time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC)
	re := regexp.MustCompile(`^2026-05-31_14-30-[a-z]+-[a-z]+-\d{4}$`)
	for _, digits := range []int{0, 9999, 10000, 12345, -1, -9999, math.MinInt, math.MaxInt} {
		name := Generate(ts, digits) // must not panic
		if !re.MatchString(name) {
			t.Errorf("Generate(%d) = %q, not a well-formed name", digits, name)
		}
	}
}

func TestSanitizeDir_StripsPrefixAndSlashes(t *testing.T) {
	if got := SanitizeDir("wt/feature/foo"); got != "feature-foo" {
		t.Errorf("SanitizeDir = %q, want feature-foo", got)
	}
	if got := SanitizeDir("plain"); got != "plain" {
		t.Errorf("SanitizeDir = %q, want plain", got)
	}
}
