package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/studyforge/study-agent/internal/state"
)

func TestBuildQuizDashboardProjectionFiltersByClassAndSummarizesHistory(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	snapshot := &quizDashboardSnapshot{
		Sections: []state.Section{
			{ID: "sec-a", Class: "math", Title: "Algebra", QuestionHistory: []state.QuestionHistoryEntry{{ID: "shared-1", Question: "Section question", Correct: false, AnsweredAt: now.Add(-2 * time.Hour)}}},
			{ID: "sec-b", Class: "history", Title: "Rome"},
		},
		Components: []state.Component{
			{
				ID:        "cmp-a",
				Class:     "math",
				SectionID: "sec-a",
				Kind:      "fact",
				Content:   "Quadratic formula",
				QuestionHistory: []state.QuestionHistoryEntry{
					{ID: "shared-1", Question: "Section question", Correct: false, AnsweredAt: now.Add(-2 * time.Hour)},
					{ID: "cmp-a-2", Question: "What is b^2-4ac?", Correct: true, AnsweredAt: now.Add(-1 * time.Hour)},
				},
			},
			{
				ID:        "cmp-b",
				Class:     "math",
				SectionID: "sec-a",
				Kind:      "concept",
				Content:   "Vertex form",
			},
			{
				ID:              "cmp-c",
				Class:           "history",
				SectionID:       "sec-b",
				Kind:            "fact",
				Content:         "Punic wars",
				QuestionHistory: []state.QuestionHistoryEntry{{ID: "hist-1", Question: "Who fought Rome?", Correct: true, AnsweredAt: now.Add(-30 * time.Minute)}},
			},
		},
		Tracked: &state.TrackedQuizCache{Quizzes: []state.TrackedQuizRecord{
			{QuizID: "quiz-math", Class: "math", QuizPath: "C:/tmp/math.yaml", RegisteredAt: now.Add(-3 * time.Hour), LastImportedAt: now.Add(-90 * time.Minute), LastSessionID: "sess-1"},
			{QuizID: "quiz-history", Class: "history", QuizPath: "C:/tmp/history.yaml", RegisteredAt: now.Add(-4 * time.Hour)},
		}},
		Quizzes: []quizDashboardQuizDoc{
			{ID: "quiz-math", Class: "math", Title: "Math adaptive quiz", Path: "C:/tmp/math.yaml", QuestionCount: 5, GeneratedAt: now.Add(-4 * time.Hour)},
			{ID: "quiz-history", Class: "history", Title: "History adaptive quiz", Path: "C:/tmp/history.yaml", QuestionCount: 6, GeneratedAt: now.Add(-5 * time.Hour)},
		},
		LoadedAt: now,
	}

	projection := buildQuizDashboardProjection(snapshot, "math")

	if projection.ClassLabel != "math" {
		t.Fatalf("expected class label math, got %q", projection.ClassLabel)
	}
	if projection.Summary.TotalComponents != 2 {
		t.Fatalf("expected 2 math components, got %d", projection.Summary.TotalComponents)
	}
	if projection.Summary.ComponentsHit != 1 {
		t.Fatalf("expected 1 hit component, got %d", projection.Summary.ComponentsHit)
	}
	if projection.Summary.TotalAttempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", projection.Summary.TotalAttempts)
	}
	if projection.Summary.CorrectAnswers != 1 {
		t.Fatalf("expected 1 correct answer, got %d", projection.Summary.CorrectAnswers)
	}
	if projection.Summary.SavedQuizzes != 1 {
		t.Fatalf("expected 1 saved quiz, got %d", projection.Summary.SavedQuizzes)
	}
	if projection.Summary.TrackedQuizzes != 1 || projection.Summary.TrackedSynced != 1 || projection.Summary.TrackedPending != 0 {
		t.Fatalf("unexpected tracked summary: %+v", projection.Summary)
	}
	if len(projection.RecentQuestions) != 2 {
		t.Fatalf("expected 2 deduplicated recent questions, got %d", len(projection.RecentQuestions))
	}
	if len(projection.Struggling) != 1 {
		t.Fatalf("expected 1 struggling component, got %d", len(projection.Struggling))
	}
	if projection.Struggling[0].Content != "Quadratic formula" {
		t.Fatalf("expected struggling component Quadratic formula, got %q", projection.Struggling[0].Content)
	}
	if len(projection.Untouched) != 1 || projection.Untouched[0].Content != "Vertex form" {
		t.Fatalf("expected untouched Vertex form component, got %+v", projection.Untouched)
	}
}

func TestQuizDashboardTabUpdateNavigatesAndScrollsDetails(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	history := make([]state.QuestionHistoryEntry, 0, 18)
	for i := 0; i < 18; i++ {
		history = append(history, state.QuestionHistoryEntry{
			ID:         "q-" + time.Duration(i).String(),
			Question:   "Question number " + time.Duration(i).String() + " with enough text to require truncation in narrow layouts",
			Correct:    i%2 == 0,
			AnsweredAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	tab := newQuizDashboardTab().resize(84, 16)
	tab, _ = tab.receive(&quizDashboardSnapshot{
		Sections: []state.Section{{ID: "sec-a", Class: "math", Title: "Algebra", QuestionHistory: history}},
		Components: []state.Component{{
			ID:              "cmp-a",
			Class:           "math",
			SectionID:       "sec-a",
			Kind:            "fact",
			Content:         "Quadratic formula",
			QuestionHistory: history,
		}},
		LoadedAt: now,
	}, nil)

	tab.selectedSection = 3
	updated, _, _, _ := tab.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if updated.activePane != quizDashboardPaneDetails {
		t.Fatalf("expected details pane focus, got %v", updated.activePane)
	}

	updated, _, _, _ = updated.update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.detailScroll == 0 {
		t.Fatalf("expected detail scroll to advance when details pane is focused")
	}

	updated.activePane = quizDashboardPaneSections
	updated.selectedSection = 0
	updated, _, _, _ = updated.update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.selectedSection != 1 {
		t.Fatalf("expected selected section to move to 1, got %d", updated.selectedSection)
	}
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

func TestDashboardTruncateLinesUsesEllipsis(t *testing.T) {
	lines := dashboardTruncateLines([]string{"This is a deliberately long dashboard line"}, 14)
	if len(lines) != 1 {
		t.Fatalf("expected one line, got %d", len(lines))
	}
	if !strings.HasSuffix(lines[0], "...") {
		t.Fatalf("expected explicit truncation marker, got %q", lines[0])
	}
}
