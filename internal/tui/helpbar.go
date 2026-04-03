package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// KeyBinding pairs a key label with a short description for the help bar.
type KeyBinding struct {
	Key  string // e.g. "Ctrl+P", "Enter", "↑/↓"
	Desc string // e.g. "actions", "send", "select"
}

var (
	helpKeyStyle = lipgloss.NewStyle().Foreground(colorText)
	helpDimStyle = lipgloss.NewStyle().Foreground(colorMuted)
	helpSepStyle = lipgloss.NewStyle().Foreground(colorDim)
)

var globalHelpKeys = []KeyBinding{
	{Key: "Tab", Desc: "switch"},
	{Key: "Ctrl+P", Desc: "actions"},
	{Key: "q", Desc: "quit"},
}

// renderHelpBar renders a single-line help bar of key–description pairs
// separated by " | ". Bindings are dropped from the right until the
// rendered string fits within the given width.
func renderHelpBar(bindings []KeyBinding, width int) string {
	if len(bindings) == 0 || width <= 0 {
		return ""
	}

	sep := helpSepStyle.Render(" | ")
	sepWidth := lipgloss.Width(sep)

	for count := len(bindings); count > 0; count-- {
		parts := make([]string, 0, count)
		totalWidth := 0
		for i := 0; i < count; i++ {
			part := helpKeyStyle.Render(bindings[i].Key) + " " + helpDimStyle.Render(bindings[i].Desc)
			partWidth := lipgloss.Width(part)
			if i > 0 {
				totalWidth += sepWidth
			}
			totalWidth += partWidth
			parts = append(parts, part)
		}
		if totalWidth <= width {
			return strings.Join(parts, sep)
		}
	}
	return ""
}
