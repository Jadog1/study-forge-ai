package quiz

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/state"
)

func applyDirectiveDifficultyGuidance(directives []OrchestratorDirective, scoreByComponent map[string]ComponentScore) []OrchestratorDirective {
	if len(directives) == 0 || len(scoreByComponent) == 0 {
		return directives
	}
	out := make([]OrchestratorDirective, len(directives))
	for i, directive := range directives {
		out[i] = directive
		score, ok := scoreByComponent[directive.ComponentID]
		if !ok {
			continue
		}
		supplement := directiveDifficultySupplement(score)
		if supplement == "" {
			continue
		}
		if strings.TrimSpace(out[i].Angle) == "" {
			out[i].Angle = supplement
			continue
		}
		out[i].Angle = strings.TrimSpace(out[i].Angle) + "; " + supplement
	}
	return out
}

func directiveDifficultySupplement(score ComponentScore) string {
	switch score.DifficultyBand {
	case "supportive":
		return "difficulty:supportive; begin with simpler confidence-building checks and concrete examples"
	case "advanced":
		if score.ThoughtProvoking >= 0.60 {
			return "difficulty:advanced-mixed; include cross-concept transfer but mix in one direct check"
		}
		return "difficulty:advanced; increase challenge with cross-concept overlap and thought-provoking reasoning"
	default:
		return "difficulty:balanced; mix straightforward checks with moderate reasoning"
	}
}

func finalizeExplicitDirectives(directives []OrchestratorDirective, targetCount int, defaultType string) ([]OrchestratorDirective, error) {
	if len(directives) == 0 {
		return nil, fmt.Errorf("at least one directive is required")
	}
	out := make([]OrchestratorDirective, len(directives))
	remaining := targetCount
	unspecified := 0

	for i, directive := range directives {
		directive.ComponentID = strings.TrimSpace(directive.ComponentID)
		directive.SectionID = strings.TrimSpace(directive.SectionID)
		directive.SectionTitle = strings.TrimSpace(directive.SectionTitle)
		directive.Angle = strings.TrimSpace(directive.Angle)
		if directive.Angle == "" {
			directive.Angle = "check understanding"
		}
		if len(directive.QuestionTypes) == 0 {
			directive.QuestionTypes = []string{defaultType}
		}
		for j := range directive.QuestionTypes {
			directive.QuestionTypes[j] = strings.TrimSpace(directive.QuestionTypes[j])
		}
		if directive.QuestionCount < 0 {
			return nil, fmt.Errorf("directive %d has negative question_count", i+1)
		}
		if directive.QuestionCount == 0 {
			unspecified++
		} else {
			remaining -= directive.QuestionCount
		}
		out[i] = directive
	}

	if targetCount <= 0 {
		for i := range out {
			if out[i].QuestionCount <= 0 {
				out[i].QuestionCount = 1
			}
		}
		return out, nil
	}
	if remaining < 0 {
		return nil, fmt.Errorf("directive question_count exceeds requested total of %d", targetCount)
	}
	if unspecified == 0 {
		if remaining != 0 {
			return nil, fmt.Errorf("directive question_count totals %d but requested count is %d", targetCount-remaining, targetCount)
		}
		return out, nil
	}
	if remaining < unspecified {
		return nil, fmt.Errorf("requested count %d is too small for %d directive(s) with unspecified question_count", targetCount, unspecified)
	}

	for i := range out {
		if out[i].QuestionCount > 0 {
			continue
		}
		out[i].QuestionCount = 1
		remaining--
	}
	for i := range out {
		if remaining == 0 {
			break
		}
		out[i].QuestionCount++
		remaining--
	}
	return out, nil
}

func sumDirectiveQuestionCount(directives []OrchestratorDirective) int {
	total := 0
	for _, directive := range directives {
		if directive.QuestionCount > 0 {
			total += directive.QuestionCount
			continue
		}
		total++
	}
	return total
}

func cleanYAML(resp string) string {
	s := strings.TrimSpace(resp)
	if strings.HasPrefix(s, "```yaml") {
		s = strings.TrimPrefix(s, "```yaml")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func mergeCoverageContext(base string, scope *classpkg.CoverageScope, roster *classpkg.NoteRoster) string {
	if scope == nil || len(scope.Groups) == 0 {
		return base
	}
	var lines []string
	lines = append(lines, "Coverage priorities:")
	for i, group := range scope.Groups {
		patterns := classpkg.ResolveGroupPatterns(group, roster)
		detail := ""
		if len(patterns) > 0 {
			detail = strings.Join(patterns, ", ")
		} else if len(group.Tags) > 0 {
			detail = "tags=" + strings.Join(group.Tags, ",")
		} else {
			detail = "(no match hints)"
		}
		lines = append(lines, fmt.Sprintf("- group %d weight=%.2f :: %s", i+1, group.Weight, detail))
	}
	if scope.ExcludeUnmatched {
		lines = append(lines, "- unmatched material excluded")
	}
	coverageText := strings.Join(lines, "\n")
	if strings.TrimSpace(base) == "" {
		return coverageText
	}
	return strings.TrimSpace(base) + "\n\n" + coverageText
}

// deduplicateQuestionIDs assigns globally unique sequential IDs to all
// sections.  Each LLM component agent independently starts its ID counter
// from q-001, so collisions are guaranteed when sections are merged.
// IDs are reformatted as q-001, q-002, … after combining all agents' output.
func deduplicateQuestionIDs(sections []state.QuizSection) {
	for i := range sections {
		sections[i].ID = fmt.Sprintf("q-%03d", i+1)
	}
}

// rebalanceChoiceAnswerPositions spreads answer patterns across choice-based
// question types while preserving question correctness. This is deterministic
// so tests and quiz rendering are stable across runs.
func rebalanceChoiceAnswerPositions(sections []state.QuizSection) {
	for i := range sections {
		sec := &sections[i]
		if len(sec.Choices) < 2 {
			continue
		}

		switch sec.Type {
		case "multiple-choice", "true-false":
			rebalanceSingleCorrectChoice(sec)
		case "multi-select", "multi-true-false":
			sec.Choices = stableShuffleChoices(*sec, sec.Choices)
		case "ordering":
			continue
		default:
			continue
		}
	}
}

func rebalanceSingleCorrectChoice(sec *state.QuizSection) {
	if sec == nil || len(sec.Choices) < 2 {
		return
	}

	correctIdx := -1
	correctCount := 0
	for j, ch := range sec.Choices {
		if ch.Correct {
			correctCount++
			correctIdx = j
		}
	}
	if correctCount != 1 || correctIdx < 0 {
		return
	}

	targetIdx := stableChoiceTarget(*sec, len(sec.Choices))
	if targetIdx == correctIdx {
		return
	}
	sec.Choices = moveChoice(sec.Choices, correctIdx, targetIdx)
}

func stableShuffleChoices(sec state.QuizSection, choices []state.QuizChoice) []state.QuizChoice {
	if len(choices) < 2 {
		return choices
	}

	type rankedChoice struct {
		choice state.QuizChoice
		rank   uint32
		idx    int
	}
	ranked := make([]rankedChoice, len(choices))
	for i, ch := range choices {
		h := fnv.New32a()
		_, _ = h.Write([]byte(stableChoiceSeed(sec)))
		_, _ = h.Write([]byte("|" + strings.ToLower(strings.TrimSpace(ch.Text))))
		_, _ = h.Write([]byte(fmt.Sprintf("|%d", i)))
		ranked[i] = rankedChoice{choice: ch, rank: h.Sum32(), idx: i}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].rank == ranked[j].rank {
			return ranked[i].idx < ranked[j].idx
		}
		return ranked[i].rank < ranked[j].rank
	})
	out := make([]state.QuizChoice, len(choices))
	for i := range ranked {
		out[i] = ranked[i].choice
	}
	return out
}

func stableChoiceTarget(sec state.QuizSection, size int) int {
	if size <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(stableChoiceSeed(sec)))
	return int(h.Sum32() % uint32(size))
}

func stableChoiceSeed(sec state.QuizSection) string {
	return strings.ToLower(strings.TrimSpace(sec.Question)) +
		"|" + strings.TrimSpace(sec.SectionID) +
		"|" + strings.TrimSpace(sec.ComponentID) +
		"|" + strings.TrimSpace(sec.Type)
}

func moveChoice(choices []state.QuizChoice, from, to int) []state.QuizChoice {
	if from < 0 || to < 0 || from >= len(choices) || to >= len(choices) || from == to {
		return choices
	}
	moved := choices[from]
	if from < to {
		copy(choices[from:to], choices[from+1:to+1])
	} else {
		copy(choices[to+1:from+1], choices[to:from])
	}
	choices[to] = moved
	return choices
}

type recentQuizSignals struct {
	ComponentLastSeen map[string]time.Time
	QuestionKeys      map[string]bool
}

func loadRecentQuizSignals(class string, maxFiles int) (recentQuizSignals, error) {
	signals := recentQuizSignals{
		ComponentLastSeen: make(map[string]time.Time),
		QuestionKeys:      make(map[string]bool),
	}
	if maxFiles <= 0 {
		return signals, nil
	}

	quizDir, err := config.Path("quizzes", class)
	if err != nil {
		return signals, err
	}
	entries, err := os.ReadDir(quizDir)
	if err != nil {
		if os.IsNotExist(err) {
			return signals, nil
		}
		return signals, fmt.Errorf("read quiz directory: %w", err)
	}

	type quizFile struct {
		path    string
		modTime time.Time
	}
	files := make([]quizFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		files = append(files, quizFile{
			path:    filepath.Join(quizDir, entry.Name()),
			modTime: info.ModTime().UTC(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}

	for _, file := range files {
		q, loadErr := LoadQuiz(file.path)
		if loadErr != nil {
			continue
		}
		normalizeQuizProvenance(q)
		for _, sec := range q.Sections {
			componentID := strings.TrimSpace(sec.ComponentID)
			if componentID == "" {
				_, componentID = extractProvenanceFromTags(sec.Tags)
			}
			if componentID != "" {
				if last, ok := signals.ComponentLastSeen[componentID]; !ok || file.modTime.After(last) {
					signals.ComponentLastSeen[componentID] = file.modTime
				}
			}
			if key := normalizeQuestionKey(sec.Question); key != "" {
				signals.QuestionKeys[key] = true
			}
		}
	}

	return signals, nil
}

func applyRecentGenerationPenalty(scores []ComponentScore, recent map[string]time.Time, now time.Time, window time.Duration, maxPenalty float64) []ComponentScore {
	if len(scores) == 0 || len(recent) == 0 || window <= 0 || maxPenalty <= 0 {
		return scores
	}
	if maxPenalty > 0.95 {
		maxPenalty = 0.95
	}

	out := make([]ComponentScore, len(scores))
	copy(out, scores)

	windowSeconds := window.Seconds()
	for i := range out {
		componentID := strings.TrimSpace(out[i].Component.ID)
		seenAt, ok := recent[componentID]
		if !ok {
			continue
		}
		age := now.Sub(seenAt)
		if age < 0 {
			age = 0
		}
		if age >= window {
			continue
		}
		freshness := 1 - (age.Seconds() / windowSeconds)
		penalty := maxPenalty * freshness
		out[i].Score = out[i].Score * (1 - penalty)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
}

func filterDuplicateQuestionSections(sections []state.QuizSection, seen map[string]bool) []state.QuizSection {
	if len(sections) == 0 {
		return sections
	}
	if seen == nil {
		seen = make(map[string]bool)
	}

	out := make([]state.QuizSection, 0, len(sections))
	for _, sec := range sections {
		key := normalizeQuestionKey(sec.Question)
		if key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, sec)
	}
	return out
}

func normalizeQuestionKey(question string) string {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return ""
	}
	return strings.Join(strings.Fields(question), " ")
}

func hasAnyTag(noteTags, filter []string) bool {
	set := make(map[string]bool, len(noteTags))
	for _, t := range noteTags {
		set[strings.ToLower(t)] = true
	}
	for _, t := range filter {
		if set[strings.ToLower(t)] {
			return true
		}
	}
	return false
}

// focusedSectionMatch reports whether a component score belongs to a section
// that matches any of the targets by exact ID or case-insensitive title
// substring.
func focusedSectionMatch(cs ComponentScore, targets []string) bool {
	sectionID := strings.ToLower(strings.TrimSpace(cs.Section.ID))
	sectionTitle := strings.ToLower(strings.TrimSpace(cs.Section.Title))
	for _, t := range targets {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if t == sectionID || strings.Contains(sectionTitle, t) {
			return true
		}
	}
	return false
}
