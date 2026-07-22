// Package naming resolves wt's worktree/branch naming templates. The <...>
// token syntax and semantics are ported from gigagit's internal/template
// package so both tools share one template language. It is pure: no I/O, all
// time/randomness drawn from an injected Ctx, so resolution is deterministic
// and fully unit-testable.
package naming

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Ctx carries everything Resolve needs beyond the <user:> inputs. Now and
// Rand are injected so resolution is deterministic in tests. The resolver
// never mutates any field (notably Seqs).
type Ctx struct {
	ParentBranch string         // <parent-branch>
	Repo         string         // <repo>
	Seqs         map[string]int // current <seq:NAME> values, supplied by the caller
	Now          func() time.Time
	Rand         *rand.Rand
}

// tokenRe matches a single <...> token, capturing the inside (no '>' allowed,
// so tokens never span).
var tokenRe = regexp.MustCompile(`<([^>]+)>`)

// Resolve substitutes every <...> token in tmpl. inputs supplies <user:LABEL>
// values. Unknown tokens, a missing user input, or malformed token arguments
// are returned as errors (never silently passed through). A <date> token
// without a format defaults to yyyy-MM-dd; any <date...> token requires
// Ctx.Now and a <random-*> token requires Ctx.Rand.
func Resolve(tmpl string, inputs map[string]string, ctx Ctx) (string, error) {
	var firstErr error
	out := tokenRe.ReplaceAllStringFunc(tmpl, func(tok string) string {
		body := tok[1 : len(tok)-1] // strip < >
		val, err := resolveToken(body, inputs, ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return ""
		}
		return val
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

func resolveToken(body string, inputs map[string]string, ctx Ctx) (string, error) {
	prefix, rest, hasColon := cutColon(body)
	switch prefix {
	case "parent-branch":
		return ctx.ParentBranch, nil
	case "repo":
		return ctx.Repo, nil
	case "date":
		if ctx.Now == nil {
			return "", fmt.Errorf("template: <date> requires Ctx.Now to be set")
		}
		if !hasColon || rest == "" {
			return ctx.Now().Format("2006-01-02"), nil
		}
		return ctx.Now().Format(goLayout(rest)), nil
	case "seq":
		return resolveSeq(rest, ctx)
	case "user":
		if !hasColon {
			return "", fmt.Errorf("template: <user> requires a label, e.g. <user:issue-id>")
		}
		v, ok := inputs[rest]
		if !ok {
			return "", fmt.Errorf("template: missing input for <user:%s>", rest)
		}
		return v, nil
	case "random-alpha", "random-num":
		return resolveRandom(prefix, rest, hasColon, ctx)
	default:
		return "", fmt.Errorf("template: unknown token <%s>", body)
	}
}

// resolveSeq handles <seq:NAME> and <seq:NAME:N>. The value comes from
// ctx.Seqs (0 if absent); N zero-pads.
func resolveSeq(rest string, ctx Ctx) (string, error) {
	name, padStr, hasPad := cutColon(rest)
	if name == "" {
		return "", fmt.Errorf("template: <seq> requires a name, e.g. <seq:issue>")
	}
	n := ctx.Seqs[name]
	if !hasPad {
		return strconv.Itoa(n), nil
	}
	pad, err := strconv.Atoi(padStr)
	if err != nil || pad < 0 {
		return "", fmt.Errorf("template: <seq:%s:%s> padding must be a non-negative integer", name, padStr)
	}
	return fmt.Sprintf("%0*d", pad, n), nil
}

const lowerAlpha = "abcdefghijklmnopqrstuvwxyz"
const digits = "0123456789"

// resolveRandom handles <random-alpha:N> (N lowercase letters) and
// <random-num:N> (N digits), drawing from ctx.Rand so seeded runs are
// reproducible. N must be a positive integer.
func resolveRandom(prefix, rest string, hasColon bool, ctx Ctx) (string, error) {
	if !hasColon {
		return "", fmt.Errorf("template: <%s> requires a length, e.g. <%s:4>", prefix, prefix)
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n <= 0 {
		return "", fmt.Errorf("template: <%s:%s> length must be a positive integer", prefix, rest)
	}
	if ctx.Rand == nil {
		return "", fmt.Errorf("template: <%s> requires Ctx.Rand to be set", prefix)
	}
	alphabet := lowerAlpha
	if prefix == "random-num" {
		alphabet = digits
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[ctx.Rand.IntN(len(alphabet))]
	}
	return string(b), nil
}

// UserLabels returns the distinct <user:LABEL> labels in order of first
// appearance, so a frontend knows which inputs to collect.
func UserLabels(tmpl string) []string {
	return distinctTokenArgs(tmpl, "user")
}

// SeqNames returns the distinct <seq:NAME> counter names in order of first
// appearance, so the create flow knows which counters to peek and bump.
func SeqNames(tmpl string) []string {
	return distinctTokenArgs(tmpl, "seq")
}

// distinctTokenArgs scans tmpl for tokens of the form <prefix:ARG...> and
// returns the first colon-separated segment after the prefix, distinct and
// ordered.
func distinctTokenArgs(tmpl, prefix string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range tokenRe.FindAllStringSubmatch(tmpl, -1) {
		body := m[1]
		p, rest, ok := cutColon(body)
		if !ok || p != prefix {
			continue
		}
		arg, _, _ := cutColon(rest)
		if arg == "" {
			arg = rest
		}
		if !seen[arg] {
			seen[arg] = true
			out = append(out, arg)
		}
	}
	return out
}

// cutColon splits s on the first ':' into (before, after, found).
func cutColon(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// dateLayoutReplacer maps human date tokens to Go's reference-time layout.
var dateLayoutReplacer = strings.NewReplacer(
	"yyyy", "2006",
	"MM", "01",
	"dd", "02",
	"HH", "15",
	"mm", "04",
	"ss", "05",
)

// goLayout converts a human date format (yyyy MM dd HH mm ss, with arbitrary
// separators) into Go's reference-time layout string.
func goLayout(human string) string {
	return dateLayoutReplacer.Replace(human)
}

// SanitizeSegment makes a branch name safe as a single path segment for the
// running OS (see SanitizeSegmentFor). Worktree directories are the sanitized
// branch inside the container — flat, never nested.
func SanitizeSegment(branch string) string {
	return SanitizeSegmentFor(branch, runtime.GOOS)
}

// SanitizeSegmentFor makes s safe as a single path segment on goos. Path
// separators ('/', '\\') and control characters become '-' everywhere; on
// Windows the additional reserved characters (<>:"|?*) become '-', trailing
// dots/spaces are trimmed, and reserved device names (CON, PRN, … COM1-9,
// LPT1-9) get a '_' suffix. A result that would be empty or a directory dot
// ('.'/'..') falls back to "tag" so the worktree always gets a real leaf dir.
func SanitizeSegmentFor(s, goos string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '/' || r == '\\' || r < 0x20:
			b.WriteByte('-')
		case goos == "windows" && strings.ContainsRune(`<>:"|?*`, r):
			b.WriteByte('-')
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	if goos == "windows" {
		out = strings.TrimRight(out, ". ")
		if isWindowsReserved(out) {
			out += "_"
		}
	}
	if out == "" || out == "." || out == ".." {
		out = "tag"
	}
	return out
}

// isWindowsReserved reports whether s (case-insensitive) is a Windows
// reserved device name.
func isWindowsReserved(s string) bool {
	up := strings.ToUpper(s)
	switch up {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	for i := 1; i <= 9; i++ {
		if up == fmt.Sprintf("COM%d", i) || up == fmt.Sprintf("LPT%d", i) {
			return true
		}
	}
	return false
}
