package tracking

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/repository"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
)

// SyncReport summarizes one tracked-session import run.
type SyncReport struct {
	ImportedSessions   int `json:"imported_sessions"`
	BackfilledSessions int `json:"backfilled_sessions"`
	FailedSessions     int `json:"failed_sessions"`
	PendingQuizzes     int `json:"pending_quizzes"`
	UnmappedAnswers    int `json:"unmapped_answers"`
}

// SyncOptions controls tracked-session import behavior.
type SyncOptions struct {
	// BackfillImported allows reprocessing already-imported sessions to
	// reconcile missing section/component question history.
	BackfillImported bool
}

// SyncService owns tracked-session sync dependencies.
// A service instance should be created at startup and reused.
type SyncService struct {
	store       repository.Store
	persistence syncPersistence
	external    syncExternal
}

type syncPersistence interface {
	LoadTrackedQuizCache() (*state.TrackedQuizCache, error)
	SaveTrackedQuizCache(*state.TrackedQuizCache) error
	SaveQuizResults(*state.QuizResults, string, string) error
	AppendQuizQuestionHistory(string, state.Quiz, state.QuizResults) error
}

type syncExternal interface {
	HistorySessions() ([]sfq.SessionResult, error)
	ResultsSession(string) (*sfq.SessionResult, error)
	LoadQuizDoc(string) (*state.Quiz, error)
}

type defaultSyncPersistence struct {
	store repository.Store
}

func (p defaultSyncPersistence) LoadTrackedQuizCache() (*state.TrackedQuizCache, error) {
	if p.store == nil {
		return loadTrackedQuizCache()
	}
	return p.store.QuizAttempts().LoadTrackedQuizCache()
}

func (p defaultSyncPersistence) SaveTrackedQuizCache(cache *state.TrackedQuizCache) error {
	if p.store == nil {
		return saveTrackedQuizCache(cache)
	}
	return p.store.QuizAttempts().SaveTrackedQuizCache(cache)
}

func (p defaultSyncPersistence) SaveQuizResults(results *state.QuizResults, class, quizID string) error {
	if p.store == nil {
		return saveQuizResults(results, class, quizID)
	}
	return p.store.QuizAttempts().SaveQuizResults(results, class, quizID)
}

func (p defaultSyncPersistence) AppendQuizQuestionHistory(class string, q state.Quiz, results state.QuizResults) error {
	if p.store == nil {
		return appendQuizQuestionHistory(class, q, results)
	}
	return p.store.QuizAttempts().AppendQuizQuestionHistory(class, q, results)
}

type defaultSyncExternal struct{}

func (defaultSyncExternal) HistorySessions() ([]sfq.SessionResult, error) {
	return historySessions()
}

func (defaultSyncExternal) ResultsSession(sessionID string) (*sfq.SessionResult, error) {
	return resultsSession(sessionID)
}

func (defaultSyncExternal) LoadQuizDoc(path string) (*state.Quiz, error) {
	return loadQuizDoc(path)
}

var (
	loadTrackedQuizCache = func() (*state.TrackedQuizCache, error) {
		return resolveStore(nil).QuizAttempts().LoadTrackedQuizCache()
	}
	saveTrackedQuizCache = func(cache *state.TrackedQuizCache) error {
		return resolveStore(nil).QuizAttempts().SaveTrackedQuizCache(cache)
	}
	historySessions = sfq.HistorySessions
	resultsSession  = sfq.ResultsSession
	loadQuizDoc     = quiz.LoadQuiz
	saveQuizResults = func(results *state.QuizResults, class, quizID string) error {
		return resolveStore(nil).QuizAttempts().SaveQuizResults(results, class, quizID)
	}
	appendQuizQuestionHistory = func(class string, q state.Quiz, results state.QuizResults) error {
		return resolveStore(nil).QuizAttempts().AppendQuizQuestionHistory(class, q, results)
	}
)

// NewSyncService constructs a sync service with injected persistence.
func NewSyncService(store repository.Store) *SyncService {
	var resolved repository.Store
	if store != nil {
		resolved = resolveStore(store)
	}
	svc := &SyncService{
		store:       resolved,
		persistence: defaultSyncPersistence{store: resolved},
		external:    defaultSyncExternal{},
	}

	return svc
}

// SyncTrackedQuizSessions imports unseen sfq tracked sessions into quiz
// results and section/component question history.
func SyncTrackedQuizSessions() (SyncReport, error) {
	return NewSyncService(nil).SyncTrackedQuizSessions()
}

// SyncTrackedQuizSessionsWithOptions imports tracked sessions with optional
// reconciliation of already-imported sessions.
func SyncTrackedQuizSessionsWithOptions(opts SyncOptions) (SyncReport, error) {
	return NewSyncService(nil).SyncTrackedQuizSessionsWithOptions(opts)
}

// SyncTrackedQuizSessionsWithStore imports tracked sessions using the provided
// repository store for persistence.
func SyncTrackedQuizSessionsWithStore(store repository.Store) (SyncReport, error) {
	return NewSyncService(store).SyncTrackedQuizSessions()
}

// SyncTrackedQuizSessionsWithOptionsAndStore imports tracked sessions with
// options using the provided persistence store.
func SyncTrackedQuizSessionsWithOptionsAndStore(opts SyncOptions, store repository.Store) (SyncReport, error) {
	return NewSyncService(store).SyncTrackedQuizSessionsWithOptions(opts)
}

// SyncTrackedQuizSessions imports unseen tracked sessions.
func (s *SyncService) SyncTrackedQuizSessions() (SyncReport, error) {
	return s.SyncTrackedQuizSessionsWithOptions(SyncOptions{})
}

// SyncTrackedQuizSessionsWithOptions imports tracked sessions with options.
func (s *SyncService) SyncTrackedQuizSessionsWithOptions(opts SyncOptions) (SyncReport, error) {
	report := SyncReport{}

	cache, err := s.persistence.LoadTrackedQuizCache()
	if err != nil {
		return report, err
	}
	if len(cache.Quizzes) == 0 {
		return report, nil
	}

	history, err := s.external.HistorySessions()
	if err != nil {
		return report, err
	}
	if len(history) == 0 {
		report.PendingQuizzes = pendingCount(cache)
		return report, nil
	}

	historyBySource := make(map[string][]sfq.SessionResult)
	for _, session := range history {
		normalized := normalizePath(session.SourcePath)
		if normalized == "" {
			continue
		}
		historyBySource[normalized] = append(historyBySource[normalized], session)
	}
	for source := range historyBySource {
		sort.Slice(historyBySource[source], func(i, j int) bool {
			return historyBySource[source][i].CompletedAt.Before(historyBySource[source][j].CompletedAt)
		})
	}

	for _, record := range cache.Quizzes {
		matches := matchingSessions(record, historyBySource)
		if len(matches) == 0 {
			continue
		}

		quizDoc, loadErr := s.external.LoadQuizDoc(record.QuizPath)
		if loadErr != nil {
			report.FailedSessions += len(matches)
			continue
		}
		sectionByQuestion := buildQuizSectionLookup(*quizDoc)
		quizClass := strings.TrimSpace(record.Class)
		if quizClass == "" {
			quizClass = strings.TrimSpace(quizDoc.Class)
		}
		if quizClass == "" {
			report.FailedSessions += len(matches)
			continue
		}

		for _, session := range matches {
			sessionID := strings.TrimSpace(session.SessionID)
			if sessionID == "" {
				continue
			}
			alreadyImported := cache.IsSessionImported(sessionID)
			if alreadyImported && !opts.BackfillImported {
				continue
			}

			details, detailsErr := s.external.ResultsSession(sessionID)
			if detailsErr != nil || details == nil {
				report.FailedSessions++
				continue
			}
			if len(details.Answers) == 0 {
				// If neither the results payload nor the history record carries a
				// completion timestamp, the session is still in-progress — leave pending.
				if details.CompletedAt.IsZero() && session.CompletedAt.IsZero() {
					continue
				}
				// Session is complete but all questions were skipped. Fall through
				// and import with empty results so it no longer shows as pending.
			}

			attemptID := record.QuizID + "-" + details.SessionID
			quizResults := state.QuizResults{
				QuizID:      attemptID,
				CompletedAt: firstNonZero(details.CompletedAt, time.Now().UTC()),
				Results:     make([]state.QuizResult, 0, len(details.Answers)),
			}
			for _, answer := range details.Answers {
				rawQuestionID := strings.TrimSpace(answer.QuestionID)
				quizSection, matched := matchQuizSection(rawQuestionID, sectionByQuestion)
				if !matched {
					report.UnmappedAnswers++
				}
				questionID := rawQuestionID
				if matched {
					questionID = quizSection.ID
				}
				quizResults.Results = append(quizResults.Results, state.QuizResult{
					QuestionID:  questionID,
					Correct:     answer.Correct,
					UserAnswer:  strings.TrimSpace(answer.UserAnswer),
					AnsweredAt:  firstNonZero(answer.AnsweredAt, details.CompletedAt),
					SectionID:   strings.TrimSpace(quizSection.SectionID),
					ComponentID: strings.TrimSpace(quizSection.ComponentID),
				})
			}

			if err := s.persistence.SaveQuizResults(&quizResults, quizClass, attemptID); err != nil {
				report.FailedSessions++
				continue
			}
			if err := s.persistence.AppendQuizQuestionHistory(quizClass, *quizDoc, quizResults); err != nil {
				report.FailedSessions++
				continue
			}
			if !alreadyImported {
				cache.MarkSessionImported(details.SessionID, record.QuizPath, firstNonZero(details.CompletedAt, time.Now().UTC()))
				report.ImportedSessions++
			} else {
				report.BackfilledSessions++
			}
		}
	}

	report.PendingQuizzes = pendingCount(cache)
	if err := s.persistence.SaveTrackedQuizCache(cache); err != nil {
		return report, fmt.Errorf("save tracked quiz cache: %w", err)
	}
	return report, nil
}

func buildQuizSectionLookup(quizDoc state.Quiz) map[string]state.QuizSection {
	lookup := make(map[string]state.QuizSection, len(quizDoc.Sections)*2)
	for _, section := range quizDoc.Sections {
		for _, key := range questionIDCandidates(section.ID) {
			if key == "" {
				continue
			}
			if _, exists := lookup[key]; !exists {
				lookup[key] = section
			}
		}
	}
	return lookup
}

func matchQuizSection(questionID string, lookup map[string]state.QuizSection) (state.QuizSection, bool) {
	for _, key := range questionIDCandidates(questionID) {
		section, ok := lookup[key]
		if ok {
			return section, true
		}
	}
	return state.QuizSection{}, false
}

func questionIDCandidates(questionID string) []string {
	normalized := strings.TrimSpace(questionID)
	// Strip surrounding quote characters that some sfq versions emit (e.g. `"q-001"`)
	normalized = strings.Trim(normalized, "\"'")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return nil
	}
	base := strings.ToLower(normalized)
	candidates := []string{base}
	if strings.HasPrefix(base, "q-") {
		rest := strings.TrimPrefix(base, "q-")
		candidates = append(candidates, rest)
		trimmedRest := strings.TrimLeft(rest, "0")
		if trimmedRest == "" {
			trimmedRest = "0"
		}
		candidates = append(candidates, trimmedRest)
	}
	if allDigits(base) {
		trimmed := strings.TrimLeft(base, "0")
		if trimmed == "" {
			trimmed = "0"
		}
		candidates = append(candidates, "q-"+base, "q-"+trimmed, trimmed)
	}
	seen := make(map[string]bool, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		unique = append(unique, candidate)
	}
	return unique
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func matchingSessions(record state.TrackedQuizRecord, historyBySource map[string][]sfq.SessionResult) []sfq.SessionResult {
	var matches []sfq.SessionResult
	candidates := []string{
		normalizePath(record.SFQPath),
		normalizePath(record.QuizPath),
		normalizePath(strings.TrimSuffix(record.QuizPath, ".yaml") + ".sfq"),
	}
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		matches = append(matches, historyBySource[candidate]...)
	}
	return matches
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	cleaned := filepath.Clean(path)
	cleaned = filepath.ToSlash(cleaned)
	return strings.ToLower(cleaned)
}

func pendingCount(cache *state.TrackedQuizCache) int {
	if cache == nil {
		return 0
	}
	pending := 0
	for _, record := range cache.Quizzes {
		if record.LastImportedAt.IsZero() {
			pending++
		}
	}
	return pending
}

func firstNonZero(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Now().UTC()
}

func resolveStore(store repository.Store) repository.Store {
	if store == nil {
		return repository.NewFilesystemStore()
	}
	return store
}
