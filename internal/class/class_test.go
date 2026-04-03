package class

import (
	"os"
	"testing"

	"github.com/studyforge/study-agent/internal/config"
)

func TestNoteRosterCRUDAndReorder(t *testing.T) {
	setupClassTestHome(t)

	if err := Create("biology"); err != nil {
		t.Fatalf("create class: %v", err)
	}

	if _, err := UpsertNoteRosterEntry("biology", NoteRosterEntry{Label: "Week 1 Lecture", SourcePattern: "week 1 lecture", Week: 1}); err != nil {
		t.Fatalf("upsert first entry: %v", err)
	}
	if _, err := UpsertNoteRosterEntry("biology", NoteRosterEntry{Label: "Week 2 Lecture", SourcePattern: "week 2 lecture", Week: 2}); err != nil {
		t.Fatalf("upsert second entry: %v", err)
	}

	roster, err := ReorderNoteRosterEntries("biology", []string{"Week 2 Lecture", "Week 1 Lecture"})
	if err != nil {
		t.Fatalf("reorder roster: %v", err)
	}
	if len(roster.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(roster.Entries))
	}
	if roster.Entries[0].Label != "Week 2 Lecture" || roster.Entries[0].Order != 1 {
		t.Fatalf("expected Week 2 first, got %#v", roster.Entries[0])
	}

	roster, err = RemoveNoteRosterEntry("biology", "Week 1 Lecture")
	if err != nil {
		t.Fatalf("remove roster entry: %v", err)
	}
	if len(roster.Entries) != 1 || roster.Entries[0].Label != "Week 2 Lecture" {
		t.Fatalf("unexpected roster after remove: %#v", roster.Entries)
	}
}

func TestCoverageScopeSaveLoadAndResolvePatterns(t *testing.T) {
	setupClassTestHome(t)

	if err := Create("chemistry"); err != nil {
		t.Fatalf("create class: %v", err)
	}
	if _, err := UpsertNoteRosterEntry("chemistry", NoteRosterEntry{Label: "Exam 2 Window", SourcePattern: "week 5"}); err != nil {
		t.Fatalf("upsert roster entry: %v", err)
	}

	scope := &CoverageScope{
		Class: "chemistry",
		Kind:  "exam",
		Groups: []ScopeGroup{
			{Labels: []string{"Exam 2 Window"}, Weight: 1.0},
			{SourcePatterns: []string{"week 1"}, Weight: 0.3},
		},
		ExcludeUnmatched: true,
	}
	if err := SaveCoverageScope("chemistry", "exam", scope); err != nil {
		t.Fatalf("save coverage scope: %v", err)
	}

	loaded, err := LoadCoverageScope("chemistry", "exam")
	if err != nil {
		t.Fatalf("load coverage scope: %v", err)
	}
	if loaded == nil || len(loaded.Groups) != 2 {
		t.Fatalf("expected 2 scope groups, got %#v", loaded)
	}
	roster, err := LoadNoteRoster("chemistry")
	if err != nil {
		t.Fatalf("load roster: %v", err)
	}
	patterns := ResolveGroupPatterns(loaded.Groups[0], roster)
	if len(patterns) != 1 || patterns[0] != "week 5" {
		t.Fatalf("expected resolved pattern week 5, got %#v", patterns)
	}
}

func setupClassTestHome(t *testing.T) {
	t.Helper()
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
}
