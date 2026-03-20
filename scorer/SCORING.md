# nxt Scoring Algorithm

This document defines how nxt computes urgency scores for work items. The goal
is to surface what needs your attention **right now** — not what's most
important in the abstract, but what will cost you the most if you ignore it
another day.

---

## Data sources

Every WorkItem can have up to three linked objects. Scoring draws signals from
all of them.

### Linear issue fields

| Field | Type | Signal |
|---|---|---|
| `priority` | int (0-4) | Base importance weight |
| `dueDate` | date | Deadline pressure |
| `createdAt` | datetime | Issue age |
| `updatedAt` | datetime | Issue staleness |
| `startedAt` | datetime | Time in progress — null if not started |
| `state.name` | string | Current workflow state |
| `state.type` | string | `started`, `unstarted`, `backlog`, `triage` |
| `cycle.startsAt` | datetime | Cycle start — used with endsAt for cycle pressure |
| `cycle.endsAt` | datetime | Cycle end — proximity = urgency |
| `estimate` | float | Story points / complexity |
| `labels[].name` | string | User-configured multipliers (see [Label scoring](#label-scoring)) |
| `slaBreachesAt` | datetime | SLA deadline — hard urgency signal |
| `snoozedUntilAt` | datetime | Suppress scoring until this date |
| `sortOrder` | float | Manual triage order from Linear |

### GitHub PR fields

| Field | Type | Signal |
|---|---|---|
| `createdAt` | datetime | PR age |
| `updatedAt` | datetime | PR staleness |
| `isDraft` | bool | Draft PRs are lower urgency than ready PRs |
| `reviewDecision` | string | `APPROVED`, `CHANGES_REQUESTED`, `REVIEW_REQUIRED` |
| `statusCheckRollup` | array | CI pass/fail/pending |
| `mergeable` | string | `MERGEABLE`, `CONFLICTING`, `UNKNOWN` |
| `mergeStateStatus` | string | `CLEAN`, `DIRTY`, `BLOCKED`, `BEHIND`, `UNSTABLE` |
| `additions` | int | Lines added |
| `deletions` | int | Lines deleted |
| `changedFiles` | int | Files changed |
| `comments` | array | Discussion count |
| `reviewRequests` | array | Pending reviewers |
| `labels[].name` | string | User-configured multipliers (same config as Linear labels) |

### Git worktree fields

| Field | Type | Signal |
|---|---|---|
| `lastCommit` | datetime | Time since last commit — staleness |
| `branch` | string | Used for linking, not scoring |
| `isMain` | bool | Main branches excluded from items |

---

## Scoring model

Score is computed as: **base + signals + multipliers**

The output is a single integer. Higher = more urgent. Each contributing factor
is recorded in the item's `Breakdown` for the detail view.

### 1. Action-required signals (base urgency)

These represent things where **you** need to do something for progress to
continue. They form the base score.

| Signal | Points | Condition |
|---|---|---|
| CI failing | 40 | `ciStatus == "failing"` |
| Changes requested | 35 | `reviewDecision == "changes_requested"` |
| Merge conflict | 30 | `mergeable == "CONFLICTING"` |
| Overdue | 30 | `dueDate < now` |
| SLA breach imminent | 35 | `slaBreachesAt` within 24h |

### 2. Ready-to-close signals

These are items where **almost no work remains**. They should bubble up because
the cost of finishing is tiny and the cost of forgetting is high.

| Signal | Points | Condition |
|---|---|---|
| Ready to merge | 25 | CI passing + approved + mergeable + not draft |
| Approved, CI pending | 10 | `approved` + CI not yet resolved |

### 3. Momentum / staleness signals

Things that were moving but have stalled. Points scale with time.

| Signal | Points | Condition |
|---|---|---|
| Stale WIP (worktree) | up to 20 | Last commit >3 days ago, scales to cap at 7+ days |
| Stale WIP (Linear) | up to 20 | `startedAt` >7 days ago with no PR yet |
| Stale PR | up to 15 | `pr.updatedAt` >3 days ago, scales to cap at 7+ days |
| Draft PR aging | up to 10 | Draft + `pr.createdAt` >5 days ago |
| Review pending (waiting) | up to 10 | `review_required` + `pr.updatedAt` >2 days |

### 4. Context multipliers

These don't generate urgency on their own but amplify other signals.

| Factor | Multiplier | Condition |
|---|---|---|
| Urgent priority | 1.5x | `priority == 1` |
| High priority | 1.2x | `priority == 2` |
| In current cycle | 1.15x | `cycle != nil` |
| Cycle ending soon | 1.3x | Cycle ends within 2 days |
| Has Linear issue | 1.1x | Tracked work > orphan PRs |
| Label (user-configured) | configurable | See [Label scoring](#label-scoring) below |
| Large PR | 1.1x | `additions + deletions > 500` or `changedFiles > 20` |

### 5. Suppressors

These reduce or zero-out the score.

| Factor | Effect | Condition |
|---|---|---|
| Snoozed | Score = 0 | `snoozedUntilAt > now` |
| Draft (no action signals) | 0.5x | `isDraft` and no CI failure or changes requested |

---

## Score computation pseudocode

```
func scoreItem(item) -> (score, breakdown):
    if item.issue.snoozedUntilAt > now:
        return (0, [{label: "Snoozed", points: 0, detail: "until <date>"}])

    base = 0
    factors = []

    // 1. Action-required signals
    if pr.ciStatus == "failing":       base += 40; record("CI failing", 40)
    if pr.review == "changes_req":     base += 35; record("Changes requested", 35)
    if pr.mergeable == "CONFLICTING":  base += 30; record("Merge conflict", 30)
    if issue.dueDate < now:            base += 30; record("Overdue", 30)
    if issue.slaBreachesAt within 24h: base += 35; record("SLA breach", 35)

    // 2. Ready-to-close
    if pr.ci == "passing" && pr.review == "approved" && pr.mergeable == "MERGEABLE" && !pr.isDraft:
        base += 25; record("Ready to merge", 25)
    else if pr.review == "approved" && pr.ci == "pending":
        base += 10; record("Approved, CI pending", 10)

    // 3. Momentum / staleness (time-scaled)
    if worktree.lastCommit > 3 days:   base += scale(3..7 days, 0..20)
    if issue.startedAt > 7 days && no PR: base += scale(7..21 days, 0..20)
    if pr.updatedAt > 3 days:          base += scale(3..7 days, 0..15)
    if pr.isDraft && pr.createdAt > 5d: base += scale(5..14 days, 0..10)
    if pr.review == "required" && pr.updatedAt > 2d: base += scale(2..7 days, 0..10)

    // 4. Context multipliers
    multiplier = 1.0
    if issue.priority == 1: multiplier *= 1.5
    if issue.priority == 2: multiplier *= 1.2
    if issue.inCycle:       multiplier *= 1.15
    if cycleEndsWithin(2d): multiplier *= 1.3
    if issue != nil:        multiplier *= 1.1
    for label in item.labels:
        if label in config.scoring.labels:
            multiplier *= config.scoring.labels[label]
    if largePR(pr):         multiplier *= 1.1

    // 5. Suppressors
    if pr.isDraft && base < 30: multiplier *= 0.5

    score = int(float(base) * multiplier)
    return (score, factors)
```

---

## Label scoring

Linear labels vary across workspaces — team names, workflows, conventions all
differ. Rather than guessing which labels matter, nxt uses a bucket system
configured through the settings UI.

**No labels are scored by default.** If you don't configure any, labels are
ignored entirely. This is intentional — the priority field already covers base
importance.

### Buckets

Labels are assigned to one of three named tiers:

| Bucket | Multiplier | Purpose |
|---|---|---|
| **Critical** | 1.5x | Drop everything — blockers, outages, SEVs |
| **Boost** | 1.2x | Important but not an emergency — bugs, regressions |
| **Dampen** | 0.7x | Low urgency — tech debt, chores, nice-to-haves |

Unassigned labels have no effect (implicit 1.0x multiplier).

### Configuration

In `~/.config/nxt/config.toml`:

```toml
[scoring.labels]
critical = ["Blocker", "SEV-1"]
boost    = ["Bug", "Regression"]
dampen   = ["Tech Debt", "Chore"]
```

Label matching is **case-insensitive**. Both Linear issue labels and GitHub PR
labels are checked. If a label appears in both, the multiplier applies only
once.

### Settings UI

Labels are configured through a sub-screen in the TUI settings (`s` → enter on
"label rules"). The main settings screen shows only a summary line:

```
label rules: 3 configured
```

The sub-screen shows current bucket assignments as a compact read-only list,
plus a search input. Typing filters against the Linear workspace labels API
(`issueLabels` with `containsIgnoreCase`). Search results show the label name,
its Linear color dot, and scope (workspace vs team name).

Assigning: select a label, press `1` (critical), `2` (boost), or `3` (dampen).
Removing: `d` on an assigned label. Changes save to config immediately.

No upfront fetch — the label list only loads when you start typing in the
search field.

### Why opt-in?

- "Blocker" in one workspace might mean "blocks a release." In another it might
  be a stale label nobody cleans up.
- Label proliferation is common — scoring all of them adds noise.
- The priority field (Urgent/High/Medium/Low) is standardized and already
  provides strong signal. Labels are for fine-tuning on top.

---

## Breakdown display

The detail view (`d` key in TUI) shows:

```
  ENG-123  Implement new auth flow
  Score: 72

  +40  CI failing           PR #456 has failing checks
  +15  Stale PR             Last updated 5 days ago
  ×1.2 High priority        Linear priority: High
  ×1.15 In cycle            Sprint ends in 4 days
```

Multipliers are shown separately from additive factors so the user understands
both the raw urgency and the amplification.

---

## Design principles

1. **Action-required beats importance.** A medium-priority item with failing CI
   scores higher than an urgent-priority item in backlog. The question is "what
   should I do next?" not "what matters most?"

2. **Almost-done items bubble up.** Finishing a PR that's approved + CI green
   costs 30 seconds. Letting it sit costs context and risks conflicts.

3. **Staleness compounds.** The longer something sits, the worse it gets. Stale
   items accrue points over time, not just a binary flag.

4. **Multipliers amplify, not create.** Priority and cycle context make existing
   urgency louder, but a P1 backlog item with no activity still scores low.

5. **Transparency.** Every point is explained in the breakdown. No magic numbers
   without rationale.

6. **Snooze is respected.** If you snoozed it in Linear, nxt won't nag you.
