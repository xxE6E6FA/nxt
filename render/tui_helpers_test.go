package render

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xxE6E6FA/nxt/config"
	"github.com/xxE6E6FA/nxt/model"
)

func TestPrepareFetchResult(t *testing.T) {
	items := []model.WorkItem{
		{Score: 10},
		{Score: 30},
		{Score: 20},
	}
	result := prepareFetchResult(FetchResult{Items: items}, 0)

	// Should be sorted descending by score
	if result.Items[0].Score != 30 {
		t.Errorf("items[0].Score = %d, want 30", result.Items[0].Score)
	}
	if result.Items[1].Score != 20 {
		t.Errorf("items[1].Score = %d, want 20", result.Items[1].Score)
	}
	if result.Items[2].Score != 10 {
		t.Errorf("items[2].Score = %d, want 10", result.Items[2].Score)
	}
}

func TestPrepareFetchResultMaxItems(t *testing.T) {
	items := []model.WorkItem{
		{Score: 30},
		{Score: 20},
		{Score: 10},
	}
	result := prepareFetchResult(FetchResult{Items: items}, 2)

	if len(result.Items) != 2 {
		t.Fatalf("len = %d, want 2", len(result.Items))
	}
	if result.Items[0].Score != 30 {
		t.Errorf("items[0].Score = %d, want 30", result.Items[0].Score)
	}
}

func TestPrepareFetchResultMaxItemsZero(t *testing.T) {
	items := []model.WorkItem{{Score: 1}, {Score: 2}, {Score: 3}}
	result := prepareFetchResult(FetchResult{Items: items}, 0)

	if len(result.Items) != 3 {
		t.Fatalf("len = %d, want 3 (maxItems=0 means no limit)", len(result.Items))
	}
}

func TestRefreshIntervalFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		want     time.Duration
	}{
		{"zero returns default 5m", 0, 5 * time.Minute},
		{"negative returns 0", -1, 0},
		{"positive returns seconds", 120, 120 * time.Second},
		{"custom value", 60, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Display: config.DisplayConfig{RefreshInterval: tt.interval}}
			got := refreshIntervalFromConfig(cfg)
			if got != tt.want {
				t.Errorf("refreshIntervalFromConfig(%d) = %v, want %v", tt.interval, got, tt.want)
			}
		})
	}
}

func TestRenderPRParts(t *testing.T) {
	t.Run("non-draft PR", func(t *testing.T) {
		pr := &model.PullRequest{Number: 42, URL: "https://github.com/org/repo/pull/42"}
		parts := renderPRParts(pr)
		if len(parts) == 0 {
			t.Fatal("expected at least 1 part")
		}
		if !strings.Contains(parts[0], "PR #42") {
			t.Errorf("parts[0] = %q, should contain 'PR #42'", parts[0])
		}
	})

	t.Run("draft PR", func(t *testing.T) {
		pr := &model.PullRequest{Number: 10, IsDraft: true, URL: "https://github.com/org/repo/pull/10"}
		parts := renderPRParts(pr)
		if len(parts) == 0 {
			t.Fatal("expected at least 1 part")
		}
		if !strings.Contains(parts[0], "draft") {
			t.Errorf("parts[0] = %q, should contain 'draft'", parts[0])
		}
	})

	t.Run("CI passing", func(t *testing.T) {
		pr := &model.PullRequest{Number: 1, CIStatus: model.CIPassing}
		parts := renderPRParts(pr)
		found := false
		for _, p := range parts {
			if strings.Contains(p, "CI") {
				found = true
			}
		}
		if !found {
			t.Error("expected CI status in parts")
		}
	})

	t.Run("CI failing", func(t *testing.T) {
		pr := &model.PullRequest{Number: 1, CIStatus: model.CIFailing}
		parts := renderPRParts(pr)
		found := false
		for _, p := range parts {
			if strings.Contains(p, "CI") {
				found = true
			}
		}
		if !found {
			t.Error("expected CI status in parts")
		}
	})

	t.Run("CI pending", func(t *testing.T) {
		pr := &model.PullRequest{Number: 1, CIStatus: model.CIPending}
		parts := renderPRParts(pr)
		found := false
		for _, p := range parts {
			if strings.Contains(p, "CI") {
				found = true
			}
		}
		if !found {
			t.Error("expected CI status in parts")
		}
	})

	t.Run("review approved", func(t *testing.T) {
		pr := &model.PullRequest{Number: 1, ReviewState: model.ReviewApproved}
		parts := renderPRParts(pr)
		found := false
		for _, p := range parts {
			if strings.Contains(p, "approved") {
				found = true
			}
		}
		if !found {
			t.Error("expected 'approved' in parts")
		}
	})

	t.Run("changes requested", func(t *testing.T) {
		pr := &model.PullRequest{Number: 1, ReviewState: model.ReviewChangesRequested}
		parts := renderPRParts(pr)
		found := false
		for _, p := range parts {
			if strings.Contains(p, "changes requested") {
				found = true
			}
		}
		if !found {
			t.Error("expected 'changes requested' in parts")
		}
	})

	t.Run("review required", func(t *testing.T) {
		pr := &model.PullRequest{Number: 1, ReviewState: model.ReviewRequired}
		parts := renderPRParts(pr)
		found := false
		for _, p := range parts {
			if strings.Contains(p, "review needed") {
				found = true
			}
		}
		if !found {
			t.Error("expected 'review needed' in parts")
		}
	})
}

func TestBuildClaudePrompt(t *testing.T) {
	t.Run("issue only", func(t *testing.T) {
		item := model.WorkItem{
			Issue: &model.LinearIssue{
				Identifier: "ENG-1",
				Title:      "Fix bug",
				Status:     "In Progress",
				URL:        "https://linear.app/team/issue/ENG-1",
			},
		}
		prompt := buildClaudePrompt(item)
		if !strings.Contains(prompt, "ENG-1") {
			t.Error("prompt should contain issue identifier")
		}
		if !strings.Contains(prompt, "Fix bug") {
			t.Error("prompt should contain issue title")
		}
		if !strings.Contains(prompt, "In Progress") {
			t.Error("prompt should contain status")
		}
		if !strings.Contains(prompt, "linear.app") {
			t.Error("prompt should contain linear URL")
		}
	})

	t.Run("issue with PR", func(t *testing.T) {
		item := model.WorkItem{
			Issue: &model.LinearIssue{Identifier: "ENG-2", Title: "Add feature", Status: "Todo"},
			PR: &model.PullRequest{
				Number:      42,
				URL:         "https://github.com/org/repo/pull/42",
				CIStatus:    model.CIFailing,
				ReviewState: model.ReviewApproved,
			},
		}
		prompt := buildClaudePrompt(item)
		if !strings.Contains(prompt, "PR #42") {
			t.Error("prompt should contain PR number")
		}
		if !strings.Contains(prompt, "CI: failing") {
			t.Error("prompt should contain CI status")
		}
		if !strings.Contains(prompt, "Review: approved") {
			t.Error("prompt should contain review state")
		}
	})

	t.Run("no issue returns empty", func(t *testing.T) {
		item := model.WorkItem{PR: &model.PullRequest{Number: 1}}
		prompt := buildClaudePrompt(item)
		if prompt != "" {
			t.Errorf("prompt = %q, want empty for no-issue item", prompt)
		}
	})
}

func TestWtPath(t *testing.T) {
	tests := []struct {
		name string
		item model.WorkItem
		want string
	}{
		{"no worktree", model.WorkItem{}, ""},
		{"main worktree", model.WorkItem{Worktree: &model.Worktree{Path: "/code/repo", IsMain: true}}, ""},
		{"feature worktree", model.WorkItem{Worktree: &model.Worktree{Path: "/code/feature", IsMain: false}}, "/code/feature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wtPath(tt.item)
			if got != tt.want {
				t.Errorf("wtPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderSourceStatus(t *testing.T) {
	m := tuiModel{spinFrame: 0}

	tests := []struct {
		name     string
		status   SourceStatus
		contains string
	}{
		{"pending", StatusPending, "○"},
		{"loading", StatusLoading, spinFrames[0]},
		{"done", StatusDone, "✓"},
		{"cached", StatusCached, "✓"},
		{"error", StatusError, "✗"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.renderSourceStatus(sourceEntry{name: "Test", status: tt.status})
			if !strings.Contains(result, tt.contains) {
				t.Errorf("renderSourceStatus(%v) = %q, should contain %q", tt.status, result, tt.contains)
			}
			if !strings.Contains(result, "Test") {
				t.Errorf("renderSourceStatus(%v) = %q, should contain source name", tt.status, result)
			}
		})
	}
}

func TestViewEmptyItems(t *testing.T) {
	m := testModel(nil)
	m.items = nil
	view := m.View()

	if !strings.Contains(view, "No active work items") {
		t.Error("empty list view should show 'No active work items' message")
	}
}

func TestViewDetailEmptyBreakdown(t *testing.T) {
	items := []model.WorkItem{{
		Issue:     &model.LinearIssue{Identifier: "ENG-1", Title: "Test"},
		Score:     5,
		Breakdown: nil,
	}}
	m := testModel(items)
	m.phase = phaseDetail
	view := m.View()

	if !strings.Contains(view, "No scoring factors") {
		t.Error("detail view with empty breakdown should show 'No scoring factors'")
	}
}

func TestViewDetailWithBreakdown(t *testing.T) {
	items := []model.WorkItem{{
		Issue: &model.LinearIssue{Identifier: "ENG-1", Title: "Test"},
		Score: 35,
		Breakdown: []model.ScoreFactor{
			{Label: "Priority", Points: 25, Detail: "urgent"},
			{Label: "Staleness", Points: 10, Detail: "3d idle"},
		},
	}}
	m := testModel(items)
	m.phase = phaseDetail
	view := m.View()

	if !strings.Contains(view, "Priority") {
		t.Error("breakdown should contain factor label 'Priority'")
	}
	if !strings.Contains(view, "Staleness") {
		t.Error("breakdown should contain factor label 'Staleness'")
	}
	if !strings.Contains(view, "urgent") {
		t.Error("breakdown should contain detail 'urgent'")
	}
}

func TestViewDetailShowsIssueMetadata(t *testing.T) {
	due := time.Now().Add(3 * 24 * time.Hour)
	started := time.Now().Add(-5 * 24 * time.Hour)
	est := 3.0
	cycleEnd := time.Now().Add(7 * 24 * time.Hour)
	items := []model.WorkItem{{
		Issue: &model.LinearIssue{
			Identifier:   "ENG-42",
			Title:        "Enrich detail",
			Status:       "In Progress",
			Priority:     2,
			DueDate:      &due,
			StartedAt:    &started,
			Estimate:     &est,
			InCycle:      true,
			CycleEndDate: &cycleEnd,
			Labels:       []string{"frontend", "ux"},
		},
		Score: 25,
	}}
	m := testModel(items)
	m.phase = phaseDetail
	view := m.View()

	for _, want := range []string{"Issue", "In Progress", "High", "away", "5d", "3 pts", "frontend", "ux"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view should contain %q", want)
		}
	}
}

func TestViewDetailShowsPRMetadata(t *testing.T) {
	items := []model.WorkItem{{
		Issue: &model.LinearIssue{Identifier: "ENG-1", Title: "Test"},
		PR: &model.PullRequest{
			Number:       99,
			Repo:         "org/repo",
			URL:          "https://github.com/org/repo/pull/99",
			CIStatus:     model.CIFailing,
			ReviewState:  model.ReviewChangesRequested,
			Additions:    142,
			Deletions:    38,
			ChangedFiles: 4,
			Mergeable:    model.MergeableConflicting,
			Comments:     3,
			Labels:       []string{"bug"},
		},
		Score: 40,
	}}
	m := testModel(items)
	m.phase = phaseDetail
	view := m.View()

	for _, want := range []string{"Pull Request", "org/repo #99", "failing", "changes requested", "+142 -38 across 4 files", "conflicts", "bug"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view should contain %q", want)
		}
	}
}

func TestViewDetailShowsWorktree(t *testing.T) {
	items := []model.WorkItem{{
		Issue: &model.LinearIssue{Identifier: "ENG-1", Title: "Test"},
		Worktree: &model.Worktree{
			Path:       "/Users/test/code/repo/worktrees/feature-x",
			Branch:     "feature-x",
			LastCommit: time.Now().Add(-2 * time.Hour),
		},
		Score: 10,
	}}
	m := testModel(items)
	m.phase = phaseDetail
	view := m.View()

	for _, want := range []string{"Worktree", "feature-x", "2h ago"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view should contain %q", want)
		}
	}
}

func TestViewDetailShowsActionKeys(t *testing.T) {
	items := []model.WorkItem{{
		Issue: &model.LinearIssue{
			Identifier: "ENG-1",
			Title:      "Test",
			URL:        "https://linear.app/test/ENG-1",
		},
		PR: &model.PullRequest{
			Number: 1,
			URL:    "https://github.com/org/repo/pull/1",
		},
		Worktree: &model.Worktree{
			Path:   "/tmp/wt",
			Branch: "feat",
		},
		Score: 10,
	}}
	m := testModel(items)
	m.phase = phaseDetail
	view := m.View()

	for _, want := range []string{"linear", "github", "back"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail footer should contain %q", want)
		}
	}
}

func TestDetailKeyEscGoesBack(t *testing.T) {
	m := testModel(sampleItems())
	m.phase = phaseDetail
	m = updateModel(m, specialKey(tea.KeyEsc))
	if m.phase != phaseReady {
		t.Errorf("esc in detail should go back to list, got phase %d", m.phase)
	}
}

func TestDetailKeyLOpensLinear(t *testing.T) {
	var opened string
	origOpen := openBrowserFunc
	openBrowserFunc = func(url string) { opened = url }
	defer func() { openBrowserFunc = origOpen }()

	m := testModel(sampleItems())
	m.phase = phaseDetail
	m.cursor = 0 // DISCO-1 has a URL
	m = updateModel(m, keyMsg("l"))

	if opened != "https://linear.app/disco/issue/DISCO-1" {
		t.Errorf("l key should open Linear URL, opened: %q", opened)
	}
	// Should stay in detail view
	if m.phase != phaseDetail {
		t.Error("l key should stay in detail view")
	}
}

func TestDetailKeyGOpensGitHub(t *testing.T) {
	var opened string
	origOpen := openBrowserFunc
	openBrowserFunc = func(url string) { opened = url }
	defer func() { openBrowserFunc = origOpen }()

	m := testModel(sampleItems())
	m.phase = phaseDetail
	m.cursor = 1 // DISCO-2 has a PR
	m = updateModel(m, keyMsg("g"))

	if opened != "https://github.com/org/repo/pull/42" {
		t.Errorf("g key should open GitHub URL, opened: %q", opened)
	}
	if m.phase != phaseDetail {
		t.Error("g key should stay in detail view")
	}
}

func TestViewWidthZero(t *testing.T) {
	m := testModel(sampleItems())
	m.width = 0
	view := m.View()
	if view != "" {
		t.Errorf("View() with width=0 should return empty, got %q", view)
	}
}

func TestRenderItemPROnly(t *testing.T) {
	m := testModel(nil)
	item := model.WorkItem{
		PR: &model.PullRequest{
			Number: 42,
			Title:  "PR-only item",
			Repo:   "org/repo",
			URL:    "https://github.com/org/repo/pull/42",
		},
		Score: 10,
	}
	out := m.renderItem(1, item, false)
	if !strings.Contains(out, "org/repo #42") {
		t.Errorf("renderItem for PR-only should show 'org/repo #42', got: %s", out)
	}
}

func TestRenderItemPROnlyNoRepo(t *testing.T) {
	m := testModel(nil)
	item := model.WorkItem{
		PR: &model.PullRequest{Number: 7, Title: "Solo PR", URL: "https://github.com/org/repo/pull/7"},
	}
	out := m.renderItem(1, item, false)
	if !strings.Contains(out, "PR #7") {
		t.Errorf("renderItem for PR without repo should show 'PR #7', got: %s", out)
	}
}

func TestHandleKeySettingsEditing(t *testing.T) {
	m := testModel(sampleItems())
	// Enter settings
	m = updateModel(m, keyMsg("s"))
	// Start editing
	m = updateModel(m, specialKey(tea.KeyEnter))
	if !m.settings.isEditing() {
		t.Fatal("expected editing mode")
	}

	// Type a character
	m = updateModel(m, keyMsg("x"))
	if !strings.HasSuffix(m.settings.fields[0].editBuf, "x") {
		t.Errorf("editBuf = %q, expected to end with 'x'", m.settings.fields[0].editBuf)
	}

	// Backspace
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if strings.HasSuffix(m.settings.fields[0].editBuf, "x") {
		t.Error("backspace should have removed 'x'")
	}

	// Cancel editing
	m = updateModel(m, specialKey(tea.KeyEsc))
	if m.settings.isEditing() {
		t.Error("esc should cancel editing")
	}
}

func TestHandleKeyDetailQuit(t *testing.T) {
	m := testModel(sampleItems())
	m.phase = phaseDetail

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model := result.(tuiModel)
	_ = model
	// cmd should be tea.Quit
	if cmd == nil {
		t.Error("q in detail should return a quit command")
	}
}

func TestHandleKeyEmptyItemsIgnoresNavigation(t *testing.T) {
	m := testModel(nil)
	m.items = nil
	m.phase = phaseReady

	// j/k should not panic on empty items
	m = updateModel(m, keyMsg("j"))
	m = updateModel(m, keyMsg("k"))
	// Should still be at cursor 0
	if m.cursor != 0 {
		t.Errorf("cursor = %d after nav on empty, want 0", m.cursor)
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := testModel(sampleItems())
	m = updateModel(m, tea.WindowSizeMsg{Width: 200, Height: 50})

	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestUpdateSpinTick(t *testing.T) {
	m := tuiModel{phase: phaseLoading, width: 120, height: 40, spinFrame: 0}
	m = updateModel(m, spinTickMsg{})

	if m.spinFrame != 1 {
		t.Errorf("spinFrame = %d, want 1", m.spinFrame)
	}
}

func TestUpdateSourceUpdate(t *testing.T) {
	m := tuiModel{
		phase: phaseLoading,
		sources: []sourceEntry{
			{name: "Linear", status: StatusPending},
			{name: "GitHub", status: StatusPending},
		},
		width:  120,
		height: 40,
	}
	m = updateModel(m, SourceUpdate{Name: "Linear", Status: StatusDone})

	if m.sources[0].status != StatusDone {
		t.Errorf("sources[0].status = %v, want StatusDone", m.sources[0].status)
	}
	if m.sources[1].status != StatusPending {
		t.Errorf("sources[1].status should remain Pending, got %v", m.sources[1].status)
	}
}

func TestRenderHelp(t *testing.T) {
	t.Run("empty items shows minimal help", func(t *testing.T) {
		m := testModel(nil)
		m.items = nil
		help := m.renderHelp()
		if !strings.Contains(help, "refresh") {
			t.Error("empty help should contain 'refresh'")
		}
		if !strings.Contains(help, "quit") {
			t.Error("empty help should contain 'quit'")
		}
	})

	t.Run("item with worktree shows editor key", func(t *testing.T) {
		m := testModel([]model.WorkItem{{
			Issue:    &model.LinearIssue{Identifier: "X-1"},
			Worktree: &model.Worktree{Path: "/code/x", IsMain: false},
		}})
		help := m.renderHelp()
		if !strings.Contains(help, "code") {
			t.Error("help should contain editor name")
		}
		if !strings.Contains(help, "claude") {
			t.Error("help should contain 'claude'")
		}
	})

	t.Run("item with issue shows linear key", func(t *testing.T) {
		m := testModel([]model.WorkItem{{Issue: &model.LinearIssue{Identifier: "X-1"}}})
		help := m.renderHelp()
		if !strings.Contains(help, "linear") {
			t.Error("help should contain 'linear'")
		}
	})

	t.Run("item with PR shows github key", func(t *testing.T) {
		m := testModel([]model.WorkItem{{PR: &model.PullRequest{Number: 1}}})
		help := m.renderHelp()
		if !strings.Contains(help, "github") {
			t.Error("help should contain 'github'")
		}
	})
}

func TestRenderStatusBar(t *testing.T) {
	m := testModel(sampleItems())
	m.fetchedAt = time.Now()
	bar := m.renderStatusBar()

	if !strings.Contains(bar, "3 items") {
		t.Error("status bar should contain item count")
	}
	if !strings.Contains(bar, "updated") {
		t.Error("status bar should contain 'updated' timestamp")
	}
}

func TestRenderStatusBarWithWarnings(t *testing.T) {
	m := testModel(sampleItems())
	m.warnings = []string{"test warning"}
	bar := m.renderStatusBar()

	if !strings.Contains(bar, "1 warning") {
		t.Error("status bar should contain warning count")
	}
}

func TestRenderStatusBarRefreshing(t *testing.T) {
	m := testModel(sampleItems())
	m.refreshing = true
	bar := m.renderStatusBar()

	if !strings.Contains(bar, "refreshing") {
		t.Error("status bar should contain 'refreshing' when refreshing")
	}
}

func TestDefaultSources(t *testing.T) {
	sources := defaultSources()
	if len(sources) != 3 {
		t.Fatalf("len = %d, want 3", len(sources))
	}
	names := make(map[string]bool)
	for _, s := range sources {
		names[s.name] = true
		if s.status != StatusPending {
			t.Errorf("source %q status = %v, want StatusPending", s.name, s.status)
		}
	}
	for _, name := range []string{"Linear", "Worktrees", "GitHub"} {
		if !names[name] {
			t.Errorf("missing source %q", name)
		}
	}
}
