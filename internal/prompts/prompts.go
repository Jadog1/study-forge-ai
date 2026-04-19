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

Output rules:
- Return plain YAML only.
- Do NOT use markdown code fences.
- Do NOT include any leading or trailing commentary.
- The first character of your response must be i from id:.
- Use ONLY the provided note text as the source of truth.
- Do NOT add facts, concepts, or tags that are not supported by the note text.
- If a detail is ambiguous, prefer a conservative summary over speculation.

Return ONLY valid YAML with this exact structure:
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

// StandardChatInstructions returns default chat behavior guidance.
func StandardChatInstructions() string {
	return `- Standard mode: answer clearly and directly while staying grounded in retrieved class material.
- If confidence is low, search first, then answer with citations to section/component context.
- Prefer concise explanations unless the user asks for depth.`
}

// SocraticTutorInstructions returns behavior guidance for Socratic tutoring.
func SocraticTutorInstructions() string {
	return `- Socratic tutor mode: lead with one guiding question at a time before revealing final conclusions.
- Encourage the learner to reason out loud and check assumptions with follow-up prompts.
- If the learner asks for direct help, provide a hint ladder (small hint -> stronger hint -> answer) instead of jumping to the full solution.
- Keep questions grounded in retrieved class sections/components and source material.`
}

// ExplainBackCoachInstructions returns behavior guidance for teach-back coaching.
func ExplainBackCoachInstructions() string {
	return `- Explain-back coach mode: prompt the learner to explain the concept in their own words first.
- Evaluate the explanation against retrieved material and respond with: strengths, gaps, and one focused next-step prompt.
- Flag misconceptions explicitly and suggest how to correct them using class terminology.
- Keep feedback specific and actionable; do not grade harshly when evidence is incomplete.`
}

// OrchestratorCandidate describes one knowledge component that the orchestrator
// LLM agent may choose to include in its quiz plan.
type OrchestratorCandidate struct {
	ComponentID          string   `json:"component_id"`
	SectionID            string   `json:"section_id"`
	SectionTitle         string   `json:"section_title"`
	SectionSummary       string   `json:"section_summary"`
	Kind                 string   `json:"kind"`
	Content              string   `json:"content"`
	Concepts             []string `json:"concepts"`
	Score                float64  `json:"score"`
	Attempts             int      `json:"attempts"`
	Accuracy             float64  `json:"accuracy"`
	RecentAccuracy       float64  `json:"recent_accuracy"`
	IncorrectStreak      int      `json:"incorrect_streak"`
	ThoughtProvokingRate float64  `json:"thought_provoking_rate"`
	DifficultyBand       string   `json:"difficulty_band"`
	DaysSince            float64  `json:"days_since"`
}

// RecentQuestionEntry is one past question shown to a component agent so it
// can avoid generating similar questions.
type RecentQuestionEntry struct {
	Question   string `json:"question"`
	Correct    bool   `json:"correct"`
	AnsweredAt string `json:"answered_at"`
}

// ComponentQuestionContext is everything a component question agent needs to
// generate questions for one specific knowledge component.
type ComponentQuestionContext struct {
	AssessmentKind   string
	ClassContext     string
	Class            string
	SectionID        string
	SectionTitle     string
	SectionSummary   string
	ComponentID      string
	ComponentKind    string
	ComponentContent string
	QuestionCount    int
	QuestionTypes    []string
	Angle            string
	DifficultyBand   string
	DifficultyGuide  string
	RecentHistory    []RecentQuestionEntry
}

// OrchestratorPrompt builds the prompt sent to the orchestrator LLM agent.
// The agent must return a JSON array of directives that assign question counts
// and types to specific components.
func OrchestratorPrompt(class, assessmentKind, classContext string, candidates []OrchestratorCandidate, totalCount int, typePreference, customContext string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are the quiz orchestrator for class: %s.\n", class)
	if strings.TrimSpace(assessmentKind) != "" {
		fmt.Fprintf(&b, "Assessment mode: %s\n", assessmentKind)
	}
	fmt.Fprintf(&b, "You must allocate exactly %d questions across the following knowledge components.\n\n", totalCount)
	fmt.Fprintf(&b, "Default question type: %s\n", typePreference)
	b.WriteString("Supported types: multiple-choice, multi-select, true-false, multi-true-false, short-answer, ordering\n\n")
	if strings.TrimSpace(classContext) != "" {
		b.WriteString("Class assessment context:\n")
		b.WriteString(classContext)
		b.WriteString("\n\n")
	}
	b.WriteString("Rules:\n")
	b.WriteString("- Source of truth: use ONLY the provided components, section summaries, concepts, and class assessment context.\n")
	b.WriteString("- Do NOT introduce external facts, definitions, or assumptions not grounded in that material.\n")
	b.WriteString("- If an angle would require outside knowledge, choose a different angle that stays grounded in the provided material.\n")
	b.WriteString("- If instructions conflict, grounding to provided material takes priority over creativity and additional instructions.\n")
	b.WriteString("- Prioritise components with high scores (weak, novel, or not seen recently).\n")
	b.WriteString("- Use difficulty_band and recent_accuracy to tune challenge per component:\n")
	b.WriteString("  supportive -> simpler, confidence-building checks with concrete contexts.\n")
	b.WriteString("  balanced -> mix straightforward and moderate reasoning.\n")
	b.WriteString("  advanced -> include cross-concept overlap, transfer, or thought-provoking prompts.\n")
	b.WriteString("- Avoid a monolithic tone. If recent thought-provoking rate is high, mix in at least one direct check.\n")
	b.WriteString("- You MAY choose a different question type per component if the content suits it better.\n")
	b.WriteString("- The sum of question_count across all directives MUST equal the requested total.\n")
	b.WriteString("- Assign an 'angle' hint to each directive as a short creative framing for the question writer.\n")
	b.WriteString("  Example angles: \"relate to everyday experience\", \"compare with <related concept>\",\n")
	b.WriteString("  \"explain why this matters\", \"predict what happens if...\", \"common misconception\",\n")
	b.WriteString("  \"connect to <other section>\", \"trace the cause-effect chain\", \"apply to a novel scenario\"\n")
	b.WriteString("- When two components share concepts or belong to the same section, consider comparative or connecting angles.\n\n")

	// Group candidates by section for richer context.
	type sectionGroup struct {
		Title      string
		Summary    string
		Candidates []OrchestratorCandidate
		Indices    []int // original 1-based indices
	}
	var sectionOrder []string
	groups := make(map[string]*sectionGroup)
	for i, c := range candidates {
		g, ok := groups[c.SectionID]
		if !ok {
			g = &sectionGroup{Title: c.SectionTitle, Summary: c.SectionSummary}
			groups[c.SectionID] = g
			sectionOrder = append(sectionOrder, c.SectionID)
		}
		g.Candidates = append(g.Candidates, c)
		g.Indices = append(g.Indices, i+1)
	}

	b.WriteString("Components (grouped by section, sorted by priority score desc):\n")
	for _, secID := range sectionOrder {
		g := groups[secID]
		fmt.Fprintf(&b, "\n── Section: %q", g.Title)
		if g.Summary != "" {
			summary := g.Summary
			if len(summary) > 200 {
				summary = summary[:197] + "..."
			}
			fmt.Fprintf(&b, "\n   Summary: %s", summary)
		}
		// Collect shared concepts across components in this section.
		conceptCounts := make(map[string]int)
		for _, c := range g.Candidates {
			for _, concept := range c.Concepts {
				conceptCounts[concept]++
			}
		}
		var shared []string
		for concept, count := range conceptCounts {
			if count > 1 {
				shared = append(shared, concept)
			}
		}
		if len(shared) > 0 {
			fmt.Fprintf(&b, "\n   Shared concepts: %s", strings.Join(shared, ", "))
		}
		b.WriteString("\n")

		for j, c := range g.Candidates {
			content := c.Content
			limit := contentLimitForKind(c.Kind)
			if len(content) > limit {
				content = content[:limit-3] + "..."
			}
			fmt.Fprintf(&b, "  [%d] component_id=%q kind=%s score=%.3f attempts=%d accuracy=%.0f%% recent_accuracy=%.0f%% incorrect_streak=%d thought_rate=%.0f%% difficulty=%s days_since=%.0f\n      %s\n",
				g.Indices[j], c.ComponentID, c.Kind, c.Score, c.Attempts, c.Accuracy*100, c.RecentAccuracy*100, c.IncorrectStreak, c.ThoughtProvokingRate*100, c.DifficultyBand, c.DaysSince, content)
		}
	}
	b.WriteString("\nRespond with ONLY a JSON array (no prose, no markdown fences):\n")
	b.WriteString("[\n  {\n    \"component_id\": \"<id>\",\n    \"section_id\": \"<id>\",\n    \"section_title\": \"<title>\",\n    \"question_count\": 1,\n    \"question_types\": [\"<type>\"],\n    \"angle\": \"<framing hint>\"\n  }\n]\n")
	if customContext != "" {
		b.WriteString("\nAdditional instructions:\n" + customContext + "\n")
	}
	return b.String()
}

// FocusedOrchestratorPrompt builds the prompt sent to the orchestrator LLM
// agent in focused quiz mode.  Unlike OrchestratorPrompt, this variant
// provides the full content of every candidate component (no truncation) and
// instructs the orchestrator to prioritise comprehensive coverage of the
// selected material rather than weakness-driven selection.  It is designed for
// hyper-focused studying immediately after reading specific sections.
func FocusedOrchestratorPrompt(class, classContext string, candidates []OrchestratorCandidate, totalCount int, typePreference, customContext string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are the focused quiz orchestrator for class: %s.\n", class)
	b.WriteString("Assessment mode: focused\n")
	fmt.Fprintf(&b, "The student has JUST studied the following material and needs to verify their understanding.\n")
	fmt.Fprintf(&b, "You must allocate exactly %d questions across the knowledge components below.\n\n", totalCount)
	fmt.Fprintf(&b, "Default question type: %s\n", typePreference)
	b.WriteString("Supported types: multiple-choice, multi-select, true-false, multi-true-false, short-answer, ordering\n\n")
	if strings.TrimSpace(classContext) != "" {
		b.WriteString("Class assessment context:\n")
		b.WriteString(classContext)
		b.WriteString("\n\n")
	}
	b.WriteString("Rules:\n")
	b.WriteString("- Source of truth: use ONLY the provided components, section summaries, concepts, and class assessment context.\n")
	b.WriteString("- Do NOT introduce external facts, definitions, or assumptions not grounded in that material.\n")
	b.WriteString("- If an angle would require outside knowledge, choose a different angle that stays grounded in the provided material.\n")
	b.WriteString("- If instructions conflict, grounding to provided material takes priority over creativity and additional instructions.\n")
	b.WriteString("- This is targeted review of recently studied material: treat ALL components as equally important.\n")
	b.WriteString("- Do NOT use accuracy or attempt history to deprioritise components — every item deserves coverage.\n")
	b.WriteString("- Balance your plan across three layers of understanding:\n")
	b.WriteString("  1. BIG PICTURE — overarching concepts, purpose, and how sections connect to each other.\n")
	b.WriteString("  2. KEY CONCEPTS — definitions, formulas, procedures, and core factual items.\n")
	b.WriteString("  3. SPECIFIC DETAILS — edge cases, exceptions, subtle distinctions, and application scenarios.\n")
	b.WriteString("- Allocate roughly one-third of questions to each layer, adjusting based on component density.\n")
	b.WriteString("- Prefer angles that test whether the student truly understands the material, not just surface recall.\n")
	b.WriteString("  Good angles: \"explain why\", \"distinguish between\", \"what happens if\", \"apply to a new scenario\",\n")
	b.WriteString("  \"common misconception\", \"trace the cause-effect chain\", \"connect component X with component Y\".\n")
	b.WriteString("- When multiple components share concepts, create at least one directive with a cross-component angle.\n")
	b.WriteString("- You MAY choose a different question type per component if the content suits it better.\n")
	b.WriteString("- The sum of question_count across all directives MUST equal the requested total.\n\n")

	// Group candidates by section, same as OrchestratorPrompt, but show full content.
	type sectionGroup struct {
		Title      string
		Summary    string
		Concepts   []string
		Candidates []OrchestratorCandidate
		Indices    []int
	}
	var sectionOrder []string
	groups := make(map[string]*sectionGroup)
	for i, c := range candidates {
		g, ok := groups[c.SectionID]
		if !ok {
			g = &sectionGroup{Title: c.SectionTitle, Summary: c.SectionSummary}
			groups[c.SectionID] = g
			sectionOrder = append(sectionOrder, c.SectionID)
		}
		g.Candidates = append(g.Candidates, c)
		g.Indices = append(g.Indices, i+1)
		// Accumulate distinct section-level concepts from components.
		for _, concept := range c.Concepts {
			found := false
			for _, existing := range g.Concepts {
				if strings.EqualFold(existing, concept) {
					found = true
					break
				}
			}
			if !found {
				g.Concepts = append(g.Concepts, concept)
			}
		}
	}

	b.WriteString("Material (full component content — no summaries truncated):\n")
	for _, secID := range sectionOrder {
		g := groups[secID]
		fmt.Fprintf(&b, "\n── Section: %q (id: %s)\n", g.Title, secID)
		if g.Summary != "" {
			fmt.Fprintf(&b, "   Summary: %s\n", g.Summary)
		}
		if len(g.Concepts) > 0 {
			fmt.Fprintf(&b, "   Concepts: %s\n", strings.Join(g.Concepts, ", "))
		}
		b.WriteString("\n")
		for j, c := range g.Candidates {
			fmt.Fprintf(&b, "  [%d] component_id=%q kind=%s\n      %s\n",
				g.Indices[j], c.ComponentID, c.Kind, c.Content)
		}
	}
	b.WriteString("\nRespond with ONLY a JSON array (no prose, no markdown fences):\n")
	b.WriteString("[\n  {\n    \"component_id\": \"<id>\",\n    \"section_id\": \"<id>\",\n    \"section_title\": \"<title>\",\n    \"question_count\": 1,\n    \"question_types\": [\"<type>\"],\n    \"angle\": \"<framing hint>\"\n  }\n]\n")
	if customContext != "" {
		b.WriteString("\nAdditional instructions:\n" + customContext + "\n")
	}
	return b.String()
}

// ComponentQuestionPrompt builds the prompt sent to an individual component
// question agent.  The agent must return a YAML list of QuizSection items.
func ComponentQuestionPrompt(ctx ComponentQuestionContext, customContext string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are a question writer for class: %s\n", ctx.Class)
	if strings.TrimSpace(ctx.AssessmentKind) != "" {
		fmt.Fprintf(&b, "Assessment mode: %s\n", ctx.AssessmentKind)
	}
	fmt.Fprintf(&b, "Section: %q (id: %s)\n", ctx.SectionTitle, ctx.SectionID)
	if ctx.SectionSummary != "" {
		fmt.Fprintf(&b, "Section summary: %s\n", ctx.SectionSummary)
	}
	fmt.Fprintf(&b, "Component: %s (id: %s, kind: %s)\n", ctx.ComponentContent, ctx.ComponentID, ctx.ComponentKind)
	fmt.Fprintf(&b, "\nGenerate exactly %d question(s).\n", ctx.QuestionCount)
	if len(ctx.QuestionTypes) > 0 {
		fmt.Fprintf(&b, "Preferred question type(s): %s\n", strings.Join(ctx.QuestionTypes, ", "))
	}
	if ctx.Angle != "" {
		fmt.Fprintf(&b, "Framing angle: %s\n", ctx.Angle)
	}
	if ctx.DifficultyBand != "" {
		fmt.Fprintf(&b, "Difficulty band: %s\n", ctx.DifficultyBand)
	}
	if ctx.DifficultyGuide != "" {
		fmt.Fprintf(&b, "Difficulty guidance: %s\n", ctx.DifficultyGuide)
	}
	if strings.TrimSpace(ctx.ClassContext) != "" {
		b.WriteString("\nClass assessment context:\n")
		b.WriteString(ctx.ClassContext)
		b.WriteString("\n")
	}
	if len(ctx.RecentHistory) > 0 {
		b.WriteString("\nRecent questions on this component (avoid repetition):\n")
		for _, h := range ctx.RecentHistory {
			mark := "x"
			if h.Correct {
				mark = "v"
			}
			fmt.Fprintf(&b, "  [%s] %s (%s)\n", mark, h.Question, h.AnsweredAt)
		}
	}
	b.WriteString("\nOutput rules:\n")
	b.WriteString("- Return ONLY a valid YAML list — the first character must be '-'.\n")
	b.WriteString("- Do NOT use markdown code fences.\n")
	b.WriteString("- Do NOT include any leading or trailing prose.\n")
	b.WriteString("- Source of truth: use ONLY the provided component content, section summary, and class assessment context.\n")
	b.WriteString("- Do NOT add outside facts, terminology, or examples unless they are directly supported by the provided material.\n")
	b.WriteString("- Every question, choice, hint, reasoning, and answer must be verifiable from the provided material.\n")
	b.WriteString("- If a requested framing is under-specified, rewrite it to remain grounded instead of guessing missing facts.\n")
	b.WriteString("- If instructions conflict, grounding to provided material takes priority over creativity and additional instructions.\n")
	b.WriteString("- Always set 'section_id' and 'component_id' from the values given above.\n")
	b.WriteString("- Adapt difficulty to the requested band and guidance while keeping questions answerable from provided context.\n")
	b.WriteString("- Mix cognitive styles when generating multiple questions; do not make every question thought-provoking.\n")
	b.WriteString("- For multiple-choice/multi-select: include a 'choices:' list with 'text:' and 'correct:' fields.\n")
	b.WriteString("- For any choice-based type (multiple-choice, multi-select, true-false, multi-true-false): vary answer positions and avoid fixed patterns like always making the first option true/correct.\n")
	b.WriteString("- For true-false/multi-true-false: use choices with 'correct: true/false'.\n")
	b.WriteString("- For ordering: list choices in correct order; set 'correct: true' for all.\n")
	b.WriteString("- For short-answer: put the answer text in the 'answer:' field.\n\n")
	b.WriteString("Example (multiple-choice):\n")
	b.WriteString("- id: q-001\n  type: multiple-choice\n  question: <question>\n  hint: <nudge>\n  reasoning: <why correct>\n")
	fmt.Fprintf(&b, "  section_id: %s\n  component_id: %s\n", ctx.SectionID, ctx.ComponentID)
	fmt.Fprintf(&b, "  tags:\n    - src_section:%s\n    - src_component:%s\n", ctx.SectionID, ctx.ComponentID)
	b.WriteString("  choices:\n    - text: <distractor>\n      correct: false\n    - text: <correct answer>\n      correct: true\n")
	if customContext != "" {
		b.WriteString("\nAdditional instructions:\n" + customContext + "\n")
	}
	return b.String()
}

// contentLimitForKind returns the maximum content preview length for a given
// component kind.  Short factual kinds (definition, formula, fact) use a
// tighter limit; richer kinds get more room so the orchestrator can craft
// better angle hints.
func contentLimitForKind(kind string) int {
	switch kind {
	case "definition", "formula", "fact":
		return 150
	case "concept", "procedure", "example":
		return 350
	default:
		return 200
	}
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

Output rules:
- Return plain YAML only.
- Do NOT use markdown code fences.
- Do NOT include any leading or trailing commentary.
- The first character of your response must be s from sections:.
- Always quote string values for title, summary, kind, and content using double quotes.
- Use ONLY the note summary and raw note excerpt as source material.
- Do NOT invent sections, components, tags, or concepts that are not supported by the provided notes.
- If a detail is ambiguous, preserve uncertainty rather than fabricating specificity.

Return ONLY valid YAML with this exact structure:
sections:
	- title: "<section title>"
		summary: "<section summary>"
    tags:
      - <tag>
    concepts:
      - <concept>
    components:
			- kind: "<formula|concept|definition|example|procedure|fact>"
				content: "<single granular learning item>"
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
Output rules:
- Return plain YAML only.
- Do NOT use markdown code fences.
- Do NOT include any leading or trailing commentary.
- The first character of your response must be d from decision:.
- Use ONLY the candidate and existing title/summary text provided here.
- Do NOT use external assumptions about topics beyond the provided text.

Return ONLY valid YAML:
decision: <merge|keep>
rationale: <one sentence>
`, candidateTitle, candidateSummary, existingTitle, existingSummary, extra)
}
