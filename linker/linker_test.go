package linker

import (
	"testing"

	"github.com/xxE6E6FA/nxt/model"
)

func TestContainsWord(t *testing.T) {
	tests := []struct {
		text string
		word string
		want bool
	}{
		// Exact match
		{"DISCO-123", "DISCO-123", true},
		// Delimited by hyphens
		{"disco-123-fix-crash", "DISCO-123", true},
		// Delimited by slashes
		{"feature/disco-123/wip", "DISCO-123", true},
		// At start
		{"disco-123-refactor", "DISCO-123", true},
		// At end
		{"fix-disco-123", "DISCO-123", true},
		// Must NOT match longer numeric suffix
		{"disco-1234-main-refactor", "DISCO-123", false},
		// Must NOT match longer prefix
		{"xdisco-123-fix", "DISCO-123", false},
		// Case insensitive
		{"Feature/Disco-123-Fix", "disco-123", true},
		// No match at all
		{"feature/other-456", "DISCO-123", false},
		// Empty strings
		{"", "DISCO-123", false},
		{"disco-123", "", true}, // empty word matches everywhere
	}

	for _, tt := range tests {
		t.Run(tt.text+"_"+tt.word, func(t *testing.T) {
			got := containsWord(tt.text, tt.word)
			if got != tt.want {
				t.Errorf("containsWord(%q, %q) = %v, want %v", tt.text, tt.word, got, tt.want)
			}
		})
	}
}

func TestMatchBranch(t *testing.T) {
	tests := []struct {
		name   string
		issue  model.LinearIssue
		branch string
		want   bool
	}{
		{
			name:   "exact branchName match",
			issue:  model.LinearIssue{Identifier: "DISCO-10", BranchName: "fix-crash"},
			branch: "fix-crash",
			want:   true,
		},
		{
			name:   "branchName case insensitive",
			issue:  model.LinearIssue{Identifier: "DISCO-10", BranchName: "Fix-Crash"},
			branch: "fix-crash",
			want:   true,
		},
		{
			name:   "identifier word match",
			issue:  model.LinearIssue{Identifier: "DISCO-10"},
			branch: "disco-10-fix-thing",
			want:   true,
		},
		{
			name:   "identifier must not match longer ID",
			issue:  model.LinearIssue{Identifier: "DISCO-10"},
			branch: "disco-100-fix-thing",
			want:   false,
		},
		{
			name:   "identifier must not match longer ID (suffix)",
			issue:  model.LinearIssue{Identifier: "DISCO-123"},
			branch: "disco-1234-branch",
			want:   false,
		},
		{
			name:   "no match",
			issue:  model.LinearIssue{Identifier: "DISCO-123"},
			branch: "main",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchBranch(&tt.issue, tt.branch)
			if got != tt.want {
				t.Errorf("matchBranch(%v, %q) = %v, want %v", tt.issue.Identifier, tt.branch, got, tt.want)
			}
		})
	}
}

func TestMatchPR(t *testing.T) {
	tests := []struct {
		name  string
		issue model.LinearIssue
		pr    model.PullRequest
		want  bool
	}{
		{
			name:  "URL match",
			issue: model.LinearIssue{Identifier: "X-1", PRURLs: []string{"https://github.com/org/repo/pull/42"}},
			pr:    model.PullRequest{URL: "https://github.com/org/repo/pull/42", Number: 42},
			want:  true,
		},
		{
			name:  "head branch match",
			issue: model.LinearIssue{Identifier: "DISCO-5"},
			pr:    model.PullRequest{HeadBranch: "disco-5-fix", Number: 1},
			want:  true,
		},
		{
			name:  "title word match",
			issue: model.LinearIssue{Identifier: "DISCO-5"},
			pr:    model.PullRequest{Title: "[DISCO-5] Fix crash", Number: 2},
			want:  true,
		},
		{
			name:  "title must not match longer ID",
			issue: model.LinearIssue{Identifier: "DISCO-5"},
			pr:    model.PullRequest{Title: "DISCO-55 refactor", Number: 3},
			want:  false,
		},
		{
			name:  "body word match",
			issue: model.LinearIssue{Identifier: "DISCO-5"},
			pr:    model.PullRequest{Title: "unrelated", Body: "Fixes DISCO-5 by adding check", Number: 4},
			want:  true,
		},
		{
			name:  "body must not match longer ID",
			issue: model.LinearIssue{Identifier: "DISCO-5"},
			pr:    model.PullRequest{Title: "unrelated", Body: "Related to DISCO-50", Number: 5},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPR(&tt.issue, &tt.pr)
			if got != tt.want {
				t.Errorf("matchPR(%v, PR#%d) = %v, want %v", tt.issue.Identifier, tt.pr.Number, got, tt.want)
			}
		})
	}
}

func TestLinkNoFalsePositiveSiblingWorktrees(t *testing.T) {
	issues := []model.LinearIssue{
		{Identifier: "DISCO-123"},
		{Identifier: "DISCO-1234"},
	}
	worktrees := []model.Worktree{
		{Path: "/code/disco-1234-refactor", Branch: "disco-1234-refactor"},
		{Path: "/code/disco-123-fix", Branch: "disco-123-fix"},
	}

	items := Link(issues, nil, worktrees, nil)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// DISCO-123 should match disco-123-fix, NOT disco-1234-refactor
	if items[0].Worktree == nil {
		t.Fatal("DISCO-123 should have a worktree")
	}
	if items[0].Worktree.Branch != "disco-123-fix" {
		t.Errorf("DISCO-123 matched branch %q, want %q", items[0].Worktree.Branch, "disco-123-fix")
	}

	// DISCO-1234 should match disco-1234-refactor
	if items[1].Worktree == nil {
		t.Fatal("DISCO-1234 should have a worktree")
	}
	if items[1].Worktree.Branch != "disco-1234-refactor" {
		t.Errorf("DISCO-1234 matched branch %q, want %q", items[1].Worktree.Branch, "disco-1234-refactor")
	}
}
