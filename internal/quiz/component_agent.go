package quiz

import (
	"fmt"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/prompts"
	"github.com/studyforge/study-agent/internal/repository"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/plugins"
	"gopkg.in/yaml.v3"
)

// componentSectionsDoc is a fallback YAML wrapper when the AI wraps its list
// under a top-level "sections:" key instead of returning a bare list.
type componentSectionsDoc struct {
	Sections []state.QuizSection `yaml:"sections"`
}

// runComponentQuestionAgent generates quiz questions for one component based
// on an OrchestratorDirective.  It runs an agentic retry loop of up to
// maxComponentSteps, parsing a YAML list of QuizSection items.
func runComponentQuestionAgent(
	assessmentKind string,
	classContext string,
	dir OrchestratorDirective,
	score ComponentScore,
	provider plugins.AIProvider,
	cfg *config.Config,
	usageRepo repository.UsageRepository,
	onProgress func(ProgressEvent),
) ([]state.QuizSection, error) {
	ctx := buildComponentQuestionContext(assessmentKind, classContext, dir, score)
	prompt := prompts.ComponentQuestionPrompt(ctx, cfg.CustomPromptContext)

	label := "Questions"
	if score.Component.Content != "" {
		label = "Questions: " + truncateCmpContent(score.Component.Content, 30)
	}
	if onProgress != nil {
		onProgress(ProgressEvent{Label: label, Detail: fmt.Sprintf("%d question(s)", dir.QuestionCount)})
	}

	const maxComponentSteps = 4
	transcript := prompt
	for step := 0; step < maxComponentSteps; step++ {
		resp, err := generateWithQuizUsage(provider, transcript, quizOperationComponent, score.Component.Class, cfg, usageRepo)
		if err != nil {
			return nil, fmt.Errorf("component question agent: %w", err)
		}

		sections, parseErr := parseComponentSections(resp)
		if parseErr != nil {
			transcript += fmt.Sprintf(
				"\n\nYour response could not be parsed as YAML:\n%v\n\nRespond with ONLY a valid YAML list of question objects. The first character must be '-'.",
				parseErr,
			)
			continue
		}
		if len(sections) == 0 {
			transcript += fmt.Sprintf(
				"\n\nNo questions found. Generate exactly %d question(s). The response must begin with '- id:'.\n",
				dir.QuestionCount,
			)
			continue
		}

		// Backfill provenance from the directive when the AI omits it.
		for i := range sections {
			if sections[i].SectionID == "" {
				sections[i].SectionID = dir.SectionID
			}
			if sections[i].ComponentID == "" {
				sections[i].ComponentID = dir.ComponentID
			}
		}

		if onProgress != nil {
			onProgress(ProgressEvent{Label: label, Detail: fmt.Sprintf("%d question(s)", len(sections)), Done: true})
		}
		return sections, nil
	}
	return nil, fmt.Errorf("component question agent exceeded max steps for component %q", dir.ComponentID)
}

func buildComponentQuestionContext(assessmentKind, classContext string, dir OrchestratorDirective, score ComponentScore) prompts.ComponentQuestionContext {
	recent := make([]prompts.RecentQuestionEntry, 0, len(score.RecentHistory))
	for _, h := range score.RecentHistory {
		recent = append(recent, prompts.RecentQuestionEntry{
			Question:   h.Question,
			Correct:    h.Correct,
			AnsweredAt: h.AnsweredAt.UTC().Format(time.DateOnly),
		})
	}
	return prompts.ComponentQuestionContext{
		AssessmentKind:   assessmentKind,
		ClassContext:     classContext,
		Class:            score.Component.Class,
		SectionID:        dir.SectionID,
		SectionTitle:     dir.SectionTitle,
		SectionSummary:   score.Section.Summary,
		ComponentID:      dir.ComponentID,
		ComponentKind:    score.Component.Kind,
		ComponentContent: score.Component.Content,
		QuestionCount:    dir.QuestionCount,
		QuestionTypes:    dir.QuestionTypes,
		Angle:            dir.Angle,
		DifficultyBand:   score.DifficultyBand,
		DifficultyGuide:  componentDifficultyGuide(score),
		RecentHistory:    recent,
	}
}

func componentDifficultyGuide(score ComponentScore) string {
	switch score.DifficultyBand {
	case "supportive":
		return "Start with direct, confidence-building prompts, concrete wording, and one-step reasoning before any stretch prompts."
	case "advanced":
		if score.ThoughtProvoking >= 0.60 {
			return "Use higher-order transfer and overlap across related concepts, but include at least one straightforward check for pacing variety."
		}
		return "Increase challenge using cross-concept overlap, explanation, prediction, or scenario-based reasoning."
	default:
		return "Balance direct recall/application with moderate reasoning; vary the cognitive style across questions."
	}
}

func parseComponentSections(resp string) ([]state.QuizSection, error) {
	cleaned := cleanYAML(resp)

	// Try direct list first — AI returns `- id: q-001\n  type: ...`
	trimmed := strings.TrimSpace(cleaned)
	if strings.HasPrefix(trimmed, "-") {
		var sections []state.QuizSection
		if err := yaml.Unmarshal([]byte(cleaned), &sections); err == nil && len(sections) > 0 {
			return sections, nil
		}
	}

	// Fallback: try wrapper document with a "sections:" key.
	var doc componentSectionsDoc
	if err := yaml.Unmarshal([]byte(cleaned), &doc); err == nil && len(doc.Sections) > 0 {
		return doc.Sections, nil
	}

	// Last attempt: unmarshal as list regardless of leading character.
	var sections []state.QuizSection
	if err := yaml.Unmarshal([]byte(cleaned), &sections); err != nil {
		return nil, fmt.Errorf("could not parse sections: %w", err)
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("empty sections list")
	}
	return sections, nil
}

func truncateCmpContent(content string, maxLen int) string {
	content = strings.TrimSpace(content)
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}
