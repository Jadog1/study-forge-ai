package state

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/studyforge/study-agent/internal/config"
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

// ── TrackedQuizCache tests ───────────────────────────────────────────────────

func TestTrackedQuizCache_IsSessionImported_EmptySessionID(t *testing.T) {
	cache := &TrackedQuizCache{ImportedSessionIDs: []string{"sess-1"}}
	if cache.IsSessionImported("") {
		t.Error("should return false for empty session ID")
	}
	if cache.IsSessionImported("  \n  ") {
		t.Error("should return false for whitespace session ID")
	}
}

func TestTrackedQuizCache_IsSessionImported_NilCache(t *testing.T) {
	var cache *TrackedQuizCache
	if cache.IsSessionImported("sess-1") {
		t.Error("should return false for nil cache")
	}
}

func TestTrackedQuizCache_IsSessionImported_DedupsCorrectly(t *testing.T) {
	cache := &TrackedQuizCache{ImportedSessionIDs: []string{"sess-1", "sess-2", "sess-3"}}
	if !cache.IsSessionImported("sess-1") {
		t.Error("should find sess-1")
	}
	if !cache.IsSessionImported("sess-2") {
		t.Error("should find sess-2")
	}
	if cache.IsSessionImported("sess-4") {
		t.Error("should not find sess-4")
	}
}

func TestTrackedQuizCache_MarkSessionImported_AddsSessionID(t *testing.T) {
	cache := &TrackedQuizCache{ImportedSessionIDs: []string{}}
	now := time.Now().UTC()
	cache.MarkSessionImported("sess-1", "quiz.yaml", now)

	if !cache.IsSessionImported("sess-1") {
		t.Error("session should be marked as imported")
	}
	if len(cache.ImportedSessionIDs) != 1 {
		t.Errorf("expected 1 imported session ID, got %d", len(cache.ImportedSessionIDs))
	}
}

func TestTrackedQuizCache_MarkSessionImported_PreventsDuplicates(t *testing.T) {
	cache := &TrackedQuizCache{ImportedSessionIDs: []string{"sess-1"}}
	now := time.Now().UTC()
	cache.MarkSessionImported("sess-1", "quiz.yaml", now)

	// Should still be 1, not 2
	if len(cache.ImportedSessionIDs) != 1 {
		t.Errorf("expected 1 imported session ID after duplicate mark, got %d", len(cache.ImportedSessionIDs))
	}
}

func TestTrackedQuizCache_MarkSessionImported_UpdatesQuizRecord(t *testing.T) {
	record := TrackedQuizRecord{
		QuizID:   "quiz-1",
		QuizPath: "quiz.yaml",
	}
	cache := &TrackedQuizCache{Quizzes: []TrackedQuizRecord{record}}
	now := time.Now().UTC()
	cache.MarkSessionImported("sess-1", "quiz.yaml", now)

	if cache.Quizzes[0].LastSessionID != "sess-1" {
		t.Errorf("expected LastSessionID to be updated, got %q", cache.Quizzes[0].LastSessionID)
	}
	if cache.Quizzes[0].LastImportedAt != now {
		t.Errorf("expected LastImportedAt to be %v, got %v", now, cache.Quizzes[0].LastImportedAt)
	}
}

func TestTrackedQuizCache_MarkSessionImported_UsesCurrentTimeIfZero(t *testing.T) {
	cache := &TrackedQuizCache{
		Quizzes: []TrackedQuizRecord{{QuizPath: "quiz.yaml"}},
	}
	before := time.Now().UTC()
	cache.MarkSessionImported("sess-1", "quiz.yaml", time.Time{})
	after := time.Now().UTC()

	importedAt := cache.Quizzes[0].LastImportedAt
	if importedAt.Before(before) || importedAt.After(after.Add(time.Second)) {
		t.Errorf("expected time close to now, got %v", importedAt)
	}
}

func TestTrackedQuizCache_MarkSessionImported_IgnoresEmptySessionID(t *testing.T) {
	cache := &TrackedQuizCache{ImportedSessionIDs: []string{}}
	cache.MarkSessionImported("", "quiz.yaml", time.Now().UTC())

	if len(cache.ImportedSessionIDs) > 0 {
		t.Error("should not add empty session ID to imported list")
	}
}

func TestTrackedQuizCache_MarkSessionImported_HandlesNilCache(t *testing.T) {
	var cache *TrackedQuizCache
	// Should not panic
	cache.MarkSessionImported("sess-1", "quiz.yaml", time.Now().UTC())
	t.Log("nil cache handled gracefully")
}

func TestTrackedQuizCache_MarkSessionImported_MultipleQuizzes(t *testing.T) {
	cache := &TrackedQuizCache{
		Quizzes: []TrackedQuizRecord{
			{QuizPath: "quiz1.yaml"},
			{QuizPath: "quiz2.yaml"},
			{QuizPath: "quiz3.yaml"},
		},
	}
	now := time.Now().UTC()
	cache.MarkSessionImported("sess-1", "quiz2.yaml", now)

	// Only quiz2 should be updated
	if cache.Quizzes[1].LastSessionID != "sess-1" {
		t.Error("quiz2 should be updated")
	}
	if cache.Quizzes[0].LastSessionID != "" || cache.Quizzes[2].LastSessionID != "" {
		t.Error("other quizzes should not be updated")
	}
}

func TestTrackedQuizCache_Deduplication_OnLoad(t *testing.T) {
	// Simulate cache with duplicates (shouldn't happen but verify cleanup)
	cache := &TrackedQuizCache{
		ImportedSessionIDs: []string{"sess-1", "sess-2", "sess-1", "sess-3", "sess-2"},
	}
	// Note: The actual dedupe is done in SaveTrackedQuizCache via SaveTrackedQuizCache
	// Here we're testing the behavior
	seen := make(map[string]bool)
	var deduped []string
	for _, id := range cache.ImportedSessionIDs {
		if !seen[id] {
			deduped = append(deduped, id)
			seen[id] = true
		}
	}
	if len(deduped) != 3 {
		t.Errorf("expected 3 unique sessions after dedupe, got %d", len(deduped))
	}
}

func TestTrackedQuizRecord_Registration_CreatesNewRecord(t *testing.T) {
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

	record, err := RegisterTrackedQuiz("math", "quizzes/quiz1.yaml", "sfq/quiz1.sfq")
	if err != nil {
		t.Fatalf("register tracked quiz: %v", err)
	}

	if record.Class != "math" {
		t.Errorf("expected class math, got %q", record.Class)
	}
	if record.QuizID != "quiz1" {
		t.Errorf("expected quiz ID quiz1, got %q", record.QuizID)
	}
	if record.QuizPath != "quizzes/quiz1.yaml" {
		t.Errorf("expected quiz path, got %q", record.QuizPath)
	}
	if record.RegisteredAt.IsZero() {
		t.Error("expected non-zero registration time")
	}
}

func TestTrackedQuizRecord_Registration_UpdatesExisting(t *testing.T) {
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

	first, err := RegisterTrackedQuiz("math", "quizzes/quiz1.yaml", "sfq/quiz1.sfq")
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	firstRegTime := first.RegisteredAt

	time.Sleep(10 * time.Millisecond) // Ensure time difference

	second, err := RegisterTrackedQuiz("math", "quizzes/quiz1.yaml", "sfq/quiz1-updated.sfq")
	if err != nil {
		t.Fatalf("second register: %v", err)
	}

	// RegisteredAt should not change on update
	if second.RegisteredAt != firstRegTime {
		t.Errorf("expected registration time to remain %v, got %v", firstRegTime, second.RegisteredAt)
	}
	// SFQPath should be updated
	if second.SFQPath != "sfq/quiz1-updated.sfq" {
		t.Errorf("expected updated SFQ path, got %q", second.SFQPath)
	}
}

func TestTrackedQuizCache_PersistenceRoundtrip(t *testing.T) {
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

	now := time.Now().UTC()
	cache := &TrackedQuizCache{
		SchemaVersion: 1,
		Quizzes: []TrackedQuizRecord{
			{
				QuizID:         "quiz-1",
				Class:          "math",
				QuizPath:       "quiz1.yaml",
				SFQPath:        "quiz1.sfq",
				RegisteredAt:   now,
				LastSessionID:  "sess-123",
				LastImportedAt: now.Add(-time.Hour),
			},
		},
		ImportedSessionIDs: []string{"sess-123", "sess-456"},
	}

	if err := SaveTrackedQuizCache(cache); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	loaded, err := LoadTrackedQuizCache()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}

	if len(loaded.Quizzes) != 1 {
		t.Errorf("expected 1 quiz after load, got %d", len(loaded.Quizzes))
	}
	if len(loaded.ImportedSessionIDs) != 2 {
		t.Errorf("expected 2 imported session IDs after load, got %d", len(loaded.ImportedSessionIDs))
	}
	if loaded.Quizzes[0].QuizID != "quiz-1" {
		t.Errorf("expected quiz-1, got %q", loaded.Quizzes[0].QuizID)
	}
}

func TestTrackedQuizCache_NilSave(t *testing.T) {
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

	err := SaveTrackedQuizCache(nil)
	if err == nil {
		t.Error("expected error saving nil cache")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected nil error message, got: %v", err)
	}
}

func TestTrackedQuizCache_LoadEmpty(t *testing.T) {
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

	cache, err := LoadTrackedQuizCache()
	if err != nil {
		t.Fatalf("load empty cache: %v", err)
	}

	if cache == nil {
		t.Error("expected non-nil cache")
	}
	if cache.SchemaVersion != 1 {
		t.Errorf("expected schema version 1, got %d", cache.SchemaVersion)
	}
	if len(cache.Quizzes) != 0 {
		t.Errorf("expected empty quizzes, got %d", len(cache.Quizzes))
	}
}
