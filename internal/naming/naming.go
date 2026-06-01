package naming

import (
	"fmt"
	"strings"
	"text/template"
	"time"
)

// NameContext holds the components a custom name_template can reference:
// {{.Date}} {{.Adjective}} {{.Noun}} {{.Digits}}.
type NameContext struct {
	Date      string // YYYY-MM-DD_HH-mm
	Adjective string
	Noun      string
	Digits    string // zero-padded to 4
}

// newContext derives the name components from a timestamp and digits. Negative
// digits are normalized so word indices never go out of range.
func newContext(ts time.Time, digits int) NameContext {
	n := ((digits % 10000) + 10000) % 10000
	return NameContext{
		Date:      ts.Format("2006-01-02_15-04"),
		Adjective: adjectives[(n/len(nouns))%len(adjectives)],
		Noun:      nouns[n%len(nouns)],
		Digits:    fmt.Sprintf("%04d", n),
	}
}

// Generate builds a default name of the form
// YYYY-MM-DD_HH-mm-<adjective>-<noun>-NNNN. The digits are caller-supplied
// (random in production) and zero-padded to 4. Word selection is derived from
// the digits so the function is pure and testable.
func Generate(ts time.Time, digits int) string {
	c := newContext(ts, digits)
	return fmt.Sprintf("%s-%s-%s-%s", c.Date, c.Adjective, c.Noun, c.Digits)
}

// GenerateFrom renders a Go text/template against NameContext. An empty template
// yields the default Generate output. A parse or execution error is returned so
// the caller can reject an invalid name_template.
func GenerateFrom(tmpl string, ts time.Time, digits int) (string, error) {
	if strings.TrimSpace(tmpl) == "" {
		return Generate(ts, digits), nil
	}
	t, err := template.New("name").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid name_template: %w", err)
	}
	var b strings.Builder
	if err := t.Execute(&b, newContext(ts, digits)); err != nil {
		return "", fmt.Errorf("name_template: %w", err)
	}
	return b.String(), nil
}

// RenderTemplate renders a user template against string variables, erroring on
// any referenced-but-missing variable (missingkey=error), mirroring GenerateFrom.
func RenderTemplate(tmpl string, vars map[string]string) (string, error) {
	t, err := template.New("tmpl").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid template: %w", err)
	}
	var b strings.Builder
	if err := t.Execute(&b, vars); err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	return b.String(), nil
}

// SanitizeDir converts a branch-style name into a filesystem-safe directory
// name: strips the given branch prefix and replaces remaining slashes with "-".
func SanitizeDir(name, prefix string) string {
	name = strings.TrimPrefix(name, prefix)
	return strings.ReplaceAll(name, "/", "-")
}
