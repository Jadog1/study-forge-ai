// Package prompts provides pre-built, parameterised prompt templates used
// across all AI interactions. Keeping templates here keeps them consistent and
// easy to iterate on without touching business logic.
package prompts

import (
	"fmt"
	"strings"
)

// SummarizeNote builds a prompt that instructs the model to summarise a raw
// note and return structured YAML metadata.
func SummarizeNote(content, class, customContext string) string {
	var parts []string

	if class != "" {
		parts = append(parts, fmt.Sprintf("This note belongs to class: %s", class))
	}
	if customContext != "" {
		parts = append(parts, "Additional instructions:\n"+customContext)
	}

	header := strings.Join(parts, "\n")
	if header != "" {
		header = "\n" + header
	}

	return fmt.Sprintf(`You are a study assistant. Analyse the following notes and return a structured YAML response.%s

Notes:
---
%s
---

Return ONLY valid YAML with this exact structure (no markdown code fences, no extra text):
id: <url-friendly slug derived from the content>
summary: <concise 2-3 sentence summary>
tags:
  - <tag1>
  - <tag2>
concepts:
  - <concept1>
  - <concept2>
`, header, content)
}

// GenerateQuestions builds a prompt that produces a complete quiz YAML document
// from a set of note summaries.
//
// weakAreas lists topic tags the student has previously struggled with; the
// model is asked to give them extra coverage.
func GenerateQuestions(summaries []string, class, customContext string, numQuestions int, weakAreas []string) string {
	notesBlock := buildNotesBlock(summaries)

	var extras []string
	if len(weakAreas) > 0 {
		extras = append(extras, fmt.Sprintf("Focus extra attention on these weak areas: %s", strings.Join(weakAreas, ", ")))
	}
	if customContext != "" {
		extras = append(extras, "Additional instructions:\n"+customContext)
	}
	extra := ""
	if len(extras) > 0 {
		extra = "\n" + strings.Join(extras, "\n")
	}

	return fmt.Sprintf(`You are a study assistant generating a quiz for class: %s.
Generate exactly %d questions that test conceptual understanding, not rote memorisation.
Prefer questions that require reasoning, real-world application, or analogy.%s

Study material:
%s

Return ONLY valid YAML (no markdown code fences, no extra text) using this exact structure:
title: <descriptive quiz title>
class: %s
tags:
  - <tag>
sections:
  - type: question
    id: q-001
    question: <question text>
    hint: <helpful nudge without giving the answer away>
    answer: <clear, complete answer>
    reasoning: <explanation of why this answer is correct — deeper insight>
    tags:
      - <tag>
`, class, numQuestions, extra, notesBlock, class)
}

// AdaptQuestions builds a prompt for follow-up adaptive questions focused on
// areas where the student has shown weakness.
func AdaptQuestions(class string, weakAreas, pastQuestions []string, customContext string) string {
	var extras []string
	if customContext != "" {
		extras = append(extras, "Additional instructions:\n"+customContext)
	}

	pastBlock := ""
	if len(pastQuestions) > 0 {
		pastBlock = "\nAvoid repeating these exact questions:\n"
		for _, q := range pastQuestions {
			pastBlock += fmt.Sprintf("  - %s\n", q)
		}
	}

	extra := strings.Join(extras, "\n")
	if extra != "" {
		extra = "\n" + extra
	}

	return fmt.Sprintf(`You are a study assistant. A student studying %s has shown weakness in: %s.
Generate 5 new questions that address these weak areas from different angles than before.
Use real-world analogies where helpful. Focus on conceptual understanding.%s%s

Return ONLY valid YAML (no markdown code fences, no extra text):
title: Adaptive Review – %s
class: %s
tags:
  - adaptive
  - review
sections:
  - type: question
    id: q-001
    question: <question>
    hint: <hint>
    answer: <answer>
    reasoning: <reasoning>
    tags:
      - <tag>
`, class, strings.Join(weakAreas, ", "), extra, pastBlock, class, class)
}

// VariationQuestion creates a reframed version of an existing question that
// tests the same underlying concept from a new angle.
func VariationQuestion(originalQuestion, concept, customContext string) string {
	extra := ""
	if customContext != "" {
		extra = "\nAdditional instructions:\n" + customContext
	}

	return fmt.Sprintf(`Reframe the following quiz question to test the same concept (%s) from a different angle or scenario.
Keep the difficulty level roughly the same.%s

Original question: %s

Return ONLY valid YAML (no markdown code fences):
type: question
id: <new-unique-id>
question: <new question text>
hint: <hint>
answer: <answer>
reasoning: <reasoning>
tags:
  - <tag>
`, concept, extra, originalQuestion)
}

// ComposeKnowledge asks the model to decompose a note into learning sections
// and granular components.
func ComposeKnowledge(noteSummary, noteContent, class, sourcePath, customContext string) string {
	var contextParts []string
	if class != "" {
		contextParts = append(contextParts, "Class: "+class)
	}
	if sourcePath != "" {
		contextParts = append(contextParts, "Source path: "+sourcePath)
	}
	if customContext != "" {
		contextParts = append(contextParts, "Additional instructions:\n"+customContext)
	}

	ctx := strings.Join(contextParts, "\n")
	if ctx != "" {
		ctx += "\n\n"
	}

	return fmt.Sprintf(`You are composing study knowledge units from notes.
%sNote summary:
---
%s
---

Raw note excerpt:
---
%s
---

Return ONLY valid YAML with this exact structure:
sections:
  - title: <section title>
    summary: <section summary>
    tags:
      - <tag>
    concepts:
      - <concept>
    components:
      - kind: <formula|concept|definition|example|procedure|fact>
        content: <single granular learning item>
        tags:
          - <tag>
        concepts:
          - <concept>
`, ctx, noteSummary, noteContent)
}

// ReviewConsolidation asks the model whether to merge a candidate with an
// existing knowledge unit.
func ReviewConsolidation(candidateTitle, candidateSummary, existingTitle, existingSummary, customContext string) string {
	extra := ""
	if customContext != "" {
		extra = "\nAdditional instructions:\n" + customContext + "\n"
	}

	return fmt.Sprintf(`You are reviewing two learning units for consolidation.
Candidate:
title: %s
summary: %s

Existing:
title: %s
summary: %s
%s
Return ONLY valid YAML:
decision: <merge|keep>
rationale: <one sentence>
`, candidateTitle, candidateSummary, existingTitle, existingSummary, extra)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildNotesBlock(summaries []string) string {
	var sb strings.Builder
	for i, s := range summaries {
		sb.WriteString(fmt.Sprintf("--- Note %d ---\n%s\n", i+1, s))
	}
	return sb.String()
}
