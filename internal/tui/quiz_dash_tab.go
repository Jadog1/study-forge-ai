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

// SFQTab holds state for the quiz dashboard tab.
type SFQTab struct {
	loaded   bool
	loading  bool
	err      error
	snapshot *quizDashboardSnapshot
}

func newSFQTab() SFQTab {
	return SFQTab{}
}

func (s SFQTab) resize(width int) SFQTab {
	return s
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

func (s SFQTab) startLoading() SFQTab {
	s.loading = true
	s.err = nil
	return s
}

func (s SFQTab) update(msg tea.Msg) (SFQTab, string, tea.Cmd, bool) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "r":
			s = s.startLoading()
			return s, "Loading quiz dashboard...", loadQuizDashboardCmd(), false
		case "s":
			return s, "Syncing tracked quiz sessions...", syncTrackedSessionsCmd(), true
		}
	}
	return s, "", nil, false
}

func (s SFQTab) receive(snapshot *quizDashboardSnapshot, err error) (SFQTab, string) {
	s.loaded = true
	s.loading = false
	s.snapshot = snapshot
	s.err = err
	if err != nil {
		return s, "Quiz dashboard load failed"
	}
	return s, "Quiz dashboard refreshed"
}

func (s SFQTab) view(width, height int, selectedClass string) string {
	if s.loading && !s.loaded {
		return dimStyle.Render("Loading quiz dashboard...")
	}
	if !s.loaded && !s.loading {
		return dimStyle.Render("Loading quiz dashboard...")
	}
	if s.err != nil {
		return errorStyle.Render("Error loading quiz dashboard: " + s.err.Error())
	}
	projection := buildQuizDashboardProjection(s.snapshot, selectedClass)
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

	listLimit := 4
	switch {
	case height >= 34:
		listLimit = 6
	case height >= 26:
		listLimit = 5
	case height <= 18:
		listLimit = 3
	}

	headerLines := []string{
		truncateWidth(fmt.Sprintf("%s %s  •  %s %s", labelStyle.Render("Class filter:"), projection.ClassLabel, labelStyle.Render("Loaded:"), dashboardTimeLabel(projection.LoadedAt)), width),
		truncateWidth(dimStyle.Render("Press r to refresh  •  s to sync tracked quiz sessions"), width),
	}

	sections := []string{
		renderSection("Overview", renderDashboardOverview(projection.Summary), width),
		renderSection("Coverage", renderDashboardCoverage(projection, listLimit), width),
		renderSection("Needs Attention", renderDashboardStruggling(projection.Struggling, listLimit), width),
		renderSection("Recent Questions", renderDashboardRecentQuestions(projection.RecentQuestions, listLimit), width),
	}
	if len(projection.PendingTracked) > 0 || len(projection.TrackedQuizzes) > 0 {
		sections = append(sections, renderSection("Tracked Quizzes", renderDashboardTracked(projection, listLimit), width))
	}
	if len(projection.RecentQuizzes) > 0 && height >= 22 {
		sections = append(sections, renderSection("Recent Quizzes", renderDashboardRecentQuizzes(projection.RecentQuizzes, projection.TrackedQuizzes, listLimit), width))
	}

	body := strings.Join(append(headerLines, sections...), "\n")
	return clipLines(body, max(6, height))
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

func renderDashboardOverview(summary quizDashboardSummary) string {
	accuracy := "-"
	if summary.TotalAttempts > 0 {
		accuracy = fmt.Sprintf("%.0f%%", summary.Accuracy*100)
	}
	return strings.Join([]string{
		fmt.Sprintf("%s %d/%d", labelStyle.Render("Components hit:"), summary.ComponentsHit, summary.TotalComponents),
		fmt.Sprintf("%s %d/%d", labelStyle.Render("Sections hit:"), summary.SectionsHit, summary.TotalSections),
		fmt.Sprintf("%s %d", labelStyle.Render("Questions answered:"), summary.TotalAttempts),
		fmt.Sprintf("%s %d (%s)", labelStyle.Render("Correct answers:"), summary.CorrectAnswers, accuracy),
		fmt.Sprintf("%s %d  •  %s %d synced / %d pending", labelStyle.Render("Saved quizzes:"), summary.SavedQuizzes, labelStyle.Render("Tracked:"), summary.TrackedSynced, summary.TrackedPending),
		fmt.Sprintf("%s %s  •  %s %s", labelStyle.Render("Latest attempt:"), dashboardTimeLabel(summary.LatestAttempt), labelStyle.Render("Latest quiz:"), dashboardTimeLabel(summary.LatestQuiz)),
	}, "\n")
}

func renderDashboardCoverage(projection quizDashboardProjection, limit int) string {
	recentlyHit := renderDashboardComponentList(projection.RecentlyHit, limit, true)
	untouched := renderDashboardComponentList(projection.Untouched, limit, false)
	return strings.Join([]string{
		fmt.Sprintf("%s %d  •  %s %d", labelStyle.Render("Recently hit:"), min(limit, len(projection.RecentlyHit)), labelStyle.Render("Untouched:"), len(projection.Untouched)),
		"",
		dimStyle.Render("Recent coverage"),
		recentlyHit,
		"",
		dimStyle.Render("Still untouched"),
		untouched,
	}, "\n")
}

func renderDashboardStruggling(entries []quizDashboardComponentEntry, limit int) string {
	if len(entries) == 0 {
		return dimStyle.Render("No struggling components yet. Wrong answers will surface here after synced attempts.")
	}
	lines := make([]string, 0, limit*2)
	for _, entry := range entries[:min(limit, len(entries))] {
		title := dashboardComponentSummary(entry)
		meta := fmt.Sprintf("%s  •  %d wrong  •  streak %d  •  last %s", dashboardMetricsLabel(entry.Metrics), entry.Metrics.Incorrect, entry.IncorrectStreak, dashboardTimeLabel(entry.Metrics.LastAnswered))
		lines = append(lines, title)
		if entry.LastIncorrect != "" {
			lines = append(lines, dimStyle.Render("  last miss: ")+truncateWidth(entry.LastIncorrect, 80))
		} else {
			lines = append(lines, dimStyle.Render("  ")+meta)
		}
	}
	return strings.Join(lines, "\n")
}

func renderDashboardRecentQuestions(entries []quizDashboardQuestionEntry, limit int) string {
	if len(entries) == 0 {
		return dimStyle.Render("No answered questions recorded yet.")
	}
	lines := make([]string, 0, limit*2)
	for _, entry := range entries[:min(limit, len(entries))] {
		status := successStyle.Render("correct")
		if !entry.Correct {
			status = errorStyle.Render("wrong")
		}
		headline := fmt.Sprintf("%s  %s  •  %s", dashboardTimeLabel(entry.AnsweredAt), status, emptyFallback(entry.ComponentLabel, emptyFallback(entry.SectionTitle, emptyFallback(entry.Class, "unknown"))))
		lines = append(lines, headline)
		lines = append(lines, truncateWidth(entry.Question, 80))
	}
	return strings.Join(lines, "\n")
}

func renderDashboardTracked(projection quizDashboardProjection, limit int) string {
	if len(projection.TrackedQuizzes) == 0 {
		return dimStyle.Render("No tracked quizzes registered yet.")
	}
	lines := []string{
		fmt.Sprintf("%s %d  •  %s %d  •  %s %s", labelStyle.Render("Registered:"), projection.Summary.TrackedQuizzes, labelStyle.Render("Pending:"), projection.Summary.TrackedPending, labelStyle.Render("Latest sync:"), dashboardTimeLabel(projection.Summary.LatestTrackedSync)),
	}
	if len(projection.PendingTracked) > 0 {
		lines = append(lines, "", dimStyle.Render("Pending import"))
		for _, entry := range projection.PendingTracked[:min(limit, len(projection.PendingTracked))] {
			lines = append(lines, truncateWidth(fmt.Sprintf("%s  •  %s", dashboardTrackedLabel(entry), dashboardTimeLabel(entry.RegisteredAt)), 80))
		}
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "", dimStyle.Render("Recently synced"))
	for _, entry := range projection.TrackedQuizzes[:min(limit, len(projection.TrackedQuizzes))] {
		lines = append(lines, truncateWidth(fmt.Sprintf("%s  •  %s", dashboardTrackedLabel(entry), dashboardTimeLabel(entry.LastImportedAt)), 80))
	}
	return strings.Join(lines, "\n")
}

func renderDashboardRecentQuizzes(entries []quizDashboardQuizDoc, tracked []quizDashboardTrackedEntry, limit int) string {
	trackedByPath := make(map[string]quizDashboardTrackedEntry, len(tracked))
	for _, entry := range tracked {
		trackedByPath[dashboardNormalizePath(entry.Path)] = entry
	}
	lines := make([]string, 0, limit*2)
	for _, entry := range entries[:min(limit, len(entries))] {
		trackedNote := dimStyle.Render("not tracked")
		if record, ok := trackedByPath[dashboardNormalizePath(entry.Path)]; ok {
			if record.LastImportedAt.IsZero() {
				trackedNote = warnStyle.Render("pending sync")
			} else {
				trackedNote = successStyle.Render("synced")
			}
		}
		lines = append(lines, truncateWidth(fmt.Sprintf("%s  •  %d question(s)  •  %s", emptyFallback(entry.Title, entry.ID), entry.QuestionCount, trackedNote), 80))
		lines = append(lines, dimStyle.Render(truncateWidth(fmt.Sprintf("%s  •  %s", entry.Class, dashboardTimeLabel(entry.GeneratedAt)), 80)))
	}
	return strings.Join(lines, "\n")
}

func renderDashboardComponentList(entries []quizDashboardComponentEntry, limit int, includeMetrics bool) string {
	if len(entries) == 0 {
		return dimStyle.Render("None")
	}
	lines := make([]string, 0, limit)
	for _, entry := range entries[:min(limit, len(entries))] {
		line := dashboardComponentSummary(entry)
		if includeMetrics {
			line += dimStyle.Render("  •  ") + dashboardMetricsLabel(entry.Metrics)
		}
		lines = append(lines, truncateWidth(line, 80))
	}
	return strings.Join(lines, "\n")
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
