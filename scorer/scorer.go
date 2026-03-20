package scorer

import (
	"fmt"
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

// Score computes urgency scores for all work items.
func Score(items []model.WorkItem) {
	now := time.Now()
	for i := range items {
		items[i].Score, items[i].Breakdown = scoreItem(&items[i], now)
	}
}

func scoreItem(item *model.WorkItem, now time.Time) (int, []model.ScoreFactor) {
	score := 0
	var factors []model.ScoreFactor

	add := func(label string, points int, detail string) {
		score += points
		factors = append(factors, model.ScoreFactor{Label: label, Points: points, Detail: detail})
	}

	// PR-based signals
	if pr := item.PR; pr != nil {
		if pr.CIStatus == "failing" {
			add("CI failing", 40, fmt.Sprintf("PR #%d has failing checks", pr.Number))
		}
		if pr.ReviewState == "changes_requested" {
			add("Changes requested", 35, fmt.Sprintf("PR #%d needs revision", pr.Number))
		}
	}

	// Issue-based signals
	if issue := item.Issue; issue != nil {
		// Deadline proximity (up to +30, linear scale over 7 days)
		if issue.DueDate != nil {
			daysUntil := issue.DueDate.Sub(now).Hours() / 24
			if daysUntil <= 7 {
				var pts int
				var detail string
				if daysUntil <= 0 {
					pts = 30
					detail = fmt.Sprintf("Overdue by %d days", int(-daysUntil))
				} else {
					pts = int(30 * (1 - daysUntil/7))
					detail = fmt.Sprintf("Due in %d days", int(daysUntil))
				}
				if pts > 0 {
					add("Deadline", pts, detail)
				}
			}
		}

		// Priority
		switch issue.Priority {
		case 1:
			add("Urgent priority", 25, "Linear priority: Urgent")
		case 2:
			add("High priority", 15, "Linear priority: High")
		case 3:
			add("Medium priority", 5, "Linear priority: Medium")
		}

		// In current cycle
		if issue.InCycle {
			add("In cycle", 10, "Part of the current sprint/cycle")
		}

		// No branch/worktree yet
		if item.Worktree == nil && item.PR == nil {
			add("No branch yet", 8, "No worktree or PR — needs to be started")
		}
	}

	// Staleness (up to +20 based on days since last commit, max at 7+ days)
	if wt := item.Worktree; wt != nil && !wt.LastCommit.IsZero() {
		daysSince := now.Sub(wt.LastCommit).Hours() / 24
		if daysSince > 3 {
			staleScore := int((daysSince - 3) / 4 * 20)
			if staleScore > 20 {
				staleScore = 20
			}
			if staleScore > 0 {
				add("Stale branch", staleScore, fmt.Sprintf("Last commit %d days ago", int(daysSince)))
			}
		}
	}

	return score, factors
}
