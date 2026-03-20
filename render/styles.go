package render

import "github.com/charmbracelet/lipgloss"

// SourceStatus tracks the loading state of a data source.
type SourceStatus int

const (
	StatusPending SourceStatus = iota
	StatusLoading
	StatusDone
	StatusCached
	StatusError
)

// Adaptive colors that work on both light and dark backgrounds.
var (
	// Urgency — rank number coloring
	colorUrgHigh = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#ff5f56"}
	colorUrgMed  = lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#ffbd2e"}
	colorUrgLow  = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#6b7280"}

	// Text
	colorTitle      = lipgloss.AdaptiveColor{Light: "#374151", Dark: "#94a3b8"}
	colorTitleBright = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#f1f5f9"}
	colorIssueID    = lipgloss.AdaptiveColor{Light: "#1f2937", Dark: "#e2e8f0"}
	colorIssueIDSel = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#f1f5f9"}
	colorStatus     = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#64748b"}
	colorPath       = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#475569"}
	colorTime       = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#475569"}

	// PR
	colorPR    = lipgloss.AdaptiveColor{Light: "#4f46e5", Dark: "#818cf8"}
	colorDraft = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#64748b"}

	// CI / Review
	colorPass    = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#27c93f"}
	colorFail    = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#ff5f56"}
	colorPending = lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#ffbd2e"}

	// UI chrome
	colorDot       = lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#334155"}
	colorHeader    = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#64748b"}
	colorSelector  = lipgloss.AdaptiveColor{Light: "#4f46e5", Dark: "#818cf8"}
	colorHelpKey   = lipgloss.AdaptiveColor{Light: "#374151", Dark: "#94a3b8"}
	colorHelpLabel = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#475569"}
	colorWarn      = lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#ffbd2e"}
	colorSpinner   = lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#ffbd2e"}
	colorCheckmark = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#27c93f"}
	colorError     = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#ff5f56"}
	colorDimSource = lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#475569"}
)
