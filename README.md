# nxt

A terminal dashboard that shows your most urgent work — not what's most important in the abstract, but what will cost you the most if you ignore it another day.

nxt pulls from Linear issues, GitHub PRs, and local git worktrees in parallel, links them together, scores them by urgency, and renders a ranked list.

## Install

Requires Go 1.26+.

```bash
go install github.com/xxE6E6FA/nxt@latest
```

Or build from source:

```bash
git clone https://github.com/xxE6E6FA/nxt.git
cd nxt
make build
```

## Setup

On first run, nxt launches an interactive setup wizard:

1. **Linear API key** — stored in macOS Keychain (never written to disk). Can also be set via `LINEAR_API_KEY`.
2. **Base directories** — folders to scan for git repos and worktrees (e.g. `~/code`).
3. **Editor command** — used to open project folders. Falls back to `$VISUAL` → `$EDITOR` → `open`.

Re-run setup any time with `nxt --setup`.

## Usage

```
nxt                  # interactive TUI (default when TTY)
nxt --json           # JSON output
nxt --no-cache       # bypass cache, fetch fresh data
nxt --debug          # print debug info to stderr
nxt --benchmark      # time each data source (implies --no-cache)
nxt --setup          # re-run setup wizard
```

## How it works

nxt fetches from three sources concurrently — Linear (assigned issues), GitHub (your open PRs via `gh`), and local git worktrees. It links them by branch name and scores each work item by urgency.

The scoring algorithm prioritizes **action-required** over raw importance: CI failing, changes requested, and merge conflicts score highest. Items that are almost done (approved + CI green) bubble up because the cost of finishing is tiny and the cost of forgetting is high. Staleness accrues over time. Priority and cycle context act as multipliers — they amplify urgency but don't create it. Items snoozed in Linear are scored at zero.

See [scorer/SCORING.md](scorer/SCORING.md) for the full algorithm.

Responses are cached with stale-while-revalidate — stale data is served immediately while a background refresh runs. Bypass with `--no-cache`.

## Configuration

Config lives at `~/.config/nxt/config.toml`:

```toml
[linear]
# API key is in macOS Keychain, not here.
# Override with LINEAR_API_KEY env var.

[local]
base_dirs = ["~/code"]

[display]
max_items = 20
editor = "cursor"
```

Label scoring is opt-in — assign Linear/GitHub labels to urgency buckets via the TUI settings screen. No labels are scored by default.

## Requirements

- **Go 1.26+**
- **`gh` CLI** — authenticated (`gh auth login`)
- **`git`**
- **macOS** — Keychain is used for secret storage

## Development

```bash
make test            # run tests with race detector
make lint            # golangci-lint
make audit           # vet + lint + test + govulncheck
```

## License

[MIT](LICENSE)
