package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// settingsField represents one editable config field.
type settingsField struct {
	label   string
	key     string // config key for saving
	value   string
	editing bool
	editBuf string
}

type settingsModel struct {
	fields []settingsField
	cursor int
}

func newSettingsModel(editor string, baseDirs []string, maxItems int) settingsModel {
	return settingsModel{
		fields: []settingsField{
			{label: "Editor", key: "editor", value: editor},
			{label: "Base dirs", key: "base_dirs", value: strings.Join(baseDirs, ", ")},
			{label: "Max items", key: "max_items", value: fmt.Sprintf("%d", maxItems)},
		},
	}
}

func (s *settingsModel) moveUp() {
	if s.cursor > 0 {
		s.cursor--
	}
}

func (s *settingsModel) moveDown() {
	if s.cursor < len(s.fields)-1 {
		s.cursor++
	}
}

func (s *settingsModel) startEdit() {
	f := &s.fields[s.cursor]
	f.editing = true
	f.editBuf = f.value
}

func (s *settingsModel) cancelEdit() {
	f := &s.fields[s.cursor]
	f.editing = false
	f.editBuf = ""
}

func (s *settingsModel) confirmEdit() {
	f := &s.fields[s.cursor]
	f.value = f.editBuf
	f.editing = false
	f.editBuf = ""
}

func (s *settingsModel) typeChar(ch string) {
	f := &s.fields[s.cursor]
	if f.editing {
		f.editBuf += ch
	}
}

func (s *settingsModel) backspace() {
	f := &s.fields[s.cursor]
	if f.editing && f.editBuf != "" {
		runes := []rune(f.editBuf)
		f.editBuf = string(runes[:len(runes)-1])
	}
}

func (s *settingsModel) isEditing() bool {
	return s.fields[s.cursor].editing
}

func (s *settingsModel) view() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Foreground(colorHeader)
	b.WriteString(headerStyle.Render("  nxt settings") + "\n\n")

	labelStyle := lipgloss.NewStyle().Foreground(colorStatus).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(colorIssueID)
	selStyle := lipgloss.NewStyle().Foreground(colorSelector)
	editStyle := lipgloss.NewStyle().Foreground(colorTitleBright).Underline(true)
	cursorChar := lipgloss.NewStyle().Foreground(colorSelector).Render("▎")

	for i, f := range s.fields {
		indicator := "  "
		if i == s.cursor {
			indicator = selStyle.Render("▸ ")
		}

		label := labelStyle.Render(f.label)

		var val string
		if f.editing {
			val = editStyle.Render(f.editBuf) + cursorChar
		} else {
			val = valueStyle.Render(f.value)
			if f.value == "" {
				val = lipgloss.NewStyle().Foreground(colorDimSource).Render("(not set)")
			}
		}

		fmt.Fprintf(&b, "%s%s  %s\n\n", indicator, label, val)
	}

	// Help
	keyStyle := lipgloss.NewStyle().Foreground(colorHelpKey)
	lblStyle := lipgloss.NewStyle().Foreground(colorHelpLabel)

	var help string
	if s.isEditing() {
		help = keyStyle.Render("enter") + lblStyle.Render(" save") + "  " +
			keyStyle.Render("esc") + lblStyle.Render(" cancel")
	} else {
		help = keyStyle.Render("enter") + lblStyle.Render(" edit") + "  " +
			keyStyle.Render("esc") + lblStyle.Render(" back")
	}

	b.WriteString("\n  " + help + "\n")
	return b.String()
}
