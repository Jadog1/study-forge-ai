package quiz

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/studyforge/study-agent/internal/state"
)

func TestSelectCandidatesDiversified_AnchorsTopAndExplores(t *testing.T) {
	scores := make([]ComponentScore, 0, 40)
	for i := 0; i < 40; i++ {
		scores = append(scores, ComponentScore{
			Component: state.Component{ID: fmt.Sprintf("cmp-%02d", i)},
			Score:     1.0 - (float64(i) * 0.01),
		})
	}

	selected := SelectCandidatesDiversified(scores, 12, 0.40, rand.New(rand.NewSource(42)))
	if len(selected) != 12 {
		t.Fatalf("expected 12 selected candidates, got %d", len(selected))
	}

	for i := 0; i < 7; i++ {
		if selected[i].Component.ID != scores[i].Component.ID {
			t.Fatalf("expected anchor candidate %q at index %d, got %q", scores[i].Component.ID, i, selected[i].Component.ID)
		}
	}

	seen := make(map[string]bool, len(selected))
	hasBeyondTopN := false
	for _, s := range selected {
		if seen[s.Component.ID] {
			t.Fatalf("duplicate component selected: %s", s.Component.ID)
		}
		seen[s.Component.ID] = true
		idx := componentIndexFromID(s.Component.ID)
		if idx >= 12 {
			hasBeyondTopN = true
		}
	}
	if !hasBeyondTopN {
		t.Fatalf("expected exploratory picks beyond strict top-N, got %#v", selected)
	}
}

func TestApplyRecentGenerationPenalty_DemotesRecentComponent(t *testing.T) {
	now := time.Now().UTC()
	scores := []ComponentScore{
		{Component: state.Component{ID: "cmp-a"}, Score: 0.90},
		{Component: state.Component{ID: "cmp-b"}, Score: 0.85},
	}
	recent := map[string]time.Time{
		"cmp-a": now.Add(-1 * time.Hour),
		"cmp-b": now.Add(-8 * 24 * time.Hour),
	}

	penalized := applyRecentGenerationPenalty(scores, recent, now, 72*time.Hour, 0.45)
	if penalized[0].Component.ID != "cmp-b" {
		t.Fatalf("expected cmp-b to rank first after cooldown penalty, got %q", penalized[0].Component.ID)
	}
	if penalized[1].Score >= scores[0].Score {
		t.Fatalf("expected cmp-a score to be reduced, before=%.3f after=%.3f", scores[0].Score, penalized[1].Score)
	}
}

func TestFilterDuplicateQuestionSections_RemovesHistoryAndInBatchDuplicates(t *testing.T) {
	seen := map[string]bool{
		normalizeQuestionKey("What is entropy?"): true,
	}
	sections := []state.QuizSection{
		{ID: "q-1", Question: "  What is entropy?  "},
		{ID: "q-2", Question: "Explain gradient descent"},
		{ID: "q-3", Question: "Explain   gradient    descent"},
	}

	filtered := filterDuplicateQuestionSections(sections, seen)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 unique question after filtering, got %d", len(filtered))
	}
	if filtered[0].ID != "q-2" {
		t.Fatalf("expected q-2 to remain, got %s", filtered[0].ID)
	}
	if !seen[normalizeQuestionKey("Explain gradient descent")] {
		t.Fatal("expected seen set to be updated with retained question key")
	}
}

func TestScoringSimulation_DiversifiedSelectorImprovesCoverageWithoutResultSync(t *testing.T) {
	class := "sim-class"
	now := time.Now().UTC().Add(-24 * time.Hour)

	secIdx := &state.SectionIndex{Sections: make([]state.Section, 0, 100)}
	cmpIdx := &state.ComponentIndex{Components: make([]state.Component, 0, 100)}

	for i := 0; i < 100; i++ {
		secID := fmt.Sprintf("sec-%03d", i)
		cmpID := fmt.Sprintf("cmp-%03d", i)
		secIdx.Sections = append(secIdx.Sections, state.Section{ID: secID, Class: class, Title: secID})

		history := []state.QuestionHistoryEntry{}
		if i < 24 {
			history = []state.QuestionHistoryEntry{{
				ID:         fmt.Sprintf("hist-%03d", i),
				QuestionID: "q-001",
				Question:   "placeholder",
				Correct:    false,
				AnsweredAt: now,
			}}
		}

		cmpIdx.Components = append(cmpIdx.Components, state.Component{
			ID:              cmpID,
			SectionID:       secID,
			Class:           class,
			Kind:            "concept",
			Content:         cmpID,
			QuestionHistory: history,
		})
	}

	scores := ScoreComponents(class, secIdx, cmpIdx)

	baselineSeen := make(map[string]bool)
	for run := 0; run < 20; run++ {
		selected := SelectCandidates(scores, 12)
		for _, s := range selected {
			baselineSeen[s.Component.ID] = true
		}
	}

	diversifiedSeen := make(map[string]bool)
	rng := rand.New(rand.NewSource(7))
	for run := 0; run < 20; run++ {
		selected := SelectCandidatesDiversified(scores, 12, 0.35, rng)
		for _, s := range selected {
			diversifiedSeen[s.Component.ID] = true
		}
	}

	if len(baselineSeen) != 12 {
		t.Fatalf("expected baseline to stay fixed at 12 unique components, got %d", len(baselineSeen))
	}
	if len(diversifiedSeen) <= len(baselineSeen) {
		t.Fatalf("expected diversified selection to improve coverage, baseline=%d diversified=%d", len(baselineSeen), len(diversifiedSeen))
	}
	if len(diversifiedSeen) < 24 {
		t.Fatalf("expected diversified selection to cover at least 24 components, got %d", len(diversifiedSeen))
	}
}

func componentIndexFromID(id string) int {
	id = strings.TrimPrefix(id, "cmp-")
	var idx int
	_, _ = fmt.Sscanf(id, "%d", &idx)
	return idx
}
