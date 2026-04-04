package quiz

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/prompts"
	"github.com/studyforge/study-agent/plugins"
)

// OrchestratorDirective is one assignment from the orchestrator to a
// per-component question agent: which component, how many questions, what
// type(s) to use, and from which angle to approach the content.
type OrchestratorDirective struct {
	ComponentID   string   `json:"component_id"`
	SectionID     string   `json:"section_id"`
	SectionTitle  string   `json:"section_title"`
	QuestionCount int      `json:"question_count"`
	QuestionTypes []string `json:"question_types"`
	Angle         string   `json:"angle"`
}

// runOrchestratorAgent submits the scored candidate list to an LLM that
// decides which components to quiz, how many questions each, what types to
// use, and from which angle.  Returns a list of OrchestratorDirectives.
func runOrchestratorAgent(
	class string,
	assessmentKind string,
	classContext string,
	candidates []ComponentScore,
	totalCount int,
	typePreference string,
	provider plugins.AIProvider,
	cfg *config.Config,
	onProgress func(ProgressEvent),
) ([]OrchestratorDirective, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no knowledge components found for class %q — run 'sfa ingest' first", class)
	}

	// Build OrchestratorCandidate list for the prompt.
	pCandidates := make([]prompts.OrchestratorCandidate, len(candidates))
	for i, s := range candidates {
		pCandidates[i] = prompts.OrchestratorCandidate{
			ComponentID:          s.Component.ID,
			SectionID:            s.Section.ID,
			SectionTitle:         s.Section.Title,
			SectionSummary:       s.Section.Summary,
			Kind:                 s.Component.Kind,
			Content:              s.Component.Content,
			Concepts:             s.Component.Concepts,
			Score:                s.Score,
			Attempts:             s.Attempts,
			Accuracy:             s.Accuracy,
			RecentAccuracy:       s.RecentAccuracy,
			DifficultyBand:       s.DifficultyBand,
			ThoughtProvokingRate: s.ThoughtProvoking,
			IncorrectStreak:      s.IncorrectStreak,
			DaysSince:            s.DaysSinceAttempt,
		}
	}

	prompt := prompts.OrchestratorPrompt(class, assessmentKind, classContext, pCandidates, totalCount, typePreference, cfg.CustomPromptContext)

	if onProgress != nil {
		onProgress(ProgressEvent{Label: "Orchestrator", Detail: "Selecting components to quiz"})
	}

	const maxSteps = 4
	transcript := prompt
	for step := 0; step < maxSteps; step++ {
		resp, err := generateWithQuizUsage(provider, transcript, quizOperationOrchestrator, class, cfg)
		if err != nil {
			return nil, fmt.Errorf("orchestrator agent: %w", err)
		}

		directives, parseErr := parseOrchestratorResponse(resp)
		if parseErr != nil {
			transcript += fmt.Sprintf(
				"\n\nYour response could not be parsed as JSON:\n%v\n\nRespond with ONLY a valid JSON array of directive objects.",
				parseErr,
			)
			continue
		}
		if len(directives) == 0 {
			transcript += "\n\nThe directive list is empty. Return at least one directive.\n"
			continue
		}

		directives = normalizeDirectiveCount(directives, totalCount)

		if onProgress != nil {
			onProgress(ProgressEvent{Label: "Orchestrator", Detail: fmt.Sprintf("%d directive(s) planned", len(directives)), Done: true})
		}
		return directives, nil
	}
	return nil, fmt.Errorf("orchestrator agent exceeded %d steps without producing valid output", maxSteps)
}

// runFocusedOrchestratorAgent is like runOrchestratorAgent but designed for
// focused quiz mode.  It exposes the full content of every candidate component
// (no truncation) and instructs the orchestrator to plan comprehensive coverage
// across big-picture, key-concept, and specific-detail layers rather than
// prioritising weak components.
func runFocusedOrchestratorAgent(
	class string,
	classContext string,
	candidates []ComponentScore,
	totalCount int,
	typePreference string,
	provider plugins.AIProvider,
	cfg *config.Config,
	onProgress func(ProgressEvent),
) ([]OrchestratorDirective, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no knowledge components found for class %q — run 'sfa ingest' first", class)
	}

	// Build candidate list for the prompt with full component content.
	pCandidates := make([]prompts.OrchestratorCandidate, len(candidates))
	for i, s := range candidates {
		pCandidates[i] = prompts.OrchestratorCandidate{
			ComponentID:    s.Component.ID,
			SectionID:      s.Section.ID,
			SectionTitle:   s.Section.Title,
			SectionSummary: s.Section.Summary,
			Kind:           s.Component.Kind,
			Content:        s.Component.Content, // full content, no truncation
			Concepts:       s.Component.Concepts,
		}
	}

	prompt := prompts.FocusedOrchestratorPrompt(class, classContext, pCandidates, totalCount, typePreference, cfg.CustomPromptContext)

	if onProgress != nil {
		onProgress(ProgressEvent{Label: "Orchestrator", Detail: "Planning focused coverage"})
	}

	const maxSteps = 4
	transcript := prompt
	for step := 0; step < maxSteps; step++ {
		resp, err := generateWithQuizUsage(provider, transcript, quizOperationOrchestrator, class, cfg)
		if err != nil {
			return nil, fmt.Errorf("focused orchestrator agent: %w", err)
		}

		directives, parseErr := parseOrchestratorResponse(resp)
		if parseErr != nil {
			transcript += fmt.Sprintf(
				"\n\nYour response could not be parsed as JSON:\n%v\n\nRespond with ONLY a valid JSON array of directive objects.",
				parseErr,
			)
			continue
		}
		if len(directives) == 0 {
			transcript += "\n\nThe directive list is empty. Return at least one directive.\n"
			continue
		}

		directives = normalizeDirectiveCount(directives, totalCount)

		if onProgress != nil {
			onProgress(ProgressEvent{Label: "Orchestrator", Detail: fmt.Sprintf("%d directive(s) planned", len(directives)), Done: true})
		}
		return directives, nil
	}
	return nil, fmt.Errorf("focused orchestrator agent exceeded %d steps without producing valid output", maxSteps)
}

func parseOrchestratorResponse(resp string) ([]OrchestratorDirective, error) {
	s := strings.TrimSpace(resp)
	// Strip markdown fences.
	if idx := strings.Index(s, "```json"); idx != -1 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	}
	s = strings.TrimSpace(s)

	// Find JSON array boundaries.
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	s = s[start : end+1]

	var directives []OrchestratorDirective
	if err := json.Unmarshal([]byte(s), &directives); err != nil {
		return nil, fmt.Errorf("unmarshal directives: %w", err)
	}
	return directives, nil
}

// normalizeDirectiveCount scales question counts so the total equals target.
// At least 1 question is assigned per directive.
func normalizeDirectiveCount(directives []OrchestratorDirective, target int) []OrchestratorDirective {
	if len(directives) == 0 || target <= 0 {
		return directives
	}
	total := 0
	for _, d := range directives {
		total += d.QuestionCount
	}
	if total == target {
		return directives
	}

	out := make([]OrchestratorDirective, len(directives))
	copy(out, directives)

	if total == 0 {
		perDir := target / len(out)
		if perDir < 1 {
			perDir = 1
		}
		for i := range out {
			out[i].QuestionCount = perDir
		}
		return out
	}

	assigned := 0
	for i := range out {
		if i == len(out)-1 {
			// Last directive gets the remainder to ensure the sum is exact.
			count := target - assigned
			if count < 1 {
				count = 1
			}
			out[i].QuestionCount = count
		} else {
			count := int(math.Round(float64(out[i].QuestionCount) / float64(total) * float64(target)))
			if count < 1 {
				count = 1
			}
			out[i].QuestionCount = count
		}
		assigned += out[i].QuestionCount
	}
	return out
}

// expandCompoundDirectives splits directives where component_id is provided as
// a comma-separated list, a pattern some providers occasionally return.
// Question counts are distributed across expanded directives, then callers can
// re-normalize totals with normalizeDirectiveCount.
func expandCompoundDirectives(directives []OrchestratorDirective) []OrchestratorDirective {
	out := make([]OrchestratorDirective, 0, len(directives))
	for _, d := range directives {
		componentIDs := splitDirectiveList(d.ComponentID)
		if len(componentIDs) <= 1 {
			d.ComponentID = strings.TrimSpace(d.ComponentID)
			d.SectionID = strings.TrimSpace(d.SectionID)
			out = append(out, d)
			continue
		}

		sectionIDs := splitDirectiveList(d.SectionID)
		limit := len(componentIDs)
		if d.QuestionCount > 0 && d.QuestionCount < limit {
			limit = d.QuestionCount
		}
		expandedIDs := componentIDs[:limit]
		counts := distributeDirectiveCount(d.QuestionCount, len(expandedIDs))
		for i, componentID := range expandedIDs {
			expanded := d
			expanded.ComponentID = componentID
			if len(counts) == len(expandedIDs) {
				expanded.QuestionCount = counts[i]
			}
			switch {
			case len(sectionIDs) == len(componentIDs):
				expanded.SectionID = sectionIDs[i]
			case len(sectionIDs) == 1:
				expanded.SectionID = sectionIDs[0]
			default:
				expanded.SectionID = strings.TrimSpace(expanded.SectionID)
			}
			out = append(out, expanded)
		}
	}
	return out
}

func splitDirectiveList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" {
			return []string{trimmed}
		}
	}
	return out
}

func distributeDirectiveCount(total, parts int) []int {
	if parts <= 0 {
		return nil
	}
	out := make([]int, parts)
	if total <= 0 {
		for i := range out {
			out[i] = 1
		}
		return out
	}
	base := total / parts
	rem := total % parts
	for i := range out {
		out[i] = base
		if rem > 0 {
			out[i]++
			rem--
		}
		if out[i] < 1 {
			out[i] = 1
		}
	}
	return out
}
