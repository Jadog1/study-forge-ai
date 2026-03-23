package quiz

import (
	"strings"
	"testing"

	"github.com/studyforge/study-agent/internal/state"
)

func TestQuizToSFQ(t *testing.T) {
	q := &state.Quiz{
		Title: "ML for Trading Quiz",
		Class: "ml4t",
		Tags:  []string{"Machine Learning"},
		Sections: []state.QuizSection{
			{
				ID:        "q-001",
				Question:  "What is the product rule?",
				Hint:      "Think probability",
				Answer:    "Multiply independent probabilities",
				Reasoning: "The product rule multiplies independent event probabilities.",
				Tags:      []string{"Probability"},
			},
			{
				ID:       "q-002",
				Question: "What does MSE stand for?",
				Answer:   "Mean Squared Error",
				Tags:     []string{"Metrics"},
			},
		},
	}

	out := string(quizToSFQ(q))

	// Header
	if !strings.Contains(out, "# ML for Trading Quiz") {
		t.Errorf("expected title header, got:\n%s", out)
	}

	// Both question blocks present
	if !strings.Contains(out, "? What is the product rule?") {
		t.Errorf("expected first question, got:\n%s", out)
	}
	if !strings.Contains(out, "? What does MSE stand for?") {
		t.Errorf("expected second question, got:\n%s", out)
	}

	// IDs
	if !strings.Contains(out, "id: q-001") {
		t.Errorf("expected id q-001, got:\n%s", out)
	}

	// Hint only on first section
	if !strings.Contains(out, "hint:") {
		t.Errorf("expected hint field, got:\n%s", out)
	}

	// Answers
	if !strings.Contains(out, `answer: "Multiply independent probabilities"`) {
		t.Errorf("expected answer for q-001, got:\n%s", out)
	}
	if !strings.Contains(out, `answer: "Mean Squared Error"`) {
		t.Errorf("expected answer for q-002, got:\n%s", out)
	}

	// Explanation from Reasoning
	if !strings.Contains(out, "explanation:") {
		t.Errorf("expected explanation field, got:\n%s", out)
	}

	// Tags
	if !strings.Contains(out, "tags: [Probability]") {
		t.Errorf("expected tags for q-001, got:\n%s", out)
	}

	// Delimiters — one per question plus trailing
	count := strings.Count(out, "---")
	if count != len(q.Sections)+1 {
		t.Errorf("expected %d --- delimiters, got %d\n%s", len(q.Sections)+1, count, out)
	}
}

func TestQuizToSFQ_EmptyQuiz(t *testing.T) {
	q := &state.Quiz{}
	out := string(quizToSFQ(q))
	// Should still produce the trailing delimiter with no question blocks
	if !strings.Contains(out, "---") {
		t.Errorf("expected at least one --- delimiter for empty quiz, got:\n%s", out)
	}
}

func TestQuizToSFQ_NoTitle(t *testing.T) {
	q := &state.Quiz{
		Sections: []state.QuizSection{
			{ID: "q-001", Question: "What is Go?", Answer: "A language"},
		},
	}
	out := string(quizToSFQ(q))
	if strings.Contains(out, "# ") {
		t.Errorf("expected no title header when Title is empty, got:\n%s", out)
	}
	if !strings.Contains(out, "? What is Go?") {
		t.Errorf("expected question, got:\n%s", out)
	}
}

func TestNormalizeQuizProvenance_FillsFromPrefixedTags(t *testing.T) {
	q := &state.Quiz{
		Sections: []state.QuizSection{
			{
				ID:       "q-001",
				Question: "What is a derivative?",
				Tags: []string{
					"calculus",
					"src_section:sec-123",
					"src_component:cmp-456",
				},
			},
		},
	}

	normalizeQuizProvenance(q)

	if got := q.Sections[0].SectionID; got != "sec-123" {
		t.Fatalf("expected section id from tag, got %q", got)
	}
	if got := q.Sections[0].ComponentID; got != "cmp-456" {
		t.Fatalf("expected component id from tag, got %q", got)
	}
}

func TestNormalizeQuizProvenance_AddsMissingPrefixedTags(t *testing.T) {
	q := &state.Quiz{
		Sections: []state.QuizSection{
			{
				ID:          "q-001",
				Question:    "What is Bayes theorem?",
				SectionID:   "sec-bayes",
				ComponentID: "cmp-bayes",
				Tags:        []string{"probability"},
			},
		},
	}

	normalizeQuizProvenance(q)

	if !hasTag(q.Sections[0].Tags, "src_section:sec-bayes") {
		t.Fatalf("expected src_section tag, got %#v", q.Sections[0].Tags)
	}
	if !hasTag(q.Sections[0].Tags, "src_component:cmp-bayes") {
		t.Fatalf("expected src_component tag, got %#v", q.Sections[0].Tags)
	}
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), want) {
			return true
		}
	}
	return false
}

func TestFinalizeExplicitDirectives_AssignsRequestedTotal(t *testing.T) {
	directives, err := finalizeExplicitDirectives([]OrchestratorDirective{{
		ComponentID: "cmp-1",
	}}, 1, "multiple-choice")
	if err != nil {
		t.Fatalf("finalizeExplicitDirectives returned error: %v", err)
	}
	if len(directives) != 1 {
		t.Fatalf("expected 1 directive, got %d", len(directives))
	}
	if directives[0].QuestionCount != 1 {
		t.Fatalf("expected question_count 1, got %d", directives[0].QuestionCount)
	}
	if len(directives[0].QuestionTypes) != 1 || directives[0].QuestionTypes[0] != "multiple-choice" {
		t.Fatalf("expected default question type, got %#v", directives[0].QuestionTypes)
	}
}

func TestFinalizeExplicitDirectives_RejectsCountMismatch(t *testing.T) {
	_, err := finalizeExplicitDirectives([]OrchestratorDirective{{
		ComponentID:   "cmp-1",
		QuestionCount: 2,
	}}, 1, "multiple-choice")
	if err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds requested total") {
		t.Fatalf("expected count mismatch error, got %v", err)
	}
}

func TestFinalizeExplicitDirectives_DefaultsMissingCountsWhenTotalAbsent(t *testing.T) {
	directives, err := finalizeExplicitDirectives([]OrchestratorDirective{
		{ComponentID: "cmp-1"},
		{ComponentID: "cmp-2", QuestionCount: 2},
	}, 0, "short-answer")
	if err != nil {
		t.Fatalf("finalizeExplicitDirectives returned error: %v", err)
	}
	if directives[0].QuestionCount != 1 {
		t.Fatalf("expected first directive to default to 1, got %d", directives[0].QuestionCount)
	}
	if directives[1].QuestionCount != 2 {
		t.Fatalf("expected second directive to preserve explicit count, got %d", directives[1].QuestionCount)
	}
	if len(directives[0].QuestionTypes) != 1 || directives[0].QuestionTypes[0] != "short-answer" {
		t.Fatalf("expected default question type, got %#v", directives[0].QuestionTypes)
	}
}
