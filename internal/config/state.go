package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Machine-local per-repo state for <seq:NAME> counters, mirroring gigagit:
// stored at <git-common-dir>/wt/state.toml so all linked worktrees of a repo
// share the counters. The stored value is the LAST CONSUMED number; the next
// value handed out is stored+1 (an absent counter previews as 1).

type seqState struct {
	Seq map[string]int `toml:"seq"`
}

func statePath(commonDir string) string {
	return filepath.Join(commonDir, "wt", "state.toml")
}

func readSeqState(commonDir string) seqState {
	st := seqState{Seq: map[string]int{}}
	data, err := os.ReadFile(statePath(commonDir))
	if err != nil {
		return st
	}
	// A corrupt state file degrades to fresh counters rather than failing.
	_ = toml.Unmarshal(data, &st)
	if st.Seq == nil {
		st.Seq = map[string]int{}
	}
	return st
}

// PeekSeq returns the next value the counter would hand out, without
// consuming it. Absent counter (or unreadable state) previews as 1.
func PeekSeq(commonDir, name string) int {
	return readSeqState(commonDir).Seq[name] + 1
}

// PeekSeqs peeks several counters at once (for template rendering).
func PeekSeqs(commonDir string, names []string) map[string]int {
	out := make(map[string]int, len(names))
	for _, n := range names {
		out[n] = PeekSeq(commonDir, n)
	}
	return out
}

// BumpSeq consumes and returns the next value of the counter, persisting the
// state atomically (temp file + rename). It refuses an empty commonDir so a
// stray wt/state.toml is never scattered into the cwd.
func BumpSeq(commonDir, name string) (int, error) {
	if commonDir == "" {
		return 0, errors.New("bump seq: empty git common dir")
	}
	st := readSeqState(commonDir)
	st.Seq[name]++
	n := st.Seq[name]

	data, err := toml.Marshal(st)
	if err != nil {
		return 0, fmt.Errorf("bump seq: marshal state: %w", err)
	}
	path := statePath(commonDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, fmt.Errorf("bump seq: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state-*.toml")
	if err != nil {
		return 0, fmt.Errorf("bump seq: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return 0, fmt.Errorf("bump seq: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return 0, fmt.Errorf("bump seq: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return 0, fmt.Errorf("bump seq: %w", err)
	}
	return n, nil
}
