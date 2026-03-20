package linker

import (
	"strings"
	"unicode"

	"github.com/xxE6E6FA/nxt/model"
)

// containsWord reports whether text contains word as a whole token,
// bounded by start/end of string or non-alphanumeric characters.
// This prevents "DISCO-123" from matching "disco-1234-branch".
func containsWord(text, word string) bool {
	if word == "" {
		return true
	}
	tl := strings.ToLower(text)
	wl := strings.ToLower(word)
	wLen := len(wl)

	for i := 0; ; {
		idx := strings.Index(tl[i:], wl)
		if idx < 0 {
			return false
		}
		pos := i + idx
		start := pos == 0 || !isAlnum(rune(tl[pos-1]))
		end := pos+wLen == len(tl) || !isAlnum(rune(tl[pos+wLen]))
		if start && end {
			return true
		}
		i = pos + 1
	}
}

func isAlnum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

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

	// Add unmatched PRs only if a local worktree is checked out on the
	// same branch. One branch → one folder — no cross-repo leaking.
	for i := range prs {
		pr := &prs[i]
		if prUsed[pr.Number] || pr.HeadBranch == "" {
			continue
		}
		for j := range worktrees {
			wt := &worktrees[j]
			if wt.IsMain || wtUsed[wt.Path] {
				continue
			}
			if strings.EqualFold(wt.Branch, pr.HeadBranch) {
				item := model.WorkItem{PR: pr, Worktree: wt}
				items = append(items, item)
				wtUsed[wt.Path] = true
				break
			}
		}
	}

	return items
}

// matchBranch checks if a worktree branch matches a Linear issue.
func matchBranch(issue *model.LinearIssue, branch string) bool {
	lower := strings.ToLower(branch)

	// Primary: exact branchName match
	if issue.BranchName != "" && strings.ToLower(issue.BranchName) == lower {
		return true
	}

	// Fallback: issue ID as whole-word match (not substring)
	return containsWord(branch, issue.Identifier)
}

// matchPR checks if a PR matches a Linear issue.
func matchPR(issue *model.LinearIssue, pr *model.PullRequest) bool {
	// Primary: direct URL match from Linear attachments
	for _, url := range issue.PRURLs {
		if url == pr.URL {
			return true
		}
	}

	// Check head branch
	if matchBranch(issue, pr.HeadBranch) {
		return true
	}

	// Check PR title and body for issue ID (whole-word match)
	if containsWord(pr.Title, issue.Identifier) {
		return true
	}
	if containsWord(pr.Body, issue.Identifier) {
		return true
	}

	return false
}
