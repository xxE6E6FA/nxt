package render

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xxE6E6FA/nxt/config"
	"github.com/xxE6E6FA/nxt/model"
)

// testModel builds a tuiModel in phaseReady with the given items and a
// reasonable terminal size so View() produces output.
func testModel(items []model.WorkItem) tuiModel {
	return tuiModel{
		phase:  phaseReady,
		items:  items,
		cursor: 0,
		width:  120,
		height: 40,
		editor: "code",
		cfg:    &config.Config{},
	}
}

// sampleItems returns a small slice of WorkItems for testing.
func sampleItems() []model.WorkItem {
	return []model.WorkItem{
		{
			Issue: &model.LinearIssue{
				Identifier: "DISCO-1",
				Title:      "First issue",
				Status:     "In Progress",
				URL:        "https://linear.app/disco/issue/DISCO-1",
			},
			Score: 30,
			Breakdown: []model.ScoreFactor{
				{Label: "Priority", Points: 20, Detail: "urgent"},
				{Label: "Staleness", Points: 10, Detail: "3d idle"},
			},
		},
		{
			Issue: &model.LinearIssue{
				Identifier: "DISCO-2",
				Title:      "Second issue",
				Status:     "Todo",
				URL:        "https://linear.app/disco/issue/DISCO-2",
			},
			PR: &model.PullRequest{
				Number:      42,
				Title:       "Fix something",
				URL:         "https://github.com/org/repo/pull/42",
				CIStatus:    model.CIPassing,
				ReviewState: model.ReviewApproved,
			},
			Score: 20,
		},
		{
			Issue: &model.LinearIssue{
				Identifier: "DISCO-3",
				Title:      "Third issue",
				Status:     "In Review",
			},
			Score: 10,
		},
	}
}

func keyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func specialKey(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func updateModel(m tuiModel, msg tea.Msg) tuiModel {
	result, _ := m.Update(msg)
	return result.(tuiModel)
}

// --- Tests ---

func TestKeyNavigationList(t *testing.T) {
	items := sampleItems()
	m := testModel(items)

	// "j" moves cursor down
	m = updateModel(m, keyMsg("j"))
	if m.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", m.cursor)
	}

	// "down" also moves cursor down
	m = updateModel(m, specialKey(tea.KeyDown))
	if m.cursor != 2 {
		t.Errorf("after down: cursor = %d, want 2", m.cursor)
	}

	// "k" moves cursor up
	m = updateModel(m, keyMsg("k"))
	if m.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", m.cursor)
	}

	// "up" also moves cursor up
	m = updateModel(m, specialKey(tea.KeyUp))
	if m.cursor != 0 {
		t.Errorf("after up: cursor = %d, want 0", m.cursor)
	}
}

func TestKeyNavigationPhaseTransitions(t *testing.T) {
	items := sampleItems()

	// "d" in phaseReady -> phaseDetail
	m := testModel(items)
	m = updateModel(m, keyMsg("d"))
	if m.phase != phaseDetail {
		t.Errorf("after d: phase = %d, want %d (phaseDetail)", m.phase, phaseDetail)
	}

	// "esc" in phaseDetail -> phaseReady (any non-quit key goes back)
	m = updateModel(m, specialKey(tea.KeyEsc))
	if m.phase != phaseReady {
		t.Errorf("after esc in detail: phase = %d, want %d (phaseReady)", m.phase, phaseReady)
	}

	// "s" in phaseReady -> phaseSettings
	m = testModel(items)
	m = updateModel(m, keyMsg("s"))
	if m.phase != phaseSettings {
		t.Errorf("after s: phase = %d, want %d (phaseSettings)", m.phase, phaseSettings)
	}

	// "esc" in phaseSettings -> phaseReady
	m = updateModel(m, specialKey(tea.KeyEsc))
	if m.phase != phaseReady {
		t.Errorf("after esc in settings: phase = %d, want %d (phaseReady)", m.phase, phaseReady)
	}
}

func TestFetchResultTransition(t *testing.T) {
	m := tuiModel{
		phase:  phaseLoading,
		width:  120,
		height: 40,
		editor: "code",
		cfg:    &config.Config{},
		sources: []sourceEntry{
			{name: "Linear", status: StatusPending},
		},
	}

	items := sampleItems()
	m = updateModel(m, FetchResult{Items: items, Warnings: []string{"test warning"}})

	if m.phase != phaseReady {
		t.Errorf("phase = %d, want %d (phaseReady)", m.phase, phaseReady)
	}
	if len(m.items) != len(items) {
		t.Errorf("items count = %d, want %d", len(m.items), len(items))
	}
	if len(m.warnings) != 1 {
		t.Errorf("warnings count = %d, want 1", len(m.warnings))
	}
}

func TestViewContainsItems(t *testing.T) {
	m := testModel(sampleItems())
	view := m.View()

	for _, item := range m.items {
		if item.Issue == nil {
			continue
		}
		if !strings.Contains(view, item.Issue.Identifier) {
			t.Errorf("view does not contain identifier %q", item.Issue.Identifier)
		}
		if !strings.Contains(view, item.Issue.Title) {
			t.Errorf("view does not contain title %q", item.Issue.Title)
		}
	}
}

func TestViewDetailContainsInfo(t *testing.T) {
	items := sampleItems()
	m := testModel(items)
	m.cursor = 1 // item with both issue and PR
	m.phase = phaseDetail

	view := m.View()

	if !strings.Contains(view, "DISCO-2") {
		t.Error("detail view does not contain issue identifier DISCO-2")
	}
	if !strings.Contains(view, "Score:") {
		t.Error("detail view does not contain 'Score:'")
	}
}

func TestCursorWrapping(t *testing.T) {
	items := sampleItems()

	// Cursor at 0, pressing "k" stays at 0
	m := testModel(items)
	m.cursor = 0
	m = updateModel(m, keyMsg("k"))
	if m.cursor != 0 {
		t.Errorf("cursor at 0 after k: got %d, want 0", m.cursor)
	}

	// Also with up arrow
	m = updateModel(m, specialKey(tea.KeyUp))
	if m.cursor != 0 {
		t.Errorf("cursor at 0 after up: got %d, want 0", m.cursor)
	}

	// Cursor at last item, pressing "j" stays at last
	last := len(items) - 1
	m.cursor = last
	m = updateModel(m, keyMsg("j"))
	if m.cursor != last {
		t.Errorf("cursor at last after j: got %d, want %d", m.cursor, last)
	}

	// Also with down arrow
	m = updateModel(m, specialKey(tea.KeyDown))
	if m.cursor != last {
		t.Errorf("cursor at last after down: got %d, want %d", m.cursor, last)
	}
}

func TestSettingsNavigation(t *testing.T) {
	items := sampleItems()
	m := testModel(items)

	// Enter settings
	m = updateModel(m, keyMsg("s"))
	if m.phase != phaseSettings {
		t.Fatalf("expected phaseSettings, got %d", m.phase)
	}

	// Settings cursor starts at 0
	if m.settings.cursor != 0 {
		t.Errorf("settings cursor = %d, want 0", m.settings.cursor)
	}

	// "j" moves settings cursor down
	m = updateModel(m, keyMsg("j"))
	if m.settings.cursor != 1 {
		t.Errorf("after j: settings cursor = %d, want 1", m.settings.cursor)
	}

	// "down" also moves settings cursor down
	m = updateModel(m, specialKey(tea.KeyDown))
	if m.settings.cursor != 2 {
		t.Errorf("after down: settings cursor = %d, want 2", m.settings.cursor)
	}

	// "k" moves settings cursor up
	m = updateModel(m, keyMsg("k"))
	if m.settings.cursor != 1 {
		t.Errorf("after k: settings cursor = %d, want 1", m.settings.cursor)
	}

	// "up" also moves settings cursor up
	m = updateModel(m, specialKey(tea.KeyUp))
	if m.settings.cursor != 0 {
		t.Errorf("after up: settings cursor = %d, want 0", m.settings.cursor)
	}
}

func TestViewLoadingPhase(t *testing.T) {
	m := tuiModel{
		phase:  phaseLoading,
		width:  120,
		height: 40,
		sources: []sourceEntry{
			{name: "Linear", status: StatusLoading},
			{name: "Worktrees", status: StatusPending},
			{name: "GitHub", status: StatusPending},
		},
	}

	view := m.View()

	if view == "" {
		t.Error("loading view is empty")
	}
	if !strings.Contains(view, "Linear") {
		t.Error("loading view does not contain source name 'Linear'")
	}
	if !strings.Contains(view, "Worktrees") {
		t.Error("loading view does not contain source name 'Worktrees'")
	}
	if !strings.Contains(view, "GitHub") {
		t.Error("loading view does not contain source name 'GitHub'")
	}
}
