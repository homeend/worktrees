package naming

import (
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

func TestSanitizeDir_StripsPrefixAndSlashes(t *testing.T) {
	if got := SanitizeDir("wt/feature/foo"); got != "feature-foo" {
		t.Errorf("SanitizeDir = %q, want feature-foo", got)
	}
	if got := SanitizeDir("plain"); got != "plain" {
		t.Errorf("SanitizeDir = %q, want plain", got)
	}
}
