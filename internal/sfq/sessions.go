package sfq

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SessionAnswer is one recorded answer from a tracked sfq session.
type SessionAnswer struct {
	QuestionID string
	Correct    bool
	UserAnswer string
	AnsweredAt time.Time
}

// SessionResult is normalized tracked-session data loaded from sfq.
type SessionResult struct {
	SessionID   string
	SourcePath  string
	CompletedAt time.Time
	Answers     []SessionAnswer
}

// HistorySessions loads session summaries from `sfq history --json`.
func HistorySessions() ([]SessionResult, error) {
	cmd := exec.Command("sfq", "history", "--json")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sfq history --json failed: %w\n%s", err, strings.TrimSpace(string(raw)))
	}
	return parseHistoryPayload(raw)
}

// ResultsSession loads detailed tracked results from `sfq results <id> --json`.
func ResultsSession(sessionID string) (*SessionResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	cmd := exec.Command("sfq", "results", sessionID, "--json")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sfq results %s --json failed: %w\n%s", sessionID, err, strings.TrimSpace(string(raw)))
	}
	return parseResultsPayload(sessionID, raw)
}

func parseHistoryPayload(raw []byte) ([]SessionResult, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, nil
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse sfq history json: %w", err)
	}
	objects := collectObjects(payload)
	out := make([]SessionResult, 0, len(objects))
	for _, obj := range objects {
		session := SessionResult{
			SessionID:   pickString(obj, "id", "session_id", "sessionId"),
			SourcePath:  pickString(obj, "source", "source_path", "source_file", "sourceFile", "quiz_path", "quizPath", "file", "path"),
			CompletedAt: pickTime(obj, "completed_at", "completedAt", "finished_at", "finishedAt", "ended_at", "endedAt", "created_at", "createdAt"),
		}
		if session.SessionID == "" {
			continue
		}
		out = append(out, session)
	}
	return out, nil
}

func parseResultsPayload(sessionID string, raw []byte) (*SessionResult, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, nil
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse sfq results json: %w", err)
	}

	obj, _ := payload.(map[string]any)
	if obj == nil {
		return &SessionResult{SessionID: sessionID}, nil
	}

	result := &SessionResult{
		SessionID: sessionID,
		SourcePath: firstNonEmpty(
			pickString(obj, "source", "source_path", "source_file", "sourceFile", "quiz_path", "quizPath", "file", "path"),
			pickNestedString(obj, "session", "source", "source_path", "source_file", "sourceFile", "quiz_path", "quizPath", "file", "path"),
		),
		CompletedAt: firstNonZeroTime(
			pickTime(obj, "completed_at", "completedAt", "finished_at", "finishedAt", "ended_at", "endedAt", "created_at", "createdAt"),
			pickNestedTime(obj, "session", "completed_at", "completedAt", "finished_at", "finishedAt", "ended_at", "endedAt", "created_at", "createdAt"),
		),
	}

	for _, answerObj := range extractAnswers(obj) {
		questionID := firstNonEmpty(
			pickString(answerObj, "question_id", "questionId", "qid", "id"),
			pickNestedString(answerObj, "question", "id", "question_id", "questionId", "qid"),
		)
		if strings.TrimSpace(questionID) == "" {
			continue
		}
		correct, ok := pickBool(answerObj, "correct", "is_correct", "isCorrect", "right")
		if !ok {
			correct = false
		}
		result.Answers = append(result.Answers, SessionAnswer{
			QuestionID: strings.TrimSpace(questionID),
			Correct:    correct,
			UserAnswer: firstNonEmpty(
				pickString(answerObj, "user_answer", "userAnswer", "answer", "response", "value", "submitted", "submitted_answer", "submittedAnswer"),
				pickNestedString(answerObj, "response", "value", "text", "answer"),
			),
			AnsweredAt: firstNonZeroTime(
				pickTime(answerObj, "answered_at", "answeredAt", "at", "timestamp", "created_at", "createdAt"),
				pickNestedTime(answerObj, "response", "answered_at", "answeredAt", "at", "timestamp", "created_at", "createdAt"),
			),
		})
	}

	return result, nil
}

func extractAnswers(obj map[string]any) []map[string]any {
	keys := []string{"answers", "results", "responses", "questions", "items", "entries"}
	for _, key := range keys {
		if arr := objectArray(obj[key]); len(arr) > 0 {
			return arr
		}
	}
	if sessionObj, _ := obj["session"].(map[string]any); sessionObj != nil {
		for _, key := range keys {
			if arr := objectArray(sessionObj[key]); len(arr) > 0 {
				return arr
			}
		}
	}
	return nil
}

func collectObjects(payload any) []map[string]any {
	if arr, ok := payload.([]any); ok {
		return objectArray(arr)
	}
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range []string{"sessions", "history", "items", "results", "data"} {
		if arr := objectArray(obj[key]); len(arr) > 0 {
			return arr
		}
	}
	return []map[string]any{obj}
}

func objectArray(value any) []map[string]any {
	arr, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if ok {
			out = append(out, obj)
		}
	}
	return out
}

func pickString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		}
	}
	return ""
}

func pickNestedString(obj map[string]any, root string, keys ...string) string {
	nested, ok := obj[root].(map[string]any)
	if !ok {
		return ""
	}
	return pickString(nested, keys...)
}

func pickTime(obj map[string]any, keys ...string) time.Time {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok {
			continue
		}
		if parsed := parseTimeValue(value); !parsed.IsZero() {
			return parsed
		}
	}
	return time.Time{}
}

func pickNestedTime(obj map[string]any, root string, keys ...string) time.Time {
	nested, ok := obj[root].(map[string]any)
	if !ok {
		return time.Time{}
	}
	return pickTime(nested, keys...)
}

func parseTimeValue(value any) time.Time {
	switch typed := value.(type) {
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(layout, strings.TrimSpace(typed)); err == nil {
				return parsed.UTC()
			}
		}
	case float64:
		if typed > 0 {
			return time.Unix(int64(typed), 0).UTC()
		}
	}
	return time.Time{}
}

func pickBool(obj map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			lower := strings.ToLower(strings.TrimSpace(typed))
			switch lower {
			case "true", "yes", "y", "1", "correct", "right":
				return true, true
			case "false", "no", "n", "0", "incorrect", "wrong":
				return false, true
			}
		case float64:
			return typed != 0, true
		}
	}
	return false, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Time{}
}
