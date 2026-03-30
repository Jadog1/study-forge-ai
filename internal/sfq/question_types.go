package sfq

import "strings"

var supportedQuestionTypes = []string{
	"multiple-choice",
	"multi-select",
	"true-false",
	"multi-true-false",
	"short-answer",
	"ordering",
}

// SupportedQuestionTypes returns the canonical sfq question type keys.
func SupportedQuestionTypes() []string {
	out := make([]string, len(supportedQuestionTypes))
	copy(out, supportedQuestionTypes)
	return out
}

// IsSupportedQuestionType reports whether t is a recognized sfq type.
func IsSupportedQuestionType(t string) bool {
	needle := strings.ToLower(strings.TrimSpace(t))
	for _, item := range supportedQuestionTypes {
		if item == needle {
			return true
		}
	}
	return false
}

// NormalizeQuestionType normalizes a type key and falls back when invalid.
func NormalizeQuestionType(t, fallback string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if IsSupportedQuestionType(t) {
		return t
	}
	fallback = strings.ToLower(strings.TrimSpace(fallback))
	if IsSupportedQuestionType(fallback) {
		return fallback
	}
	return supportedQuestionTypes[0]
}
