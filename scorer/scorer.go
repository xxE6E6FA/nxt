package scorer

import (
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

// Score computes urgency scores for all work items.
func Score(items []model.WorkItem) {
	now := time.Now()
	for i := range items {
		items[i].Score = scoreItem(&items[i], now)
	}
}

func scoreItem(item *model.WorkItem, now time.Time) int {
	score := 0

	// PR-based signals
	if pr := item.PR; pr != nil {
		if pr.CIStatus == "failing" {
			score += 40
		}
		if pr.ReviewState == "changes_requested" {
			score += 35
		}
	}

	// Issue-based signals
	if issue := item.Issue; issue != nil {
		// Deadline proximity (up to +30, linear scale over 7 days)
		if issue.DueDate != nil {
			daysUntil := issue.DueDate.Sub(now).Hours() / 24
			if daysUntil <= 7 {
				if daysUntil <= 0 {
					score += 30 // overdue
				} else {
					score += int(30 * (1 - daysUntil/7))
				}
			}
		}

		// Priority
		switch issue.Priority {
		case 1: // urgent
			score += 25
		case 2: // high
			score += 15
		case 3: // medium
			score += 5
		}

		// In current cycle
		if issue.InCycle {
			score += 10
		}

		// No branch/worktree yet
		if item.Worktree == nil && item.PR == nil {
			score += 8
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
			score += staleScore
		}
	}

	return score
}
