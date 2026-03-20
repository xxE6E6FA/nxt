package linker

import (
	"fmt"
	"testing"

	"github.com/xxE6E6FA/nxt/model"
)

func generateLinkData(nIssues, nPRs, nWorktrees int) ([]model.LinearIssue, []model.PullRequest, []model.Worktree, map[string]string) {
	issues := make([]model.LinearIssue, nIssues)
	for i := range issues {
		issues[i] = model.LinearIssue{
			Identifier: fmt.Sprintf("TEAM-%d", i+1),
			BranchName: fmt.Sprintf("team-%d-feature", i+1),
		}
	}

	prs := make([]model.PullRequest, nPRs)
	for i := range prs {
		prs[i] = model.PullRequest{
			Number:     i + 1,
			HeadBranch: fmt.Sprintf("team-%d-feature", i+1),
			Title:      fmt.Sprintf("TEAM-%d: implement feature", i+1),
		}
	}

	worktrees := make([]model.Worktree, nWorktrees)
	for i := range worktrees {
		if i < nIssues {
			worktrees[i] = model.Worktree{
				Path:   fmt.Sprintf("/code/repo/team-%d-feature", i+1),
				Branch: fmt.Sprintf("team-%d-feature", i+1),
			}
		} else {
			worktrees[i] = model.Worktree{
				Path:   fmt.Sprintf("/code/repo/other-branch-%d", i),
				Branch: fmt.Sprintf("other-branch-%d", i),
			}
		}
	}

	return issues, prs, worktrees, map[string]string{"org/repo": "/code/repo"}
}

func BenchmarkLinkSmall(b *testing.B) {
	issues, prs, worktrees, repoMap := generateLinkData(5, 5, 10)
	b.ResetTimer()
	for b.Loop() {
		Link(issues, prs, worktrees, repoMap)
	}
}

func BenchmarkLinkMedium(b *testing.B) {
	issues, prs, worktrees, repoMap := generateLinkData(20, 20, 30)
	b.ResetTimer()
	for b.Loop() {
		Link(issues, prs, worktrees, repoMap)
	}
}

func BenchmarkLinkLarge(b *testing.B) {
	issues, prs, worktrees, repoMap := generateLinkData(50, 50, 100)
	b.ResetTimer()
	for b.Loop() {
		Link(issues, prs, worktrees, repoMap)
	}
}

func BenchmarkContainsWord(b *testing.B) {
	cases := []struct {
		name string
		text string
		word string
	}{
		{"BranchMatch", "team-123-feature-branch", "TEAM-123"},
		{"TitleMatch", "TEAM-456: implement new feature", "TEAM-456"},
		{"NoMatch", "team-123-feature-branch", "TEAM-999"},
		{"SubstringReject", "disco-1234-branch", "DISCO-123"},
		{"LongText", "this is a very long pull request title that mentions TEAM-42 somewhere in the middle of it all", "TEAM-42"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				containsWord(tc.text, tc.word)
			}
		})
	}
}
