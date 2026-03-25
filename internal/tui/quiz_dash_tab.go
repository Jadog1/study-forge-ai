package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/studyforge/study-agent/internal/state"
)

type quizDashboardPane int

const (
	quizDashboardPaneSections quizDashboardPane = iota
	quizDashboardPaneDetails
)

const quizDashboardSectionRowHeight = 2

// QuizDashboardTab holds state for the quiz dashboard tab.
type QuizDashboardTab struct {
	loaded          bool
	loading         bool
	err             error
	snapshot        *quizDashboardSnapshot
	selectedSection int
	activePane      quizDashboardPane
	detailScroll    int
	width           int
	height          int
}

func newQuizDashboardTab() QuizDashboardTab {
	return QuizDashboardTab{activePane: quizDashboardPaneSections}
}

func (q QuizDashboardTab) resize(width, height int) QuizDashboardTab {
	q.width = width
	q.height = height
	return q
}

type quizDashboardSnapshot struct {
	Sections   []state.Section
	Components []state.Component
	Tracked    *state.TrackedQuizCache
	Quizzes    []quizDashboardQuizDoc
	LoadedAt   time.Time
}

type quizDashboardQuizDoc struct {
	ID            string
	Class         string
	Title         string
	Path          string
	QuestionCount int
	GeneratedAt   time.Time
}

type quizDashboardSummary struct {
	SectionsHit       int
	TotalSections     int
	ComponentsHit     int
	TotalComponents   int
	TotalAttempts     int
	CorrectAnswers    int
	Accuracy          float64
	SavedQuizzes      int
	TrackedQuizzes    int
	TrackedSynced     int
	TrackedPending    int
	LatestAttempt     time.Time
	LatestQuiz        time.Time
	LatestTrackedSync time.Time
}

type quizDashboardComponentEntry struct {
	Class           string
	SectionTitle    string
	Kind            string
	Content         string
	Metrics         knowledgeQuizMetrics
	IncorrectStreak int
	LastIncorrect   string
}

type quizDashboardQuestionEntry struct {
	Class          string
	SectionTitle   string
	ComponentLabel string
	Question       string
	Correct        bool
	AnsweredAt     time.Time
	UserAnswer     string
	Expected       string
	QuizID         string
}

type quizDashboardTrackedEntry struct {
	QuizID         string
	Class          string
	Path           string
	RegisteredAt   time.Time
	LastSessionID  string
	LastImportedAt time.Time
}

type quizDashboardProjection struct {
	ClassLabel       string
	Summary          quizDashboardSummary
	RecentlyHit      []quizDashboardComponentEntry
	Untouched        []quizDashboardComponentEntry
	Struggling       []quizDashboardComponentEntry
	RecentQuestions  []quizDashboardQuestionEntry
	RecentQuizzes    []quizDashboardQuizDoc
	TrackedQuizzes   []quizDashboardTrackedEntry
	PendingTracked   []quizDashboardTrackedEntry
	LoadedAt         time.Time
	HasAnyQuizSignal bool
	EmptyReason      string
	SelectedClass    string
	AllClasses       bool
}

type quizDashboardSectionView struct {
	Title   string
	Summary string
	Lines   []string
}

type quizDashboardLayout struct {
	stacked     bool
	leftWidth   int
	rightWidth  int
	leftHeight  int
	rightHeight int
}

func (q QuizDashboardTab) startLoading() QuizDashboardTab {
	q.loading = true
	q.err = nil
	return q
}

func (q QuizDashboardTab) update(msg tea.Msg) (QuizDashboardTab, string, tea.Cmd, bool) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "r":
			q = q.startLoading()
			return q, "Loading quiz dashboard...", loadQuizDashboardCmd(), false
		case "s":
			return q, "Syncing tracked quiz sessions...", syncTrackedSessionsCmd(), true
		case "h":
			q.activePane = quizDashboardPaneSections
			return q, "Dashboard sections focused", nil, false
		case "l":
			q.activePane = quizDashboardPaneDetails
			return q, "Dashboard details focused", nil, false
		case "up", "k":
			return q.nudgeActivePane(-1), "", nil, false
		case "down", "j":
			return q.nudgeActivePane(1), "", nil, false
		case "pgup":
			return q.pageActivePane(-1), "", nil, false
		case "pgdn":
			return q.pageActivePane(1), "", nil, false
		case "home":
			return q.moveActivePaneHome(), "", nil, false
		case "end":
			return q.moveActivePaneEnd(), "", nil, false
		}
	}
	return q, "", nil, false
}

func (q QuizDashboardTab) nudgeActivePane(delta int) QuizDashboardTab {
	if q.activePane == quizDashboardPaneDetails {
		q.detailScroll = clamp(q.detailScroll+delta, 0, q.detailMaxScroll())
		return q
	}
	q.selectedSection = clamp(q.selectedSection+delta, 0, q.sectionCount()-1)
	q.detailScroll = 0
	return q
}

func (q QuizDashboardTab) pageActivePane(direction int) QuizDashboardTab {
	if q.activePane == quizDashboardPaneDetails {
		page := max(3, q.detailViewportHeight()-2)
		q.detailScroll = clamp(q.detailScroll+(direction*page), 0, q.detailMaxScroll())
		return q
	}
	page := max(1, q.sectionPageSize())
	q.selectedSection = clamp(q.selectedSection+(direction*page), 0, q.sectionCount()-1)
	q.detailScroll = 0
	return q
}

func (q QuizDashboardTab) moveActivePaneHome() QuizDashboardTab {
	if q.activePane == quizDashboardPaneDetails {
		q.detailScroll = 0
		return q
	}
	q.selectedSection = 0
	q.detailScroll = 0
	return q
}

func (q QuizDashboardTab) moveActivePaneEnd() QuizDashboardTab {
	if q.activePane == quizDashboardPaneDetails {
		q.detailScroll = q.detailMaxScroll()
		return q
	}
	q.selectedSection = q.sectionCount() - 1
	q.detailScroll = 0
	return q
}

func (q QuizDashboardTab) receive(snapshot *quizDashboardSnapshot, err error) (QuizDashboardTab, string) {
	q.loaded = true
	q.loading = false
	q.snapshot = snapshot
	q.err = err
	q.detailScroll = 0
	if err != nil {
		return q, "Quiz dashboard load failed"
	}
	q.selectedSection = clamp(q.selectedSection, 0, q.sectionCount()-1)
	return q, "Quiz dashboard refreshed"
}

func (q QuizDashboardTab) view(width, height int, selectedClass string) string {
	if q.loading && !q.loaded {
		return dimStyle.Render("Loading quiz dashboard...")
	}
	if !q.loaded && !q.loading {
		return dimStyle.Render("Loading quiz dashboard...")
	}
	if q.err != nil {
		return errorStyle.Render("Error loading quiz dashboard: " + q.err.Error())
	}
	projection := buildQuizDashboardProjection(q.snapshot, selectedClass)
	if !projection.HasAnyQuizSignal {
		body := projection.EmptyReason
		if body == "" {
			body = "No quiz history yet. Generate a quiz and sync tracked sessions to populate this dashboard."
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			dimStyle.Render(body),
			dimStyle.Render("Press r to refresh  •  s to sync tracked quiz sessions"),
		)
	}

	sections := buildQuizDashboardSections(projection)
	selectedIndex := clamp(q.selectedSection, 0, len(sections)-1)
	detailScroll := clamp(q.detailScroll, 0, quizDashboardDetailMaxScrollFor(sections[selectedIndex], width, height))

	headerLines := []string{
		truncateWidth(fmt.Sprintf("%s %s  •  %s %s", labelStyle.Render("Class filter:"), projection.ClassLabel, labelStyle.Render("Loaded:"), dashboardTimeLabel(projection.LoadedAt)), width),
		truncateWidth(fmt.Sprintf("%s %s  •  %s", labelStyle.Render("Focus:"), q.activePane.label(), dimStyle.Render("h/l switch pane  •  ↑/↓ scroll  •  PgUp/PgDn jump  •  r refresh  •  s sync")), width),
	}
	layout := quizDashboardLayoutFor(width, height, len(headerLines))
	sectionPane := q.renderSectionsPane(sections, layout.leftWidth, layout.leftHeight, selectedIndex)
	detailPane := q.renderDetailPane(sections[selectedIndex], layout.rightWidth, layout.rightHeight, detailScroll)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sectionPane, " ", detailPane)
	if layout.stacked {
		body = lipgloss.JoinVertical(lipgloss.Left, sectionPane, detailPane)
	}
	return strings.Join(append(headerLines, body), "\n")
}

func (q QuizDashboardTab) renderSectionsPane(sections []quizDashboardSectionView, width, height, selectedIndex int) string {
	style := knowledgePaneBorderStyle(q.activePane == quizDashboardPaneSections).Width(width)
	innerWidth := max(18, width-style.GetHorizontalFrameSize())
	contentHeight := max(3, height-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-2)
	pageSize := max(1, innerHeight/quizDashboardSectionRowHeight)

	start := 0
	if selectedIndex >= pageSize {
		start = selectedIndex - pageSize + 1
		if selectedIndex < len(sections)-1 {
			centered := selectedIndex - (pageSize / 2)
			if centered > 0 {
				start = centered
			}
		}
	}
	start = clamp(start, 0, max(0, len(sections)-pageSize))
	end := min(len(sections), start+pageSize)

	contentLines := make([]string, 0, innerHeight)
	for i := start; i < end; i++ {
		contentLines = append(contentLines, strings.Split(q.renderSectionRow(sections[i], innerWidth, i == selectedIndex), "\n")...)
	}
	for len(contentLines) < innerHeight {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > innerHeight {
		contentLines = contentLines[:innerHeight]
	}

	footerHint := "l inspect"
	if q.activePane != quizDashboardPaneSections {
		footerHint = "h focus"
	}
	footer := dimStyle.Render(truncateWidth(fmt.Sprintf("%d/%d  •  %s", selectedIndex+1, len(sections), footerHint), innerWidth))
	content := lipgloss.NewStyle().
		Width(innerWidth).
		MaxWidth(innerWidth).
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render(sectionTitleStyle.Render("Sections") + "\n" + strings.Join(contentLines, "\n") + "\n" + footer)
	return style.Render(content)
}

func (q QuizDashboardTab) renderSectionRow(section quizDashboardSectionView, width int, selected bool) string {
	marker := " "
	if selected {
		marker = ">"
	}
	rowText := strings.Join([]string{
		truncateWidth(marker+" "+emptyFallback(section.Title, "Section"), width),
		truncateWidth("  "+emptyFallback(section.Summary, "No details yet"), width),
	}, "\n")
	return knowledgeSectionRowStyle(selected, selected && q.activePane == quizDashboardPaneSections).
		Width(width).
		MaxWidth(width).
		Render(rowText)
}

func (q QuizDashboardTab) renderDetailPane(section quizDashboardSectionView, width, height, scroll int) string {
	style := knowledgePaneBorderStyle(q.activePane == quizDashboardPaneDetails).Width(width)
	innerWidth := max(20, width-style.GetHorizontalFrameSize())
	contentHeight := max(3, height-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-2)

	lines := dashboardTruncateLines(section.Lines, innerWidth)
	if len(lines) == 0 {
		lines = []string{dimStyle.Render("No details available.")}
	}
	maxScroll := max(0, len(lines)-innerHeight)
	start := clamp(scroll, 0, maxScroll)
	end := min(len(lines), start+innerHeight)
	visible := append([]string{}, lines[start:end]...)
	for len(visible) < innerHeight {
		visible = append(visible, "")
	}

	footerHint := "l focus"
	if q.activePane == quizDashboardPaneDetails {
		footerHint = "h sections"
	}
	footer := fmt.Sprintf("%d line(s)  •  %s", len(lines), footerHint)
	if len(lines) > innerHeight {
		footer = fmt.Sprintf("Showing %d-%d of %d  •  %s", start+1, end, len(lines), footerHint)
	}

	content := lipgloss.NewStyle().
		Width(innerWidth).
		MaxWidth(innerWidth).
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render(sectionTitleStyle.Render(truncateWidth(section.Title, innerWidth)) + "\n" + strings.Join(visible, "\n") + "\n" + dimStyle.Render(truncateWidth(footer, innerWidth)))
	return style.Render(content)
}

func (q QuizDashboardTab) sectionCount() int {
	return 6
}

func (q QuizDashboardTab) sectionPageSize() int {
	return max(1, q.sectionViewportHeight()/quizDashboardSectionRowHeight)
}

func (q QuizDashboardTab) sectionViewportHeight() int {
	if q.width <= 0 || q.height <= 0 {
		return 1
	}
	layout := quizDashboardLayoutFor(q.width, q.height, 2)
	style := knowledgePaneBorderStyle(false)
	contentHeight := max(3, layout.leftHeight-style.GetVerticalFrameSize())
	return max(1, contentHeight-2)
}

func (q QuizDashboardTab) detailViewportHeight() int {
	if q.width <= 0 || q.height <= 0 {
		return 1
	}
	_, innerHeight := q.detailInnerDimensions()
	return innerHeight
}

func (q QuizDashboardTab) detailInnerDimensions() (int, int) {
	if q.width <= 0 || q.height <= 0 {
		return 0, 0
	}
	layout := quizDashboardLayoutFor(q.width, q.height, 2)
	style := knowledgePaneBorderStyle(false)
	innerWidth := max(20, layout.rightWidth-style.GetHorizontalFrameSize())
	contentHeight := max(3, layout.rightHeight-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-2)
	return innerWidth, innerHeight
}

func (q QuizDashboardTab) detailMaxScroll() int {
	projection := buildQuizDashboardProjection(q.snapshot, "")
	sections := buildQuizDashboardSections(projection)
	if len(sections) == 0 {
		return 0
	}
	selectedIndex := clamp(q.selectedSection, 0, len(sections)-1)
	_, innerHeight := q.detailInnerDimensions()
	if innerHeight <= 0 {
		return 0
	}
	lines := sections[selectedIndex].Lines
	if len(lines) <= innerHeight {
		return 0
	}
	return len(lines) - innerHeight
}

func buildQuizDashboardSections(projection quizDashboardProjection) []quizDashboardSectionView {
	return []quizDashboardSectionView{
		{
			Title:   "Overview",
			Summary: dashboardOverviewSummary(projection.Summary),
			Lines:   buildDashboardOverviewLines(projection.Summary),
		},
		{
			Title:   "Coverage",
			Summary: fmt.Sprintf("%d hit  •  %d untouched", projection.Summary.ComponentsHit, len(projection.Untouched)),
			Lines:   buildDashboardCoverageLines(projection),
		},
		{
			Title:   "Needs Attention",
			Summary: fmt.Sprintf("%d struggling component(s)", len(projection.Struggling)),
			Lines:   buildDashboardStrugglingLines(projection.Struggling),
		},
		{
			Title:   "Recent Questions",
			Summary: fmt.Sprintf("%d recorded question(s)", len(projection.RecentQuestions)),
			Lines:   buildDashboardRecentQuestionLines(projection.RecentQuestions),
		},
		{
			Title:   "Tracked Quizzes",
			Summary: fmt.Sprintf("%d tracked  •  %d pending", projection.Summary.TrackedQuizzes, projection.Summary.TrackedPending),
			Lines:   buildDashboardTrackedLines(projection),
		},
		{
			Title:   "Recent Quizzes",
			Summary: fmt.Sprintf("%d saved quiz file(s)", len(projection.RecentQuizzes)),
			Lines:   buildDashboardRecentQuizLines(projection.RecentQuizzes, projection.TrackedQuizzes),
		},
	}
}

func buildDashboardOverviewLines(summary quizDashboardSummary) []string {
	accuracy := "-"
	if summary.TotalAttempts > 0 {
		accuracy = fmt.Sprintf("%.0f%%", summary.Accuracy*100)
	}
	return []string{
		fmt.Sprintf("%s %d/%d", labelStyle.Render("Components hit:"), summary.ComponentsHit, summary.TotalComponents),
		fmt.Sprintf("%s %d/%d", labelStyle.Render("Sections hit:"), summary.SectionsHit, summary.TotalSections),
		fmt.Sprintf("%s %d", labelStyle.Render("Questions answered:"), summary.TotalAttempts),
		fmt.Sprintf("%s %d (%s)", labelStyle.Render("Correct answers:"), summary.CorrectAnswers, accuracy),
		fmt.Sprintf("%s %d", labelStyle.Render("Saved quizzes:"), summary.SavedQuizzes),
		fmt.Sprintf("%s %d synced / %d pending", labelStyle.Render("Tracked quizzes:"), summary.TrackedSynced, summary.TrackedPending),
		fmt.Sprintf("%s %s", labelStyle.Render("Latest attempt:"), dashboardTimeLabel(summary.LatestAttempt)),
		fmt.Sprintf("%s %s", labelStyle.Render("Latest quiz:"), dashboardTimeLabel(summary.LatestQuiz)),
		fmt.Sprintf("%s %s", labelStyle.Render("Latest tracked sync:"), dashboardTimeLabel(summary.LatestTrackedSync)),
	}
}

func buildDashboardCoverageLines(projection quizDashboardProjection) []string {
	lines := []string{
		fmt.Sprintf("%s %d  •  %s %d", labelStyle.Render("Recently hit:"), len(projection.RecentlyHit), labelStyle.Render("Untouched:"), len(projection.Untouched)),
		"",
		dimStyle.Render("Recent coverage"),
	}
	lines = append(lines, buildDashboardComponentLines(projection.RecentlyHit, true)...)
	lines = append(lines, "", dimStyle.Render("Still untouched"))
	lines = append(lines, buildDashboardComponentLines(projection.Untouched, false)...)
	return lines
}

func buildDashboardStrugglingLines(entries []quizDashboardComponentEntry) []string {
	if len(entries) == 0 {
		return []string{dimStyle.Render("No struggling components yet. Wrong answers will surface here after synced attempts.")}
	}
	lines := make([]string, 0, len(entries)*2)
	for _, entry := range entries {
		lines = append(lines, dashboardComponentSummary(entry))
		meta := fmt.Sprintf("%s  •  %d wrong  •  streak %d  •  last %s", dashboardMetricsLabel(entry.Metrics), entry.Metrics.Incorrect, entry.IncorrectStreak, dashboardTimeLabel(entry.Metrics.LastAnswered))
		if entry.LastIncorrect != "" {
			lines = append(lines, dimStyle.Render("  last miss: ")+entry.LastIncorrect)
			continue
		}
		lines = append(lines, dimStyle.Render("  ")+meta)
	}
	return lines
}

func buildDashboardRecentQuestionLines(entries []quizDashboardQuestionEntry) []string {
	if len(entries) == 0 {
		return []string{dimStyle.Render("No answered questions recorded yet.")}
	}
	lines := make([]string, 0, len(entries)*2)
	for _, entry := range entries {
		status := successStyle.Render("correct")
		if !entry.Correct {
			status = errorStyle.Render("wrong")
		}
		headline := fmt.Sprintf("%s  %s  •  %s", dashboardTimeLabel(entry.AnsweredAt), status, emptyFallback(entry.ComponentLabel, emptyFallback(entry.SectionTitle, emptyFallback(entry.Class, "unknown"))))
		lines = append(lines, headline, entry.Question)
	}
	return lines
}

func buildDashboardTrackedLines(projection quizDashboardProjection) []string {
	if len(projection.TrackedQuizzes) == 0 {
		return []string{dimStyle.Render("No tracked quizzes registered yet.")}
	}
	lines := []string{
		fmt.Sprintf("%s %d  •  %s %d  •  %s %s", labelStyle.Render("Registered:"), projection.Summary.TrackedQuizzes, labelStyle.Render("Pending:"), projection.Summary.TrackedPending, labelStyle.Render("Latest sync:"), dashboardTimeLabel(projection.Summary.LatestTrackedSync)),
	}
	if len(projection.PendingTracked) > 0 {
		lines = append(lines, "", dimStyle.Render("Pending import"))
		for _, entry := range projection.PendingTracked {
			lines = append(lines, fmt.Sprintf("%s  •  %s", dashboardTrackedLabel(entry), dashboardTimeLabel(entry.RegisteredAt)))
		}
	}
	lines = append(lines, "", dimStyle.Render("Recently synced"))
	for _, entry := range projection.TrackedQuizzes {
		stamp := dashboardTimeLabel(entry.LastImportedAt)
		if entry.LastImportedAt.IsZero() {
			stamp = "not imported yet"
		}
		lines = append(lines, fmt.Sprintf("%s  •  %s", dashboardTrackedLabel(entry), stamp))
	}
	return lines
}

func buildDashboardRecentQuizLines(entries []quizDashboardQuizDoc, tracked []quizDashboardTrackedEntry) []string {
	if len(entries) == 0 {
		return []string{dimStyle.Render("No saved quiz files yet.")}
	}
	trackedByPath := make(map[string]quizDashboardTrackedEntry, len(tracked))
	for _, entry := range tracked {
		trackedByPath[dashboardNormalizePath(entry.Path)] = entry
	}
	lines := make([]string, 0, len(entries)*2)
	for _, entry := range entries {
		trackedNote := dimStyle.Render("not tracked")
		if record, ok := trackedByPath[dashboardNormalizePath(entry.Path)]; ok {
			if record.LastImportedAt.IsZero() {
				trackedNote = warnStyle.Render("pending sync")
			} else {
				trackedNote = successStyle.Render("synced")
			}
		}
		lines = append(lines,
			fmt.Sprintf("%s  •  %d question(s)  •  %s", emptyFallback(entry.Title, entry.ID), entry.QuestionCount, trackedNote),
			dimStyle.Render(fmt.Sprintf("%s  •  %s", entry.Class, dashboardTimeLabel(entry.GeneratedAt))),
		)
	}
	return lines
}

func buildDashboardComponentLines(entries []quizDashboardComponentEntry, includeMetrics bool) []string {
	if len(entries) == 0 {
		return []string{dimStyle.Render("None")}
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		line := dashboardComponentSummary(entry)
		if includeMetrics {
			line += dimStyle.Render("  •  ") + dashboardMetricsLabel(entry.Metrics)
		}
		lines = append(lines, line)
	}
	return lines
}

func dashboardOverviewSummary(summary quizDashboardSummary) string {
	if summary.TotalAttempts == 0 {
		return "No attempts yet"
	}
	return fmt.Sprintf("%d answered  •  %.0f%% accuracy", summary.TotalAttempts, summary.Accuracy*100)
}

func dashboardTruncateLines(lines []string, width int) []string {
	if width <= 0 {
		return nil
	}
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			trimmed = append(trimmed, "")
			continue
		}
		trimmed = append(trimmed, truncateWidth(line, width))
	}
	return trimmed
}

func quizDashboardDetailMaxScrollFor(section quizDashboardSectionView, width, height int) int {
	layout := quizDashboardLayoutFor(width, height, 2)
	style := knowledgePaneBorderStyle(false)
	innerWidth := max(20, layout.rightWidth-style.GetHorizontalFrameSize())
	contentHeight := max(3, layout.rightHeight-style.GetVerticalFrameSize())
	innerHeight := max(1, contentHeight-2)
	lines := dashboardTruncateLines(section.Lines, innerWidth)
	if len(lines) <= innerHeight {
		return 0
	}
	return len(lines) - innerHeight
}

func quizDashboardLayoutFor(width, height, metaHeight int) quizDashboardLayout {
	if width <= 0 || height <= 0 {
		return quizDashboardLayout{}
	}

	paneHeight := max(8, height-metaHeight-1)
	leftWidth := clamp(width/3, 22, max(22, width-32-1))
	rightWidth := width - leftWidth - 1
	canSplit := width >= 68 && rightWidth >= 32
	if canSplit {
		return quizDashboardLayout{
			leftWidth:   leftWidth,
			rightWidth:  rightWidth,
			leftHeight:  paneHeight,
			rightHeight: paneHeight,
		}
	}

	leftHeight := max(6, paneHeight/3)
	rightHeight := max(6, paneHeight-leftHeight)
	if leftHeight+rightHeight > paneHeight {
		rightHeight = paneHeight - leftHeight
	}
	if rightHeight < 6 {
		rightHeight = 6
		leftHeight = max(6, paneHeight-rightHeight)
	}

	return quizDashboardLayout{
		stacked:     true,
		leftWidth:   width,
		rightWidth:  width,
		leftHeight:  leftHeight,
		rightHeight: rightHeight,
	}
}

func (p quizDashboardPane) label() string {
	if p == quizDashboardPaneDetails {
		return "details"
	}
	return "sections"
}

func buildQuizDashboardProjection(snapshot *quizDashboardSnapshot, selectedClass string) quizDashboardProjection {
	projection := quizDashboardProjection{
		ClassLabel:    "all classes",
		SelectedClass: strings.TrimSpace(selectedClass),
		AllClasses:    strings.TrimSpace(selectedClass) == "",
	}
	if projection.SelectedClass != "" {
		projection.ClassLabel = projection.SelectedClass
	}
	if snapshot == nil {
		projection.EmptyReason = "No quiz data has been loaded yet."
		return projection
	}
	projection.LoadedAt = snapshot.LoadedAt

	sections := filterDashboardSections(snapshot.Sections, projection.SelectedClass)
	components := filterDashboardComponents(snapshot.Components, projection.SelectedClass)
	quizzes := filterDashboardQuizzes(snapshot.Quizzes, projection.SelectedClass)
	tracked := filterDashboardTracked(snapshot.Tracked, projection.SelectedClass)

	sectionTitleByID := make(map[string]string, len(sections))
	sectionClassByID := make(map[string]string, len(sections))
	for _, section := range sections {
		sectionTitleByID[strings.TrimSpace(section.ID)] = strings.TrimSpace(section.Title)
		sectionClassByID[strings.TrimSpace(section.ID)] = strings.TrimSpace(section.Class)
	}

	practicedSections := make(map[string]bool)
	recentlyHit := make([]quizDashboardComponentEntry, 0)
	untouched := make([]quizDashboardComponentEntry, 0)
	struggling := make([]quizDashboardComponentEntry, 0)
	recentQuestions := make([]quizDashboardQuestionEntry, 0)
	seenQuestionEvents := make(map[string]bool)

	for _, section := range sections {
		for _, item := range section.QuestionHistory {
			appendDashboardQuestionEvent(&recentQuestions, seenQuestionEvents, item, strings.TrimSpace(section.Class), strings.TrimSpace(section.Title), "")
		}
	}

	for _, component := range components {
		metrics := knowledgeMetricsFromHistory(component.QuestionHistory)
		entry := quizDashboardComponentEntry{
			Class:           strings.TrimSpace(component.Class),
			SectionTitle:    sectionTitleByID[strings.TrimSpace(component.SectionID)],
			Kind:            strings.TrimSpace(component.Kind),
			Content:         strings.TrimSpace(component.Content),
			Metrics:         metrics,
			IncorrectStreak: dashboardIncorrectStreak(component.QuestionHistory),
			LastIncorrect:   dashboardLastIncorrectQuestion(component.QuestionHistory),
		}
		if metrics.Attempts > 0 {
			projection.Summary.ComponentsHit++
			if strings.TrimSpace(component.SectionID) != "" {
				practicedSections[strings.TrimSpace(component.SectionID)] = true
			}
			recentlyHit = append(recentlyHit, entry)
			if metrics.Incorrect > 0 {
				struggling = append(struggling, entry)
			}
		} else {
			untouched = append(untouched, entry)
		}
		projection.Summary.TotalComponents++
		projection.Summary.TotalAttempts += metrics.Attempts
		projection.Summary.CorrectAnswers += metrics.Correct
		if metrics.LastAnswered.After(projection.Summary.LatestAttempt) {
			projection.Summary.LatestAttempt = metrics.LastAnswered
		}
		componentLabel := dashboardComponentLabel(component.Kind, component.Content)
		for _, item := range component.QuestionHistory {
			appendDashboardQuestionEvent(&recentQuestions, seenQuestionEvents, item, strings.TrimSpace(component.Class), sectionTitleByID[strings.TrimSpace(component.SectionID)], componentLabel)
		}
	}

	projection.Summary.TotalSections = len(sections)
	projection.Summary.SectionsHit = len(practicedSections)
	for _, section := range sections {
		if len(section.QuestionHistory) > 0 {
			practicedSections[strings.TrimSpace(section.ID)] = true
		}
	}
	projection.Summary.SectionsHit = len(practicedSections)
	if projection.Summary.TotalAttempts > 0 {
		projection.Summary.Accuracy = float64(projection.Summary.CorrectAnswers) / float64(projection.Summary.TotalAttempts)
	}

	projection.Summary.SavedQuizzes = len(quizzes)
	projection.RecentQuizzes = quizzes
	if len(quizzes) > 0 {
		projection.HasAnyQuizSignal = true
		for _, quizDoc := range quizzes {
			if quizDoc.GeneratedAt.After(projection.Summary.LatestQuiz) {
				projection.Summary.LatestQuiz = quizDoc.GeneratedAt
			}
		}
	}

	projection.TrackedQuizzes = tracked
	projection.Summary.TrackedQuizzes = len(tracked)
	for _, record := range tracked {
		if record.LastImportedAt.IsZero() {
			projection.PendingTracked = append(projection.PendingTracked, record)
			projection.Summary.TrackedPending++
			continue
		}
		projection.Summary.TrackedSynced++
		if record.LastImportedAt.After(projection.Summary.LatestTrackedSync) {
			projection.Summary.LatestTrackedSync = record.LastImportedAt
		}
	}

	sort.Slice(recentlyHit, func(i, j int) bool {
		if recentlyHit[i].Metrics.LastAnswered.Equal(recentlyHit[j].Metrics.LastAnswered) {
			return strings.ToLower(recentlyHit[i].Content) < strings.ToLower(recentlyHit[j].Content)
		}
		return recentlyHit[i].Metrics.LastAnswered.After(recentlyHit[j].Metrics.LastAnswered)
	})
	sort.Slice(untouched, func(i, j int) bool {
		if !strings.EqualFold(untouched[i].Class, untouched[j].Class) {
			return strings.ToLower(untouched[i].Class) < strings.ToLower(untouched[j].Class)
		}
		if !strings.EqualFold(untouched[i].SectionTitle, untouched[j].SectionTitle) {
			return strings.ToLower(untouched[i].SectionTitle) < strings.ToLower(untouched[j].SectionTitle)
		}
		return strings.ToLower(untouched[i].Content) < strings.ToLower(untouched[j].Content)
	})
	sort.Slice(struggling, func(i, j int) bool {
		left := struggling[i]
		right := struggling[j]
		if left.Metrics.Accuracy != right.Metrics.Accuracy {
			return left.Metrics.Accuracy < right.Metrics.Accuracy
		}
		if left.IncorrectStreak != right.IncorrectStreak {
			return left.IncorrectStreak > right.IncorrectStreak
		}
		if left.Metrics.Incorrect != right.Metrics.Incorrect {
			return left.Metrics.Incorrect > right.Metrics.Incorrect
		}
		return left.Metrics.LastAnswered.After(right.Metrics.LastAnswered)
	})
	sort.Slice(recentQuestions, func(i, j int) bool {
		if recentQuestions[i].AnsweredAt.Equal(recentQuestions[j].AnsweredAt) {
			return recentQuestions[i].Question < recentQuestions[j].Question
		}
		return recentQuestions[i].AnsweredAt.After(recentQuestions[j].AnsweredAt)
	})
	sort.Slice(projection.PendingTracked, func(i, j int) bool {
		return projection.PendingTracked[i].RegisteredAt.After(projection.PendingTracked[j].RegisteredAt)
	})
	sort.Slice(projection.TrackedQuizzes, func(i, j int) bool {
		left := projection.TrackedQuizzes[i]
		right := projection.TrackedQuizzes[j]
		if left.LastImportedAt.Equal(right.LastImportedAt) {
			return left.RegisteredAt.After(right.RegisteredAt)
		}
		return left.LastImportedAt.After(right.LastImportedAt)
	})

	projection.RecentlyHit = recentlyHit
	projection.Untouched = untouched
	projection.Struggling = struggling
	projection.RecentQuestions = recentQuestions
	projection.HasAnyQuizSignal = projection.HasAnyQuizSignal || projection.Summary.TotalAttempts > 0 || projection.Summary.TrackedQuizzes > 0 || projection.Summary.TotalComponents > 0
	if projection.Summary.TotalComponents == 0 && projection.Summary.SavedQuizzes == 0 {
		projection.EmptyReason = "No quiz-related knowledge exists for this class yet. Ingest notes and generate a quiz first."
	}
	return projection
}

func filterDashboardSections(sections []state.Section, selectedClass string) []state.Section {
	if strings.TrimSpace(selectedClass) == "" {
		return append([]state.Section(nil), sections...)
	}
	filtered := make([]state.Section, 0, len(sections))
	for _, section := range sections {
		if strings.EqualFold(strings.TrimSpace(section.Class), selectedClass) {
			filtered = append(filtered, section)
		}
	}
	return filtered
}

func filterDashboardComponents(components []state.Component, selectedClass string) []state.Component {
	if strings.TrimSpace(selectedClass) == "" {
		return append([]state.Component(nil), components...)
	}
	filtered := make([]state.Component, 0, len(components))
	for _, component := range components {
		if strings.EqualFold(strings.TrimSpace(component.Class), selectedClass) {
			filtered = append(filtered, component)
		}
	}
	return filtered
}

func filterDashboardQuizzes(quizzes []quizDashboardQuizDoc, selectedClass string) []quizDashboardQuizDoc {
	filtered := make([]quizDashboardQuizDoc, 0, len(quizzes))
	for _, quizDoc := range quizzes {
		if strings.TrimSpace(selectedClass) != "" && !strings.EqualFold(strings.TrimSpace(quizDoc.Class), selectedClass) {
			continue
		}
		filtered = append(filtered, quizDoc)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].GeneratedAt.After(filtered[j].GeneratedAt)
	})
	return filtered
}

func filterDashboardTracked(tracked *state.TrackedQuizCache, selectedClass string) []quizDashboardTrackedEntry {
	if tracked == nil {
		return nil
	}
	entries := make([]quizDashboardTrackedEntry, 0, len(tracked.Quizzes))
	for _, record := range tracked.Quizzes {
		if strings.TrimSpace(selectedClass) != "" && !strings.EqualFold(strings.TrimSpace(record.Class), selectedClass) {
			continue
		}
		entries = append(entries, quizDashboardTrackedEntry{
			QuizID:         strings.TrimSpace(record.QuizID),
			Class:          strings.TrimSpace(record.Class),
			Path:           strings.TrimSpace(record.QuizPath),
			RegisteredAt:   record.RegisteredAt,
			LastSessionID:  strings.TrimSpace(record.LastSessionID),
			LastImportedAt: record.LastImportedAt,
		})
	}
	return entries
}

func appendDashboardQuestionEvent(target *[]quizDashboardQuestionEntry, seen map[string]bool, item state.QuestionHistoryEntry, className, sectionTitle, componentLabel string) {
	key := strings.TrimSpace(item.ID)
	if key == "" {
		key = fmt.Sprintf("%s|%s|%s|%t|%s", strings.TrimSpace(item.QuizID), strings.TrimSpace(item.QuestionID), strings.TrimSpace(item.Question), item.Correct, item.AnsweredAt.UTC().Format(time.RFC3339Nano))
	}
	if seen[key] {
		return
	}
	seen[key] = true
	*target = append(*target, quizDashboardQuestionEntry{
		Class:          className,
		SectionTitle:   strings.TrimSpace(sectionTitle),
		ComponentLabel: strings.TrimSpace(componentLabel),
		Question:       strings.TrimSpace(item.Question),
		Correct:        item.Correct,
		AnsweredAt:     item.AnsweredAt,
		UserAnswer:     strings.TrimSpace(item.UserAnswer),
		Expected:       strings.TrimSpace(item.Expected),
		QuizID:         strings.TrimSpace(item.QuizID),
	})
}

func dashboardIncorrectStreak(history []state.QuestionHistoryEntry) int {
	streak := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Correct {
			break
		}
		streak++
	}
	return streak
}

func dashboardLastIncorrectQuestion(history []state.QuestionHistoryEntry) string {
	for i := len(history) - 1; i >= 0; i-- {
		if !history[i].Correct {
			return strings.TrimSpace(history[i].Question)
		}
	}
	return ""
}

func dashboardComponentSummary(entry quizDashboardComponentEntry) string {
	parts := []string{}
	if strings.TrimSpace(entry.SectionTitle) != "" {
		parts = append(parts, entry.SectionTitle)
	}
	parts = append(parts, dashboardComponentLabel(entry.Kind, entry.Content))
	if len(parts) == 0 {
		return dimStyle.Render("Unnamed component")
	}
	return strings.Join(parts, "  •  ")
}

func dashboardComponentLabel(kind, content string) string {
	kind = emptyFallback(kind, "component")
	content = emptyFallback(content, "(no content)")
	return fmt.Sprintf("[%s] %s", kind, content)
}

func dashboardMetricsLabel(metrics knowledgeQuizMetrics) string {
	if metrics.Attempts == 0 {
		return dimStyle.Render("no attempts")
	}
	return fmt.Sprintf("%d attempts, %.0f%% accuracy", metrics.Attempts, metrics.Accuracy*100)
}

func dashboardTrackedLabel(entry quizDashboardTrackedEntry) string {
	name := strings.TrimSuffix(filepath.Base(entry.Path), filepath.Ext(entry.Path))
	if strings.TrimSpace(name) == "" {
		name = emptyFallback(entry.QuizID, "tracked quiz")
	}
	if strings.TrimSpace(entry.Class) == "" {
		return name
	}
	return entry.Class + "  •  " + name
}

func dashboardTimeLabel(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format("2006-01-02 15:04")
}

func dashboardNormalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
}
