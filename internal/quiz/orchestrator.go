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
		resp, err := provider.Generate(transcript)
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
