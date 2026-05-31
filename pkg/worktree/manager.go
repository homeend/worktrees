package worktree

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/code-drill/wt/internal/naming"
)

// Manager orchestrates worktree operations over injected collaborators.
type Manager struct {
	git    GitRunner
	hooks  HookRunner
	cfg    ConfigProvider
	now    func() time.Time
	digits func() int
}

// New constructs a Manager with default time/random sources.
func New(g GitRunner, h HookRunner, c ConfigProvider) *Manager {
	return &Manager{
		git:    g,
		hooks:  h,
		cfg:    c,
		now:    time.Now,
		digits: defaultDigits,
	}
}

// SetDigits overrides the digit source used for generated names (e.g. random in
// production). Intended for wiring and tests.
func (m *Manager) SetDigits(fn func() int) { m.digits = fn }

// containerPath returns the worktree container for a repo root. A configured
// container overrides the default sibling and is used verbatim.
func (m *Manager) containerPath(repoRoot string) string {
	if c := m.cfg.Container(); c != "" {
		return c
	}
	return repoRoot + ".worktrees"
}

// resolveNames computes (name, branch). name omits the wt/ prefix; branch always
// carries it. An explicit Branch overrides the derived one (still prefixed).
func (m *Manager) resolveNames(opts AddOptions) (name, branch string) {
	name = opts.Name
	if name == "" {
		name = naming.Generate(m.now(), m.digits())
	}
	base := opts.Branch
	if base == "" {
		base = name
	}
	branch = "wt/" + strings.TrimPrefix(base, "wt/")
	return name, branch
}

// worktreePath returns the on-disk path for a branch within the container.
func (m *Manager) worktreePath(repoRoot, branch string) string {
	return filepath.Join(m.containerPath(repoRoot), naming.SanitizeDir(branch))
}

func defaultDigits() int {
	return 1
}
