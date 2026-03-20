package model

import "time"

// CI status values for PullRequest.CIStatus.
const (
	CIPassing = "passing"
	CIFailing = "failing"
	CIPending = "pending"
)

// Review state values for PullRequest.ReviewState.
const (
	ReviewApproved         = "approved"
	ReviewChangesRequested = "changes_requested"
	ReviewRequired         = "review_required"
)

// GitHub check state values (from StatusCheckRollup / CheckRun).
const (
	CheckStateSuccess = "SUCCESS"
	CheckStateFailure = "FAILURE"
	CheckStateError   = "ERROR"
)

// Mergeable status values for PullRequest.Mergeable.
const (
	MergeableMergeable   = "MERGEABLE"
	MergeableConflicting = "CONFLICTING"
	MergeableUnknown     = "UNKNOWN"
)

// Merge state status values for PullRequest.MergeStateStatus.
const (
	MergeStateClean   = "CLEAN"
	MergeStateDirty   = "DIRTY"
	MergeStateBlocked = "BLOCKED"
)

// LinearIssue represents a Linear issue assigned to the user.
type LinearIssue struct {
	ID             string     `json:"id"`
	Identifier     string     `json:"identifier"` // e.g. "DISCO-123"
	Title          string     `json:"title"`
	Status         string     `json:"status"`
	Priority       int        `json:"priority"` // 0=none, 1=urgent, 2=high, 3=medium, 4=low
	BranchName     string     `json:"branchName"`
	DueDate        *time.Time `json:"dueDate,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	Labels         []string   `json:"labels,omitempty"`
	Estimate       *float64   `json:"estimate,omitempty"`
	SLABreachesAt  *time.Time `json:"slaBreachesAt,omitempty"`
	SnoozedUntilAt *time.Time `json:"snoozedUntilAt,omitempty"`
	SortOrder      float64    `json:"sortOrder"`
	CycleID        string     `json:"cycleId,omitempty"`
	CycleStartDate *time.Time `json:"cycleStartDate,omitempty"`
	CycleEndDate   *time.Time `json:"cycleEndDate,omitempty"`
	InCycle        bool       `json:"inCycle"`
	URL            string     `json:"url"`
	PRURLs         []string   `json:"prUrls,omitempty"` // GitHub PR URLs from Linear attachments
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number           int       `json:"number"`
	Title            string    `json:"title"`
	HeadBranch       string    `json:"headRefName"`
	Repo             string    `json:"repo"` // "owner/name"
	URL              string    `json:"url"`
	State            string    `json:"state"`
	IsDraft          bool      `json:"isDraft"`
	CIStatus         string    `json:"ciStatus"`    // CIPassing, CIFailing, CIPending, or ""
	ReviewState      string    `json:"reviewState"` // ReviewApproved, ReviewChangesRequested, ReviewRequired, or ""
	Body             string    `json:"body"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Additions        int       `json:"additions"`
	Deletions        int       `json:"deletions"`
	ChangedFiles     int       `json:"changedFiles"`
	Mergeable        string    `json:"mergeable"`        // MergeableMergeable, MergeableConflicting, MergeableUnknown
	MergeStateStatus string    `json:"mergeStateStatus"` // MergeStateClean, MergeStateDirty, MergeStateBlocked, etc.
	Comments         int       `json:"comments"`
	ReviewRequests   int       `json:"reviewRequests"`
	Labels           []string  `json:"labels,omitempty"`
}

// Worktree represents a local git worktree.
type Worktree struct {
	Path       string    `json:"path"`
	Branch     string    `json:"branch"`
	RepoRoot   string    `json:"repoRoot"`
	LastCommit time.Time `json:"lastCommit"`
	IsMain     bool      `json:"isMain"` // true if main/master branch
}

// ScoreFactor is a single contributing factor to the urgency score.
type ScoreFactor struct {
	Label  string `json:"label"`
	Points int    `json:"points"`
	Detail string `json:"detail,omitempty"` // e.g. "2 days until due"
}

// WorkItem is the unified domain object after linking.
type WorkItem struct {
	Issue     *LinearIssue  `json:"issue,omitempty"`
	PR        *PullRequest  `json:"pr,omitempty"`
	Worktree  *Worktree     `json:"worktree,omitempty"`
	Score     int           `json:"score"`
	Breakdown []ScoreFactor `json:"breakdown,omitempty"`
}
