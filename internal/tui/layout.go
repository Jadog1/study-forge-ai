package tui

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/studyforge/study-agent/internal/config"
)

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func cloneConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}
	copy := *cfg
	return &copy
}

func configsEqual(left, right *config.Config) bool {
	return reflect.DeepEqual(left, right)
}

func truncateWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "."
	}
	if width == 2 {
		return ".."
	}

	const ellipsis = "..."
	var b strings.Builder
	for _, r := range text {
		candidate := b.String() + string(r) + ellipsis
		if lipgloss.Width(candidate) > width {
			break
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return ellipsis[:width]
	}
	return b.String() + ellipsis
}

func padRightWidth(text string, width int) string {
	text = truncateWidth(text, width)
	padding := width - lipgloss.Width(text)
	if padding <= 0 {
		return text
	}
	return text + strings.Repeat(" ", padding)
}

func clipLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) <= maxLines {
		return text
	}
	if maxLines == 1 {
		return "..."
	}
	return strings.Join(append(lines[:maxLines-1], dimStyle.Render("...")), "\n")
}

// scrollToView returns the scroll offset that keeps cursor within the visible
// range [scroll, scroll+visH). Safe to use before any render.
func scrollToView(scroll, cursor, visH int) int {
	if visH <= 0 {
		return 0
	}
	if cursor < scroll {
		return cursor
	}
	if cursor >= scroll+visH {
		return cursor - visH + 1
	}
	return scroll
}

// windowLines renders a scrollable window from a pre-rendered slice of lines.
// scroll is the index of the first visible line; visH is the visible row count.
// Shows ↑/↓ indicators when content is clipped at either end.
func windowLines(lines []string, scroll, visH int) string {
	total := len(lines)
	if total == 0 {
		return ""
	}
	start := clamp(scroll, 0, max(0, total-1))
	end := min(start+visH, total)
	parts := make([]string, 0, end-start+2)
	if start > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
	}
	parts = append(parts, lines[start:end]...)
	if end < total {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("  ↓ %d more", total-end)))
	}
	return strings.Join(parts, "\n")
}

// tailLines keeps the last maxLines lines of text, clipping from the top.
// Used for the conversation view so that the newest (bottom) messages are
// always fully visible when content overflows the available height.
func tailLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) <= maxLines {
		return text
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func renderSection(title, body string, width int) string {
	innerWidth := clamp(width-sectionStyle.GetHorizontalFrameSize(), 12, width)
	content := lipgloss.NewStyle().Width(innerWidth).MaxWidth(innerWidth).Render(body)
	return sectionStyle.Width(innerWidth).Render(sectionTitleStyle.Render(title) + "\n" + content)
}
