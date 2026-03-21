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
const knowledgeMetaHeight = 2
const knowledgeCompactMinWidth = 88
const knowledgeCompactMinHeight = 18

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

func (k KnowledgeTab) update(msg tea.Msg, selectedClass string) (KnowledgeTab, tea.Cmd) {
	k = k.prepare(selectedClass)
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "r":
			k = k.startLoading()
			return k, loadKnowledgeCmd()
		case "enter":
			if k.activePane == knowledgePaneSections {
				k.activePane = knowledgePaneComponents
				k.componentScroll = 0
			}
			return k, nil
		case "h":
			k.activePane = knowledgePaneSections
			return k, nil
		case "l":
			k.activePane = knowledgePaneComponents
			return k, nil
		case "up", "k":
			return k.nudgeActivePane(-1, selectedClass), nil
		case "down", "j":
			return k.nudgeActivePane(1, selectedClass), nil
		case "pgup":
			return k.pageActivePane(-1, selectedClass), nil
		case "pgdn":
			return k.pageActivePane(1, selectedClass), nil
		case "home":
			return k.moveActivePaneHome(selectedClass), nil
		case "end":
			return k.moveActivePaneEnd(selectedClass), nil
		}
	}
	return k, nil
}

func (k KnowledgeTab) nudgeActivePane(delta int, selectedClass string) KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		k.componentScroll = clamp(k.componentScroll+delta, 0, k.componentMaxScroll(selectedClass))
		return k
	}
	entries := k.filteredEntries(selectedClass)
	if len(entries) == 0 {
		return k
	}
	previous := k.selectedSection
	k.selectedSection = clamp(k.selectedSection+delta, 0, len(entries)-1)
	if previous != k.selectedSection {
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) pageActivePane(direction int, selectedClass string) KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		page := max(3, k.componentViewportHeight()-2)
		k.componentScroll = clamp(k.componentScroll+(direction*page), 0, k.componentMaxScroll(selectedClass))
		return k
	}
	entries := k.filteredEntries(selectedClass)
	if len(entries) == 0 {
		return k
	}
	page := max(1, k.sectionPageSize())
	previous := k.selectedSection
	k.selectedSection = clamp(k.selectedSection+(direction*page), 0, len(entries)-1)
	if previous != k.selectedSection {
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) moveActivePaneHome(selectedClass string) KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		k.componentScroll = 0
		return k
	}
	if len(k.filteredEntries(selectedClass)) == 0 {
		return k
	}
	if k.selectedSection != 0 {
		k.selectedSection = 0
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) moveActivePaneEnd(selectedClass string) KnowledgeTab {
	if k.activePane == knowledgePaneComponents {
		k.componentScroll = k.componentMaxScroll(selectedClass)
		return k
	}
	entries := k.filteredEntries(selectedClass)
	if len(entries) > 0 && k.selectedSection != len(entries)-1 {
		k.selectedSection = len(entries) - 1
		k.componentScroll = 0
	}
	return k
}

func (k KnowledgeTab) view(width, height int, selectedClass string) string {
	k = k.prepare(selectedClass)
	if k.loading && !k.loaded {
		return dimStyle.Render("Loading learned sections…")
	}
	if !k.loaded && !k.loading {
		return dimStyle.Render("Loading learned sections…")
	}
	if k.err != nil {
		return errorStyle.Render("Error loading learned sections: " + k.err.Error())
	}

	entries := k.filteredEntries(selectedClass)
	sectionCount, componentCount := knowledgeCounts(entries)
	if len(entries) == 0 {
		if strings.TrimSpace(selectedClass) != "" {
			return lipgloss.JoinVertical(lipgloss.Left,
				dimStyle.Render("No learned sections for the selected class yet."),
				dimStyle.Render("Run ingest for that class, then refresh with r."),
			)
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			dimStyle.Render("No learned sections yet."),
			dimStyle.Render("Run ingest to build the knowledge index, then refresh with r."),
		)
	}

	metaLine1 := truncateWidth(fmt.Sprintf("%s %d  •  %s %d", labelStyle.Render("Sections:"), sectionCount, labelStyle.Render("Components:"), componentCount), width)
	metaLine2 := truncateWidth(k.metaHint(width), width)
	meta := metaLine1 + "\n" + metaLine2

	layout := knowledgeLayoutFor(width, height, knowledgeMetaHeight)
	if layout.compact {
		if k.activePane == knowledgePaneComponents {
			return meta + "\n" + k.renderComponentsPane(entries, width, layout.paneHeight, true)
		}
		return meta + "\n" + k.renderSectionsPane(entries, width, layout.paneHeight, true)
	}

	leftPane := k.renderSectionsPane(entries, layout.leftWidth, layout.paneHeight, false)
	rightPane := k.renderComponentsPane(entries, layout.rightWidth, layout.paneHeight, false)
	return meta + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, leftPane, dimStyle.Render(" │ "), rightPane)
}

func (k KnowledgeTab) metaHint(width int) string {
	if width < knowledgeCompactMinWidth {
		if k.activePane == knowledgePaneComponents {
			return fmt.Sprintf("%s %s  •  %s", labelStyle.Render("View:"), k.activePane.label(), dimStyle.Render("j/k scroll  •  h back  •  r refresh"))
		}
		return fmt.Sprintf("%s %s  •  %s", labelStyle.Render("View:"), k.activePane.label(), dimStyle.Render("j/k move  •  enter/l open  •  r refresh"))
	}
	return fmt.Sprintf("%s %s  •  %s", labelStyle.Render("Focus:"), k.activePane.label(), dimStyle.Render("h/l switch  •  j/k scroll  •  r refresh"))
}

func (k KnowledgeTab) renderSectionsPane(entries []knowledgeSectionEntry, width, height int, compact bool) string {
	innerWidth := max(12, width)
	innerHeight := max(1, height-2)

	contentLines := []string{}
	pageSize := max(1, innerHeight/knowledgeSectionRowHeight)
	start := 0
	if k.selectedSection >= pageSize {
		start = k.selectedSection - pageSize + 1
		if k.selectedSection < len(entries)-1 {
			centered := k.selectedSection - (pageSize / 2)
			if centered > 0 {
				start = centered
			}
		}
	}
	maxStart := max(0, len(entries)-pageSize)
	start = clamp(start, 0, maxStart)
	end := min(len(entries), start+pageSize)

	for i := start; i < end; i++ {
		selected := i == k.selectedSection
		contentLines = append(contentLines, strings.Split(k.renderSectionRow(entries[i], innerWidth, selected), "\n")...)
	}
	for len(contentLines) < innerHeight {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > innerHeight {
		contentLines = contentLines[:innerHeight]
	}

	title := sectionTitleStyle.Render("Sections")
	if compact {
		title = sectionTitleStyle.Render("Sections") + dimStyle.Render("  choose a section")
	}
	divider := dimStyle.Render(strings.Repeat("─", max(8, innerWidth)))
	body := strings.Join(contentLines, "\n")
	return lipgloss.NewStyle().Width(innerWidth).MaxWidth(innerWidth).Height(height).MaxHeight(height).Render(title + "\n" + divider + "\n" + body)
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
	preview := strings.TrimSpace(strings.ReplaceAll(entry.Section.Summary, "\n", " "))
	if preview == "" {
		preview = "No summary"
	}
	details := fmt.Sprintf("%d comp  •  %s  •  %s", len(entry.Components), knowledgeSummaryMetrics(entry.Metrics), preview)

	rowText := strings.Join([]string{
		truncateWidth(marker+" "+title, width),
		truncateWidth("  "+details, width),
	}, "\n")

	style := knowledgeSectionRowStyle(selected, selected && k.activePane == knowledgePaneSections).Width(width).MaxWidth(width)
	return style.Render(rowText)
}

func (k KnowledgeTab) renderComponentsPane(entries []knowledgeSectionEntry, width, height int, compact bool) string {
	innerWidth := max(14, width)
	innerHeight := max(1, height-2)

	lines := k.buildComponentLines(entries, innerWidth)
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

	title := sectionTitleStyle.Render("Details")
	if compact {
		title = sectionTitleStyle.Render("Details") + dimStyle.Render("  h to go back")
	}
	divider := dimStyle.Render(strings.Repeat("─", max(8, innerWidth)))
	return lipgloss.NewStyle().Width(innerWidth).MaxWidth(innerWidth).Height(height).MaxHeight(height).Render(title + "\n" + divider + "\n" + strings.Join(visible, "\n"))
}

func (k KnowledgeTab) buildComponentLines(entries []knowledgeSectionEntry, width int) []string {
	entry, ok := k.currentSectionFromEntries(entries)
	if !ok {
		return []string{dimStyle.Render("Select a section to inspect its components.")}
	}

	lines := []string{
		headerStyle.Render(truncateWidth(entry.Section.Title, width)),
		truncateWidth(fmt.Sprintf("Components: %d  •  Quiz: %s", len(entry.Components), knowledgeDetailedMetrics(entry.Metrics)), width),
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

func (k KnowledgeTab) currentSectionFromEntries(entries []knowledgeSectionEntry) (knowledgeSectionEntry, bool) {
	if len(entries) == 0 || k.selectedSection < 0 || k.selectedSection >= len(entries) {
		return knowledgeSectionEntry{}, false
	}
	return entries[k.selectedSection], true
}

func (k KnowledgeTab) componentMaxScroll(selectedClass string) int {
	innerWidth, innerHeight := k.componentInnerDimensions()
	if innerWidth <= 0 || innerHeight <= 0 {
		return 0
	}
	lines := k.buildComponentLines(k.filteredEntries(selectedClass), innerWidth)
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
	layout := knowledgeLayoutFor(k.width, k.height, knowledgeMetaHeight)
	return max(14, layout.detailWidth()), max(1, layout.paneHeight-2)
}

func (k KnowledgeTab) sectionInnerDimensions() (int, int) {
	if k.width <= 0 || k.height <= 0 {
		return 0, 0
	}
	layout := knowledgeLayoutFor(k.width, k.height, knowledgeMetaHeight)
	return max(12, layout.listWidth()), max(1, layout.paneHeight-2)
}

type knowledgeLayout struct {
	compact    bool
	leftWidth  int
	rightWidth int
	paneHeight int
}

func knowledgeLayoutFor(width, height, metaHeight int) knowledgeLayout {
	if width <= 0 || height <= 0 {
		return knowledgeLayout{}
	}

	paneHeight := max(6, height-metaHeight-1)
	if width < knowledgeCompactMinWidth || height < knowledgeCompactMinHeight {
		return knowledgeLayout{compact: true, leftWidth: width, rightWidth: width, paneHeight: paneHeight}
	}

	leftWidth := clamp(width/3, 24, max(24, width-36))
	rightWidth := max(28, width-leftWidth-3)
	return knowledgeLayout{leftWidth: leftWidth, rightWidth: rightWidth, paneHeight: paneHeight}
}

func (l knowledgeLayout) listWidth() int {
	return l.leftWidth
}

func (l knowledgeLayout) detailWidth() int {
	return l.rightWidth
}

func (p knowledgePane) label() string {
	if p == knowledgePaneComponents {
		return "details"
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

func knowledgeSummaryMetrics(metrics knowledgeQuizMetrics) string {
	if metrics.Attempts == 0 {
		return "no quiz yet"
	}
	return fmt.Sprintf("%.0f%% accuracy", metrics.Accuracy*100)
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

func (k KnowledgeTab) prepare(selectedClass string) KnowledgeTab {
	entries := k.filteredEntries(selectedClass)
	if len(entries) == 0 {
		k.selectedSection = 0
		k.componentScroll = 0
		return k
	}

	selectedID := ""
	if len(k.entries) > 0 && k.selectedSection >= 0 && k.selectedSection < len(k.entries) {
		selectedID = k.entries[k.selectedSection].Section.ID
	}
	if selectedID != "" {
		found := false
		for i, entry := range entries {
			if entry.Section.ID == selectedID {
				k.selectedSection = i
				found = true
				break
			}
		}
		if !found {
			k.selectedSection = 0
		}
	} else {
		k.selectedSection = clamp(k.selectedSection, 0, len(entries)-1)
	}

	k.componentScroll = clamp(k.componentScroll, 0, k.componentMaxScroll(selectedClass))
	return k
}

func (k KnowledgeTab) filteredEntries(selectedClass string) []knowledgeSectionEntry {
	className := strings.TrimSpace(selectedClass)
	if className == "" {
		return k.entries
	}
	filtered := make([]knowledgeSectionEntry, 0, len(k.entries))
	for _, entry := range k.entries {
		if strings.EqualFold(strings.TrimSpace(entry.Section.Class), className) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func knowledgeCounts(entries []knowledgeSectionEntry) (int, int) {
	components := 0
	for _, entry := range entries {
		components += len(entry.Components)
	}
	return len(entries), components
}

func knowledgeWrapTextLines(text string, width int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{dimStyle.Render("No details available.")}
	}
	rendered := lipgloss.NewStyle().Width(width).MaxWidth(width).Render(trimmed)
	return strings.Split(strings.ReplaceAll(rendered, "\r\n", "\n"), "\n")
}

func knowledgeSectionRowStyle(selected, active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	if selected {
		style = style.Background(lipgloss.Color("236"))
	}
	if selected && active {
		style = style.Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
	}
	return style
}
