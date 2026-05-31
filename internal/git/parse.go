package git

import "strings"

// WorktreeInfo is one entry from `git worktree list --porcelain -z`.
type WorktreeInfo struct {
	Path           string
	HEAD           string
	Branch         string // refs/heads/...; empty for bare or detached
	Bare           bool
	Detached       bool
	Locked         bool
	LockedReason   string
	Prunable       bool
	PrunableReason string
}

// parsePorcelainZ parses NUL-delimited porcelain output. Records are separated
// by a blank line, which in -z form is an empty attribute (i.e. "\x00\x00").
func parsePorcelainZ(data []byte) ([]WorktreeInfo, error) {
	s := string(data)
	var out []WorktreeInfo
	var cur *WorktreeInfo

	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}

	for _, attr := range strings.Split(s, "\x00") {
		if attr == "" {
			flush()
			continue
		}
		key, val, _ := strings.Cut(attr, " ")
		switch key {
		case "worktree":
			flush()
			cur = &WorktreeInfo{Path: val}
		case "HEAD":
			if cur != nil {
				cur.HEAD = val
			}
		case "branch":
			if cur != nil {
				cur.Branch = val
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
				cur.LockedReason = val
			}
		case "prunable":
			if cur != nil {
				cur.Prunable = true
				cur.PrunableReason = val
			}
		}
	}
	flush()
	return out, nil
}
