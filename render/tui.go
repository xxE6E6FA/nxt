package render

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/xxE6E6FA/nxt/config"
	"github.com/xxE6E6FA/nxt/model"
)

const (
	keyEsc   = "esc"
	keyEnter = "enter"
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

// FetchFunc is the function signature for fetching work items.
// When noCache is true, all sources are fetched fresh (bypassing cache).
type FetchFunc func(updateSource func(name string, status SourceStatus), noCache bool) FetchResult

// execDoneMsg is sent when an exec'd process finishes.
type execDoneMsg struct{ err error }

// refreshTickMsg triggers an auto-refresh.
type refreshTickMsg struct{}

type phase int

const (
	phaseLoading phase = iota
	phaseReady
	phaseDetail
	phaseSettings
	phaseWarnings
)

type tuiModel struct {
	phase      phase
	items      []model.WorkItem
	cursor     int
	width      int
	height     int
	action     Action
	lastAction Action // records the most recent action decided by handleKey (for testability)
	warnings   []string
	fetchedAt  time.Time
	refreshing bool // true while a background refresh is in progress

	// Config
	editor          string // command to open folders
	cfg             *config.Config
	refreshInterval time.Duration

	// Fetch
	fetchFunc FetchFunc
	program   **tea.Program // double pointer so the model copy inside tea.Program sees the assignment

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

var spinFrames = []string{"◐", "◓", "◑", "◒"}

type spinTickMsg struct{}

// RunInteractive launches the TUI immediately (showing spinner), then populates
// with data when fetching completes. Returns an action only if the user wants
// to quit entirely (currently always empty — actions execute inline via tea.Exec).
func RunInteractive(cfg *config.Config, editor string, fetchFunc FetchFunc) Action {
	interval := refreshIntervalFromConfig(cfg)

	var pp *tea.Program

	m := tuiModel{
		phase:           phaseLoading,
		editor:          editor,
		cfg:             cfg,
		fetchFunc:       fetchFunc,
		refreshInterval: interval,
		sources:         defaultSources(),
		program:         &pp,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	pp = p

	go func() {
		result := fetchFunc(func(name string, status SourceStatus) {
			p.Send(SourceUpdate{Name: name, Status: status})
		}, false)
		p.Send(prepareFetchResult(result, cfg.Display.MaxItems))
	}()

	result, err := p.Run()
	if err != nil {
		return Action{}
	}

	return result.(tuiModel).action
}

func defaultSources() []sourceEntry {
	return []sourceEntry{
		{name: "Linear", status: StatusPending},
		{name: "Worktrees", status: StatusPending},
		{name: "GitHub", status: StatusPending},
	}
}

func prepareFetchResult(result FetchResult, maxItems int) FetchResult {
	sort.Slice(result.Items, func(i, j int) bool {
		return result.Items[i].Score > result.Items[j].Score
	})
	if maxItems > 0 && len(result.Items) > maxItems {
		result.Items = result.Items[:maxItems]
	}
	return result
}

func refreshIntervalFromConfig(cfg *config.Config) time.Duration {
	if cfg.Display.RefreshInterval < 0 {
		return 0
	}
	if cfg.Display.RefreshInterval == 0 {
		return 5 * time.Minute // default
	}
	return time.Duration(cfg.Display.RefreshInterval) * time.Second
}

func (m tuiModel) Init() tea.Cmd {
	cmds := []tea.Cmd{spinTick()}
	if m.refreshInterval > 0 {
		cmds = append(cmds, m.scheduleRefresh())
	}
	return tea.Batch(cmds...)
}

func (m tuiModel) scheduleRefresh() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(_ time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

// triggerRefresh kicks off a background fetch while keeping the list visible.
// On initial load (no items yet), falls back to the loading screen.
func (m tuiModel) triggerRefresh() (tuiModel, tea.Cmd) {
	if len(m.items) == 0 {
		// First load — show spinner screen
		m.phase = phaseLoading
		m.sources = defaultSources()
	}
	m.refreshing = true

	progPtr := m.program
	fetchFn := m.fetchFunc
	maxItems := m.cfg.Display.MaxItems

	cmd := func() tea.Msg {
		result := fetchFn(func(name string, status SourceStatus) {
			if progPtr != nil && *progPtr != nil {
				(*progPtr).Send(SourceUpdate{Name: name, Status: status})
			}
		}, true) // noCache=true for refresh
		return prepareFetchResult(result, maxItems)
	}

	cmds := []tea.Cmd{cmd, spinTick()}
	if m.refreshInterval > 0 {
		cmds = append(cmds, m.scheduleRefresh())
	}
	return m, tea.Batch(cmds...)
}

func spinTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(_ time.Time) tea.Msg {
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
		if m.phase == phaseLoading || m.refreshing {
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
		m.fetchedAt = time.Now()
		m.refreshing = false
		// Clamp cursor if items shrunk (e.g. after refresh)
		if m.cursor >= len(m.items) && len(m.items) > 0 {
			m.cursor = len(m.items) - 1
		} else if len(m.items) == 0 {
			m.cursor = 0
		}
		return m, nil

	case refreshTickMsg:
		// Auto-refresh: only refresh if we're on the list view (not mid-action)
		if m.phase == phaseReady {
			return m.triggerRefresh()
		}
		// Reschedule if we skipped (e.g. in settings or detail view)
		if m.refreshInterval > 0 {
			return m, m.scheduleRefresh()
		}
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

	// Detail phase — any key goes back to list
	if m.phase == phaseDetail {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		default:
			m.phase = phaseReady
			return m, nil
		}
	}

	// Warnings phase — any key goes back to list
	if m.phase == phaseWarnings {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		default:
			m.phase = phaseReady
			return m, nil
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case keyEsc:
		return m, tea.Quit
	}

	// Refresh and warnings work even with empty items
	if msg.String() == "r" && m.phase == phaseReady {
		return m.triggerRefresh()
	}
	if msg.String() == "w" && m.phase == phaseReady && len(m.warnings) > 0 {
		m.phase = phaseWarnings
		return m, nil
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

	case keyEnter, "e":
		// Open worktree in configured editor
		if path := wtPath(item); path != "" {
			m.lastAction = Action{Kind: "editor", Path: path}
			c := exec.Command(m.editor, path) //nolint:gosec // editor command is from user config
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return execDoneMsg{err}
			})
		}

	case "c":
		// Open Claude in worktree with issue context (blocks until claude exits)
		if path := wtPath(item); path != "" {
			m.lastAction = Action{Kind: "claude", Path: path}
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
			m.lastAction = Action{Kind: "open-linear", Path: item.Issue.URL}
			openBrowserFunc(item.Issue.URL)
		}

	case "g":
		if item.PR != nil && item.PR.URL != "" {
			m.lastAction = Action{Kind: "open-github", Path: item.PR.URL}
			openBrowserFunc(item.PR.URL)
		}

	case "d":
		// Show score breakdown detail
		m.phase = phaseDetail
		return m, nil
	}

	return m, nil
}

func (m tuiModel) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settings.isEditing() {
		switch msg.String() {
		case keyEnter:
			m.settings.confirmEdit()
			m.applySettings()
			return m, nil
		case keyEsc:
			m.settings.cancelEdit()
			return m, nil
		case "backspace":
			m.settings.backspace()
			return m, nil
		default:
			// Type printable characters
			if utf8.RuneCountInString(msg.String()) == 1 {
				m.settings.typeChar(msg.String())
			}
			return m, nil
		}
	}

	switch msg.String() {
	case keyEsc, "q":
		m.phase = phaseReady
		return m, nil
	case "j", "down":
		m.settings.moveDown()
	case "k", "up":
		m.settings.moveUp()
	case keyEnter:
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
	case phaseDetail:
		b.WriteString(m.viewDetail())
	case phaseWarnings:
		b.WriteString(m.viewWarnings())
	case phaseSettings:
		return m.settings.view()
	default:
		b.WriteString(m.viewList())
	}

	return b.String()
}

func (m tuiModel) viewLoading() string {
	parts := make([]string, 0, len(m.sources))
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
		return checkStyle.Render("✓ "+src.name) + dimStyle.Render(" ·")
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
		b.WriteString(m.renderStatusBar())
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

	b.WriteString(m.renderStatusBar())
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
	switch {
	case item.Score >= 30:
		rankStr = lipgloss.NewStyle().Bold(true).Foreground(colorUrgHigh).Render(rankLabel)
	case item.Score >= 15:
		rankStr = lipgloss.NewStyle().Bold(true).Foreground(colorUrgMed).Render(rankLabel)
	default:
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

	fmt.Fprintf(&b, "%s%s %s  %s\n", indicator, rankStr, idRendered,
		lipgloss.NewStyle().Foreground(titleColor).Render(truncate(title, titleMax)))

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
	case model.CIPassing:
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPass).Render("✓ CI"))
	case model.CIFailing:
		parts = append(parts, lipgloss.NewStyle().Foreground(colorFail).Render("✗ CI"))
	case model.CIPending:
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPending).Render("◎ CI"))
	}

	switch pr.ReviewState {
	case model.ReviewApproved:
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPass).Render("✓ approved"))
	case model.ReviewChangesRequested:
		parts = append(parts, lipgloss.NewStyle().Foreground(colorFail).Render("⚠ changes requested"))
	case model.ReviewRequired:
		parts = append(parts, lipgloss.NewStyle().Foreground(colorPending).Render("◎ review needed"))
	}

	return parts
}

func (m tuiModel) renderHelp() string {
	keyStyle := lipgloss.NewStyle().Foreground(colorHelpKey)
	lblStyle := lipgloss.NewStyle().Foreground(colorHelpLabel)

	if len(m.items) == 0 {
		help := keyStyle.Render("r") + lblStyle.Render(" refresh") + "  "
		if len(m.warnings) > 0 {
			warnLbl := lipgloss.NewStyle().Foreground(colorWarn)
			help += keyStyle.Render("w") + warnLbl.Render(fmt.Sprintf(" %d warning(s)", len(m.warnings))) + "  "
		}
		help += keyStyle.Render("s") + lblStyle.Render(" settings") + "  " +
			keyStyle.Render("q") + lblStyle.Render(" quit")
		return help
	}

	item := m.items[m.cursor]
	hasWt := item.Worktree != nil && !item.Worktree.IsMain
	hasIssue := item.Issue != nil
	hasPR := item.PR != nil

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
		keyStyle.Render("d")+lblStyle.Render(" detail"),
	)
	if len(m.warnings) > 0 {
		warnLbl := lipgloss.NewStyle().Foreground(colorWarn)
		keys = append(keys, keyStyle.Render("w")+warnLbl.Render(fmt.Sprintf(" %d warning(s)", len(m.warnings))))
	}
	keys = append(keys,
		keyStyle.Render("r")+lblStyle.Render(" refresh"),
		keyStyle.Render("s")+lblStyle.Render(" settings"),
		keyStyle.Render("q")+lblStyle.Render(" quit"),
	)

	return strings.Join(keys, "  ")
}

func (m tuiModel) renderStatusBar() string {
	dim := lipgloss.NewStyle().Foreground(colorStatusBar)
	warnStyle := lipgloss.NewStyle().Foreground(colorWarn)

	// Left side: item count + warnings + refresh status
	var left []string
	left = append(left, dim.Render(fmt.Sprintf("%d items", len(m.items))))

	if len(m.warnings) > 0 {
		left = append(left, warnStyle.Render(fmt.Sprintf("⚠ %d warning(s)", len(m.warnings))))
	}

	if m.refreshing {
		spinStyle := lipgloss.NewStyle().Foreground(colorSpinner)
		left = append(left, spinStyle.Render(spinFrames[m.spinFrame]+" refreshing"))
	} else if !m.fetchedAt.IsZero() {
		left = append(left, dim.Render("updated "+humanDuration(time.Since(m.fetchedAt))))
	}

	leftStr := "  " + strings.Join(left, dim.Render(" · "))

	// Right side: contextual help keys
	rightStr := m.renderHelp() + "  "

	// Calculate padding between left and right
	leftWidth := lipgloss.Width(leftStr)
	rightWidth := lipgloss.Width(rightStr)
	gap := m.width - leftWidth - rightWidth
	if gap < 2 {
		gap = 2
	}

	rule := lipgloss.NewStyle().Foreground(colorDot).Render(strings.Repeat("─", m.width))
	return "\n" + rule + "\n" + leftStr + strings.Repeat(" ", gap) + rightStr
}

func (m tuiModel) viewDetail() string {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return ""
	}

	item := m.items[m.cursor]
	var b strings.Builder

	// Item header
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorTitleBright)
	idStyle := lipgloss.NewStyle().Bold(true).Foreground(colorIssueIDSel)
	dimStyle := lipgloss.NewStyle().Foreground(colorStatus)

	if item.Issue != nil {
		fmt.Fprintf(&b, "  %s  %s\n",
			idStyle.Render(item.Issue.Identifier),
			titleStyle.Render(item.Issue.Title))
	} else if item.PR != nil {
		fmt.Fprintf(&b, "  %s  %s\n",
			idStyle.Render(fmt.Sprintf("PR #%d", item.PR.Number)),
			titleStyle.Render(item.PR.Title))
	}

	// Score total
	var scoreColor lipgloss.AdaptiveColor
	switch {
	case item.Score >= 30:
		scoreColor = colorUrgHigh
	case item.Score >= 15:
		scoreColor = colorUrgMed
	default:
		scoreColor = colorUrgLow
	}
	scoreStyle := lipgloss.NewStyle().Bold(true).Foreground(scoreColor)
	fmt.Fprintf(&b, "  Score: %s\n\n", scoreStyle.Render(fmt.Sprintf("%d", item.Score)))

	// Breakdown
	if len(item.Breakdown) == 0 {
		b.WriteString(dimStyle.Render("  No scoring factors — base score 0") + "\n")
	} else {
		renderBreakdown(&b, item.Breakdown)
	}

	// Warnings (if any)
	if len(m.warnings) > 0 {
		warnStyle := lipgloss.NewStyle().Foreground(colorWarn)
		b.WriteString("\n")
		b.WriteString(warnStyle.Render(fmt.Sprintf("  Warnings (%d)", len(m.warnings))) + "\n")
		for _, w := range m.warnings {
			b.WriteString("  " + warnStyle.Render("⚠ ") + dimStyle.Render(w) + "\n")
		}
	}

	// Hint
	b.WriteString("\n")
	hintStyle := lipgloss.NewStyle().Foreground(colorHelpLabel)
	b.WriteString(hintStyle.Render("  Press any key to go back") + "\n")

	return b.String()
}

func (m tuiModel) viewWarnings() string {
	var b strings.Builder

	warnStyle := lipgloss.NewStyle().Foreground(colorWarn)
	dimStyle := lipgloss.NewStyle().Foreground(colorStatus)
	hintStyle := lipgloss.NewStyle().Foreground(colorHelpLabel)

	b.WriteString(warnStyle.Render(fmt.Sprintf("  Warnings (%d)", len(m.warnings))) + "\n\n")

	for _, w := range m.warnings {
		b.WriteString("  " + warnStyle.Render("⚠ ") + dimStyle.Render(w) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  Press any key to go back") + "\n")
	return b.String()
}

func renderBreakdown(b *strings.Builder, factors []model.ScoreFactor) {
	labelStyle := lipgloss.NewStyle().Foreground(colorTitleBright).Width(20)
	ptsStyle := lipgloss.NewStyle().Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(colorStatus)

	for _, f := range factors {
		var ptsColor lipgloss.AdaptiveColor
		switch {
		case f.Points >= 25:
			ptsColor = colorUrgHigh
		case f.Points >= 10:
			ptsColor = colorUrgMed
		default:
			ptsColor = colorUrgLow
		}

		pts := ptsStyle.Foreground(ptsColor).Render(fmt.Sprintf("+%d", f.Points))
		label := labelStyle.Render(f.Label)
		line := fmt.Sprintf("  %s  %s", pts, label)
		if f.Detail != "" {
			line += detailStyle.Render(f.Detail)
		}
		b.WriteString(line + "\n")
	}
}

func wtPath(item model.WorkItem) string {
	if item.Worktree != nil && !item.Worktree.IsMain {
		return item.Worktree.Path
	}
	return ""
}

// openBrowserFunc is the function used to open URLs in a browser.
// Replaced in tests to avoid spawning processes.
var openBrowserFunc = func(url string) {
	_ = exec.Command("open", url).Start()
}
