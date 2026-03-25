package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/studyforge/study-agent/internal/state"
)

type knowledgePane int

const (
	knowledgePaneSections knowledgePane = iota
	knowledgePaneComponents
)

const knowledgeSectionRowHeight = 2
const knowledgeSplitMinRightWidth = 24
const knowledgeSplitMinTotalWidth = 60
const knowledgeSidebarCompactWidth = 24

type knowledgeQuizMetrics struct {
	Attempts     int
	Correct      int
	Incorrect    int
	Accuracy     float64
	LastAnswered time.Time
}

type knowledgeComponentEntry struct {
	Component state.Component
	Metrics   knowledgeQuizMetrics
}

type knowledgeSectionEntry struct {
	Section    state.Section
	Components []knowledgeComponentEntry
	Metrics    knowledgeQuizMetrics
}

type KnowledgeTab struct {
	entries              []knowledgeSectionEntry
	err                  error
	loaded               bool
	loading              bool
	selectedSection      int
	activePane           knowledgePane
	componentScroll      int
	width                int
	height               int
	totalComponentsCount int
}

func newKnowledgeTab() KnowledgeTab {
	return KnowledgeTab{activePane: knowledgePaneSections}
}

func (k KnowledgeTab) resize(width, height int) KnowledgeTab {
	k.width = width
	k.height = height
	return k
}

func (k KnowledgeTab) startLoading() KnowledgeTab {
	k.loading = true
	k.err = nil
	return k
}

func (k KnowledgeTab) receive(sectionIndex *state.SectionIndex, componentIndex *state.ComponentIndex, err error) KnowledgeTab {
	selectedID := ""
	if current, ok := k.currentSection(); ok {
		selectedID = current.Section.ID
	}

	k.loaded = true
	k.loading = false
	k.err = err
	k.entries = nil
	k.totalComponentsCount = 0
	k.componentScroll = 0

	if err != nil || sectionIndex == nil || componentIndex == nil {
		k.selectedSection = 0
		return k
	}

	componentsBySection := make(map[string][]state.Component)
	for _, component := range componentIndex.Components {
		sectionID := strings.TrimSpace(component.SectionID)
		componentsBySection[sectionID] = append(componentsBySection[sectionID], component)
		k.totalComponentsCount++
	}

	entries := make([]knowledgeSectionEntry, 0, len(sectionIndex.Sections))
	for _, section := range sectionIndex.Sections {
		components := componentsBySection[strings.TrimSpace(section.ID)]
		componentEntries := make([]knowledgeComponentEntry, 0, len(components))
		for _, component := range components {
			componentEntries = append(componentEntries, knowledgeComponentEntry{
				Component: component,
				Metrics:   knowledgeMetricsFromHistory(component.QuestionHistory),
			})
		}
		sort.Slice(componentEntries, func(i, j int) bool {
			left := componentEntries[i].Component
			right := componentEntries[j].Component
			if left.UpdatedAt.Equal(right.UpdatedAt) {
				if !strings.EqualFold(left.Kind, right.Kind) {
					return strings.ToLower(left.Kind) < strings.ToLower(right.Kind)
				}
				return strings.ToLower(left.Content) < strings.ToLower(right.Content)
			}
			return left.UpdatedAt.After(right.UpdatedAt)
		})

		entries = append(entries, knowledgeSectionEntry{
			Section:    section,
			Components: componentEntries,
			Metrics:    aggregateSectionMetrics(section, components),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		left := entries[i].Section
		right := entries[j].Section
		if strings.EqualFold(left.Class, right.Class) {
			if strings.EqualFold(left.Title, right.Title) {
				return left.UpdatedAt.After(right.UpdatedAt)
			}
			return strings.ToLower(left.Title) < strings.ToLower(right.Title)
		}
		return strings.ToLower(left.Class) < strings.ToLower(right.Class)
	})

	k.entries = entries
	k.selectedSection = 0
	if selectedID != "" {
		for i, entry := range entries {
			if entry.Section.ID == selectedID {
				k.selectedSection = i
				break
			}
		}
	}
	if k.selectedSection >= len(k.entries) {
		k.selectedSection = max(0, len(k.entries)-1)
	}
	return k
}

func (k KnowledgeTab) update(msg tea.Msg) (KnowledgeTab, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "r":
			k = k.startLoading()
			return k, loadKnowledgeCmd()
		case "h":
			k.activePane = knowledgePaneSections
			return k, nil
		case "l":
			k.activePane = knowledgePaneComponents
			return k, nil
		case "up", "k":
			return k.nudgeActivePane(-1), nil
		case "down", "j":
			return k.nudgeActivePane(1), nil
		case "pgup":
			return k.pageActivePane(-1), nil
		case "pgdn":
			return k.pageActivePane(1), nil
		case "home":
			return k.moveActivePaneHome(), nil
		case "end":
			return k.moveActivePaneEnd(), nil
		}
	}
	return k, nil
}

func (k KnowledgeTab) nudgeActivePane(delta int) KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		k.componentScroll = clamp(k.componentScroll+delta, 0, k.componentMaxScroll())
		return k
	}
	if len(k.entries) == 0 {
		return k
	}
	previous := k.selectedSection
	k.selectedSection = clamp(k.selectedSection+delta, 0, len(k.entries)-1)
	if previous != k.selectedSection {
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) pageActivePane(direction int) KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		page := max(3, k.componentViewportHeight()-2)
		k.componentScroll = clamp(k.componentScroll+(direction*page), 0, k.componentMaxScroll())
		return k
	}
	if len(k.entries) == 0 {
		return k
	}
	page := max(1, k.sectionPageSize())
	previous := k.selectedSection
	k.selectedSection = clamp(k.selectedSection+(direction*page), 0, len(k.entries)-1)
	if previous != k.selectedSection {
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) moveActivePaneHome() KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		k.componentScroll = 0
		return k
	}
	if k.selectedSection != 0 {
		k.selectedSection = 0
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) moveActivePaneEnd() KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		k.componentScroll = k.componentMaxScroll()
		return k
	}
	if len(k.entries) > 0 && k.selectedSection != len(k.entries)-1 {
		k.selectedSection = len(k.entries) - 1
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) view(width, height int, selectedClass string) string {
	if k.loading && !k.loaded {
		return dimStyle.Render("Loading learned sections…")
	}
	if !k.loaded && !k.loading {
		return dimStyle.Render("Loading learned sections…")
	}
	if k.err != nil {
		return errorStyle.Render("Error loading learned sections: " + k.err.Error())
	}
	if len(k.entries) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			dimStyle.Render("No learned sections yet."),
			dimStyle.Render("Run ingest to build the knowledge index, then refresh with r."),
		)
	}

	classLabel := "all classes"
	if strings.TrimSpace(selectedClass) != "" {
		classLabel = selectedClass
	}
	const metaHeight = 2
	metaLine1 := truncateWidth(fmt.Sprintf("%s %d  %s %d  %s %s", labelStyle.Render("Sections:"), len(k.entries), labelStyle.Render("Components:"), k.totalComponentsCount, labelStyle.Render("Selected class:"), classLabel), width)
	metaLine2 := truncateWidth(fmt.Sprintf("%s %s  •  %s", labelStyle.Render("Focus:"), k.activePane.label(), dimStyle.Render("h/l switch pane  •  up/down scroll  •  r refresh")), width)
	meta := metaLine1 + "\n" + metaLine2
	layout := knowledgeLayoutFor(width, height, metaHeight, k.activePane)
	leftPane := k.renderSectionsPane(layout.leftWidth, layout.leftHeight)
	rightPane := k.renderComponentsPane(layout.rightWidth, layout.rightHeight)

	if layout.stacked {
		return meta + "\n" + lipgloss.JoinVertical(lipgloss.Left, leftPane, rightPane)
	}
	return meta + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)
}

func (k KnowledgeTab) renderSectionsPane(width, height int) string {
	style := knowledgePaneBorderStyle(k.activePane == knowledgePaneSections).Width(width)
	innerWidth := max(12, width-style.GetHorizontalFrameSize())
	contentHeight := max(2, height-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-1)

	contentLines := []string{}
	pageSize := max(1, innerHeight/knowledgeSectionRowHeight)
	start := 0
	if k.selectedSection >= pageSize {
		start = k.selectedSection - pageSize + 1
		if k.selectedSection < len(k.entries)-1 {
			centered := k.selectedSection - (pageSize / 2)
			if centered > 0 {
				start = centered
			}
		}
	}
	maxStart := max(0, len(k.entries)-pageSize)
	start = clamp(start, 0, maxStart)
	end := min(len(k.entries), start+pageSize)

	for i := start; i < end; i++ {
		selected := i == k.selectedSection
		contentLines = append(contentLines, strings.Split(k.renderSectionRow(k.entries[i], innerWidth, selected), "\n")...)
	}
	for len(contentLines) < innerHeight {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > innerHeight {
		contentLines = contentLines[:innerHeight]
	}

	body := strings.Join(contentLines, "\n")
	title := sectionTitleStyle.Render("Sections")
	content := lipgloss.NewStyle().
		Width(innerWidth).
		MaxWidth(innerWidth).
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render(title + "\n" + body)
	return style.Render(content)
}

func (k KnowledgeTab) renderSectionRow(entry knowledgeSectionEntry, width int, selected bool) string {
	title := truncateWidth(entry.Section.Title, width-2)
	if title == "" {
		title = entry.Section.ID
	}
	marker := " "
	if selected {
		marker = ">"
	}
	details := emptyFallback(entry.Section.Class, "unclassified") + "  •  " + fmt.Sprintf("%d comp", len(entry.Components)) + "  •  " + knowledgeCompactMetrics(entry.Metrics)

	rowText := strings.Join([]string{
		truncateWidth(marker+" "+title, width),
		truncateWidth("  "+details, width),
	}, "\n")

	style := knowledgeSectionRowStyle(selected, selected && k.activePane == knowledgePaneSections).Width(width).MaxWidth(width)
	return style.Render(rowText)
}

func (k KnowledgeTab) renderComponentsPane(width, height int) string {
	style := knowledgePaneBorderStyle(k.activePane == knowledgePaneComponents).Width(width)
	innerWidth := max(14, width-style.GetHorizontalFrameSize())
	contentHeight := max(2, height-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-1)

	lines := k.buildComponentLines(innerWidth)
	maxScroll := 0
	if len(lines) > innerHeight {
		maxScroll = len(lines) - innerHeight
	}
	start := clamp(k.componentScroll, 0, maxScroll)
	end := min(len(lines), start+innerHeight)
	visible := append([]string{}, lines[start:end]...)
	for len(visible) < innerHeight {
		visible = append(visible, "")
	}

	title := sectionTitleStyle.Render("Components")
	content := lipgloss.NewStyle().
		Width(innerWidth).
		MaxWidth(innerWidth).
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render(title + "\n" + strings.Join(visible, "\n"))
	return style.Render(content)
}

func (k KnowledgeTab) buildComponentLines(width int) []string {
	entry, ok := k.currentSection()
	if !ok {
		return []string{dimStyle.Render("Select a section to inspect its components.")}
	}

	lines := []string{
		headerStyle.Render(truncateWidth(entry.Section.Title, width)),
		truncateWidth(fmt.Sprintf("Class: %s  •  Components: %d", emptyFallback(entry.Section.Class, "unclassified"), len(entry.Components)), width),
		truncateWidth(fmt.Sprintf("Quiz: %s", knowledgeDetailedMetrics(entry.Metrics)), width),
	}
	if len(entry.Section.Tags) > 0 {
		lines = append(lines, truncateWidth("Tags: "+strings.Join(entry.Section.Tags, ", "), width))
	}
	if len(entry.Section.Concepts) > 0 {
		lines = append(lines, truncateWidth("Concepts: "+strings.Join(entry.Section.Concepts, ", "), width))
	}
	lines = append(lines, "", dimStyle.Render("Summary"))
	lines = append(lines, knowledgeWrapTextLines(entry.Section.Summary, width)...)

	lines = append(lines, "", dimStyle.Render(fmt.Sprintf("Components (%d)", len(entry.Components))))
	if len(entry.Components) == 0 {
		lines = append(lines, dimStyle.Render("No components linked to this section."))
		return lines
	}

	for i, component := range entry.Components {
		if i > 0 {
			lines = append(lines, dimStyle.Render(strings.Repeat("-", width)))
		}
		title := fmt.Sprintf("[%s] %s", emptyFallback(component.Component.Kind, "component"), knowledgeDetailedMetrics(component.Metrics))
		lines = append(lines, truncateWidth(title, width))
		lines = append(lines, knowledgeWrapTextLines(component.Component.Content, width)...)
		if len(component.Component.Tags) > 0 {
			lines = append(lines, truncateWidth(dimStyle.Render("Tags: "+strings.Join(component.Component.Tags, ", ")), width))
		}
		if len(component.Component.Concepts) > 0 {
			lines = append(lines, truncateWidth(dimStyle.Render("Concepts: "+strings.Join(component.Component.Concepts, ", ")), width))
		}
		if len(component.Component.SourcePaths) > 0 {
			lines = append(lines, truncateWidth(dimStyle.Render(fmt.Sprintf("Sources: %d", len(component.Component.SourcePaths))), width))
		}
	}

	return lines
}

func (k KnowledgeTab) currentSection() (knowledgeSectionEntry, bool) {
	if len(k.entries) == 0 || k.selectedSection < 0 || k.selectedSection >= len(k.entries) {
		return knowledgeSectionEntry{}, false
	}
	return k.entries[k.selectedSection], true
}

func (k KnowledgeTab) componentMaxScroll() int {
	innerWidth, innerHeight := k.componentInnerDimensions()
	if innerWidth <= 0 || innerHeight <= 0 {
		return 0
	}
	lines := k.buildComponentLines(innerWidth)
	if len(lines) <= innerHeight {
		return 0
	}
	return len(lines) - innerHeight
}

func (k KnowledgeTab) sectionPageSize() int {
	return max(1, k.sectionViewportHeight()/knowledgeSectionRowHeight)
}

func (k KnowledgeTab) componentViewportHeight() int {
	_, innerHeight := k.componentInnerDimensions()
	return innerHeight
}

func (k KnowledgeTab) sectionViewportHeight() int {
	_, innerHeight := k.sectionInnerDimensions()
	return innerHeight
}

func (k KnowledgeTab) componentInnerDimensions() (int, int) {
	if k.width <= 0 || k.height <= 0 {
		return 0, 0
	}
	layout := knowledgeLayoutFor(k.width, k.height, 3, k.activePane)
	style := knowledgePaneBorderStyle(false)
	innerWidth := max(14, layout.rightWidth-style.GetHorizontalFrameSize())
	contentHeight := max(2, layout.rightHeight-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-1)
	return innerWidth, innerHeight
}

func (k KnowledgeTab) sectionInnerDimensions() (int, int) {
	if k.width <= 0 || k.height <= 0 {
		return 0, 0
	}
	layout := knowledgeLayoutFor(k.width, k.height, 3, k.activePane)
	style := knowledgePaneBorderStyle(false)
	innerWidth := max(12, layout.leftWidth-style.GetHorizontalFrameSize())
	contentHeight := max(2, layout.leftHeight-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-1)
	return innerWidth, innerHeight
}

type knowledgeLayout struct {
	stacked     bool
	leftWidth   int
	rightWidth  int
	leftHeight  int
	rightHeight int
}

func knowledgeLayoutFor(width, height, metaHeight int, activePane knowledgePane) knowledgeLayout {
	if width <= 0 || height <= 0 {
		return knowledgeLayout{}
	}

	paneHeight := max(8, height-metaHeight-1)
	leftWidth := clamp(width/3, 20, max(20, width-knowledgeSplitMinRightWidth-1))
	if activePane == knowledgePaneComponents {
		leftWidth = clamp(knowledgeSidebarCompactWidth, 20, max(20, width-knowledgeSplitMinRightWidth-1))
	}
	rightWidth := width - leftWidth - 1
	canSplit := width >= knowledgeSplitMinTotalWidth && rightWidth >= knowledgeSplitMinRightWidth
	if canSplit {
		return knowledgeLayout{
			stacked:     false,
			leftWidth:   leftWidth,
			rightWidth:  rightWidth,
			leftHeight:  paneHeight,
			rightHeight: paneHeight,
		}
	}

	leftHeight := max(4, paneHeight/3)
	rightHeight := max(4, paneHeight-leftHeight)
	if leftHeight+rightHeight > paneHeight {
		rightHeight = paneHeight - leftHeight
	}
	if rightHeight < 4 {
		rightHeight = 4
		leftHeight = max(4, paneHeight-rightHeight)
	}

	return knowledgeLayout{
		stacked:     true,
		leftWidth:   width,
		rightWidth:  width,
		leftHeight:  leftHeight,
		rightHeight: rightHeight,
	}
}

func (p knowledgePane) label() string {
	if p == knowledgePaneComponents {
		return "components"
	}
	return "sections"
}

func knowledgeMetricsFromHistory(history []state.QuestionHistoryEntry) knowledgeQuizMetrics {
	metrics := knowledgeQuizMetrics{Attempts: len(history)}
	for _, item := range history {
		if item.Correct {
			metrics.Correct++
		} else {
			metrics.Incorrect++
		}
		if item.AnsweredAt.After(metrics.LastAnswered) {
			metrics.LastAnswered = item.AnsweredAt
		}
	}
	if metrics.Attempts > 0 {
		metrics.Accuracy = float64(metrics.Correct) / float64(metrics.Attempts)
	}
	return metrics
}

func aggregateSectionMetrics(section state.Section, components []state.Component) knowledgeQuizMetrics {
	seen := make(map[string]bool)
	history := make([]state.QuestionHistoryEntry, 0, len(section.QuestionHistory))
	appendUnique := func(items []state.QuestionHistoryEntry) {
		for _, item := range items {
			key := strings.TrimSpace(item.ID)
			if key == "" {
				key = fmt.Sprintf("%s|%s|%t|%s", strings.TrimSpace(item.QuizID), strings.TrimSpace(item.QuestionID), item.Correct, item.AnsweredAt.UTC().Format(time.RFC3339Nano))
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			history = append(history, item)
		}
	}

	appendUnique(section.QuestionHistory)
	for _, component := range components {
		appendUnique(component.QuestionHistory)
	}
	return knowledgeMetricsFromHistory(history)
}

func knowledgeCompactMetrics(metrics knowledgeQuizMetrics) string {
	if metrics.Attempts == 0 {
		return "Quiz: no attempts yet"
	}
	return fmt.Sprintf("Quiz: %d attempts  •  %.0f%% accuracy", metrics.Attempts, metrics.Accuracy*100)
}

func knowledgeDetailedMetrics(metrics knowledgeQuizMetrics) string {
	if metrics.Attempts == 0 {
		return "no attempts yet"
	}
	last := metrics.LastAnswered.Format("2006-01-02")
	if metrics.LastAnswered.IsZero() {
		last = "unknown"
	}
	return fmt.Sprintf("%d attempts  •  %d correct  •  %.0f%% accuracy  •  last %s", metrics.Attempts, metrics.Correct, metrics.Accuracy*100, last)
}

func emptyFallback(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func knowledgeWrapTextLines(text string, width int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{dimStyle.Render("No details available.")}
	}
	rendered := lipgloss.NewStyle().Width(width).MaxWidth(width).Render(trimmed)
	return strings.Split(strings.ReplaceAll(rendered, "\r\n", "\n"), "\n")
}

func knowledgePaneBorderStyle(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1)
	if active {
		style = style.BorderForeground(lipgloss.Color("244"))
	}
	return style
}

func knowledgeSectionRowStyle(selected, active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	if selected {
		style = style.Background(lipgloss.Color("236")).Foreground(lipgloss.Color("230"))
	}
	if selected && active {
		style = style.Bold(true)
	}
	return style
}
