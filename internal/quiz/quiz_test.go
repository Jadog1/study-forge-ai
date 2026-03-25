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

func TestRebalanceChoiceAnswerPositions_MovesCorrectFromFirst(t *testing.T) {
	sections := []state.QuizSection{
		{
			Type:        "multiple-choice",
			Question:    "Alpha question?",
			SectionID:   "sec-1",
			ComponentID: "cmp-1",
			Choices: []state.QuizChoice{
				{Text: "Correct", Correct: true},
				{Text: "Wrong 1", Correct: false},
				{Text: "Wrong 2", Correct: false},
				{Text: "Wrong 3", Correct: false},
			},
		},
	}

	rebalanceChoiceAnswerPositions(sections)

	correctIdx := -1
	for i, ch := range sections[0].Choices {
		if ch.Correct {
			correctIdx = i
			break
		}
	}
	if correctIdx == -1 {
		t.Fatal("expected one correct choice to remain")
	}
	if correctIdx == 0 {
		t.Fatalf("expected correct choice to move away from position A, got index %d", correctIdx)
	}
}

func TestRebalanceChoiceAnswerPositions_PreservesChoiceSet(t *testing.T) {
	sections := []state.QuizSection{
		{
			Type:        "multiple-choice",
			Question:    "Beta question?",
			SectionID:   "sec-2",
			ComponentID: "cmp-2",
			Choices: []state.QuizChoice{
				{Text: "A", Correct: true},
				{Text: "B", Correct: false},
				{Text: "C", Correct: false},
				{Text: "D", Correct: false},
			},
		},
	}

	rebalanceChoiceAnswerPositions(sections)

	if len(sections[0].Choices) != 4 {
		t.Fatalf("expected 4 choices to remain, got %d", len(sections[0].Choices))
	}
	correctCount := 0
	texts := make(map[string]bool)
	for _, ch := range sections[0].Choices {
		texts[ch.Text] = true
		if ch.Correct {
			correctCount++
		}
	}
	if correctCount != 1 {
		t.Fatalf("expected exactly one correct choice, got %d", correctCount)
	}
	for _, label := range []string{"A", "B", "C", "D"} {
		if !texts[label] {
			t.Fatalf("missing choice %q after rebalance", label)
		}
	}
}

func TestRebalanceChoiceAnswerPositions_MultiSelectShufflesDeterministically(t *testing.T) {
	sections := []state.QuizSection{
		{
			Type:        "multi-select",
			Question:    "Pick all prime numbers",
			SectionID:   "sec-ms",
			ComponentID: "cmp-ms",
			Choices: []state.QuizChoice{
				{Text: "2", Correct: true},
				{Text: "3", Correct: true},
				{Text: "4", Correct: false},
				{Text: "6", Correct: false},
			},
		},
	}

	original := append([]state.QuizChoice(nil), sections[0].Choices...)
	rebalanceChoiceAnswerPositions(sections)

	if len(sections[0].Choices) != len(original) {
		t.Fatalf("expected same number of choices, got %d", len(sections[0].Choices))
	}
	if sections[0].Choices[0].Text == original[0].Text && sections[0].Choices[1].Text == original[1].Text {
		t.Fatal("expected multi-select choice order to change to reduce fixed answer-position patterns")
	}
}

func TestRebalanceChoiceAnswerPositions_TrueFalseMovesCorrectFromFirst(t *testing.T) {
	sections := []state.QuizSection{
		{
			Type:        "true-false",
			Question:    "The sky is blue.",
			SectionID:   "sec-tf",
			ComponentID: "cmp-tf",
			Choices: []state.QuizChoice{
				{Text: "True", Correct: true},
				{Text: "False", Correct: false},
			},
		},
	}

	target := stableChoiceTarget(sections[0], len(sections[0].Choices))
	rebalanceChoiceAnswerPositions(sections)

	correctIdx := -1
	for i, ch := range sections[0].Choices {
		if ch.Correct {
			correctIdx = i
			break
		}
	}
	if correctIdx != target {
		t.Fatalf("expected true-false correct option at index %d, got %d", target, correctIdx)
	}
}

func TestApplyDirectiveDifficultyGuidance_AppendsSupportiveAngleHint(t *testing.T) {
	directives := []OrchestratorDirective{{
		ComponentID: "cmp-1",
		Angle:       "focus on fundamentals",
	}}
	scoreByComponent := map[string]ComponentScore{
		"cmp-1": {
			Component:      state.Component{ID: "cmp-1"},
			DifficultyBand: "supportive",
		},
	}

	got := applyDirectiveDifficultyGuidance(directives, scoreByComponent)
	if !strings.Contains(got[0].Angle, "difficulty:supportive") {
		t.Fatalf("expected supportive guidance in angle, got %q", got[0].Angle)
	}
}

func TestDirectiveDifficultySupplement_AdvancedMixedWhenRecentThoughtProvokingHigh(t *testing.T) {
	supplement := directiveDifficultySupplement(ComponentScore{
		DifficultyBand:   "advanced",
		ThoughtProvoking: 0.75,
	})
	if !strings.Contains(supplement, "advanced-mixed") {
		t.Fatalf("expected advanced-mixed supplement, got %q", supplement)
	}
}

func TestRebalanceChoiceAnswerPositions_OrderingUnchanged(t *testing.T) {
	sections := []state.QuizSection{
		{
			Type:        "ordering",
			Question:    "Order the lifecycle stages.",
			SectionID:   "sec-order",
			ComponentID: "cmp-order",
			Choices: []state.QuizChoice{
				{Text: "Stage 1", Correct: true},
				{Text: "Stage 2", Correct: true},
				{Text: "Stage 3", Correct: true},
			},
		},
	}
	original := append([]state.QuizChoice(nil), sections[0].Choices...)

	rebalanceChoiceAnswerPositions(sections)

	for i := range original {
		if sections[0].Choices[i].Text != original[i].Text {
			t.Fatalf("expected ordering choice %d to remain unchanged", i)
		}
	}
}
