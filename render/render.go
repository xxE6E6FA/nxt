package render

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/xxE6E6FA/nxt/model"
)

var (
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	bold   = lipgloss.NewStyle().Bold(true)
	rank   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
)

// Render outputs work items to stdout, sorted by score descending.
func Render(items []model.WorkItem, maxItems int) {
	// Sort by score descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	if maxItems > 0 && len(items) > maxItems {
		items = items[:maxItems]
	}

	if len(items) == 0 {
		fmt.Println(dim.Render("  No active work items found."))
		return
	}

	fmt.Println()
	for i, item := range items {
		renderItem(i+1, item)
	}
}

func renderItem(idx int, item model.WorkItem) {
	// Line 1: rank, identifier, title, score bar
	idStr := ""
	title := ""

	if item.Issue != nil {
		idStr = bold.Render(item.Issue.Identifier)
		title = item.Issue.Title
	} else if item.PR != nil {
		idStr = bold.Render(fmt.Sprintf("PR #%d", item.PR.Number))
		title = item.PR.Title
		if item.PR.Repo != "" {
			idStr = bold.Render(fmt.Sprintf("%s #%d", item.PR.Repo, item.PR.Number))
		}
	}

	scoreBar := renderScoreBar(item.Score)
	fmt.Printf(" %s  %s  %-50s %s\n",
		rank.Render(fmt.Sprintf("#%d", idx)),
		idStr,
		truncate(title, 50),
		scoreBar,
	)

	// Line 2: status, PR info, review state
	var parts []string

	if item.Issue != nil {
		parts = append(parts, item.Issue.Status)
	}

	if item.PR != nil {
		prStr := fmt.Sprintf("● PR #%d", item.PR.Number)
		if item.PR.IsDraft {
			prStr = fmt.Sprintf("◌ PR #%d (draft)", item.PR.Number)
		}

		// CI status
		switch item.PR.CIStatus {
		case "passing":
			prStr += " " + green.Render("✓ CI passing")
		case "failing":
			prStr += " " + red.Render("✗ CI failing")
		case "pending":
			prStr += " " + yellow.Render("◎ CI pending")
		}

		parts = append(parts, prStr)

		// Review state
		switch item.PR.ReviewState {
		case "approved":
			parts = append(parts, green.Render("✓ Approved"))
		case "changes_requested":
			parts = append(parts, red.Render("⚠ Changes requested"))
		case "review_required":
			parts = append(parts, yellow.Render("◎ Review pending"))
		}
	} else if item.Issue != nil {
		parts = append(parts, dim.Render("(no PR)"))
	}

	if len(parts) > 0 {
		fmt.Printf("     %s\n", strings.Join(parts, "  "))
	}

	// Line 3: worktree path, last commit
	if item.Worktree != nil {
		lastCommit := ""
		if !item.Worktree.LastCommit.IsZero() {
			lastCommit = "  last commit: " + humanDuration(time.Since(item.Worktree.LastCommit))
		}
		fmt.Printf("     %s%s\n", dim.Render(item.Worktree.Path), dim.Render(lastCommit))
	} else if item.Issue != nil {
		fmt.Printf("     %s\n", dim.Render("(no branch)"))
	}

	fmt.Println()
}

func renderScoreBar(score int) string {
	// 3-block bar: each block fills at 33, 66, 100
	blocks := 0
	if score >= 33 {
		blocks = 1
	}
	if score >= 66 {
		blocks = 2
	}
	if score >= 90 {
		blocks = 3
	}

	bar := strings.Repeat("▓", blocks) + strings.Repeat("░", 3-blocks)

	color := green
	if score >= 60 {
		color = red
	} else if score >= 30 {
		color = yellow
	}

	return color.Render(fmt.Sprintf("%s %d", bar, score))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd ago", days)
}
