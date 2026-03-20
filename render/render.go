package render

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/xxE6E6FA/nxt/model"
)

// hyperlink wraps text in an OSC 8 terminal hyperlink.
func hyperlink(url, text string) string {
	if url == "" {
		return text
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd())) //nolint:gosec // Fd() fits in int on all supported platforms
	if err != nil || w < 60 {
		return 80
	}
	return w
}

// Render outputs work items to stdout (non-interactive / piped mode).
func Render(items []model.WorkItem, maxItems int) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	if maxItems > 0 && len(items) > maxItems {
		items = items[:maxItems]
	}

	if len(items) == 0 {
		fmt.Println(lipgloss.NewStyle().Foreground(colorStatus).Render("  No active work items found."))
		return
	}

	width := termWidth()
	fmt.Println()

	for i, item := range items {
		renderStaticItem(i+1, item, width)
	}
}

func renderStaticItem(idx int, item model.WorkItem, width int) {
	// Rank
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

	idRendered := hyperlink(idURL, lipgloss.NewStyle().Bold(true).Foreground(colorIssueID).Render(id))
	titleMax := width - 5 - len(id) - 4
	if titleMax < 20 {
		titleMax = 20
	}

	fmt.Printf(" %s %s  %s\n", rankStr, idRendered,
		lipgloss.NewStyle().Foreground(colorTitle).Render(truncate(title, titleMax)))

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
		fmt.Printf("    %s\n", strings.Join(parts, dotStr))
	}

	fmt.Println()
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	p = strings.Replace(p, home, "~", 1)

	if parts := strings.Split(p, "/worktrees/"); len(parts) == 2 {
		return "…/" + parts[1]
	}

	p = strings.TrimPrefix(p, "~/code/")
	if filepath.IsAbs(p) {
		return p
	}
	return p
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
