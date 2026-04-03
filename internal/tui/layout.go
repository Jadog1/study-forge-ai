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

func renderSection(title, body string, width int) string {
	innerWidth := clamp(width-sectionStyle.GetHorizontalFrameSize(), 12, width)
	content := lipgloss.NewStyle().Width(innerWidth).MaxWidth(innerWidth).Render(body)
	return sectionStyle.Width(innerWidth).Render(sectionTitleStyle.Render(title) + "\n" + content)
}

// SectionDef describes one section in a vertical stack rendered by renderSections.
// When Flex is true the section expands to consume remaining height after all
// fixed-height sections have been measured.
type SectionDef struct {
	Title string
	Body  string
	Flex  bool
}

// renderSections composes multiple SectionDef entries into a single vertical
// stack that fits within the given width×height budget.
//
// Non-flex sections are rendered first at their natural height. Remaining
// vertical space is distributed equally among flex sections whose bodies are
// clipped to fit. If total content still exceeds height the output is clipped.
func renderSections(sections []SectionDef, width, height int) string {
	if len(sections) == 0 {
		return ""
	}
	if height <= 0 {
		return ""
	}

	// Chrome overhead per section: vertical border (2) + title line (1).
	sectionChrome := sectionStyle.GetVerticalFrameSize() + 1

	type part struct {
		rendered string
		height   int
	}
	parts := make([]part, len(sections))
	fixedTotal := 0
	flexCount := 0

	for i, sec := range sections {
		if sec.Flex {
			flexCount++
			continue
		}
		rendered := renderSection(sec.Title, sec.Body, width)
		h := lipgloss.Height(rendered)
		parts[i] = part{rendered: rendered, height: h}
		fixedTotal += h
	}

	remaining := height - fixedTotal
	flexLeft := flexCount
	for i, sec := range sections {
		if !sec.Flex {
			continue
		}
		budget := remaining / flexLeft
		if budget < sectionChrome+1 {
			budget = sectionChrome + 1
		}
		bodyBudget := budget - sectionChrome
		if bodyBudget < 1 {
			bodyBudget = 1
		}
		clipped := clipLines(sec.Body, bodyBudget)
		rendered := renderSection(sec.Title, clipped, width)
		h := lipgloss.Height(rendered)
		parts[i] = part{rendered: rendered, height: h}
		remaining -= h
		flexLeft--
	}

	strs := make([]string, len(parts))
	for i, p := range parts {
		strs[i] = p.rendered
	}
	joined := lipgloss.JoinVertical(lipgloss.Left, strs...)

	if lipgloss.Height(joined) > height {
		joined = clipLines(joined, height)
	}
	return joined
}
