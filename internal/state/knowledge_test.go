package state

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/studyforge/study-agent/internal/config"
)

func TestSectionIndexAddOrUpdateMergesSources(t *testing.T) {
	idx := &SectionIndex{SchemaVersion: 1}
	idx.AddOrUpdate(Section{
		ID:          "sec-1",
		Title:       "Derivatives",
		SourcePaths: []string{"a.md"},
		SourceTags:  []string{"markdown"},
		Tags:        []string{"calc"},
	})
	idx.AddOrUpdate(Section{
		ID:          "sec-1",
		Summary:     "rate of change",
		SourcePaths: []string{"b.md", "a.md"},
		SourceTags:  []string{"markdown", "text"},
		Tags:        []string{"limits", "calc"},
	})

	if len(idx.Sections) != 1 {
		t.Fatalf("expected one section after merge, got %d", len(idx.Sections))
	}
	section := idx.Sections[0]
	if len(section.SourcePaths) != 2 {
		t.Fatalf("expected merged source paths, got %#v", section.SourcePaths)
	}
	if len(section.Tags) != 2 {
		t.Fatalf("expected deduped tags, got %#v", section.Tags)
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{1, 0, 0}
	c := []float64{0, 1, 0}

	if score := CosineSimilarity(a, b); score < 0.999 {
		t.Fatalf("expected near 1.0 similarity, got %f", score)
	}
	if score := CosineSimilarity(a, c); score > 0.001 {
		t.Fatalf("expected near 0.0 similarity, got %f", score)
	}
}

func TestBuildUsageTotalsRepricesHistoricalEvents(t *testing.T) {
	now := time.Now().UTC()
	ledger := &UsageLedger{Events: []UsageEvent{{
		Provider:     "openai",
		Model:        "custom-model",
		InputTokens:  2_000,
		OutputTokens: 3_000,
		TotalTokens:  5_000,
		CostUSD:      0,
		CreatedAt:    now,
	}}}
	cfg := &config.Config{ModelPrices: map[string]config.ModelPrice{
		"custom-model": {InputPerMillion: 1.5, OutputPerMillion: 2.5},
	}}

	totals := BuildUsageTotals(ledger, cfg, UsageFilter{})
	want := config.ComputeCost(2_000, 3_000, 1.5, 2.5)
	if math.Abs(totals.TotalCostUSD-want) > 1e-9 {
		t.Fatalf("expected repriced total cost %.6f, got %.6f", want, totals.TotalCostUSD)
	}
	modelTotals, ok := totals.ByModel["openai:custom-model"]
	if !ok {
		t.Fatalf("expected per-model totals entry")
	}
	if math.Abs(modelTotals.CostUSD-want) > 1e-9 {
		t.Fatalf("expected repriced model cost %.6f, got %.6f", want, modelTotals.CostUSD)
	}
}

func TestBuildUsageTotalsAppliesTimestampFilter(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour)
	end := now
	ledger := &UsageLedger{Events: []UsageEvent{
		{
			Provider:     "openai",
			Model:        "gpt-4o-mini",
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
			CreatedAt:    now.Add(-48 * time.Hour),
		},
		{
			Provider:     "openai",
			Model:        "gpt-4o-mini",
			InputTokens:  500,
			OutputTokens: 200,
			TotalTokens:  700,
			CreatedAt:    now.Add(-10 * 24 * time.Hour),
		},
	}}

	totals := BuildUsageTotals(ledger, &config.Config{}, UsageFilter{CreatedAfter: &start, CreatedBefore: &end})
	if totals.TotalTokens != 150 {
		t.Fatalf("expected filtered total tokens 150, got %d", totals.TotalTokens)
	}
	if len(totals.ByModel) != 1 {
		t.Fatalf("expected one model in filtered totals, got %d", len(totals.ByModel))
	}
}

func TestAppendQuizQuestionHistory_AppendsToSectionAndComponent(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	})
	_ = os.Setenv("HOME", tmp)
	_ = os.Setenv("USERPROFILE", tmp)

	if _, err := config.EnsureInitialized(); err != nil {
		t.Fatalf("ensure initialized: %v", err)
	}

	section := Section{
		ID:      "sec-derivatives",
		Class:   "math",
		Title:   "Derivatives",
		Summary: "Rate of change",
	}
	component := Component{
		ID:        "cmp-product-rule",
		SectionID: "sec-derivatives",
		Class:     "math",
		Kind:      "fact",
		Content:   "Product rule definition",
	}
	secIdx := &SectionIndex{SchemaVersion: 1}
	secIdx.AddOrUpdate(section)
	cmpIdx := &ComponentIndex{SchemaVersion: 1}
	cmpIdx.AddOrUpdate(component)
	if err := SaveSectionIndex(secIdx); err != nil {
		t.Fatalf("save section index: %v", err)
	}
	if err := SaveComponentIndex(cmpIdx); err != nil {
		t.Fatalf("save component index: %v", err)
	}

	quizDoc := Quiz{
		Title: "Quiz",
		Class: "math",
		Sections: []QuizSection{
			{
				ID:          "q-001",
				Question:    "What is the product rule?",
				Answer:      "f'g + fg'",
				SectionID:   "sec-derivatives",
				ComponentID: "cmp-product-rule",
			},
		},
	}
	results := QuizResults{
		QuizID:      strings.TrimSuffix(filepath.Base("quiz-1.yaml"), ".yaml"),
		CompletedAt: time.Now().UTC(),
		Results: []QuizResult{{
			QuestionID:  "q-001",
			Correct:     false,
			UserAnswer:  "f+g",
			AnsweredAt:  time.Now().UTC(),
			SectionID:   "sec-derivatives",
			ComponentID: "cmp-product-rule",
		}},
	}

	if err := AppendQuizQuestionHistory("math", quizDoc, results); err != nil {
		t.Fatalf("append quiz history: %v", err)
	}

	updatedSec, err := LoadSectionIndex()
	if err != nil {
		t.Fatalf("load section index: %v", err)
	}
	updatedCmp, err := LoadComponentIndex()
	if err != nil {
		t.Fatalf("load component index: %v", err)
	}

	if len(updatedSec.Sections) != 1 || len(updatedSec.Sections[0].QuestionHistory) != 1 {
		t.Fatalf("expected one section history entry, got %#v", updatedSec.Sections)
	}
	if len(updatedCmp.Components) != 1 || len(updatedCmp.Components[0].QuestionHistory) != 1 {
		t.Fatalf("expected one component history entry, got %#v", updatedCmp.Components)
	}

	entry := updatedSec.Sections[0].QuestionHistory[0]
	if entry.QuestionID != "q-001" || entry.Correct {
		t.Fatalf("unexpected section history entry: %#v", entry)
	}
	if entry.UserAnswer != "f+g" {
		t.Fatalf("expected stored user answer, got %#v", entry)
	}
}
