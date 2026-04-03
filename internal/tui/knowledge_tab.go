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
		innerWidth, innerHeight := k.componentInnerDimensions()
		lines := k.buildComponentLines(innerWidth)
		k.componentScroll = clampPaneScroll(k.componentScroll+delta, len(lines), innerHeight)
		return k
	}
	previous := k.selectedSection
	k.selectedSection = nudgePaneSelection(k.selectedSection, delta, len(k.entries))
	if previous != k.selectedSection {
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) pageActivePane(direction int) KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		page := max(3, k.componentViewportHeight()-2)
		innerWidth, innerHeight := k.componentInnerDimensions()
		lines := k.buildComponentLines(innerWidth)
		k.componentScroll = clampPaneScroll(k.componentScroll+(direction*page), len(lines), innerHeight)
		return k
	}
	page := max(1, k.sectionPageSize())
	previous := k.selectedSection
	k.selectedSection = pagePaneSelection(k.selectedSection, direction*page, len(k.entries))
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
		innerWidth, innerHeight := k.componentInnerDimensions()
		lines := k.buildComponentLines(innerWidth)
		k.componentScroll = clampPaneScroll(len(lines), len(lines), innerHeight)
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
		return renderSkeleton(width, height)
	}
	if !k.loaded && !k.loading {
		return renderSkeleton(width, height)
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
	metaLine2 := truncateWidth(fmt.Sprintf("%s %s", labelStyle.Render("Focus:"), k.activePane.label()), width)
	meta := metaLine1 + "\n" + metaLine2
	layout := splitPaneLayoutFor(knowledgeSplitConfig(), width, height, metaHeight, k.activePane == knowledgePaneComponents)
	leftPane := k.renderSectionsPane(layout.leftWidth, layout.leftHeight)
	rightPane := k.renderComponentsPane(layout.rightWidth, layout.rightHeight)

	constrain := lipgloss.NewStyle().Width(width).MaxWidth(width)
	if layout.stacked {
		return constrain.Render(meta + "\n" + lipgloss.JoinVertical(lipgloss.Left, leftPane, rightPane))
	}
	return constrain.Render(meta + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane))
}

func (k KnowledgeTab) renderSectionsPane(width, height int) string {
	style := knowledgePaneBorderStyle(k.activePane == knowledgePaneSections).Width(width)
	innerWidth := max(12, width-style.GetHorizontalFrameSize())
	contentHeight := max(2, height-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-1)

	items := make([]string, len(k.entries))
	for i, entry := range k.entries {
		items[i] = k.renderSectionRow(entry, innerWidth, i == k.selectedSection)
	}
	return renderListPane(items, k.selectedSection, knowledgeSectionRowHeight, innerWidth, innerHeight, style, "Sections", "")
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
	return renderScrollPane(lines, k.componentScroll, innerWidth, innerHeight, style, "Components", "")
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
	layout := splitPaneLayoutFor(knowledgeSplitConfig(), k.width, k.height, 3, k.activePane == knowledgePaneComponents)
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
	layout := splitPaneLayoutFor(knowledgeSplitConfig(), k.width, k.height, 3, k.activePane == knowledgePaneComponents)
	style := knowledgePaneBorderStyle(false)
	innerWidth := max(12, layout.leftWidth-style.GetHorizontalFrameSize())
	contentHeight := max(2, layout.leftHeight-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-1)
	return innerWidth, innerHeight
}

func knowledgeSplitConfig() splitPaneConfig {
	return splitPaneConfig{
		minTotalWidth:    knowledgeSplitMinTotalWidth,
		minRightWidth:    knowledgeSplitMinRightWidth,
		sidebarWidth:     knowledgeSidebarCompactWidth,
		leftFraction:     3,
		minLeftWidth:     20,
		minStackedHeight: 4,
	}
}

func (p knowledgePane) label() string {
	if p == knowledgePaneComponents {
		return "components"
	}
	return "sections"
}

func (k KnowledgeTab) helpKeys() []KeyBinding {
	return []KeyBinding{
		{Key: "h/l", Desc: "panes"},
		{Key: "↑/↓", Desc: "select"},
		{Key: "PgUp/PgDn", Desc: "page"},
		{Key: "Enter", Desc: "expand"},
	}
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

func knowledgeWrapTextLines(text string, width int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{dimStyle.Render("No details available.")}
	}
	rendered := lipgloss.NewStyle().Width(width).MaxWidth(width).Render(trimmed)
	return strings.Split(strings.ReplaceAll(rendered, "\r\n", "\n"), "\n")
}

