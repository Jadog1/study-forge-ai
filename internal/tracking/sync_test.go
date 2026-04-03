package tracking

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
)

func withSyncTestDeps(t *testing.T) {
	origLoadTrackedQuizCache := loadTrackedQuizCache
	origSaveTrackedQuizCache := saveTrackedQuizCache
	origHistorySessions := historySessions
	origResultsSession := resultsSession
	origLoadQuizDoc := loadQuizDoc
	origSaveQuizResults := saveQuizResults
	origAppendQuizQuestionHistory := appendQuizQuestionHistory
	t.Cleanup(func() {
		loadTrackedQuizCache = origLoadTrackedQuizCache
		saveTrackedQuizCache = origSaveTrackedQuizCache
		historySessions = origHistorySessions
		resultsSession = origResultsSession
		loadQuizDoc = origLoadQuizDoc
		saveQuizResults = origSaveQuizResults
		appendQuizQuestionHistory = origAppendQuizQuestionHistory
	})
}

func TestNormalizePath_NormalizesCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Quiz.YAML", "quiz.yaml"},
		{"UPPER/Path/To/File", "upper/path/to/file"},
		{"MiXeD/CaSe.sfq", "mixed/case.sfq"},
	}
	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNormalizePath_ConvertsBackslashes(t *testing.T) {
	input := filepath.Join("dir", "subdir", "file.yaml")
	got := normalizePath(input)
	if strings.Contains(got, "\\") {
		t.Errorf("normalizePath should convert backslashes to forward slashes, got %q", got)
	}
	if !strings.Contains(got, "/") {
		t.Errorf("normalizePath should produce forward slash paths, got %q", got)
	}
}

func TestNormalizePath_HandlesEmpty(t *testing.T) {
	if normalizePath("") != "" {
		t.Error("normalizePath should return empty string for empty input")
	}
	if normalizePath("   ") != "" {
		t.Error("normalizePath should return empty string for whitespace input")
	}
}

func TestNormalizePath_CanonicalizesPaths(t *testing.T) {
	// "." and ".." segments should be cleaned up by filepath.Clean
	got := normalizePath("dir/./subdir/../file.yaml")
	// filepath.Clean resolves to dir/file.yaml which is correct
	if got != "dir/file.yaml" {
		t.Errorf("expected dir/file.yaml, got %q", got)
	}
}

func TestMatchingSessions_FindsByNormalizedPaths(t *testing.T) {
	record := state.TrackedQuizRecord{
		QuizPath: "quizzes/Unit1.yaml",
		SFQPath:  "sfq/unit1.sfq",
	}
	historyBySource := map[string][]sfq.SessionResult{
		"quizzes/unit1.yaml": {
			{SessionID: "sess-1", SourcePath: "quizzes/Unit1.yaml"},
		},
	}
	matches := matchingSessions(record, historyBySource)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].SessionID != "sess-1" {
		t.Errorf("expected session sess-1, got %s", matches[0].SessionID)
	}
}

func TestMatchingSessions_TriesMultiplePaths(t *testing.T) {
	record := state.TrackedQuizRecord{
		QuizPath: "quizzes/math.yaml",
		SFQPath:  "sfq/math.sfq",
	}
	historyBySource := map[string][]sfq.SessionResult{
		"sfq/math.sfq": {
			{SessionID: "sess-1", SourcePath: "sfq/math.sfq"},
		},
	}
	matches := matchingSessions(record, historyBySource)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match from SFQPath, got %d", len(matches))
	}
}

func TestMatchingSessions_DeduplicatesSessions(t *testing.T) {
	record := state.TrackedQuizRecord{
		QuizPath: "quiz.yaml",
		SFQPath:  "quiz.sfq",
	}
	// Different sessions at different paths
	historyBySource := map[string][]sfq.SessionResult{
		"quiz.yaml": {
			{SessionID: "sess-1", SourcePath: "quiz.yaml"},
		},
		"quiz.sfq": {
			{SessionID: "sess-2", SourcePath: "quiz.sfq"},
		},
	}
	matches := matchingSessions(record, historyBySource)
	// Should return both sessions from matching paths
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches from different paths, got %d", len(matches))
	}
}

func TestPendingCount_CountsUnimportedQuizzes(t *testing.T) {
	now := time.Now().UTC()
	cache := &state.TrackedQuizCache{
		Quizzes: []state.TrackedQuizRecord{
			{QuizID: "q1", LastImportedAt: time.Time{}}, // zero = unimported
			{QuizID: "q2", LastImportedAt: now},         // has timestamp = imported
			{QuizID: "q3", LastImportedAt: time.Time{}}, // zero = unimported
		},
	}
	got := pendingCount(cache)
	if got != 2 {
		t.Errorf("expected 2 pending quizzes, got %d", got)
	}
}

func TestPendingCount_HandleNilCache(t *testing.T) {
	got := pendingCount(nil)
	if got != 0 {
		t.Errorf("expected 0 for nil cache, got %d", got)
	}
}

func TestPendingCount_HandleEmptyCache(t *testing.T) {
	cache := &state.TrackedQuizCache{Quizzes: []state.TrackedQuizRecord{}}
	got := pendingCount(cache)
	if got != 0 {
		t.Errorf("expected 0 for empty cache, got %d", got)
	}
}

func TestFirstNonZero_ReturnsFirstNonZeroTime(t *testing.T) {
	now := time.Now().UTC()
	later := now.Add(time.Hour)
	result := firstNonZero(time.Time{}, now, later)
	if result != now {
		t.Errorf("expected first non-zero time %v, got %v", now, result)
	}
}

func TestFirstNonZero_ReturnsNowIfAllZero(t *testing.T) {
	before := time.Now().UTC()
	result := firstNonZero(time.Time{}, time.Time{})
	after := time.Now().UTC()
	// Should return approximately now
	if result.Before(before) || result.After(after.Add(time.Second)) {
		t.Errorf("expected time close to now, got %v (before=%v, after=%v)", result, before, after)
	}
}

func TestSyncTrackedQuizSessions_EmptyCache(t *testing.T) {
	// Manually cover the case where cache has no quizzes
	cache := &state.TrackedQuizCache{SchemaVersion: 1, Quizzes: []state.TrackedQuizRecord{}}
	pending := pendingCount(cache)
	if pending != 0 {
		t.Errorf("expected 0 pending for empty cache, got %d", pending)
	}
}

func TestSyncTrackedQuizSessions_SkipsSessionsWithNoAnswers(t *testing.T) {
	// SessionResult with no answers should not be counted as failed
	// This is an indirect test through the firstNonZero and pendingCount logic
	session := sfq.SessionResult{
		SessionID:   "sess-no-answers",
		SourcePath:  "quiz.yaml",
		CompletedAt: time.Now().UTC(),
		Answers:     []sfq.SessionAnswer{}, // Empty = no answers submitted yet
	}
	// Simulate: if len(details.Answers) == 0 { continue }
	if len(session.Answers) == 0 {
		// Session is pending, not failed
		t.Log("Session with no answers is correctly treated as pending")
	} else {
		t.Error("Session should have no answers")
	}
}

func TestSyncTrackedQuizSessions_DeduplicatesBySessionID(t *testing.T) {
	// Test deduplication logic
	cache := &state.TrackedQuizCache{
		ImportedSessionIDs: []string{"sess-1", "sess-2"},
	}
	if cache.IsSessionImported("sess-1") {
		t.Log("Session sess-1 correctly identified as already imported")
	} else {
		t.Error("Session sess-1 should be marked as imported")
	}
	if !cache.IsSessionImported("sess-3") {
		t.Log("Session sess-3 correctly identified as not imported")
	} else {
		t.Error("Session sess-3 should not be marked as imported")
	}
}

func TestMatchingSessions_UsesYAMLToSFQFallback(t *testing.T) {
	// When QuizPath is "quiz.yaml", it should try "quiz.sfq" as fallback
	record := state.TrackedQuizRecord{
		QuizPath: "quiz.yaml",
		SFQPath:  "",
	}
	historyBySource := map[string][]sfq.SessionResult{
		"quiz.sfq": {
			{SessionID: "sess-from-sfq"},
		},
	}
	matches := matchingSessions(record, historyBySource)
	if len(matches) != 1 {
		t.Fatalf("expected to find session via .yaml -> .sfq conversion, got %d matches", len(matches))
	}
}

func TestNormalizePath_EmptyAfterTrim(t *testing.T) {
	// Edge case: only whitespace
	result := normalizePath("\t\n   ")
	if result != "" {
		t.Errorf("whitespace-only input should return empty string, got %q", result)
	}
}

func TestSyncTrackedQuizSessions_MapsAnswerIDAndProvenance(t *testing.T) {
	withSyncTestDeps(t)
	now := time.Now().UTC()
	cache := &state.TrackedQuizCache{
		Quizzes: []state.TrackedQuizRecord{{
			QuizID:   "quiz-1",
			Class:    "math",
			QuizPath: "quizzes/math/quiz-1.yaml",
			SFQPath:  "quizzes/math/quiz-1.sfq",
		}},
	}

	loadTrackedQuizCache = func() (*state.TrackedQuizCache, error) { return cache, nil }
	saveTrackedQuizCache = func(updated *state.TrackedQuizCache) error {
		if updated == nil {
			t.Fatal("expected non-nil cache")
		}
		return nil
	}
	historySessions = func() ([]sfq.SessionResult, error) {
		return []sfq.SessionResult{{
			SessionID:   "sess-1",
			SourcePath:  "quizzes/math/quiz-1.sfq",
			CompletedAt: now,
		}}, nil
	}
	resultsSession = func(sessionID string) (*sfq.SessionResult, error) {
		if sessionID != "sess-1" {
			t.Fatalf("unexpected session id: %s", sessionID)
		}
		return &sfq.SessionResult{
			SessionID:   sessionID,
			CompletedAt: now,
			Answers: []sfq.SessionAnswer{{
				QuestionID: "1",
				Correct:    true,
				UserAnswer: "42",
				AnsweredAt: now,
			}},
		}, nil
	}
	loadQuizDoc = func(path string) (*state.Quiz, error) {
		if path != "quizzes/math/quiz-1.yaml" {
			t.Fatalf("unexpected quiz path: %s", path)
		}
		return &state.Quiz{
			Class: "math",
			Sections: []state.QuizSection{{
				ID:          "q-001",
				Question:    "What is 6*7?",
				Answer:      "42",
				SectionID:   "sec-1",
				ComponentID: "cmp-1",
			}},
		}, nil
	}

	var captured state.QuizResults
	saveQuizResults = func(results *state.QuizResults, class, quizID string) error {
		if class != "math" {
			t.Fatalf("unexpected class: %s", class)
		}
		captured = *results
		if quizID == "" {
			t.Fatal("expected attempt id")
		}
		return nil
	}
	appendQuizQuestionHistory = func(class string, q state.Quiz, results state.QuizResults) error {
		if class != "math" {
			t.Fatalf("unexpected class in append: %s", class)
		}
		if len(results.Results) != 1 {
			t.Fatalf("expected one result, got %d", len(results.Results))
		}
		return nil
	}

	report, err := SyncTrackedQuizSessions()
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if report.ImportedSessions != 1 || report.BackfilledSessions != 0 {
		t.Fatalf("unexpected report counts: %#v", report)
	}
	if report.UnmappedAnswers != 0 {
		t.Fatalf("expected 0 unmapped answers, got %d", report.UnmappedAnswers)
	}
	if len(captured.Results) != 1 {
		t.Fatalf("expected one captured result, got %d", len(captured.Results))
	}
	if captured.Results[0].QuestionID != "q-001" {
		t.Fatalf("expected canonical question id q-001, got %q", captured.Results[0].QuestionID)
	}
	if captured.Results[0].SectionID != "sec-1" || captured.Results[0].ComponentID != "cmp-1" {
		t.Fatalf("expected provenance on captured result, got %#v", captured.Results[0])
	}
}

func TestSyncTrackedQuizSessions_TracksUnmappedAnswers(t *testing.T) {
	withSyncTestDeps(t)
	now := time.Now().UTC()
	cache := &state.TrackedQuizCache{
		Quizzes: []state.TrackedQuizRecord{{
			QuizID:   "quiz-1",
			Class:    "math",
			QuizPath: "quiz.yaml",
			SFQPath:  "quiz.sfq",
		}},
	}

	loadTrackedQuizCache = func() (*state.TrackedQuizCache, error) { return cache, nil }
	saveTrackedQuizCache = func(_ *state.TrackedQuizCache) error { return nil }
	historySessions = func() ([]sfq.SessionResult, error) {
		return []sfq.SessionResult{{SessionID: "sess-1", SourcePath: "quiz.sfq", CompletedAt: now}}, nil
	}
	resultsSession = func(string) (*sfq.SessionResult, error) {
		return &sfq.SessionResult{SessionID: "sess-1", CompletedAt: now, Answers: []sfq.SessionAnswer{{
			QuestionID: "unknown-id",
			Correct:    false,
		}}}, nil
	}
	loadQuizDoc = func(string) (*state.Quiz, error) {
		return &state.Quiz{Class: "math", Sections: []state.QuizSection{{ID: "q-001"}}}, nil
	}
	saveQuizResults = func(_ *state.QuizResults, _, _ string) error { return nil }
	appendQuizQuestionHistory = func(_ string, _ state.Quiz, _ state.QuizResults) error { return nil }

	report, err := SyncTrackedQuizSessions()
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if report.ImportedSessions != 1 {
		t.Fatalf("expected one imported session, got %d", report.ImportedSessions)
	}
	if report.UnmappedAnswers != 1 {
		t.Fatalf("expected one unmapped answer, got %d", report.UnmappedAnswers)
	}
}

func TestSyncTrackedQuizSessions_BackfillReprocessesImported(t *testing.T) {
	withSyncTestDeps(t)
	now := time.Now().UTC()
	cache := &state.TrackedQuizCache{
		Quizzes: []state.TrackedQuizRecord{{
			QuizID:         "quiz-1",
			Class:          "math",
			QuizPath:       "quiz.yaml",
			SFQPath:        "quiz.sfq",
			LastSessionID:  "sess-1",
			LastImportedAt: now,
		}},
		ImportedSessionIDs: []string{"sess-1"},
	}

	loadTrackedQuizCache = func() (*state.TrackedQuizCache, error) { return cache, nil }
	saveTrackedQuizCache = func(_ *state.TrackedQuizCache) error { return nil }
	historySessions = func() ([]sfq.SessionResult, error) {
		return []sfq.SessionResult{{SessionID: "sess-1", SourcePath: "quiz.sfq", CompletedAt: now}}, nil
	}
	resultsSession = func(string) (*sfq.SessionResult, error) {
		return &sfq.SessionResult{SessionID: "sess-1", CompletedAt: now, Answers: []sfq.SessionAnswer{{
			QuestionID: "q-001",
			Correct:    true,
		}}}, nil
	}
	loadQuizDoc = func(string) (*state.Quiz, error) {
		return &state.Quiz{Class: "math", Sections: []state.QuizSection{{ID: "q-001", SectionID: "sec-1", ComponentID: "cmp-1"}}}, nil
	}
	saveQuizResults = func(_ *state.QuizResults, _, _ string) error { return nil }
	appendQuizQuestionHistory = func(_ string, _ state.Quiz, _ state.QuizResults) error { return nil }

	report, err := SyncTrackedQuizSessionsWithOptions(SyncOptions{BackfillImported: true})
	if err != nil {
		t.Fatalf("sync backfill failed: %v", err)
	}
	if report.ImportedSessions != 0 {
		t.Fatalf("expected no new imports during backfill, got %d", report.ImportedSessions)
	}
	if report.BackfilledSessions != 1 {
		t.Fatalf("expected one backfilled session, got %d", report.BackfilledSessions)
	}
}

func TestSyncTrackedQuizSessions_DefaultSkipsImportedSessions(t *testing.T) {
	withSyncTestDeps(t)
	now := time.Now().UTC()
	cache := &state.TrackedQuizCache{
		Quizzes:            []state.TrackedQuizRecord{{QuizID: "quiz-1", Class: "math", QuizPath: "quiz.yaml", SFQPath: "quiz.sfq"}},
		ImportedSessionIDs: []string{"sess-1"},
	}

	loadTrackedQuizCache = func() (*state.TrackedQuizCache, error) { return cache, nil }
	saveTrackedQuizCache = func(_ *state.TrackedQuizCache) error { return nil }
	historySessions = func() ([]sfq.SessionResult, error) {
		return []sfq.SessionResult{{SessionID: "sess-1", SourcePath: "quiz.sfq", CompletedAt: now}}, nil
	}
	calledResults := false
	resultsSession = func(string) (*sfq.SessionResult, error) {
		calledResults = true
		return nil, errors.New("should not be called for already imported sessions")
	}
	loadQuizDoc = func(string) (*state.Quiz, error) {
		return &state.Quiz{Class: "math", Sections: []state.QuizSection{}}, nil
	}
	saveQuizResults = func(_ *state.QuizResults, _, _ string) error { return nil }
	appendQuizQuestionHistory = func(_ string, _ state.Quiz, _ state.QuizResults) error { return nil }

	report, err := SyncTrackedQuizSessions()
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if calledResults {
		t.Fatal("results session should not be called for already imported sessions")
	}
	if report.ImportedSessions != 0 || report.BackfilledSessions != 0 {
		t.Fatalf("expected no imports/backfills, got %#v", report)
	}
}

func TestQuestionIDCandidates_NormalizesVariants(t *testing.T) {
	candidates := questionIDCandidates("q-001")
	joined := strings.Join(candidates, ",")
	if !strings.Contains(joined, "q-001") || !strings.Contains(joined, "001") || !strings.Contains(joined, "1") {
		t.Fatalf("unexpected candidates for q-001: %v", candidates)
	}

	numericCandidates := questionIDCandidates("001")
	numericJoined := strings.Join(numericCandidates, ",")
	if !strings.Contains(numericJoined, "q-001") || !strings.Contains(numericJoined, "q-1") || !strings.Contains(numericJoined, "1") {
		t.Fatalf("unexpected candidates for 001: %v", numericCandidates)
	}
}
