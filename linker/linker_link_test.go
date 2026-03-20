package linker

import (
	"testing"

	"github.com/xxE6E6FA/nxt/model"
)

func TestLinkEmptyInputs(t *testing.T) {
	items := Link(nil, nil, nil, nil)
	if len(items) != 0 {
		t.Errorf("Link(nil, nil, nil, nil) returned %d items, want 0", len(items))
	}
}

func TestLinkIssueOnly(t *testing.T) {
	issues := []model.LinearIssue{{Identifier: "ENG-1", Title: "Solo issue"}}
	items := Link(issues, nil, nil, nil)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Issue == nil {
		t.Fatal("expected issue to be set")
	}
	if items[0].Issue.Identifier != "ENG-1" {
		t.Errorf("issue identifier = %q, want ENG-1", items[0].Issue.Identifier)
	}
	if items[0].PR != nil {
		t.Error("expected PR to be nil for issue-only item")
	}
	if items[0].Worktree != nil {
		t.Error("expected Worktree to be nil for issue-only item")
	}
}

func TestLinkAllUnmatched(t *testing.T) {
	issues := []model.LinearIssue{{Identifier: "ENG-1"}}
	prs := []model.PullRequest{{Number: 42, HeadBranch: "unrelated-branch", URL: "https://github.com/org/repo/pull/42"}}
	worktrees := []model.Worktree{{Path: "/code/other", Branch: "other-branch"}}

	items := Link(issues, prs, worktrees, nil)

	// Issue becomes a standalone item; PR has no matching worktree branch → not included
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].PR != nil {
		t.Error("issue should not have matched the unrelated PR")
	}
	if items[0].Worktree != nil {
		t.Error("issue should not have matched the unrelated worktree")
	}
}

func TestLinkUnmatchedPRWithMatchingWorktree(t *testing.T) {
	// PR not matched to any issue, but has a worktree on the same branch
	prs := []model.PullRequest{{
		Number:     10,
		HeadBranch: "feature-x",
		URL:        "https://github.com/org/repo/pull/10",
	}}
	worktrees := []model.Worktree{{
		Path:   "/code/feature-x",
		Branch: "feature-x",
	}}

	items := Link(nil, prs, worktrees, nil)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].PR == nil {
		t.Fatal("expected PR to be set for unmatched-PR-with-worktree item")
	}
	if items[0].Worktree == nil {
		t.Fatal("expected Worktree to be set")
	}
	if items[0].Worktree.Branch != "feature-x" {
		t.Errorf("worktree branch = %q, want feature-x", items[0].Worktree.Branch)
	}
	if items[0].Issue != nil {
		t.Error("expected Issue to be nil")
	}
}

func TestLinkUnmatchedPRSkippedWhenNoWorktree(t *testing.T) {
	// Unmatched PR without a local worktree on the same branch → not included
	prs := []model.PullRequest{{
		Number:     10,
		HeadBranch: "feature-x",
		URL:        "https://github.com/org/repo/pull/10",
	}}

	items := Link(nil, prs, nil, nil)

	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (unmatched PR without worktree should be excluded)", len(items))
	}
}

func TestLinkUnmatchedPRSkipsMainBranch(t *testing.T) {
	prs := []model.PullRequest{{
		Number:     10,
		HeadBranch: "feature-x",
		URL:        "https://github.com/org/repo/pull/10",
	}}
	worktrees := []model.Worktree{{
		Path:   "/code/repo",
		Branch: "main",
		IsMain: true,
	}}

	items := Link(nil, prs, worktrees, nil)

	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (main worktree should not match unmatched PRs)", len(items))
	}
}

func TestLinkUnmatchedPRSkipsEmptyHeadBranch(t *testing.T) {
	prs := []model.PullRequest{{
		Number:     10,
		HeadBranch: "",
		URL:        "https://github.com/org/repo/pull/10",
	}}
	worktrees := []model.Worktree{{
		Path:   "/code/feature",
		Branch: "",
	}}

	items := Link(nil, prs, worktrees, nil)

	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (empty HeadBranch PRs should be skipped)", len(items))
	}
}

func TestLinkRepoMapFallback(t *testing.T) {
	// Issue has a matching PR but no worktree on the branch.
	// repoMap provides the local path for the PR's repo.
	issues := []model.LinearIssue{{
		Identifier: "ENG-5",
		PRURLs:     []string{"https://github.com/org/repo/pull/55"},
	}}
	prs := []model.PullRequest{{
		Number:     55,
		HeadBranch: "eng-5-fix",
		Repo:       "org/repo",
		URL:        "https://github.com/org/repo/pull/55",
	}}
	repoMap := map[string]string{"org/repo": "/code/repo"}

	items := Link(issues, prs, nil, repoMap)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Worktree == nil {
		t.Fatal("expected Worktree to be set via repoMap fallback")
	}
	if items[0].Worktree.Path != "/code/repo" {
		t.Errorf("worktree path = %q, want /code/repo", items[0].Worktree.Path)
	}
	if items[0].Worktree.Branch != "eng-5-fix" {
		t.Errorf("worktree branch = %q, want eng-5-fix", items[0].Worktree.Branch)
	}
}

func TestLinkRepoMapNotUsedWhenWorktreeExists(t *testing.T) {
	issues := []model.LinearIssue{{
		Identifier: "ENG-5",
		BranchName: "eng-5-fix",
	}}
	prs := []model.PullRequest{{
		Number:     55,
		HeadBranch: "eng-5-fix",
		Repo:       "org/repo",
		URL:        "https://github.com/org/repo/pull/55",
	}}
	worktrees := []model.Worktree{{
		Path:   "/code/worktrees/eng-5-fix",
		Branch: "eng-5-fix",
	}}
	repoMap := map[string]string{"org/repo": "/code/repo"}

	items := Link(issues, prs, worktrees, repoMap)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	// Should use the actual worktree, not the repoMap fallback
	if items[0].Worktree.Path != "/code/worktrees/eng-5-fix" {
		t.Errorf("worktree path = %q, want /code/worktrees/eng-5-fix (real worktree preferred over repoMap)", items[0].Worktree.Path)
	}
}

func TestLinkUsedWorktreeNotReusedByUnmatchedPR(t *testing.T) {
	// Worktree matched to an issue should not also be used by an unmatched PR
	issues := []model.LinearIssue{{
		Identifier: "ENG-1",
		BranchName: "eng-1-fix",
	}}
	prs := []model.PullRequest{{
		Number:     99,
		HeadBranch: "eng-1-fix",
		URL:        "https://github.com/org/repo/pull/99",
	}}
	worktrees := []model.Worktree{{
		Path:   "/code/eng-1-fix",
		Branch: "eng-1-fix",
	}}

	items := Link(issues, prs, worktrees, nil)

	// Issue claims the worktree and PR; the PR should not create a second item
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (worktree already used by issue)", len(items))
	}
}
