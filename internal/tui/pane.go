package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type splitPaneConfig struct {
	minTotalWidth    int
	minRightWidth    int
	sidebarWidth     int
	leftFraction     int
	minLeftWidth     int
	minStackedHeight int
}

type splitPaneLayout struct {
	stacked     bool
	leftWidth   int
	rightWidth  int
	leftHeight  int
	rightHeight int
}

func splitPaneLayoutFor(cfg splitPaneConfig, width, height, metaHeight int, rightFocused bool) splitPaneLayout {
	if width <= 0 || height <= 0 {
		return splitPaneLayout{}
	}

	paneHeight := max(8, height-metaHeight-1)
	maxLeft := max(cfg.minLeftWidth, width-cfg.minRightWidth-1)
	leftWidth := clamp(width/cfg.leftFraction, cfg.minLeftWidth, maxLeft)
	if rightFocused {
		leftWidth = clamp(cfg.sidebarWidth, cfg.minLeftWidth, maxLeft)
	}
	rightWidth := width - leftWidth - 1
	canSplit := width >= cfg.minTotalWidth && rightWidth >= cfg.minRightWidth

	if canSplit {
		return splitPaneLayout{
			leftWidth:   leftWidth,
			rightWidth:  rightWidth,
			leftHeight:  paneHeight,
			rightHeight: paneHeight,
		}
	}

	leftHeight := max(cfg.minStackedHeight, paneHeight/3)
	rightHeight := max(cfg.minStackedHeight, paneHeight-leftHeight)
	if leftHeight+rightHeight > paneHeight {
		rightHeight = paneHeight - leftHeight
	}
	if rightHeight < cfg.minStackedHeight {
		rightHeight = cfg.minStackedHeight
		leftHeight = max(cfg.minStackedHeight, paneHeight-rightHeight)
	}

	return splitPaneLayout{
		stacked:     true,
		leftWidth:   width,
		rightWidth:  width,
		leftHeight:  leftHeight,
		rightHeight: rightHeight,
	}
}

// renderListPane renders a bordered pane containing a windowed list of items.
// items are pre-rendered row strings (may be multi-line). innerWidth and
// innerHeight are the space available for list content only (excluding title,
// footer, and border). The function adds the title line and optional footer
// inside the bordered area.
func renderListPane(items []string, selectedIndex, rowHeight, innerWidth, innerHeight int, borderStyle lipgloss.Style, title, footer string) string {
	pageSize := max(1, innerHeight/rowHeight)
	start := 0
	if selectedIndex >= pageSize {
		start = selectedIndex - pageSize + 1
		if selectedIndex < len(items)-1 {
			centered := selectedIndex - (pageSize / 2)
			if centered > 0 {
				start = centered
			}
		}
	}
	maxStart := max(0, len(items)-pageSize)
	start = clamp(start, 0, maxStart)
	end := min(len(items), start+pageSize)

	contentLines := make([]string, 0, innerHeight)
	for i := start; i < end; i++ {
		contentLines = append(contentLines, strings.Split(items[i], "\n")...)
	}
	for len(contentLines) < innerHeight {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > innerHeight {
		contentLines = contentLines[:innerHeight]
	}

	parts := []string{sectionTitleStyle.Render(title), strings.Join(contentLines, "\n")}
	if footer != "" {
		parts = append(parts, footer)
	}
	contentHeight := innerHeight + len(parts) - 1 // title + body(=innerHeight lines) + optional footer

	content := lipgloss.NewStyle().
		Width(innerWidth).MaxWidth(innerWidth).
		Height(contentHeight).MaxHeight(contentHeight).
		Render(strings.Join(parts, "\n"))
	return borderStyle.Render(content)
}

// renderScrollPane renders a bordered pane containing scrollable text lines.
// innerWidth and innerHeight are the space available for text content only
// (excluding title, footer, and border). The function adds the title line
// and optional footer inside the bordered area.
func renderScrollPane(lines []string, scroll, innerWidth, innerHeight int, borderStyle lipgloss.Style, title, footer string) string {
	if len(lines) == 0 {
		lines = []string{dimStyle.Render("No details available.")}
	}
	start := clampPaneScroll(scroll, len(lines), innerHeight)
	end := min(len(lines), start+innerHeight)
	visible := append([]string{}, lines[start:end]...)
	for len(visible) < innerHeight {
		visible = append(visible, "")
	}

	parts := []string{sectionTitleStyle.Render(title), strings.Join(visible, "\n")}
	if footer != "" {
		parts = append(parts, footer)
	}
	contentHeight := innerHeight + len(parts) - 1

	content := lipgloss.NewStyle().
		Width(innerWidth).MaxWidth(innerWidth).
		Height(contentHeight).MaxHeight(contentHeight).
		Render(strings.Join(parts, "\n"))
	return borderStyle.Render(content)
}

func nudgePaneSelection(current, delta, maxItems int) int {
	if maxItems <= 0 {
		return 0
	}
	return clamp(current+delta, 0, maxItems-1)
}

func pagePaneSelection(current, delta, maxItems int) int {
	if maxItems <= 0 {
		return 0
	}
	return clamp(current+delta, 0, maxItems-1)
}

func clampPaneScroll(scroll, totalLines, visibleHeight int) int {
	maxScroll := 0
	if totalLines > visibleHeight {
		maxScroll = totalLines - visibleHeight
	}
	return clamp(scroll, 0, maxScroll)
}
