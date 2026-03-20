package linker

import (
	"strings"

	"github.com/xxE6E6FA/nxt/model"
)

// Link correlates Linear issues with worktrees and PRs, producing WorkItems.
// Issues are the primary axis — each issue becomes a WorkItem.
// PRs and worktrees not matched to any issue are added as standalone items.
// repoMap maps "owner/name" → local repo root path for fallback linking via PR repo.
func Link(issues []model.LinearIssue, prs []model.PullRequest, worktrees []model.Worktree, repoMap map[string]string) []model.WorkItem {
	prUsed := make(map[int]bool)
	wtUsed := make(map[string]bool)

	var items []model.WorkItem

	for i := range issues {
		issue := &issues[i]
		item := model.WorkItem{Issue: issue}

		// Find matching worktree
		for j := range worktrees {
			wt := &worktrees[j]
			if wt.IsMain {
				continue
			}
			if matchBranch(issue, wt.Branch) {
				item.Worktree = wt
				wtUsed[wt.Path] = true
				break
			}
		}

		// Find matching PR
		for j := range prs {
			pr := &prs[j]
			if matchPR(issue, pr) {
				item.PR = pr
				prUsed[pr.Number] = true
				break
			}
		}

		// Fallback: if we have a PR but no worktree, use the repo map to find
		// the local folder for the PR's repo.
		if item.Worktree == nil && item.PR != nil && item.PR.Repo != "" {
			if localPath, ok := repoMap[item.PR.Repo]; ok {
				item.Worktree = &model.Worktree{
					Path:     localPath,
					Branch:   item.PR.HeadBranch,
					RepoRoot: localPath,
				}
			}
		}

		items = append(items, item)
	}

	// Add unmatched PRs as standalone items, with repo-based folder linking
	for i := range prs {
		pr := &prs[i]
		if !prUsed[pr.Number] {
			item := model.WorkItem{PR: pr}
			if pr.Repo != "" {
				if localPath, ok := repoMap[pr.Repo]; ok {
					item.Worktree = &model.Worktree{
						Path:     localPath,
						Branch:   pr.HeadBranch,
						RepoRoot: localPath,
					}
				}
			}
			items = append(items, item)
		}
	}

	return items
}

// matchBranch checks if a worktree branch matches a Linear issue.
func matchBranch(issue *model.LinearIssue, branch string) bool {
	lower := strings.ToLower(branch)
	idLower := strings.ToLower(issue.Identifier)

	// Primary: exact branchName match
	if issue.BranchName != "" && strings.ToLower(issue.BranchName) == lower {
		return true
	}

	// Fallback: issue ID as substring
	return strings.Contains(lower, idLower)
}

// matchPR checks if a PR matches a Linear issue.
func matchPR(issue *model.LinearIssue, pr *model.PullRequest) bool {
	// Primary: direct URL match from Linear attachments
	for _, url := range issue.PRURLs {
		if url == pr.URL {
			return true
		}
	}

	idLower := strings.ToLower(issue.Identifier)

	// Check head branch
	if matchBranch(issue, pr.HeadBranch) {
		return true
	}

	// Check PR title and body for issue ID
	if strings.Contains(strings.ToLower(pr.Title), idLower) {
		return true
	}
	if strings.Contains(strings.ToLower(pr.Body), idLower) {
		return true
	}

	return false
}
