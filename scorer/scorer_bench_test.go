package scorer

import (
	"testing"
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

func generateItems(n int) []model.WorkItem {
	now := time.Now()
	items := make([]model.WorkItem, n)
	for i := range items {
		switch i % 4 {
		case 0: // Full item: PR + Issue + Worktree
			items[i] = model.WorkItem{
				Issue: &model.LinearIssue{
					Priority: (i % 4) + 1,
					InCycle:  i%3 == 0,
					DueDate:  timePtr(now.Add(time.Duration(i-n/2) * 24 * time.Hour)),
				},
				PR: &model.PullRequest{
					Number:      i,
					CIStatus:    [...]string{model.CIPassing, model.CIFailing, model.CIPending}[i%3],
					ReviewState: [...]string{model.ReviewApproved, model.ReviewChangesRequested, ""}[i%3],
				},
				Worktree: &model.Worktree{
					LastCommit: now.Add(-time.Duration(i) * 24 * time.Hour),
				},
			}
		case 1: // Issue only
			items[i] = model.WorkItem{
				Issue: &model.LinearIssue{Priority: (i % 4) + 1},
			}
		case 2: // PR only
			items[i] = model.WorkItem{
				PR: &model.PullRequest{Number: i, CIStatus: model.CIFailing},
			}
		case 3: // PR + Worktree
			items[i] = model.WorkItem{
				PR:       &model.PullRequest{Number: i},
				Worktree: &model.Worktree{LastCommit: now.Add(-5 * 24 * time.Hour)},
			}
		}
	}
	return items
}

func timePtr(t time.Time) *time.Time { return &t }

func BenchmarkScoreSmall(b *testing.B) {
	items := generateItems(5)
	b.ResetTimer()
	for b.Loop() {
		Score(items)
	}
}

func BenchmarkScoreMedium(b *testing.B) {
	items := generateItems(20)
	b.ResetTimer()
	for b.Loop() {
		Score(items)
	}
}

func BenchmarkScoreLarge(b *testing.B) {
	items := generateItems(100)
	b.ResetTimer()
	for b.Loop() {
		Score(items)
	}
}
