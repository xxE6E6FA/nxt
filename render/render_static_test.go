package render

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/xxE6E6FA/nxt/model"
)

// captureStdout runs fn and returns whatever it wrote to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = origStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(out)
}

func TestRenderStaticItems(t *testing.T) {
	items := []model.WorkItem{
		{
			Issue: &model.LinearIssue{Identifier: "ENG-1", Title: "First task", Status: "In Progress"},
			Score: 30,
		},
		{
			Issue: &model.LinearIssue{Identifier: "ENG-2", Title: "Second task", Status: "Todo"},
			Score: 10,
		},
	}

	out := captureStdout(t, func() {
		Render(items, 0)
	})

	if !strings.Contains(out, "ENG-1") {
		t.Error("output should contain ENG-1")
	}
	if !strings.Contains(out, "First task") {
		t.Error("output should contain 'First task'")
	}
	if !strings.Contains(out, "ENG-2") {
		t.Error("output should contain ENG-2")
	}
}

func TestRenderStaticEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		Render(nil, 0)
	})

	if !strings.Contains(out, "No active work items") {
		t.Error("empty render should show 'No active work items'")
	}
}

func TestRenderStaticMaxItems(t *testing.T) {
	items := []model.WorkItem{
		{Issue: &model.LinearIssue{Identifier: "A-1", Title: "First"}, Score: 30},
		{Issue: &model.LinearIssue{Identifier: "A-2", Title: "Second"}, Score: 20},
		{Issue: &model.LinearIssue{Identifier: "A-3", Title: "Third"}, Score: 10},
	}

	out := captureStdout(t, func() {
		Render(items, 2)
	})

	if !strings.Contains(out, "A-1") {
		t.Error("output should contain A-1 (highest score)")
	}
	if !strings.Contains(out, "A-2") {
		t.Error("output should contain A-2 (second highest)")
	}
	if strings.Contains(out, "A-3") {
		t.Error("output should NOT contain A-3 (truncated by maxItems)")
	}
}

func TestRenderStaticSortsDescending(t *testing.T) {
	items := []model.WorkItem{
		{Issue: &model.LinearIssue{Identifier: "LOW-1", Title: "Low"}, Score: 5},
		{Issue: &model.LinearIssue{Identifier: "HIGH-1", Title: "High"}, Score: 50},
		{Issue: &model.LinearIssue{Identifier: "MED-1", Title: "Med"}, Score: 20},
	}

	out := captureStdout(t, func() {
		Render(items, 0)
	})

	highIdx := strings.Index(out, "HIGH-1")
	medIdx := strings.Index(out, "MED-1")
	lowIdx := strings.Index(out, "LOW-1")

	if highIdx > medIdx || medIdx > lowIdx {
		t.Errorf("items not sorted descending: HIGH=%d MED=%d LOW=%d", highIdx, medIdx, lowIdx)
	}
}

func TestRenderStaticPROnly(t *testing.T) {
	items := []model.WorkItem{
		{
			PR:    &model.PullRequest{Number: 42, Title: "Fix bug", Repo: "org/repo", URL: "https://github.com/org/repo/pull/42"},
			Score: 15,
		},
	}

	out := captureStdout(t, func() {
		Render(items, 0)
	})

	if !strings.Contains(out, "org/repo #42") {
		t.Error("output should contain 'org/repo #42'")
	}
	if !strings.Contains(out, "Fix bug") {
		t.Error("output should contain PR title")
	}
}

func TestRenderStaticWithWorktree(t *testing.T) {
	items := []model.WorkItem{
		{
			Issue:    &model.LinearIssue{Identifier: "WT-1", Title: "Worktree item", Status: "In Progress"},
			Worktree: &model.Worktree{Path: "/some/repo/.git/worktrees/feature", IsMain: false},
			Score:    20,
		},
	}

	out := captureStdout(t, func() {
		Render(items, 0)
	})

	if !strings.Contains(out, "feature") {
		t.Error("output should contain shortened worktree path")
	}
}

func TestRenderStaticScoreColoring(t *testing.T) {
	// Just verify all three score tiers render without panic
	items := []model.WorkItem{
		{Issue: &model.LinearIssue{Identifier: "H-1", Title: "High"}, Score: 35},
		{Issue: &model.LinearIssue{Identifier: "M-1", Title: "Med"}, Score: 20},
		{Issue: &model.LinearIssue{Identifier: "L-1", Title: "Low"}, Score: 5},
	}

	out := captureStdout(t, func() {
		Render(items, 0)
	})

	for _, id := range []string{"H-1", "M-1", "L-1"} {
		if !strings.Contains(out, id) {
			t.Errorf("output should contain %s", id)
		}
	}
}
