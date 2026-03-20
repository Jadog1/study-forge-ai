package state

import "testing"

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
