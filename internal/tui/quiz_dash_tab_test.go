package tui

import (
	"testing"
	"time"

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
