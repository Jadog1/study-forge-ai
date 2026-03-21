package sfq

import (
	"strings"
	"testing"
	"time"
)

// TestParseHistoryPayload_EmptyPayload tests handling of empty/null input.
func TestParseHistoryPayload_EmptyPayload(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"null", "null"},
		{"whitespace", "   \n   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseHistoryPayload([]byte(tt.input))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != nil {
				t.Errorf("expected nil for empty payload, got %#v", result)
			}
		})
	}
}

// TestParseHistoryPayload_InvalidJSON rejects malformed JSON.
func TestParseHistoryPayload_InvalidJSON(t *testing.T) {
	_, err := parseHistoryPayload([]byte("{invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse sfq history json") {
		t.Errorf("expected parse error message, got: %v", err)
	}
}

// TestParseHistoryPayload_ArrayOfSessions parses direct array of sessions.
func TestParseHistoryPayload_ArrayOfSessions(t *testing.T) {
	payload := `[
		{"id": "sess-1", "source_path": "quiz1.yaml", "completed_at": "2026-01-01T12:00:00Z"},
		{"id": "sess-2", "source": "quiz2.yaml"}
	]`
	result, err := parseHistoryPayload([]byte(payload))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(result))
	}
	if result[0].SessionID != "sess-1" {
		t.Errorf("expected session ID sess-1, got %q", result[0].SessionID)
	}
	if result[1].SourcePath != "quiz2.yaml" {
		t.Errorf("expected source quiz2.yaml, got %q", result[1].SourcePath)
	}
}

// TestParseHistoryPayload_SessionsInObject parses sessions nested in object.
func TestParseHistoryPayload_SessionsInObject(t *testing.T) {
	payload := `{
		"sessions": [
			{"id": "sess-1", "source": "quiz.yaml"},
			{"session_id": "sess-2", "source_file": "another.yaml"}
		]
	}`
	result, err := parseHistoryPayload([]byte(payload))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 sessions in nested object, got %d", len(result))
	}
	if result[0].SessionID != "sess-1" {
		t.Errorf("expected sess-1, got %q", result[0].SessionID)
	}
	if result[1].SessionID != "sess-2" {
		t.Errorf("expected sess-2, got %q", result[1].SessionID)
	}
}

// TestParseHistoryPayload_SkipsSessionsWithoutID ignores sessions missing ID.
func TestParseHistoryPayload_SkipsSessionsWithoutID(t *testing.T) {
	payload := `[
		{"id": "sess-1", "source": "quiz.yaml"},
		{"source": "quiz2.yaml"}
	]`
	result, err := parseHistoryPayload([]byte(payload))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 session (other was skipped), got %d", len(result))
	}
}

// TestParseHistoryPayload_AlternativeFieldNames tests multiple schema variants.
func TestParseHistoryPayload_AlternativeFieldNames(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		expectID  string
		expectSrc string
	}{
		{
			"snake_case session fields",
			`[{"session_id": "s1", "source_path": "src.yaml"}]`,
			"s1",
			"src.yaml",
		},
		{
			"camelCase session fields",
			`[{"sessionId": "s2", "sourceFile": "src2.yaml"}]`,
			"s2",
			"src2.yaml",
		},
		{
			"mixed case variants",
			`[{"id": "s3", "quiz_path": "src3.yaml"}]`,
			"s3",
			"src3.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseHistoryPayload([]byte(tt.payload))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if len(result) != 1 {
				t.Fatalf("expected 1 session, got %d", len(result))
			}
			if result[0].SessionID != tt.expectID {
				t.Errorf("expected ID %q, got %q", tt.expectID, result[0].SessionID)
			}
			if result[0].SourcePath != tt.expectSrc {
				t.Errorf("expected source %q, got %q", tt.expectSrc, result[0].SourcePath)
			}
		})
	}
}

// TestParseResultsPayload_EmptyPayload handles empty/null results.
func TestParseResultsPayload_EmptyPayload(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"null", "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseResultsPayload("session-1", []byte(tt.input))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			// parseResultsPayload returns nil for empty/null payloads
			if result != nil {
				t.Errorf("expected nil result for empty payload, got %#v", result)
			}
		})
	}
}

// TestParseResultsPayload_InvalidJSON rejects malformed JSON.
func TestParseResultsPayload_InvalidJSON(t *testing.T) {
	_, err := parseResultsPayload("s1", []byte("{broken"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse sfq results json") {
		t.Errorf("got unexpected error: %v", err)
	}
}

// TestParseResultsPayload_WithAnswers extracts answers in multiple schema formats.
func TestParseResultsPayload_WithAnswers(t *testing.T) {
	payload := `{
		"answers": [
			{
				"question_id": "q1",
				"correct": true,
				"user_answer": "correct answer",
				"answered_at": "2026-01-01T12:00:00Z"
			},
			{
				"questionId": "q2",
				"isCorrect": false,
				"userAnswer": "wrong",
				"answeredAt": "2026-01-01T12:05:00Z"
			}
		]
	}`
	result, err := parseResultsPayload("sess-1", []byte(payload))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(result.Answers) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(result.Answers))
	}
	if result.Answers[0].QuestionID != "q1" {
		t.Errorf("expected q1, got %q", result.Answers[0].QuestionID)
	}
	if result.Answers[0].Correct != true {
		t.Error("expected answer 0 to be correct")
	}
	if result.Answers[1].QuestionID != "q2" {
		t.Errorf("expected q2, got %q", result.Answers[1].QuestionID)
	}
	if result.Answers[1].Correct != false {
		t.Error("expected answer 1 to be incorrect")
	}
}

// TestParseResultsPayload_NestedAnswers extracts answers from nested session object.
func TestParseResultsPayload_NestedAnswers(t *testing.T) {
	payload := `{
		"session": {
			"completed_at": "2026-01-01T12:00:00Z",
			"source_path": "quiz.yaml",
			"answers": [
				{"question_id": "q1", "correct": true}
			]
		}
	}`
	result, err := parseResultsPayload("sess-1", []byte(payload))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(result.Answers) != 1 {
		t.Fatalf("expected 1 answer from nested session, got %d", len(result.Answers))
	}
	if result.Answers[0].QuestionID != "q1" {
		t.Errorf("expected q1, got %q", result.Answers[0].QuestionID)
	}
}

// TestParseResultsPayload_SkipsAnswersWithoutQuestionID ignores incomplete answers.
func TestParseResultsPayload_SkipsAnswersWithoutQuestionID(t *testing.T) {
	payload := `{
		"answers": [
			{"question_id": "q1", "correct": true},
			{"correct": true},
			{"question_id": "q2", "correct": false}
		]
	}`
	result, err := parseResultsPayload("sess-1", []byte(payload))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(result.Answers) != 2 {
		t.Fatalf("expected 2 answers (middle skipped), got %d", len(result.Answers))
	}
}

// TestParseResultsPayload_AlternativeAnswerArrayNames searches multiple keys.
func TestParseResultsPayload_AlternativeAnswerArrayNames(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			"results key",
			`{"results": [{"question_id": "q1", "correct": true}]}`,
		},
		{
			"responses key",
			`{"responses": [{"question_id": "q1", "correct": true}]}`,
		},
		{
			"items key",
			`{"items": [{"question_id": "q1", "correct": true}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseResultsPayload("sess-1", []byte(tt.payload))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if len(result.Answers) != 1 {
				t.Fatalf("expected 1 answer, got %d", len(result.Answers))
			}
		})
	}
}

// TestParseTimeValue_RFC3339Formats parses various time formats.
func TestParseTimeValue_RFC3339Formats(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		hasError bool
	}{
		{
			"RFC3339Nano string",
			"2026-01-01T12:30:45.123456789Z",
			false,
		},
		{
			"RFC3339 string",
			"2026-01-01T12:30:45Z",
			false,
		},
		{
			"standard datetime string",
			"2026-01-01 12:30:45",
			false,
		},
		{
			"date only string",
			"2026-01-01",
			false,
		},
		{
			"unix timestamp (float64)",
			float64(1704110445),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTimeValue(tt.value)
			if tt.hasError && !result.IsZero() {
				t.Errorf("expected zero time, got %v", result)
			}
			if !tt.hasError && result.IsZero() {
				t.Errorf("expected non-zero time for %#v", tt.value)
			}
		})
	}
}

// TestParseTimeValue_InvalidFormats rejects unparseable times.
func TestParseTimeValue_InvalidFormats(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"invalid string", "not-a-time"},
		{"empty string", ""},
		{"zero float64", float64(0)},
		{"negative float64", float64(-1)},
		{"wrong type", map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTimeValue(tt.value)
			if !result.IsZero() {
				t.Errorf("expected zero time for invalid input, got %v", result)
			}
		})
	}
}

// TestPickBool_VariantStrings handles boolean string representations.
func TestPickBool_VariantStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected bool
		found    bool
	}{
		{
			"true variants",
			map[string]any{"correct": "yes"},
			true, true,
		},
		{
			"true: y",
			map[string]any{"is_correct": "Y"},
			true, true,
		},
		{
			"true: 1",
			map[string]any{"right": "1"},
			true, true,
		},
		{
			"true: correct",
			map[string]any{"val": "correct"},
			true, true,
		},
		{
			"false: no",
			map[string]any{"correct": "no"},
			false, true,
		},
		{
			"false: 0",
			map[string]any{"is_correct": "0"},
			false, true,
		},
		{
			"false: wrong",
			map[string]any{"val": "wrong"},
			false, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k := range tt.input {
				result, found := pickBool(tt.input, k)
				if found != tt.found {
					t.Errorf("expected found=%v, got %v", tt.found, found)
				}
				if found && result != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

// TestPickBool_NumericBooleans handles numeric boolean values.
func TestPickBool_NumericBooleans(t *testing.T) {
	obj := map[string]any{
		"true_val":  float64(1),
		"false_val": float64(0),
		"truth":     float64(123), // any non-zero
	}
	truVal, found := pickBool(obj, "true_val")
	if !found || !truVal {
		t.Errorf("expected true for 1.0")
	}
	falseVal, found := pickBool(obj, "false_val")
	if !found || falseVal {
		t.Errorf("expected false for 0.0")
	}
	truthVal, found := pickBool(obj, "truth")
	if !found || !truthVal {
		t.Errorf("expected true for any non-zero float")
	}
}

// TestPickBool_ActualBoolType handles native bool values.
func TestPickBool_ActualBoolType(t *testing.T) {
	obj := map[string]any{
		"native_true":  true,
		"native_false": false,
	}
	trueVal, found := pickBool(obj, "native_true")
	if !found || !trueVal {
		t.Errorf("expected true for native bool true")
	}
	falseVal, found := pickBool(obj, "native_false")
	if !found || falseVal {
		t.Errorf("expected false for native bool false")
	}
}

// TestFirstNonEmpty_ReturnsFirstNonEmptyString tests the firstNonEmpty helper.
func TestFirstNonEmpty_ReturnsFirstNonEmptyString(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []string
		expected string
	}{
		{"first is empty", []string{"", "second"}, "second"},
		{"first is whitespace", []string{"  \n", "default"}, "default"},
		{"first is valid", []string{"first", "second"}, "first"},
		{"all empty", []string{"", "  ", ""}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstNonEmpty(tt.inputs...)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestFirstNonZeroTime_ReturnsFirstNonZeroTime tests the firstNonZeroTime helper.
func TestFirstNonZeroTime_ReturnsFirstNonZeroTime(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	result := firstNonZeroTime(time.Time{}, now, future)
	if result != now {
		t.Errorf("expected %v, got %v", now, result)
	}
	result = firstNonZeroTime(time.Time{}, time.Time{})
	if !result.IsZero() {
		t.Errorf("expected zero time when all inputs are zero, got %v", result)
	}
}

// TestExtractAnswers_TriesMultipleKeys finds answers under various keys.
func TestExtractAnswers_TriesMultipleKeys(t *testing.T) {
	tests := []struct {
		name          string
		obj           map[string]any
		expectedCount int
	}{
		{
			"answers key",
			map[string]any{"answers": []any{map[string]any{"question_id": "q1"}}},
			1,
		},
		{
			"results key",
			map[string]any{"results": []any{map[string]any{"question_id": "q1"}}},
			1,
		},
		{
			"nested session.answers",
			map[string]any{"session": map[string]any{"answers": []any{
				map[string]any{"question_id": "q1"},
			}}},
			1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAnswers(tt.obj)
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d answers, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

// TestCollectObjects_HandlesVariousPayloadFormats processes different JSON structures.
func TestCollectObjects_HandlesVariousPayloadFormats(t *testing.T) {
	tests := []struct {
		name          string
		payload       any
		expectedCount int
	}{
		{
			"direct array",
			[]any{
				map[string]any{"id": "1"},
				map[string]any{"id": "2"},
			},
			2,
		},
		{
			"object with sessions key",
			map[string]any{"sessions": []any{
				map[string]any{"id": "1"},
			}},
			1,
		},
		{
			"object with history key",
			map[string]any{"history": []any{
				map[string]any{"id": "1"},
				map[string]any{"id": "2"},
			}},
			2,
		},
		{
			"single object",
			map[string]any{"id": "single"},
			1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collectObjects(tt.payload)
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d objects, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

// TestObjectArray_FiltersNonObjectItems excludes non-objects from arrays.
func TestObjectArray_FiltersNonObjectItems(t *testing.T) {
	input := []any{
		map[string]any{"id": "1"},
		"not an object",
		map[string]any{"id": "2"},
		42,
		map[string]any{"id": "3"},
	}
	result := objectArray(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 objects (non-objects filtered), got %d", len(result))
	}
	if result[0]["id"] != "1" || result[1]["id"] != "2" || result[2]["id"] != "3" {
		t.Error("expected objects to be in order")
	}
}

// TestPickString_TriesMultipleKeys tries keys in fallback order.
func TestPickString_TriesMultipleKeys(t *testing.T) {
	obj := map[string]any{
		"second_choice": "preferred",
	}
	result := pickString(obj, "first_choice", "second_choice", "third_choice")
	if result != "preferred" {
		t.Errorf("expected preferred, got %q", result)
	}
}

// TestPickString_IgnoresWhitespaceOnlyStrings skips empty/whitespace strings.
func TestPickString_IgnoresWhitespaceOnlyStrings(t *testing.T) {
	obj := map[string]any{
		"empty":      "",
		"whitespace": "   \n   ",
		"real":       "value",
	}
	result := pickString(obj, "empty", "whitespace", "real")
	if result != "value" {
		t.Errorf("expected value, got %q", result)
	}
}

// TestPickString_WrongTypeIgnored skips non-string values.
func TestPickString_WrongTypeIgnored(t *testing.T) {
	obj := map[string]any{
		"number": 42,
		"bool":   true,
		"string": "correct",
		"array":  []string{"a", "b"},
	}
	result := pickString(obj, "number", "bool", "string", "array")
	if result != "correct" {
		t.Errorf("expected correct, got %q", result)
	}
}

// TestPickNestedString_AccessesNestedFields retrieves values from nested objects.
func TestPickNestedString_AccessesNestedFields(t *testing.T) {
	obj := map[string]any{
		"session": map[string]any{
			"source": "nested.yaml",
		},
	}
	result := pickNestedString(obj, "session", "source")
	if result != "nested.yaml" {
		t.Errorf("expected nested.yaml, got %q", result)
	}
}

// TestPickNestedString_HandlesMissingNesting returns empty when nesting missing.
func TestPickNestedString_HandlesMissingNesting(t *testing.T) {
	obj := map[string]any{
		"other": "value",
	}
	result := pickNestedString(obj, "session", "source")
	if result != "" {
		t.Errorf("expected empty string for missing nesting, got %q", result)
	}
}

// TestJSONParseRoundtrip_ComplexSchema handles deeply nested real-world-like JSON.
func TestJSONParseRoundtrip_ComplexSchema(t *testing.T) {
	payload := `{
		"meta": {"version": "1.0"},
		"session": {
			"id": "complex-sess",
			"source_path": "complex/quiz.yaml",
			"completed_at": "2026-01-01T12:00:00Z",
			"responses": [
				{
					"question": {"id": "q1", "text": "What is X?"},
					"response": {
						"value": "answer to q1",
						"answered_at": "2026-01-01T12:01:00Z"
					},
					"isCorrect": true
				},
				{
					"question": {"id": "q2"},
					"response": {
						"text": "answer to q2",
						"timestamp": "2026-01-01T12:02:00Z"
					},
					"correct": "no"
				}
			]
		},
		"status": "completed"
	}`

	result, err := parseResultsPayload("complex-sess", []byte(payload))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if result.SessionID != "complex-sess" {
		t.Errorf("expected sess ID complex-sess, got %q", result.SessionID)
	}
	if result.SourcePath != "complex/quiz.yaml" {
		t.Errorf("expected source complex/quiz.yaml, got %q", result.SourcePath)
	}
	if len(result.Answers) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(result.Answers))
	}
	if result.Answers[0].QuestionID != "q1" {
		t.Errorf("expected q1, got %q", result.Answers[0].QuestionID)
	}
	if result.Answers[0].Correct != true {
		t.Error("expected answer 0 to be correct")
	}
	if result.Answers[1].QuestionID != "q2" {
		t.Errorf("expected q2, got %q", result.Answers[1].QuestionID)
	}
	if result.Answers[1].Correct != false {
		t.Error("expected answer 1 to be incorrect")
	}
}
