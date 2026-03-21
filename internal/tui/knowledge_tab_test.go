package tui

import (
	"testing"
	"time"

	"github.com/studyforge/study-agent/internal/state"
)

func TestKnowledgeMetricsFromHistory(t *testing.T) {
	now := time.Now().UTC()
	metrics := knowledgeMetricsFromHistory([]state.QuestionHistoryEntry{
		{ID: "qh-1", Correct: true, AnsweredAt: now.Add(-2 * time.Hour)},
		{ID: "qh-2", Correct: false, AnsweredAt: now},
	})

	if metrics.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", metrics.Attempts)
	}
	if metrics.Correct != 1 || metrics.Incorrect != 1 {
		t.Fatalf("unexpected correct/incorrect counts: %+v", metrics)
	}
	if metrics.Accuracy != 0.5 {
		t.Fatalf("expected 0.5 accuracy, got %f", metrics.Accuracy)
	}
	if !metrics.LastAnswered.Equal(now) {
		t.Fatalf("expected latest timestamp %s, got %s", now, metrics.LastAnswered)
	}
}

func TestAggregateSectionMetricsDedupesSectionAndComponentHistory(t *testing.T) {
	base := time.Now().UTC()
	section := state.Section{
		ID: "sec-1",
		QuestionHistory: []state.QuestionHistoryEntry{
			{ID: "qh-1", Correct: true, AnsweredAt: base.Add(-time.Hour)},
		},
	}
	components := []state.Component{
		{
			ID: "cmp-1",
			QuestionHistory: []state.QuestionHistoryEntry{
				{ID: "qh-1", Correct: true, AnsweredAt: base.Add(-time.Hour)},
				{ID: "qh-2", Correct: false, AnsweredAt: base},
			},
		},
	}

	metrics := aggregateSectionMetrics(section, components)
	if metrics.Attempts != 2 {
		t.Fatalf("expected duplicate question history to be removed, got %d attempts", metrics.Attempts)
	}
	if metrics.Correct != 1 || metrics.Incorrect != 1 {
		t.Fatalf("unexpected aggregate counts: %+v", metrics)
	}
	if !metrics.LastAnswered.Equal(base) {
		t.Fatalf("expected latest answered time %s, got %s", base, metrics.LastAnswered)
	}
}
