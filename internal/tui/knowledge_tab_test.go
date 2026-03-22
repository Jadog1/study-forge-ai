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

func TestKnowledgeTabReceiveBuildsSortedEntriesAndComponents(t *testing.T) {
	now := time.Now().UTC()
	sectionIdx := &state.SectionIndex{SchemaVersion: 1, Sections: []state.Section{
		{ID: "sec-math", Class: "math", Title: "Algebra", UpdatedAt: now.Add(-time.Minute)},
		{ID: "sec-history", Class: "history", Title: "Rome", UpdatedAt: now},
	}}
	componentIdx := &state.ComponentIndex{SchemaVersion: 1, Components: []state.Component{
		{ID: "cmp-new", SectionID: "sec-math", Class: "math", Kind: "fact", Content: "new", UpdatedAt: now},
		{ID: "cmp-old", SectionID: "sec-math", Class: "math", Kind: "fact", Content: "old", UpdatedAt: now.Add(-time.Hour)},
	}}

	tab := newKnowledgeTab().receive(sectionIdx, componentIdx, nil)

	if !tab.loaded || tab.loading {
		t.Fatalf("expected tab to be loaded and not loading: loaded=%t loading=%t", tab.loaded, tab.loading)
	}
	if tab.totalComponentsCount != 2 {
		t.Fatalf("expected total components count 2, got %d", tab.totalComponentsCount)
	}
	if len(tab.entries) != 2 {
		t.Fatalf("expected 2 section entries, got %d", len(tab.entries))
	}
	if tab.entries[0].Section.ID != "sec-history" || tab.entries[1].Section.ID != "sec-math" {
		t.Fatalf("unexpected section order: %+v", tab.entries)
	}
	if len(tab.entries[1].Components) != 2 {
		t.Fatalf("expected 2 linked components for sec-math, got %d", len(tab.entries[1].Components))
	}
	if tab.entries[1].Components[0].Component.ID != "cmp-new" {
		t.Fatalf("expected newest component first, got %s", tab.entries[1].Components[0].Component.ID)
	}
}

func TestKnowledgeTabReceivePreservesSelectedSectionByID(t *testing.T) {
	now := time.Now().UTC()
	tab := KnowledgeTab{
		entries: []knowledgeSectionEntry{
			{Section: state.Section{ID: "sec-keep", Class: "science", Title: "Atoms"}},
			{Section: state.Section{ID: "sec-other", Class: "art", Title: "Painting"}},
		},
		selectedSection: 0,
		componentScroll: 25,
	}

	sectionIdx := &state.SectionIndex{SchemaVersion: 1, Sections: []state.Section{
		{ID: "sec-other", Class: "art", Title: "Painting", UpdatedAt: now},
		{ID: "sec-keep", Class: "science", Title: "Atoms", UpdatedAt: now},
	}}
	componentIdx := &state.ComponentIndex{SchemaVersion: 1, Components: []state.Component{}}

	updated := tab.receive(sectionIdx, componentIdx, nil)

	if updated.componentScroll != 0 {
		t.Fatalf("expected component scroll reset to 0, got %d", updated.componentScroll)
	}
	if updated.selectedSection != 1 {
		t.Fatalf("expected selected section to track sec-keep at index 1, got %d", updated.selectedSection)
	}
	if updated.entries[updated.selectedSection].Section.ID != "sec-keep" {
		t.Fatalf("expected selected section ID sec-keep, got %s", updated.entries[updated.selectedSection].Section.ID)
	}
}
