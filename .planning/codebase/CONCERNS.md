# Codebase Concerns

**Analysis Date:** 2026-06-05

---

## Tech Debt

**`config.Set` only supports one key:**
- Issue: `config.Set` in `internal/config/config.go` hard-codes support for `branch_prefix` only. Every other config field (`base_ref`, `container`, `name_template`, `templates`) is file-only and has no `wt set` CLI path. A comment in the source says "Today only branch_prefix is supported".
- Files: `internal/config/config.go` (lines 112–135), `cmd/wt/set.go`
- Impact: Users cannot change `base_ref` or `container` through the CLI; they must hand-edit YAML. This is user-visible friction and the restriction is not documented in `--help`.
- Fix approach: Extend `Set` to handle the other scalar keys (`base_ref`, `container`, `name_template`), or at minimum document the limitation in the command's `Short` description.

**`upsertLine` is a line-based YAML mutator with no parser:**
- Issue: `internal/config/config.go:upsertLine` rewrites `.worktrees/config.yaml` by string-matching on `key:` prefixes rather than parsing YAML, round-tripping, and re-serializing. The code comment acknowledges: "This is sound only because every config key is a flat top-level scalar."
- Files: `internal/config/config.go` (lines 139–155)
- Impact: Will silently corrupt the file if a user adds non-trivial YAML (multi-line scalars, anchor references, nested structures for `templates`). The `templates` block is a YAML sequence and would not survive an upsert of any key once templates are present — the list lines would not be touched, but line ordering assumptions could break under future keys.
- Fix approach: Use `gopkg.in/yaml.v3` round-trip (unmarshal → mutate struct → marshal) so structural integrity is guaranteed. Adds a tradeoff: comments would be stripped. A comment-preserving option is to keep the current approach but add a structural parse-validate step before writing.

**`envCfg` only layers one environment variable:**
- Issue: `internal/config/config.go:envCfg` reads only `WT_BRANCH_PREFIX`. Other fields such as `WT_BASE_REF` or `WT_CONTAINER` are not supported from the environment. The comment says "Today only WT_BRANCH_PREFIX is supported; other fields stay file-only."
- Files: `internal/config/config.go` (lines 68–70)
- Impact: Partial env-override story; users building scripts cannot override `base_ref` per-run without file mutation.
- Fix approach: Expand `envCfg` to read `WT_BASE_REF`, `WT_CONTAINER`, etc., parallel to the existing pattern.

**Global package-level flag variables in `cmd/wt`:**
- Issue: Every command in `cmd/wt/` stores its flags as package-level `var` declarations (e.g., `newBranch`, `newBase`, `rmForce`, `killYes`). This is the standard Cobra pattern but makes concurrent invocation of commands unsafe and means test isolation requires re-running a fresh process.
- Files: `cmd/wt/new.go` (lines 13–21), `cmd/wt/rm.go` (lines 12–15), `cmd/wt/kill_em_all.go` (line 17)
- Impact: Low risk in practice (single-process CLI), but the pattern means adding new commands requires discipline to not accidentally share flag variables across commands. The testable functions (`buildAddOptions`, `runKillEmAll`) correctly receive values as parameters and sidestep this, so the architecture is mitigated.
- Fix approach: No urgent change required, but document the pattern and ensure new commands follow the "extract-and-parameterize" convention already established.

**`defaultDigits` always returns `1`:**
- Issue: `pkg/worktree/manager.go:defaultDigits` returns `1` as the fallback digit source. The production wiring in `cmd/wt/root.go` overrides this with `randomDigits` via `m.SetDigits(randomDigits)`. However, any path that constructs a `Manager` without calling `SetDigits` (e.g., future code, the integration test adapter in `pkg/worktree/manager_integration_test.go`) will silently produce deterministic, non-random names.
- Files: `pkg/worktree/manager.go` (lines 22–32, 87–89)
- Impact: Names generated in `manager_integration_test.go` are deterministic (digit=1), which is fine for tests but would be a subtle bug if a second `Manager` construction site were added without the `SetDigits` call.
- Fix approach: Consider making `randomDigits` the default, or require callers to inject the digit function at construction time (remove `defaultDigits`).

---

## Known Bugs

**`resolveWorktree` can match wrong worktree via `byLeaf` on name collisions:**
- Symptoms: When two worktrees share the same leaf directory name (possible in a nested layout: e.g., `wt/team/feat` and `wt/other/feat` both have leaf `feat`), the `byLeaf` match in `resolveWorktree` returns the first one found in list order rather than erroring or preferring an exact match.
- Files: `pkg/worktree/manager.go` (lines 232–238)
- Trigger: `wt rm feat` when the container contains both `wt/team/feat` and `wt/other/feat`.
- Workaround: Pass the full branch path: `wt rm wt/team/feat`.

**`classify` uses substring matching on error messages, not sentinel errors:**
- Symptoms: `classify` in `cmd/wt/errors.go` checks `strings.Contains(msg, "hook failed")`, `"already exists"`, etc. If a git error message from a future git version changes wording, the wrong exit code is returned. Additionally, an error whose *cause* contains these phrases (e.g., a branch named "already exists") could be misclassified.
- Files: `cmd/wt/errors.go` (lines 22–36)
- Trigger: Rare; git error messages are stable, but the pattern is inherently fragile.
- Workaround: None; exit codes may be wrong but the error text is always correct.

**`parsePorcelainZ` returns no error on malformed input:**
- Symptoms: `parsePorcelainZ` in `internal/git/parse.go` always returns `nil` error. Unexpected attributes from future git versions are silently skipped. A malformed record (e.g., `worktree` attribute missing) will result in an empty `WorktreeInfo` being appended to the output.
- Files: `internal/git/parse.go` (lines 20–72)
- Trigger: Would occur with git versions that add new porcelain fields or malformed git output (git bug, disk corruption).
- Workaround: None; output degradation is silent.

**`BranchExists` swallows git errors silently:**
- Symptoms: `BranchExists` in `internal/git/branch.go` (lines 25–28) collapses any `VerifyRef` error (operational git errors included) to `false`. A transient git failure (e.g., repo corruption, lock file) returns `false`, which causes `Add` to proceed with creating a new worktree, only to fail later at the actual `git worktree add` step with a less clear error.
- Files: `internal/git/branch.go` (lines 23–28)
- Impact: Error locality is degraded; the user sees a confusing `git worktree add` error rather than an upfront "could not verify branch existence" message.
- Fix approach: Add a tri-state return (exists bool, err error) and thread the error back through `Manager.Add`.

---

## Security Considerations

**Hook execution runs arbitrary user-supplied scripts with inherited environment:**
- Risk: Hooks discovered at `<repoRoot>/.worktrees/<event>` are executed with `os.Environ()` fully inherited plus `WT_*` variables appended. A malicious or misconfigured hook in a cloned repo executes immediately on `wt new` or `wt rm` with the user's full environment.
- Files: `internal/hooks/hooks.go` (lines 25–46)
- Current mitigation: Hooks must be executable (permission bit check at line 34). Non-executable hooks are skipped. The `--no-hooks` flag allows bypassing all hooks.
- Recommendations: Document in README that `.worktrees/` hooks are executed automatically and should be reviewed before use, especially in cloned repositories. Consider a one-time opt-in (e.g., a `.worktrees/.hooks-enabled` sentinel file) before running hooks from an unreviewed repo. This is a standard git-hooks-style concern, not a novel vulnerability.

**`WT_*` environment variables injected into hook processes contain user-controlled data:**
- Risk: `WT_NAME`, `WT_BRANCH`, `WT_BASE_REF`, `WT_REPO_NAME`, and `WT_CONTAINER` are derived from user input or config without sanitization. If a hook script uses these variables in a shell command without quoting (e.g., `cd $WT_TARGET_ROOT`), a branch name containing shell metacharacters could cause code injection within the hook script.
- Files: `internal/hooks/hooks.go` (lines 49–59)
- Current mitigation: `CheckRefFormat` validates that branch names are legal git refs (line 36 in `resolve.go`). Git ref names cannot contain shell metacharacters like `;`, `` ` ``, or `$`, so the attack surface is narrow.
- Recommendations: Clarify in hook stub templates that variables should be double-quoted.

**`--repo` flag accepts arbitrary filesystem paths without validation:**
- Risk: `--repo /any/path` is passed as `dir` to `buildManager` which calls `r.MainRoot(dir)` (a `git rev-parse`). This is safe because if the path is not a git repo, git returns a non-zero exit and the error is surfaced. However, a world-readable git repo could be specified to list or manipulate its worktrees.
- Files: `cmd/wt/root.go` (lines 47–51, 121–131)
- Current mitigation: Git permission model applies; the user can only operate on repos they have filesystem access to.
- Recommendations: Not a bug; standard behavior for a CLI tool.

**`os.CreateTemp` log file is created in the system temp directory without restricted permissions:**
- Risk: The TUI action logger creates temp files with `os.CreateTemp("", "wt-action-*.log")`, which uses the OS default temp directory. On shared systems, other users in the same temp directory could potentially read action logs if the default `umask` is permissive.
- Files: `internal/tui/model.go` (lines 138–141)
- Current mitigation: Modern OS defaults typically set 0600 permissions on temp files.
- Recommendations: Explicitly pass `0o600` mode, or use `os.MkdirTemp` for a private subdirectory. Note: `os.CreateTemp` in Go 1.17+ uses mode `0600` on Unix by default, so this is low severity.

---

## Performance Bottlenecks

**`resolveWorktree` calls `List` and `MainRoot` separately, causing two git invocations:**
- Problem: `resolveWorktree` calls `m.List(dir)` (which itself calls `m.git.MainRoot(dir)` and `m.git.ListWorktrees(dir)`), then calls `m.git.MainRoot(dir)` again to get `container`. This results in two `git rev-parse --git-common-dir` invocations per `wt rm` or `wt path` call.
- Files: `pkg/worktree/manager.go` (lines 219–246)
- Cause: `List` does not return the resolved repo root, so callers that need it must re-resolve.
- Improvement path: Refactor to return `([]WorktreeInfo, repoRoot string, error)` from `List`, or cache `MainRoot` result in `Manager`.

**`RemoveAll` iterates worktrees and branches sequentially with no parallelism:**
- Problem: `RemoveAll` in `pkg/worktree/manager.go` (lines 390–418) removes each worktree and deletes each branch one at a time. Each operation is a `git` subprocess. On repos with many worktrees this is O(n) serial subprocess launches.
- Files: `pkg/worktree/manager.go` (lines 400–416)
- Cause: Sequential for-loops with no goroutines.
- Improvement path: Low priority (kill-em-all is rare and worktree counts are typically small). Could use `errgroup` for parallel removal if needed.

---

## Fragile Areas

**`resolveWorktree` matching priority is undocumented and ambiguous:**
- Files: `pkg/worktree/manager.go` (lines 219–246)
- Why fragile: Three match criteria (`byBranch`, `byPath`, `byLeaf`) are evaluated with `||`. The first worktree in the list that satisfies any criterion wins. There is no disambiguation when multiple worktrees satisfy different criteria. As worktree directory layouts become more nested (the feature is new as of recent commits), the `byLeaf` match becomes increasingly likely to produce ambiguous results.
- Safe modification: Always prefer `byBranch` and `byPath` matches over `byLeaf`. Return an error if `byLeaf` would match multiple worktrees.
- Test coverage: `TestResolveWorktree_ByDirThenBranch` (`pkg/worktree/manager_test.go`) does not test the collision scenario described above.

**`pruneEmptyParents` silently stops on first `os.Remove` failure:**
- Files: `pkg/worktree/manager.go` (lines 251–258)
- Why fragile: If `os.Remove` fails for a reason other than "directory not empty" (e.g., permission denied, symlink), the walk stops silently and the parent hierarchy is left partially cleaned. There is no error returned to the caller.
- Safe modification: Log or surface unexpected `os.Remove` errors. Currently the pattern is intentional (non-empty stops walk), but a permission error is indistinguishable from a non-empty directory error with this approach.
- Test coverage: Not directly tested in unit tests; covered only by the integration test `TestManager_NestedLayoutAndPrune_RealGit`.

**TUI `updateConfirm` resolves name from `filepath.Base(it.Path)` (leaf only):**
- Files: `internal/tui/model.go` (lines 298–325)
- Why fragile: The TUI's delete confirmation passes `filepath.Base(it.Path)` as the worktree name to `wt rm`. With nested worktree paths (e.g., `wt/team/feat`), the leaf is `feat`, which is correct only if `resolveWorktree` can unambiguously match `feat`. If two worktrees share a leaf name, the wrong one could be deleted. This is the same underlying issue as the `byLeaf` ambiguity above, surfaced from the TUI.
- Safe modification: Pass the full branch name or container-relative path instead of `filepath.Base`.
- Test coverage: `TestDelete_ConfirmYesRemovesByDirName` (`internal/tui/model_test.go`) only tests the non-nested case.

**`config.Set` has a non-atomic write (read-modify-write race):**
- Files: `internal/config/config.go` (lines 112–135)
- Why fragile: `Set` reads the file, modifies it in memory, then writes it back. Two concurrent `wt set` invocations (or a user editing the file simultaneously) could result in a lost write. No file locking is used.
- Safe modification: Use `os.OpenFile` with `O_CREATE|O_WRONLY|O_EXCL` or an advisory lock. Low severity in practice (CLI tools are not typically run concurrently), but worth noting for automation scripts.
- Test coverage: Not tested.

**`init.go:scaffold` uses `os.Stat` + `os.WriteFile` with a TOCTOU race:**
- Files: `cmd/wt/init.go` (lines 51–56)
- Why fragile: `writeIfAbsent` checks file existence with `os.Stat`, then writes with `os.WriteFile`. A concurrent `wt init` invocation between those two calls could result in both instances writing the file. Impact is minimal (both write the same content), but the pattern is worth noting.
- Safe modification: Use `os.OpenFile` with `O_CREATE|O_EXCL` to make the check-and-create atomic.

**`loggedExec.Run` defers `bufio.NewReader(in).ReadString('\n')` on non-nil `in`:**
- Files: `internal/tui/model.go` (lines 119–123)
- Why fragile: The pause waits for exactly one newline from the user. If the user types multiple lines or pastes text, subsequent input is buffered in `bufio.Reader` and lost — it does not reach Bubble Tea's event loop after the TUI resumes. On a terminal with line buffering disabled (raw mode), `ReadString('\n')` may block indefinitely if no newline is produced (e.g., user presses Ctrl-D). This could hang the TUI.
- Safe modification: Read a single byte instead of a full line, or use `term.ReadPassword`-style raw read.

---

## Scaling Limits

**Word list for generated names is 16 adjectives × 16 nouns = 256 combinations:**
- Current capacity: 256 unique `<adjective>-<noun>` pairs. The 4-digit suffix (0000–9999) extends this to 2,560,000 combinations, but because the digit also selects the word pair deterministically (`adjective = digits/16 % 16`, `noun = digits % 16`), all 10,000 digit values resolve to only 256 word pairs. The same pair repeats across different digit values.
- Limit: Branch name uniqueness relies on the timestamp (`YYYY-MM-DD_HH-mm`) component. Multiple creates within the same minute with the same random digit value produce identical names, causing `Add` to fail with "branch already exists."
- Files: `internal/naming/words.go`, `internal/naming/naming.go` (lines 21–28)
- Scaling path: Increase word lists, use a larger digit space, or include seconds in the timestamp format.

---

## Dependencies at Risk

**`go 1.25.0` in `go.mod` does not exist yet:**
- Risk: `go.mod` declares `go 1.25.0`. As of mid-2026, Go 1.25 has not been released (Go releases follow a 6-month cadence; 1.24 was released early 2025). This may cause `go mod tidy` or toolchain downloads to fail in CI environments that strictly validate the Go version directive.
- Files: `go.mod` (line 3)
- Impact: Build toolchain warnings or failures depending on the Go toolchain in use.
- Migration plan: Change to the actual current stable version (`go 1.24` or whatever is current) or use `go 1.23` if that is the tested minimum.

**`charmbracelet/bubbletea v1.3.10` and `charmbracelet/lipgloss v1.1.0`:**
- Risk: These are indirectly depended upon (listed as `// indirect` in `go.mod`). The TUI layer in `internal/tui/` directly imports them without a direct `require` line. If a transitive dependency update pulls a different version, the TUI behavior could change without an explicit version pin.
- Files: `go.mod`, `internal/tui/model.go`, `internal/tui/view.go`
- Impact: Low; go.sum pins exact versions. Noting for awareness.
- Migration plan: Move `bubbletea` and `lipgloss` to direct `require` entries to make the dependency explicit.

---

## Missing Critical Features

**No `wt set` support for `base_ref`, `container`, `name_template`, `templates`:**
- Problem: The `wt set` command only supports `branch_prefix`. Users cannot configure other settings from the CLI.
- Blocks: Scriptable configuration management; onboarding automation.

**No rollback on `Add` failure after git worktree is created:**
- Problem: If `git worktree add` succeeds but the post-create hook fails, the worktree is left on disk. The error message says "worktree left in place at <path>" but provides no automated cleanup. This is documented as "by design" in the code comment.
- Files: `pkg/worktree/manager.go` (lines 91–96)
- Blocks: Clean failure modes for hook-heavy workflows.

**TUI has no template-based worktree creation flow:**
- Problem: The TUI shows a template list (`t` key) but pressing any key just returns to normal mode. There is no way to select a template and create a worktree from within the TUI — only from the CLI (`wt new -t <name>`).
- Files: `internal/tui/model.go` (lines 277–283), `internal/tui/view.go` (lines 53–58)
- Blocks: Fully keyboard-driven workflow with templates.

---

## Test Coverage Gaps

**`resolveWorktree` multi-match / collision scenario not tested:**
- What's not tested: Behavior when two worktrees share a leaf directory name in the nested layout (e.g., `wt/team/feat` and `wt/other/feat` both have leaf `feat`).
- Files: `pkg/worktree/manager.go` (lines 232–238), `pkg/worktree/manager_test.go`
- Risk: Silent wrong-worktree deletion.
- Priority: High

**`pruneEmptyParents` error path not unit-tested:**
- What's not tested: `os.Remove` failing for reasons other than non-empty directory.
- Files: `pkg/worktree/manager.go` (lines 251–258)
- Risk: Unexpected errors swallowed; partial directory cleanup not detected.
- Priority: Medium

**`classify` error mapping not tested against real git error messages:**
- What's not tested: That `classify` correctly wraps errors with the right sentinels when git produces specific error text.
- Files: `cmd/wt/errors.go`
- Risk: Exit code regression if git error message wording changes.
- Priority: Medium

**`config.Set` concurrent-write behavior not tested:**
- What's not tested: Concurrent or interleaved `Set` calls; TOCTOU race in `scaffold`.
- Files: `internal/config/config.go`, `cmd/wt/init.go`
- Risk: Lost writes in automation scripts.
- Priority: Low

**`loggedExec.Run` pause behavior not tested with closed stdin:**
- What's not tested: `ReadString('\n')` behavior when `in` is closed (e.g., Ctrl-D) or when no newline arrives.
- Files: `internal/tui/model.go` (lines 118–124)
- Risk: TUI hang on unusual terminal conditions.
- Priority: Low

**Integration tests are opt-in behind `//go:build integration`:**
- What's not tested: In a default `go test ./...` run, all integration tests are skipped. The unit tests use fakes that cannot catch real git behavorial changes.
- Files: `internal/git/git_integration_test.go`, `pkg/worktree/manager_integration_test.go`, `cmd/wt/cli_integration_test.go`
- Risk: Real git API changes or version-specific behavior goes undetected until manual test run.
- Priority: Medium — consider running integration tests in CI on every PR.

---

*Concerns audit: 2026-06-05*
