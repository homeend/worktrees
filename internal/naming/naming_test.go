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

func TestGenerateFrom_EmptyTemplateMatchesGenerate(t *testing.T) {
	ts := time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC)
	got, err := GenerateFrom("", ts, 4821)
	if err != nil {
		t.Fatalf("GenerateFrom: %v", err)
	}
	if got != Generate(ts, 4821) {
		t.Errorf("empty template = %q, want default %q", got, Generate(ts, 4821))
	}
}

func TestGenerateFrom_RendersCustomTemplate(t *testing.T) {
	ts := time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC)
	got, err := GenerateFrom("{{.Adjective}}_{{.Noun}}_{{.Digits}}", ts, 4821)
	if err != nil {
		t.Fatalf("GenerateFrom: %v", err)
	}
	if got != "eager_canyon_4821" {
		t.Errorf("custom template = %q, want eager_canyon_4821", got)
	}
}

func TestGenerateFrom_InvalidTemplateErrors(t *testing.T) {
	ts := time.Date(2026, 5, 31, 14, 30, 0, 0, time.UTC)
	if _, err := GenerateFrom("{{.Nope}}", ts, 1); err == nil {
		t.Error("unknown field should error (missingkey=error)")
	}
	if _, err := GenerateFrom("{{.Adjective", ts, 1); err == nil {
		t.Error("malformed template should error")
	}
}

func TestRenderTemplate_Renders(t *testing.T) {
	got, err := RenderTemplate("autofix/{{.ticketName}}", map[string]string{"ticketName": "ZX-12"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "autofix/ZX-12" {
		t.Errorf("RenderTemplate = %q, want autofix/ZX-12", got)
	}
}

func TestRenderTemplate_MissingVarErrors(t *testing.T) {
	if _, err := RenderTemplate("{{.nope}}", map[string]string{}); err == nil {
		t.Error("missing variable should error")
	}
}

func TestRenderTemplate_InvalidTemplateErrors(t *testing.T) {
	if _, err := RenderTemplate("{{.x", nil); err == nil {
		t.Error("malformed template should error")
	}
}
