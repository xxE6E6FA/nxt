package model

import "time"

// LinearIssue represents a Linear issue assigned to the user.
type LinearIssue struct {
	ID         string    `json:"id"`
	Identifier string    `json:"identifier"` // e.g. "DISCO-123"
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Priority   int       `json:"priority"` // 0=none, 1=urgent, 2=high, 3=medium, 4=low
	BranchName string    `json:"branchName"`
	DueDate    *time.Time `json:"dueDate,omitempty"`
	CycleID    string    `json:"cycleId,omitempty"`
	InCycle    bool      `json:"inCycle"`
	URL        string    `json:"url"`
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number     int       `json:"number"`
	Title      string    `json:"title"`
	HeadBranch string    `json:"headRefName"`
	Repo       string    `json:"repo"` // "owner/name"
	URL        string    `json:"url"`
	State      string    `json:"state"`
	IsDraft    bool      `json:"isDraft"`
	CIStatus   string    `json:"ciStatus"`   // "passing", "failing", "pending", ""
	ReviewState string   `json:"reviewState"` // "approved", "changes_requested", "review_required", ""
	Body       string    `json:"body"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Worktree represents a local git worktree.
type Worktree struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	RepoRoot   string `json:"repoRoot"`
	LastCommit time.Time `json:"lastCommit"`
	IsMain     bool   `json:"isMain"` // true if main/master branch
}

// WorkItem is the unified domain object after linking.
type WorkItem struct {
	Issue    *LinearIssue  `json:"issue,omitempty"`
	PR       *PullRequest  `json:"pr,omitempty"`
	Worktree *Worktree     `json:"worktree,omitempty"`
	Score    int           `json:"score"`
}
