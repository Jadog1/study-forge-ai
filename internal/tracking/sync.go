package tracking

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
)

// SyncReport summarizes one tracked-session import run.
type SyncReport struct {
	ImportedSessions int
	FailedSessions   int
	PendingQuizzes   int
}

// SyncTrackedQuizSessions imports unseen sfq tracked sessions into quiz
// results and section/component question history.
func SyncTrackedQuizSessions() (SyncReport, error) {
	report := SyncReport{}

	cache, err := state.LoadTrackedQuizCache()
	if err != nil {
		return report, err
	}
	if len(cache.Quizzes) == 0 {
		return report, nil
	}

	history, err := sfq.HistorySessions()
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

		quizDoc, loadErr := quiz.LoadQuiz(record.QuizPath)
		if loadErr != nil {
			report.FailedSessions += len(matches)
			continue
		}
		quizClass := strings.TrimSpace(record.Class)
		if quizClass == "" {
			quizClass = strings.TrimSpace(quizDoc.Class)
		}
		if quizClass == "" {
			report.FailedSessions += len(matches)
			continue
		}

		for _, session := range matches {
			if strings.TrimSpace(session.SessionID) == "" || cache.IsSessionImported(session.SessionID) {
				continue
			}

			details, detailsErr := sfq.ResultsSession(session.SessionID)
			if detailsErr != nil || details == nil {
				report.FailedSessions++
				continue
			}
			if len(details.Answers) == 0 {
				// Session exists but no submitted answers yet; keep it pending.
				continue
			}

			attemptID := record.QuizID + "-" + details.SessionID
			quizResults := state.QuizResults{
				QuizID:      attemptID,
				CompletedAt: firstNonZero(details.CompletedAt, time.Now().UTC()),
				Results:     make([]state.QuizResult, 0, len(details.Answers)),
			}
			for _, answer := range details.Answers {
				quizResults.Results = append(quizResults.Results, state.QuizResult{
					QuestionID: strings.TrimSpace(answer.QuestionID),
					Correct:    answer.Correct,
					UserAnswer: strings.TrimSpace(answer.UserAnswer),
					AnsweredAt: firstNonZero(answer.AnsweredAt, details.CompletedAt),
				})
			}

			if err := state.SaveQuizResults(&quizResults, quizClass, attemptID); err != nil {
				report.FailedSessions++
				continue
			}
			if err := state.AppendQuizQuestionHistory(quizClass, *quizDoc, quizResults); err != nil {
				report.FailedSessions++
				continue
			}
			cache.MarkSessionImported(details.SessionID, record.QuizPath, firstNonZero(details.CompletedAt, time.Now().UTC()))
			report.ImportedSessions++
		}
	}

	report.PendingQuizzes = pendingCount(cache)
	if err := state.SaveTrackedQuizCache(cache); err != nil {
		return report, fmt.Errorf("save tracked quiz cache: %w", err)
	}
	return report, nil
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
