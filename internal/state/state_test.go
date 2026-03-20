package state

import (
	"testing"
	"time"
)

func TestAddOrUpdate_MergesBySourceWithoutDuplicateSourceTags(t *testing.T) {
	idx := &NotesIndex{}
	createdAt := time.Now().UTC()

	first := Note{
		ID:        "n-1",
		Source:    "notes/unit1.md",
		SourceTag: "markdown",
		Class:     "math",
		Summary:   "first summary",
		Tags:      []string{"limits"},
		Concepts:  []string{"derivative"},
		CreatedAt: createdAt,
	}
	idx.AddOrUpdate(first)

	// Same source path but different generated id should consolidate into one note.
	second := Note{
		ID:        "n-2",
		Source:    "notes/unit1.md",
		SourceTag: "markdown",
		Class:     "math",
		Summary:   "updated summary",
		Tags:      []string{"chain-rule"},
		Concepts:  []string{"derivative"},
		CreatedAt: createdAt.Add(time.Minute),
	}
	idx.AddOrUpdate(second)

	if len(idx.Notes) != 1 {
		t.Fatalf("expected one consolidated note, got %d", len(idx.Notes))
	}

	note := idx.Notes[0]
	if note.Summary != "updated summary" {
		t.Fatalf("expected latest summary, got %q", note.Summary)
	}
	if countExact(note.Tags, "source:markdown") != 1 {
		t.Fatalf("expected one source tag, got tags: %#v", note.Tags)
	}
	if countExact(note.Sources, "notes/unit1.md") != 1 {
		t.Fatalf("expected one source path in sources, got: %#v", note.Sources)
	}
}

func TestAddOrUpdate_MergesByIDAcrossDifferentSources(t *testing.T) {
	idx := &NotesIndex{}

	idx.AddOrUpdate(Note{
		ID:        "limits",
		Source:    "notes/limits-a.md",
		SourceTag: "markdown",
		Tags:      []string{"calc"},
		Concepts:  []string{"limits"},
		CreatedAt: time.Now().UTC(),
	})
	idx.AddOrUpdate(Note{
		ID:        "limits",
		Source:    "notes/limits-b.txt",
		SourceTag: "text",
		Tags:      []string{"review"},
		Concepts:  []string{"continuity"},
		CreatedAt: time.Now().UTC().Add(time.Minute),
	})

	if len(idx.Notes) != 1 {
		t.Fatalf("expected merge by id to keep one note, got %d", len(idx.Notes))
	}
	note := idx.Notes[0]
	if countExact(note.Sources, "notes/limits-a.md") != 1 || countExact(note.Sources, "notes/limits-b.txt") != 1 {
		t.Fatalf("expected both provenance sources present exactly once, got: %#v", note.Sources)
	}
	if countExact(note.Tags, "source:markdown") != 1 || countExact(note.Tags, "source:text") != 1 {
		t.Fatalf("expected source tags for both sources, got: %#v", note.Tags)
	}
}

func countExact(items []string, want string) int {
	count := 0
	for _, item := range items {
		if item == want {
			count++
		}
	}
	return count
}
