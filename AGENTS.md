# Agent Instructions

## What is nxt?

**nxt** is a personal CLI dashboard that shows your most urgent work items, ranked by priority. It pulls data from three sources in parallel — Linear issues, GitHub PRs, and local git worktrees — links them together, scores them by urgency, and renders a ranked list in the terminal.

- **Interactive mode** (default when TTY): Bubbletea TUI with keyboard navigation, inline editor/Claude launching, and a settings screen.
- **Non-interactive mode** (`--json` or piped): Prints sorted JSON or styled text to stdout.

## Architecture

```
main.go              ← entry point, orchestrates fetch → link → score → render
cmd/root.go          ← CLI flag parsing (--json, --no-cache, --setup, --debug)
config/config.go     ← TOML config at ~/.config/nxt/config.toml + keychain secrets
setup/setup.go       ← first-run wizard (Linear API key → macOS Keychain, base dirs)
cache/cache.go       ← JSON file cache with 60s TTL (~/.cache/nxt/)
model/types.go       ← domain types: LinearIssue, PullRequest, Worktree, WorkItem
source/linear.go     ← Linear GraphQL API (viewer.assignedIssues)
source/github.go     ← GitHub PR fetching via gh CLI (linked PRs + authored search)
source/git.go        ← local worktree/branch scanning from configured base_dirs
linker/linker.go     ← correlates issues ↔ PRs ↔ worktrees by branch name / ID
scorer/scorer.go     ← urgency scoring (CI failure, review state, deadline, priority, staleness)
render/tui.go        ← Bubbletea interactive TUI (loading spinner → list → actions)
render/render.go     ← non-interactive styled terminal output
render/styles.go     ← adaptive color palette (light + dark terminal support)
render/settings.go   ← inline settings editor (editor command, base_dirs, max_items)
```

## Key concepts

- **WorkItem**: the unified domain object after linking. Always has at least one of: Issue, PR, Worktree.
- **Linking**: issues are the primary axis. PRs and worktrees match by branch name (Linear's `branchName` field or issue identifier as substring). Unmatched PRs become standalone items. A `repoMap` provides fallback folder linking via PR repo → local path.
- **Scoring**: purely additive. Signals: CI failing (+40), changes requested (+35), deadline proximity (+30 max), priority (+25/15/5), in-cycle (+10), no branch yet (+8), staleness (+20 max).
- **Cache**: JSON files under `~/.cache/nxt/` with stale-while-revalidate pattern (Linear: 2m fresh/10m stale, GitHub: 5m/10m, Worktrees: 30s/10m). Bypass with `--no-cache`.
- **GitHub auth**: uses `gh auth` multi-account support. Resolves the right token per repo owner by probing the GitHub API.

## Tech stack

- **Go 1.26+** — standard library plus:
  - `charmbracelet/bubbletea` + `lipgloss` for TUI
  - `BurntSushi/toml` for config
  - `golang.org/x/sync/errgroup` for parallel fetching
  - `golang.org/x/term` for terminal detection
- **External CLIs**: `gh` (GitHub), `git` (worktrees), `security` (macOS Keychain)
- **No database** — file-based cache only

## Config

Config lives at `~/.config/nxt/config.toml`:
```toml
[linear]
# api_key is NOT stored here — it's in macOS Keychain (account: "nxt", service: "linear-api-key")
# Can also be set via LINEAR_API_KEY env var

[local]
base_dirs = ["~/code"]   # directories to scan for git repos/worktrees

[display]
max_items = 20
editor = "cursor"        # command to open folders; falls back to $VISUAL → $EDITOR → "open"
```

## Building and running

```bash
make build               # compile (outputs ./nxt)
make install             # build and install to $GOPATH/bin (with version from git)
make run                 # build and run
./nxt                    # interactive TUI
./nxt --json             # JSON output
./nxt --no-cache         # fresh fetch
./nxt --debug            # debug info to stderr
./nxt --setup            # re-run setup wizard
./nxt --version          # print version
./nxt --update           # self-update from latest GitHub release
```

## Build and release workflow

### When to build locally

After any code change, run `make install` to update the local binary. This ensures the user is always running the latest version during development. Do this:
- After finishing a feature or bug fix (post-commit)
- When the user asks to try or test a change
- As part of session completion (see Landing the Plane)

### When to cut a release

Cut a release with `make release V=x.y.z` when:
- A meaningful set of changes has landed on main (new feature, significant fix)
- The user explicitly asks for a release
- **Do NOT release for docs-only, config-only, or trivial changes**

Versioning follows semver:
- **Patch** (0.1.x): bug fixes, small improvements
- **Minor** (0.x.0): new features, new flags, new data sources
- **Major** (x.0.0): breaking config changes, removed flags

Before releasing, always:
1. Run `make audit` — all checks must pass
2. Verify the version tag doesn't already exist
3. Push all commits to main first

`make release` tags and pushes; GitHub Actions handles the rest (cross-compile + upload binaries).

### After pushing to main

Always run `make install` after pushing so the local binary matches what's on main. If a release was cut, verify it with `nxt --version`.

## Quality gates

**Before committing any code change, run:**
```bash
make audit    # runs: go vet, golangci-lint, go test -race, govulncheck
```

### Linting

golangci-lint v2 is configured in `.golangci.yml` with 15 linters. Key points:
- Run `make lint` to check, `make fmt` to auto-format
- Bubbletea-specific exclusions are pre-configured (hugeParam, ireturn on tea types, unexported-return)
- Use `//nolint:lintername // reason` sparingly and always with a reason
- golangci-lint and govulncheck are pinned as Go tool deps in `go.mod` — no separate install needed

### Testing

- Use stdlib `testing` with table-driven tests and subtests (see `scorer/scorer_test.go` for the pattern)
- Always run tests with `-race` (the Makefile does this by default)
- Use `model.*` constants (e.g., `model.CIFailing`, `model.ReviewApproved`) instead of raw strings
- Check coverage with `make cover` — generates `coverage.html`
- Source fetchers (`source/*.go`) shell out to `gh`/`git`; test the parsing/conversion functions, not the fetch functions

### Pre-commit hooks

lefthook runs `gofmt`, `go vet`, and `go test -short` on every commit. Install with:
```bash
brew install lefthook && lefthook install --force
```

### CI

GitHub Actions runs three parallel jobs on push to main and PRs: test (with race detector), lint (golangci-lint), and vuln (govulncheck).

## Conventions

- All source fetching happens in `source/` — each source is independent and returns model types.
- Linking and scoring are pure functions over model types — no I/O.
- The `render` package owns all terminal output. `Render()` is for non-interactive; `RunInteractive()` is the TUI.
- Errors from sources are collected as warnings, not fatal — nxt gracefully degrades when a source is unavailable.
- Secrets never touch disk — Linear API key goes to macOS Keychain only.
- Colors use `lipgloss.AdaptiveColor` for light/dark terminal support.

## Issue tracking

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work atomically
bd close <id>         # Complete work
bd dolt push          # Push beads data to remote
```

## Non-Interactive Shell Commands

**ALWAYS use non-interactive flags** with file operations to avoid hanging on confirmation prompts.

Shell commands like `cp`, `mv`, and `rm` may be aliased to include `-i` (interactive) mode on some systems, causing the agent to hang indefinitely waiting for y/n input.

**Use these forms instead:**
```bash
# Force overwrite without prompting
cp -f source dest           # NOT: cp source dest
mv -f source dest           # NOT: mv source dest
rm -f file                  # NOT: rm file

# For recursive operations
rm -rf directory            # NOT: rm -r directory
cp -rf source dest          # NOT: cp -r source dest
```

**Other commands that may prompt:**
- `scp` - use `-o BatchMode=yes` for non-interactive
- `ssh` - use `-o BatchMode=yes` to fail instead of prompting
- `apt-get` - use `-y` flag
- `brew` - use `HOMEBREW_NO_AUTO_UPDATE=1` env var

<!-- BEGIN BEADS INTEGRATION profile:full hash:d4f96305 -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Dolt-powered version control with native sync
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update <id> --claim --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task atomically**: `bd update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Auto-Sync

bd automatically syncs via Dolt:

- Each write auto-commits to Dolt history
- Use `bd dolt push`/`bd dolt pull` for remote sync
- No manual export/import needed!

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and docs/QUICKSTART.md.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Install locally** (if code changed):
   ```bash
   make install
   nxt --version  # verify the binary is up to date
   ```
6. **Clean up** - Clear stashes, prune remote branches
7. **Verify** - All changes committed AND pushed, local binary updated
8. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

<!-- END BEADS INTEGRATION -->
