package render

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/xxE6E6FA/nxt/config"
	"github.com/xxE6E6FA/nxt/model"
)

// Action represents what the user wants to do after quitting the TUI.
type Action struct {
	Kind string // "editor", "claude", "" (quit)
	Path string
}

// FetchResult is sent when all data fetching is complete.
type FetchResult struct {
	Items    []model.WorkItem
	Warnings []string
}

// SourceUpdate is sent when a source's loading state changes.
type SourceUpdate struct {
	Name   string
	Status SourceStatus
}

// execDoneMsg is sent when an exec'd process finishes.
type execDoneMsg struct{ err error }

type phase int

const (
	phaseLoading phase = iota
	phaseReady
	phaseSettings
)

type tuiModel struct {
	phase   phase
	items   []model.WorkItem
	cursor  int
	width   int
	height  int
	action  Action
	warnings []string

	// Config
	editor   string // command to open folders
	cfg      *config.Config

	// Settings sub-model
	settings settingsModel

	// Loading state
	sources   []sourceEntry
	spinFrame int
}

type sourceEntry struct {
	name   string
	status SourceStatus
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinTickMsg struct{}

// RunInteractive launches the TUI immediately (showing spinner), then populates
// with data when fetching completes. Returns an action only if the user wants
// to quit entirely (currently always empty — actions execute inline via tea.Exec).
func RunInteractive(cfg *config.Config, editor string, fetchFunc func(updateSource func(name string, status SourceStatus)) FetchResult) Action {
	m := tuiModel{
		phase:  phaseLoading,
		editor: editor,
		cfg:    cfg,
		sources: []sourceEntry{
			{name: "Linear", status: StatusPending},
			{name: "Worktrees", status: StatusPending},
			{name: "GitHub", status: StatusPending},
		},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		result := fetchFunc(func(name string, status SourceStatus) {
			p.Send(SourceUpdate{Name: name, Status: status})
		})
		sort.Slice(result.Items, func(i, j int) bool {
			return result.Items[i].Score > result.Items[j].Score
		})
		if cfg.Display.MaxItems > 0 && len(result.Items) > cfg.Display.MaxItems {
			result.Items = result.Items[:cfg.Display.MaxItems]
		}
		p.Send(result)
	}()

	result, err := p.Run()
	if err != nil {
		return Action{}
	}

	return result.(tuiModel).action
}

func (m tuiModel) Init() tea.Cmd {
	return spinTick()
}

func spinTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(_ time.Time) tea.Msg {
		return spinTickMsg{}
	})
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinTickMsg:
		m.spinFrame = (m.spinFrame + 1) % len(spinFrames)
		if m.phase == phaseLoading {
			return m, spinTick()
		}
		return m, nil

	case SourceUpdate:
		for i := range m.sources {
			if m.sources[i].name == msg.Name {
				m.sources[i].status = msg.Status
				break
			}
		}
		return m, nil

	case FetchResult:
		m.phase = phaseReady
		m.items = msg.Items
		m.warnings = msg.Warnings
		return m, nil

	case execDoneMsg:
		// Returned from editor/claude — TUI resumes
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Settings phase has its own key handling
	if m.phase == phaseSettings {
		return m.handleSettingsKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		return m, tea.Quit
	}

	if m.phase != phaseReady || len(m.items) == 0 {
		return m, nil
	}

	item := m.items[m.cursor]

	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}

	case "s":
		// Open settings
		m.settings = newSettingsModel(m.editor, m.cfg.Local.BaseDirs, m.cfg.Display.MaxItems)
		m.phase = phaseSettings
		return m, nil

	case "enter", "e":
		// Open worktree in configured editor
		if path := wtPath(item); path != "" {
			c := exec.Command(m.editor, path)
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return execDoneMsg{err}
			})
		}

	case "c":
		// Open Claude in worktree with issue context (blocks until claude exits)
		if path := wtPath(item); path != "" {
			prompt := buildClaudePrompt(item)
			var c *exec.Cmd
			if prompt != "" {
				c = exec.Command("claude", prompt)
			} else {
				c = exec.Command("claude")
			}
			c.Dir = path
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return execDoneMsg{err}
			})
		}

	case "l":
		if item.Issue != nil && item.Issue.URL != "" {
			openBrowser(item.Issue.URL)
		}

	case "g":
		if item.PR != nil && item.PR.URL != "" {
			openBrowser(item.PR.URL)
		}
	}

	return m, nil
}

func (m tuiModel) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settings.isEditing() {
		switch msg.String() {
		case "enter":
			m.settings.confirmEdit()
			m.applySettings()
			return m, nil
		case "esc":
			m.settings.cancelEdit()
			return m, nil
		case "backspace":
			m.settings.backspace()
			return m, nil
		default:
			// Type printable characters
			if len(msg.String()) == 1 || msg.String() == " " {
				m.settings.typeChar(msg.String())
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "esc", "q":
		m.phase = phaseReady
		return m, nil
	case "j", "down":
		m.settings.moveDown()
	case "k", "up":
		m.settings.moveUp()
	case "enter":
		m.settings.startEdit()
	}
	return m, nil
}

// applySettings writes the current settings values back to config and saves to disk.
func (m *tuiModel) applySettings() {
	for _, f := range m.settings.fields {
		switch f.key {
		case "editor":
			m.cfg.Display.Editor = f.value
			m.editor = f.value
		case "base_dirs":
			dirs := strings.Split(f.value, ",")
			var trimmed []string
			for _, d := range dirs {
				d = strings.TrimSpace(d)
				if d != "" {
					trimmed = append(trimmed, d)
				}
			}
			m.cfg.Local.BaseDirs = trimmed
		case "max_items":
			var n int
			if _, err := fmt.Sscanf(f.value, "%d", &n); err == nil && n > 0 {
				m.cfg.Display.MaxItems = n
			}
		}
	}
	_ = config.Write(m.cfg)
}

func buildClaudePrompt(item model.WorkItem) string {
	if item.Issue == nil {
		return ""
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("I'm picking up %s: %s", item.Issue.Identifier, item.Issue.Title))
	parts = append(parts, fmt.Sprintf("Status: %s", item.Issue.Status))

	if item.Issue.URL != "" {
		parts = append(parts, fmt.Sprintf("Linear: %s", item.Issue.URL))
	}

	if item.PR != nil {
		prDesc := fmt.Sprintf("PR #%d: %s", item.PR.Number, item.PR.URL)
		if item.PR.CIStatus != "" {
			prDesc += fmt.Sprintf(" (CI: %s)", item.PR.CIStatus)
		}
		if item.PR.ReviewState != "" {
			prDesc += fmt.Sprintf(" (Review: %s)", item.PR.ReviewState)
		}
		parts = append(parts, prDesc)
	}

	parts = append(parts, "\nGet me up to speed on where this stands and what needs to happen next.")

	return strings.Join(parts, "\n")
}

// --- View ---

func (m tuiModel) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Foreground(colorHeader)
	b.WriteString(headerStyle.Render("  nxt") + "\n\n")

	switch m.phase {
	case phaseLoading:
		b.WriteString(m.viewLoading())
	case phaseSettings:
		return m.settings.view(m.width)
	default:
		b.WriteString(m.viewList())
	}

	return b.String()
}

func (m tuiModel) viewLoading() string {
	var parts []string
	for _, src := range m.sources {
		parts = append(parts, m.renderSourceStatus(src))
	}
	return "  " + strings.Join(parts, "   ") + "\n"
}

func (m tuiModel) renderSourceStatus(src sourceEntry) string {
	dimStyle := lipgloss.NewStyle().Foreground(colorDimSource)
	checkStyle := lipgloss.NewStyle().Foreground(colorCheckmark)
	errStyle := lipgloss.NewStyle().Foreground(colorError)
	spinStyle := lipgloss.NewStyle().Foreground(colorSpinner)

	switch src.status {
	case StatusPending:
		return dimStyle.Render("○ " + src.name)
	case StatusLoading:
		return spinStyle.Render(spinFrames[m.spinFrame]) + " " + src.name
	case StatusDone:
		return checkStyle.Render("✓ " + src.name)
	case StatusCached:
		return checkStyle.Render("✓ " + src.name) + dimStyle.Render(" ·")
	case StatusError:
		return errStyle.Render("✗ " + src.name)
	}
	return src.name
}

func (m tuiModel) viewList() string {
	var b strings.Builder

	if len(m.items) == 0 {
		noItems := lipgloss.NewStyle().Foreground(colorStatus)
		b.WriteString(noItems.Render("  No active work items found.") + "\n")
		return b.String()
	}

	itemHeight := 3
	footerHeight := 4
	maxVisible := (m.height - footerHeight - 3) / itemHeight
	if maxVisible < 1 {
		maxVisible = 1
	}

	offset := 0
	if m.cursor >= maxVisible {
		offset = m.cursor - maxVisible + 1
	}
	end := offset + maxVisible
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := offset; i < end; i++ {
		b.WriteString(m.renderItem(i+1, m.items[i], i == m.cursor))
	}

	warnStyle := lipgloss.NewStyle().Foreground(colorWarn)
	for _, w := range m.warnings {
		b.WriteString("  " + warnStyle.Render("⚠ "+w) + "\n")
	}

	b.WriteString(m.renderHelp())
	return b.String()
}

func (m tuiModel) renderItem(idx int, item model.WorkItem, selected bool) string {
	var b strings.Builder

	indicator := "  "
	if selected {
		selStyle := lipgloss.NewStyle().Foreground(colorSelector)
		indicator = selStyle.Render("▸ ")
	}

	rankLabel := fmt.Sprintf("%2d", idx)
	var rankStr string
	if item.Score >= 30 {
		rankStr = lipgloss.NewStyle().Bold(true).Foreground(colorUrgHigh).Render(rankLabel)
	} else if item.Score >= 15 {
		rankStr = lipgloss.NewStyle().Bold(true).Foreground(colorUrgMed).Render(rankLabel)
	} else {
		rankStr = lipgloss.NewStyle().Bold(true).Foreground(colorUrgLow).Render(rankLabel)
	}

	id, title, idURL := "", "", ""
	if item.Issue != nil {
		id = item.Issue.Identifier
		idURL = item.Issue.URL
		title = item.Issue.Title
	} else if item.PR != nil {
		if item.PR.Repo != "" {
			id = fmt.Sprintf("%s #%d", item.PR.Repo, item.PR.Number)
		} else {
			id = fmt.Sprintf("PR #%d", item.PR.Number)
		}
		idURL = item.PR.URL
		title = item.PR.Title
	}

	var idColor, titleColor lipgloss.AdaptiveColor
	if selected {
		idColor = colorIssueIDSel
		titleColor = colorTitleBright
	} else {
		idColor = colorIssueID
		titleColor = colorTitle
	}

	idRendered := hyperlink(idURL, lipgloss.NewStyle().Bold(true).Foreground(idColor).Render(id))
	titleMax := m.width - 5 - len(id) - 4
	if titleMax < 20 {
		titleMax = 20
	}

	b.WriteString(fmt.Sprintf("%s%s %s  %s\n", indicator, rankStr, idRendered,
		lipgloss.NewStyle().Foreground(titleColor).Render(truncate(title, titleMax))))

	dotStr := lipgloss.NewStyle().Foreground(colorDot).Render(" · ")
	var parts []string

	if item.Issue != nil {
		parts = append(parts, lipgloss.NewStyle().Foreground(colorStatus).Render(item.Issue.Status))
	}
	if item.PR != nil {
		parts = append(parts, renderPRParts(item.PR)...)
	}
	if item.Worktree != nil && !item.Worktree.IsMain {
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPath).Render(shortenPath(item.Worktree.Path)))
		if !item.Worktree.LastCommit.IsZero() {
			parts = append(parts, lipgloss.NewStyle().Foreground(colorTime).Render(humanDuration(time.Since(item.Worktree.LastCommit))))
		}
	}

	if len(parts) > 0 {
		b.WriteString("      " + strings.Join(parts, dotStr) + "\n")
	} else {
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

func renderPRParts(pr *model.PullRequest) []string {
	var parts []string

	label := fmt.Sprintf("PR #%d", pr.Number)
	if pr.IsDraft {
		parts = append(parts, hyperlink(pr.URL, lipgloss.NewStyle().Foreground(colorDraft).Render("◌ "+label+" (draft)")))
	} else {
		parts = append(parts, hyperlink(pr.URL, lipgloss.NewStyle().Foreground(colorPR).Render("● "+label)))
	}

	switch pr.CIStatus {
	case "passing":
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPass).Render("✓ CI"))
	case "failing":
		parts = append(parts, lipgloss.NewStyle().Foreground(colorFail).Render("✗ CI"))
	case "pending":
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPending).Render("◎ CI"))
	}

	switch pr.ReviewState {
	case "approved":
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPass).Render("✓ approved"))
	case "changes_requested":
		parts = append(parts, lipgloss.NewStyle().Foreground(colorFail).Render("⚠ changes requested"))
	case "review_required":
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPending).Render("◎ review needed"))
	}

	return parts
}

func (m tuiModel) renderHelp() string {
	if len(m.items) == 0 {
		return ""
	}

	item := m.items[m.cursor]
	hasWt := item.Worktree != nil && !item.Worktree.IsMain
	hasIssue := item.Issue != nil
	hasPR := item.PR != nil

	keyStyle := lipgloss.NewStyle().Foreground(colorHelpKey)
	lblStyle := lipgloss.NewStyle().Foreground(colorHelpLabel)

	var keys []string
	if hasWt {
		keys = append(keys,
			keyStyle.Render("enter")+lblStyle.Render(" "+m.editor),
			keyStyle.Render("c")+lblStyle.Render(" claude"),
		)
	}
	if hasIssue {
		keys = append(keys, keyStyle.Render("l")+lblStyle.Render(" linear"))
	}
	if hasPR {
		keys = append(keys, keyStyle.Render("g")+lblStyle.Render(" github"))
	}
	keys = append(keys,
		keyStyle.Render("s")+lblStyle.Render(" settings"),
		keyStyle.Render("q")+lblStyle.Render(" quit"),
	)

	return "\n  " + strings.Join(keys, "  ") + "\n"
}

func wtPath(item model.WorkItem) string {
	if item.Worktree != nil && !item.Worktree.IsMain {
		return item.Worktree.Path
	}
	return ""
}

func openBrowser(url string) {
	_ = exec.Command("open", url).Start()
}
