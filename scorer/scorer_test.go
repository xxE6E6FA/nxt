package scorer

import (
	"testing"
	"time"

	"github.com/xxE6E6FA/nxt/model"
)

// daysFromNow returns a pointer to a time that is the given number of days
// from now. Negative values produce a time in the past (overdue).
func daysFromNow(days float64) *time.Time {
	t := time.Now().Add(time.Duration(days*24) * time.Hour)
	return &t
}

// daysAgo returns a time that is the given number of days in the past.
func daysAgo(days float64) time.Time {
	return time.Now().Add(-time.Duration(days*24) * time.Hour)
}

func TestScoreEmpty(t *testing.T) {
	// Scoring an empty slice should not panic.
	Score(nil)
	Score([]model.WorkItem{})
}

func TestScoreCIFailing(t *testing.T) {
	items := []model.WorkItem{
		{PR: &model.PullRequest{Number: 1, CIStatus: "failing"}},
	}
	Score(items)

	if items[0].Score != 40 {
		t.Errorf("expected score 40, got %d", items[0].Score)
	}
	if len(items[0].Breakdown) != 1 {
		t.Fatalf("expected 1 factor, got %d", len(items[0].Breakdown))
	}
	if items[0].Breakdown[0].Label != "CI failing" {
		t.Errorf("expected label 'CI failing', got %q", items[0].Breakdown[0].Label)
	}
}

func TestScoreChangesRequested(t *testing.T) {
	items := []model.WorkItem{
		{PR: &model.PullRequest{Number: 2, ReviewState: "changes_requested"}},
	}
	Score(items)

	if items[0].Score != 35 {
		t.Errorf("expected score 35, got %d", items[0].Score)
	}
	if len(items[0].Breakdown) != 1 {
		t.Fatalf("expected 1 factor, got %d", len(items[0].Breakdown))
	}
	if items[0].Breakdown[0].Label != "Changes requested" {
		t.Errorf("expected label 'Changes requested', got %q", items[0].Breakdown[0].Label)
	}
}

func TestScoreDeadline(t *testing.T) {
	tests := []struct {
		name      string
		dueDate   *time.Time
		wantPts   int
		wantLabel string
	}{
		{
			name:      "overdue by 2 days",
			dueDate:   daysFromNow(-2),
			wantPts:   30,
			wantLabel: "Deadline",
		},
		{
			name:      "due today",
			dueDate:   daysFromNow(-0.001), // slightly past to ensure daysUntil <= 0
			wantPts:   30,
			wantLabel: "Deadline",
		},
		{
			name:      "due in 1 day",
			dueDate:   daysFromNow(1),
			wantPts:   25, // int(30 * (1 - 1/7)) = int(25.71) = 25
			wantLabel: "Deadline",
		},
		{
			name:      "due in 3.5 days",
			dueDate:   daysFromNow(3.5),
			wantPts:   15, // int(30 * (1 - 3.5/7)) = int(15) = 15
			wantLabel: "Deadline",
		},
		{
			name:      "due in 7 days",
			dueDate:   daysFromNow(7),
			wantPts:   0, // int(30 * 0) = 0, skipped by pts > 0 check
			wantLabel: "",
		},
		{
			name:      "due in 10 days",
			dueDate:   daysFromNow(10),
			wantPts:   0,
			wantLabel: "",
		},
		{
			name:      "no due date",
			dueDate:   nil,
			wantPts:   0,
			wantLabel: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := []model.WorkItem{
				{
					Issue: &model.LinearIssue{DueDate: tc.dueDate},
					// Provide a PR so "No branch yet" doesn't fire.
					PR: &model.PullRequest{},
				},
			}
			Score(items)

			if items[0].Score != tc.wantPts {
				t.Errorf("expected score %d, got %d", tc.wantPts, items[0].Score)
			}

			if tc.wantLabel == "" {
				if len(items[0].Breakdown) != 0 {
					t.Errorf("expected no breakdown factors, got %v", items[0].Breakdown)
				}
				return
			}
			found := false
			for _, f := range items[0].Breakdown {
				if f.Label == tc.wantLabel {
					found = true
					if f.Points != tc.wantPts {
						t.Errorf("expected %d points for %q, got %d", tc.wantPts, tc.wantLabel, f.Points)
					}
				}
			}
			if !found {
				t.Errorf("expected breakdown factor %q, not found in %v", tc.wantLabel, items[0].Breakdown)
			}
		})
	}
}

func TestScorePriority(t *testing.T) {
	tests := []struct {
		name      string
		priority  int
		wantPts   int
		wantLabel string
	}{
		{"urgent", 1, 25, "Urgent priority"},
		{"high", 2, 15, "High priority"},
		{"medium", 3, 5, "Medium priority"},
		{"low", 4, 0, ""},
		{"none", 0, 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := []model.WorkItem{
				{
					Issue: &model.LinearIssue{Priority: tc.priority},
					// Provide a PR so "No branch yet" doesn't fire.
					PR: &model.PullRequest{},
				},
			}
			Score(items)

			if items[0].Score != tc.wantPts {
				t.Errorf("expected score %d, got %d", tc.wantPts, items[0].Score)
			}

			if tc.wantLabel == "" {
				for _, f := range items[0].Breakdown {
					t.Errorf("unexpected factor: %v", f)
				}
			} else if len(items[0].Breakdown) != 1 || items[0].Breakdown[0].Label != tc.wantLabel {
				t.Errorf("expected label %q, got %v", tc.wantLabel, items[0].Breakdown)
			}
		})
	}
}

func TestScoreInCycle(t *testing.T) {
	items := []model.WorkItem{
		{
			Issue: &model.LinearIssue{InCycle: true},
			// Provide a PR so "No branch yet" doesn't fire.
			PR: &model.PullRequest{},
		},
	}
	Score(items)

	if items[0].Score != 10 {
		t.Errorf("expected score 10, got %d", items[0].Score)
	}
	if len(items[0].Breakdown) != 1 {
		t.Fatalf("expected 1 factor, got %d", len(items[0].Breakdown))
	}
	if items[0].Breakdown[0].Label != "In cycle" {
		t.Errorf("expected label 'In cycle', got %q", items[0].Breakdown[0].Label)
	}
}

func TestScoreNoBranch(t *testing.T) {
	items := []model.WorkItem{
		{
			Issue:    &model.LinearIssue{},
			Worktree: nil,
			PR:       nil,
		},
	}
	Score(items)

	if items[0].Score != 8 {
		t.Errorf("expected score 8, got %d", items[0].Score)
	}
	if len(items[0].Breakdown) != 1 {
		t.Fatalf("expected 1 factor, got %d", len(items[0].Breakdown))
	}
	if items[0].Breakdown[0].Label != "No branch yet" {
		t.Errorf("expected label 'No branch yet', got %q", items[0].Breakdown[0].Label)
	}
}

func TestScoreStaleBranch(t *testing.T) {
	tests := []struct {
		name    string
		daysAgo float64
		wantPts int
	}{
		{"1 day ago", 1, 0},
		{"3 days ago", 3, 0},
		{"4 days ago", 4, 5},    // int((4-3)/4 * 20) = int(5) = 5
		{"5 days ago", 5, 10},   // int((5-3)/4 * 20) = int(10) = 10
		{"7 days ago", 7, 20},   // int((7-3)/4 * 20) = int(20) = 20
		{"10 days ago", 10, 20}, // capped at 20
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := []model.WorkItem{
				{
					Worktree: &model.Worktree{
						LastCommit: daysAgo(tc.daysAgo),
					},
				},
			}
			Score(items)

			if items[0].Score != tc.wantPts {
				t.Errorf("expected score %d, got %d", tc.wantPts, items[0].Score)
			}

			if tc.wantPts == 0 {
				if len(items[0].Breakdown) != 0 {
					t.Errorf("expected no factors, got %v", items[0].Breakdown)
				}
				return
			}
			found := false
			for _, f := range items[0].Breakdown {
				if f.Label == "Stale branch" {
					found = true
					if f.Points != tc.wantPts {
						t.Errorf("expected %d points, got %d", tc.wantPts, f.Points)
					}
				}
			}
			if !found {
				t.Errorf("expected 'Stale branch' factor, got %v", items[0].Breakdown)
			}
		})
	}
}

func TestScoreMultipleFactors(t *testing.T) {
	items := []model.WorkItem{
		{
			Issue: &model.LinearIssue{
				Priority: 1,    // +25
				InCycle:  true, // +10
			},
			PR: &model.PullRequest{
				Number:   10,
				CIStatus: "failing", // +40
			},
		},
	}
	Score(items)

	want := 40 + 25 + 10 // CI failing + Urgent priority + In cycle
	if items[0].Score != want {
		t.Errorf("expected score %d, got %d", want, items[0].Score)
	}
	if len(items[0].Breakdown) != 3 {
		t.Errorf("expected 3 factors, got %d: %v", len(items[0].Breakdown), items[0].Breakdown)
	}
}

func TestScoreBreakdownLabels(t *testing.T) {
	overdue := daysFromNow(-1)
	items := []model.WorkItem{
		{
			Issue: &model.LinearIssue{
				Priority: 2,
				InCycle:  true,
				DueDate:  overdue,
			},
			PR: &model.PullRequest{
				Number:      5,
				CIStatus:    "failing",
				ReviewState: "changes_requested",
			},
		},
	}
	Score(items)

	expectedLabels := map[string]int{
		"CI failing":        40,
		"Changes requested": 35,
		"Deadline":          30,
		"High priority":     15,
		"In cycle":          10,
	}

	if len(items[0].Breakdown) != len(expectedLabels) {
		t.Fatalf("expected %d factors, got %d: %v", len(expectedLabels), len(items[0].Breakdown), items[0].Breakdown)
	}

	for _, f := range items[0].Breakdown {
		wantPts, ok := expectedLabels[f.Label]
		if !ok {
			t.Errorf("unexpected label %q", f.Label)
			continue
		}
		if f.Points != wantPts {
			t.Errorf("label %q: expected %d points, got %d", f.Label, wantPts, f.Points)
		}
	}

	wantTotal := 40 + 35 + 30 + 15 + 10
	if items[0].Score != wantTotal {
		t.Errorf("expected total score %d, got %d", wantTotal, items[0].Score)
	}
}
