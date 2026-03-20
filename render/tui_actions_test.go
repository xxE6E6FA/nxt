package render

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xxE6E6FA/nxt/config"
	"github.com/xxE6E6FA/nxt/model"
)

func init() {
	// Stub out browser opening so tests don't spawn "open" processes.
	openBrowserFunc = func(_ string) {}
}

// --- nxt-bmc: Test handleKey action branches ---

func itemWithWorktree() model.WorkItem {
	return model.WorkItem{
		Issue: &model.LinearIssue{
			Identifier: "ENG-1",
			Title:      "Test issue",
			Status:     "In Progress",
			URL:        "https://linear.app/team/issue/ENG-1",
		},
		PR: &model.PullRequest{
			Number: 42,
			URL:    "https://github.com/org/repo/pull/42",
		},
		Worktree: &model.Worktree{Path: "/code/feature-branch", IsMain: false},
		Score:    20,
	}
}

func itemWithoutWorktree() model.WorkItem {
	return model.WorkItem{
		Issue: &model.LinearIssue{
			Identifier: "ENG-2",
			Title:      "No worktree issue",
			Status:     "Todo",
			URL:        "https://linear.app/team/issue/ENG-2",
		},
		PR: &model.PullRequest{
			Number: 99,
			URL:    "https://github.com/org/repo/pull/99",
		},
		Score: 10,
	}
}

func TestHandleKeyEnterOpensEditor(t *testing.T) {
	m := testModel([]model.WorkItem{itemWithWorktree()})
	result, cmd := m.handleKey(keyMsg("e"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "editor" {
		t.Errorf("lastAction.Kind = %q, want %q", got.lastAction.Kind, "editor")
	}
	if got.lastAction.Path != "/code/feature-branch" {
		t.Errorf("lastAction.Path = %q, want %q", got.lastAction.Path, "/code/feature-branch")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for editor exec")
	}
}

func TestHandleKeyEnterNoWorktreeNoop(t *testing.T) {
	m := testModel([]model.WorkItem{itemWithoutWorktree()})
	result, _ := m.handleKey(keyMsg("e"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "" {
		t.Errorf("lastAction.Kind = %q, want empty (no worktree)", got.lastAction.Kind)
	}
}

func TestHandleKeyClaudeOpens(t *testing.T) {
	m := testModel([]model.WorkItem{itemWithWorktree()})
	result, cmd := m.handleKey(keyMsg("c"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "claude" {
		t.Errorf("lastAction.Kind = %q, want %q", got.lastAction.Kind, "claude")
	}
	if got.lastAction.Path != "/code/feature-branch" {
		t.Errorf("lastAction.Path = %q, want %q", got.lastAction.Path, "/code/feature-branch")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for claude exec")
	}
}

func TestHandleKeyClaudeNoWorktreeNoop(t *testing.T) {
	m := testModel([]model.WorkItem{itemWithoutWorktree()})
	result, _ := m.handleKey(keyMsg("c"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "" {
		t.Errorf("lastAction.Kind = %q, want empty (no worktree)", got.lastAction.Kind)
	}
}

func TestHandleKeyLinearOpensURL(t *testing.T) {
	m := testModel([]model.WorkItem{itemWithWorktree()})
	result, _ := m.handleKey(keyMsg("l"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "open-linear" {
		t.Errorf("lastAction.Kind = %q, want %q", got.lastAction.Kind, "open-linear")
	}
	if got.lastAction.Path != "https://linear.app/team/issue/ENG-1" {
		t.Errorf("lastAction.Path = %q, want linear URL", got.lastAction.Path)
	}
}

func TestHandleKeyLinearNoIssueNoop(t *testing.T) {
	m := testModel([]model.WorkItem{{
		PR:    &model.PullRequest{Number: 1, URL: "https://github.com/org/repo/pull/1"},
		Score: 5,
	}})
	result, _ := m.handleKey(keyMsg("l"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "" {
		t.Errorf("lastAction.Kind = %q, want empty (no issue)", got.lastAction.Kind)
	}
}

func TestHandleKeyLinearEmptyURLNoop(t *testing.T) {
	m := testModel([]model.WorkItem{{
		Issue: &model.LinearIssue{Identifier: "X-1", Title: "No URL", URL: ""},
		Score: 5,
	}})
	result, _ := m.handleKey(keyMsg("l"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "" {
		t.Errorf("lastAction.Kind = %q, want empty (empty URL)", got.lastAction.Kind)
	}
}

func TestHandleKeyGithubOpensURL(t *testing.T) {
	m := testModel([]model.WorkItem{itemWithWorktree()})
	result, _ := m.handleKey(keyMsg("g"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "open-github" {
		t.Errorf("lastAction.Kind = %q, want %q", got.lastAction.Kind, "open-github")
	}
	if got.lastAction.Path != "https://github.com/org/repo/pull/42" {
		t.Errorf("lastAction.Path = %q, want github URL", got.lastAction.Path)
	}
}

func TestHandleKeyGithubNoPRNoop(t *testing.T) {
	m := testModel([]model.WorkItem{{
		Issue: &model.LinearIssue{Identifier: "X-1", Title: "No PR"},
		Score: 5,
	}})
	result, _ := m.handleKey(keyMsg("g"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "" {
		t.Errorf("lastAction.Kind = %q, want empty (no PR)", got.lastAction.Kind)
	}
}

func TestHandleKeyGithubEmptyURLNoop(t *testing.T) {
	m := testModel([]model.WorkItem{{
		PR:    &model.PullRequest{Number: 1, URL: ""},
		Score: 5,
	}})
	result, _ := m.handleKey(keyMsg("g"))
	got := result.(tuiModel)

	if got.lastAction.Kind != "" {
		t.Errorf("lastAction.Kind = %q, want empty (empty PR URL)", got.lastAction.Kind)
	}
}

func TestHandleKeyEnterWithWorktree(t *testing.T) {
	// enter key (KeyEnter type) should also trigger editor
	m := testModel([]model.WorkItem{itemWithWorktree()})
	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := result.(tuiModel)

	if got.lastAction.Kind != "editor" {
		t.Errorf("lastAction.Kind = %q, want %q", got.lastAction.Kind, "editor")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for enter key editor exec")
	}
}

// --- nxt-2hn: Test applySettings config writes ---

func TestApplySettingsEditor(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create config dir so Write succeeds
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{}
	m := &tuiModel{
		editor: "code",
		cfg:    cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "editor", value: "cursor"},
			},
		},
	}

	m.applySettings()

	if m.cfg.Display.Editor != "cursor" {
		t.Errorf("cfg.Display.Editor = %q, want %q", m.cfg.Display.Editor, "cursor")
	}
	if m.editor != "cursor" {
		t.Errorf("m.editor = %q, want %q", m.editor, "cursor")
	}
}

func TestApplySettingsBaseDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{}
	m := &tuiModel{
		cfg: cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "base_dirs", value: "/code, /projects, /work"},
			},
		},
	}

	m.applySettings()

	want := []string{"/code", "/projects", "/work"}
	if len(m.cfg.Local.BaseDirs) != len(want) {
		t.Fatalf("BaseDirs len = %d, want %d", len(m.cfg.Local.BaseDirs), len(want))
	}
	for i, d := range want {
		if m.cfg.Local.BaseDirs[i] != d {
			t.Errorf("BaseDirs[%d] = %q, want %q", i, m.cfg.Local.BaseDirs[i], d)
		}
	}
}

func TestApplySettingsBaseDirsTrimsEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{}
	m := &tuiModel{
		cfg: cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "base_dirs", value: "/code, , /work, "},
			},
		},
	}

	m.applySettings()

	if len(m.cfg.Local.BaseDirs) != 2 {
		t.Fatalf("BaseDirs len = %d, want 2 (empty entries trimmed)", len(m.cfg.Local.BaseDirs))
	}
	if m.cfg.Local.BaseDirs[0] != "/code" || m.cfg.Local.BaseDirs[1] != "/work" {
		t.Errorf("BaseDirs = %v, want [/code /work]", m.cfg.Local.BaseDirs)
	}
}

func TestApplySettingsMaxItems(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{}
	m := &tuiModel{
		cfg: cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "max_items", value: "50"},
			},
		},
	}

	m.applySettings()

	if m.cfg.Display.MaxItems != 50 {
		t.Errorf("cfg.Display.MaxItems = %d, want 50", m.cfg.Display.MaxItems)
	}
}

func TestApplySettingsMaxItemsInvalidNoChange(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{Display: config.DisplayConfig{MaxItems: 20}}
	m := &tuiModel{
		cfg: cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "max_items", value: "abc"},
			},
		},
	}

	m.applySettings()

	if m.cfg.Display.MaxItems != 20 {
		t.Errorf("cfg.Display.MaxItems = %d, want 20 (unchanged on invalid input)", m.cfg.Display.MaxItems)
	}
}

func TestApplySettingsMaxItemsZeroNoChange(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{Display: config.DisplayConfig{MaxItems: 20}}
	m := &tuiModel{
		cfg: cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "max_items", value: "0"},
			},
		},
	}

	m.applySettings()

	if m.cfg.Display.MaxItems != 20 {
		t.Errorf("cfg.Display.MaxItems = %d, want 20 (unchanged on zero)", m.cfg.Display.MaxItems)
	}
}

func TestApplySettingsNegativeMaxItemsNoChange(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{Display: config.DisplayConfig{MaxItems: 20}}
	m := &tuiModel{
		cfg: cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "max_items", value: "-5"},
			},
		},
	}

	m.applySettings()

	if m.cfg.Display.MaxItems != 20 {
		t.Errorf("cfg.Display.MaxItems = %d, want 20 (unchanged on negative)", m.cfg.Display.MaxItems)
	}
}

func TestApplySettingsWritesToDisk(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{}
	m := &tuiModel{
		editor: "code",
		cfg:    cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "editor", value: "zed"},
				{key: "base_dirs", value: "/projects"},
				{key: "max_items", value: "30"},
			},
		},
	}

	m.applySettings()

	// Verify config file was written
	configPath := filepath.Join(tmp, ".config", "nxt", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	content := string(data)
	if content == "" {
		t.Error("config file is empty")
	}
}

func TestApplySettingsAllFieldsTogether(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	os.MkdirAll(filepath.Join(tmp, ".config", "nxt"), 0o750)

	cfg := &config.Config{
		Display: config.DisplayConfig{Editor: "code", MaxItems: 20},
		Local:   config.LocalConfig{BaseDirs: []string{"/old"}},
	}
	m := &tuiModel{
		editor: "code",
		cfg:    cfg,
		settings: settingsModel{
			fields: []settingsField{
				{key: "editor", value: "zed"},
				{key: "base_dirs", value: "/new, /also-new"},
				{key: "max_items", value: "100"},
			},
		},
	}

	m.applySettings()

	if m.cfg.Display.Editor != "zed" {
		t.Errorf("Editor = %q, want %q", m.cfg.Display.Editor, "zed")
	}
	if m.editor != "zed" {
		t.Errorf("m.editor = %q, want %q", m.editor, "zed")
	}
	if len(m.cfg.Local.BaseDirs) != 2 || m.cfg.Local.BaseDirs[0] != "/new" {
		t.Errorf("BaseDirs = %v, want [/new /also-new]", m.cfg.Local.BaseDirs)
	}
	if m.cfg.Display.MaxItems != 100 {
		t.Errorf("MaxItems = %d, want 100", m.cfg.Display.MaxItems)
	}
}
