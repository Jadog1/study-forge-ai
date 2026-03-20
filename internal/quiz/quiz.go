// Package quiz handles quiz generation, adaptive quiz creation, and loading
// quiz YAML files from disk.
package quiz

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/plugins"
	"gopkg.in/yaml.v3"
)

// ProgressEvent carries a quiz-generation progress update emitted during the
// agent loop so callers can show live tool-call feedback.
type ProgressEvent struct {
	Label  string
	Detail string
	Done   bool
	Err    error
}

// Generate creates a new quiz for class, optionally filtered to notes matching
// tags. It saves the quiz under ~/.study-forge-ai/quizzes/<class>/ and returns the path.
func Generate(class string, tags []string, provider plugins.AIProvider, cfg *config.Config) (*state.Quiz, string, error) {
	return GenerateStream(class, tags, provider, cfg, nil)
}

// GenerateStream is like Generate but calls onProgress with agent step events
// so callers can show live tool-call feedback in the UI.
func GenerateStream(class string, tags []string, provider plugins.AIProvider, cfg *config.Config, onProgress func(ProgressEvent)) (*state.Quiz, string, error) {
	idx, err := state.LoadNotesIndex()
	if err != nil {
		return nil, "", fmt.Errorf("load notes index: %w", err)
	}

	relevant := filterNotes(idx.Notes, class, tags)
	if len(relevant) == 0 {
		return nil, "", fmt.Errorf("no notes found for class %q with tags %v — run 'sfa ingest' first", class, tags)
	}

	summaries := make([]string, len(relevant))
	for i, n := range relevant {
		summaries[i] = fmt.Sprintf("Summary: %s\nConcepts: %s\nTags: %s",
			n.Summary, strings.Join(n.Concepts, ", "), strings.Join(n.Tags, ", "))
	}

	weakAreas := weakAreasForClass(class)
	knowledgeCtx := knowledgeContextForClass(class)
	return runQuizAgent(class, "quiz", provider, cfg, summaries, 5, weakAreas, nil, knowledgeCtx, onProgress)
}

// Adapt generates performance-driven follow-up questions for class based on
// previously stored results. Returns an error if no results exist yet.
func Adapt(class string, provider plugins.AIProvider, cfg *config.Config) (*state.Quiz, string, error) {
	return AdaptStream(class, provider, cfg, nil)
}

// AdaptStream is like Adapt but calls onProgress with agent step events.
func AdaptStream(class string, provider plugins.AIProvider, cfg *config.Config, onProgress func(ProgressEvent)) (*state.Quiz, string, error) {
	weakAreas := weakAreasForClass(class)
	if len(weakAreas) == 0 {
		return nil, "", fmt.Errorf("no weak areas found for class %q — complete some quizzes first with 'sfa complete'", class)
	}

	past := pastQuestionsForClass(class)
	knowledgeCtx := knowledgeContextForClass(class)
	return runQuizAgent(class, "adaptive", provider, cfg, nil, 5, weakAreas, past, knowledgeCtx, onProgress)
}

// LoadQuiz reads and unmarshals a quiz YAML file from path.
func LoadQuiz(path string) (*state.Quiz, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read quiz file %q: %w", path, err)
	}
	var q state.Quiz
	if err := yaml.Unmarshal(data, &q); err != nil {
		return nil, fmt.Errorf("parse quiz file %q: %w", path, err)
	}
	return &q, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// runQuizAgent drives an agentic generation loop. The agent is instructed to
// call sfq_schema to obtain the exact YAML schema before producing output,
// which prevents the YAML parse errors that occur with single-shot generation.
// If the AI produces bad YAML, the error is fed back and the agent retries,
// up to maxSteps total rounds.
func runQuizAgent(class, prefix string, provider plugins.AIProvider, cfg *config.Config, summaries []string, numQuestions int, weakAreas, pastQuestions, knowledgeCtx []string, onProgress func(ProgressEvent)) (*state.Quiz, string, error) {
	transcript := buildQuizAgentPrompt(class, summaries, numQuestions, weakAreas, pastQuestions, knowledgeCtx, cfg)

	const maxSteps = 6
	for step := 0; step < maxSteps; step++ {
		resp, err := provider.Generate(transcript)
		if err != nil {
			return nil, "", fmt.Errorf("AI generate: %w", err)
		}

		// Check for tool call.
		name, args, found, parseErr := extractQuizToolCall(resp)
		if parseErr != nil {
			transcript += "\n\nTool call parse error: " + parseErr.Error() + "\nPlease try again."
			continue
		}
		if found {
			label := strings.ReplaceAll(name, "_", " ")
			if onProgress != nil {
				onProgress(ProgressEvent{Label: label, Detail: describeQuizTool(name)})
			}
			result, toolErr := executeQuizTool(cfg, name, args)
			if onProgress != nil {
				onProgress(ProgressEvent{Label: label, Detail: summarizeQuizResult(result), Done: true, Err: toolErr})
			}
			if toolErr != nil {
				result = "Tool error: " + toolErr.Error()
			}
			transcript += "\n\nYou called tool: " + name
			transcript += "\n\nTool result:\n" + result
			transcript += "\n\nNow generate the quiz. Respond with ONLY valid YAML matching the schema above. Do not add any prose, markdown fences, or extra fields."
			continue
		}

		// Attempt to parse the response as a quiz.
		cleaned := cleanYAML(resp)
		var q state.Quiz
		if err := yaml.Unmarshal([]byte(cleaned), &q); err != nil {
			transcript += fmt.Sprintf("\n\nYour response could not be parsed as valid YAML:\n%v\n\nCall sfq_schema to review the exact schema, then respond with ONLY valid YAML.", err)
			continue
		}
		if len(q.Sections) == 0 {
			transcript += fmt.Sprintf("\n\nThe YAML has no sections. Generate at least %d questions and respond with ONLY valid YAML.", numQuestions)
			continue
		}

		return saveQuiz(class, prefix, &q)
	}
	return nil, "", fmt.Errorf("quiz agent exceeded %d steps without producing valid output", maxSteps)
}

func buildQuizAgentPrompt(class string, summaries []string, numQuestions int, weakAreas, pastQuestions, knowledgeCtx []string, cfg *config.Config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are generating a quiz for class: %s.\n", class)
	fmt.Fprintf(&b, "Task: produce exactly %d questions as a YAML document.\n\n", numQuestions)

	b.WriteString("Available tool:\n")
	b.WriteString("- sfq_schema: returns the exact YAML schema you must use. No arguments.\n\n")
	b.WriteString("To call the tool respond with ONLY this block:\n")
	b.WriteString("<tool_call>\n{\"name\":\"sfq_schema\",\"arguments\":{}}\n</tool_call>\n\n")
	b.WriteString("After receiving the schema, respond with ONLY the YAML quiz document.\n\n")
	b.WriteString("Start by calling sfq_schema.\n")

	if len(summaries) > 0 {
		b.WriteString("\nStudy material to base questions on:\n")
		for i, s := range summaries {
			fmt.Fprintf(&b, "[%d] %s\n\n", i+1, s)
		}
	}
	if len(knowledgeCtx) > 0 {
		b.WriteString("\nKnowledge base context (include section_id and component_id in questions when the source is identifiable):\n")
		for _, line := range knowledgeCtx {
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}
	if len(weakAreas) > 0 {
		fmt.Fprintf(&b, "\nFocus extra attention on these weak areas: %s\n", strings.Join(weakAreas, ", "))
	}
	if len(pastQuestions) > 0 {
		b.WriteString("\nAvoid repeating these past questions:\n")
		for _, q := range pastQuestions {
			b.WriteString("- " + q + "\n")
		}
	}
	if cfg.CustomPromptContext != "" {
		b.WriteString("\nAdditional instructions:\n" + cfg.CustomPromptContext + "\n")
	}
	return b.String()
}

func executeQuizTool(cfg *config.Config, name string, _ map[string]any) (string, error) {
	switch name {
	case "sfq_schema":
		return sfq.Schema(cfg.SFQ.Command), nil
	default:
		return "", fmt.Errorf("unknown quiz tool %q", name)
	}
}

func describeQuizTool(name string) string {
	switch name {
	case "sfq_schema":
		return "Fetching quiz YAML schema"
	default:
		return "Running tool"
	}
}

func summarizeQuizResult(result string) string {
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return "Done"
	}
	lines := strings.SplitN(trimmed, "\n", 2)
	if len(lines[0]) > 80 {
		return lines[0][:77] + "..."
	}
	return lines[0]
}

func extractQuizToolCall(resp string) (name string, args map[string]any, found bool, err error) {
	start := strings.Index(resp, "<tool_call>")
	if start == -1 {
		return "", nil, false, nil
	}
	end := strings.Index(resp, "</tool_call>")
	if end == -1 || end < start {
		return "", nil, false, fmt.Errorf("unterminated tool_call block")
	}
	jsonBlock := strings.TrimSpace(resp[start+len("<tool_call>") : end])
	jsonBlock = strings.TrimPrefix(jsonBlock, "```json")
	jsonBlock = strings.TrimPrefix(jsonBlock, "```")
	jsonBlock = strings.TrimSuffix(jsonBlock, "```")
	jsonBlock = strings.TrimSpace(jsonBlock)

	var call struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(jsonBlock), &call); err != nil {
		return "", nil, false, fmt.Errorf("parse tool JSON: %w", err)
	}
	if call.Name == "" {
		return "", nil, false, fmt.Errorf("tool name is required")
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	return call.Name, call.Arguments, true, nil
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

func saveQuiz(class, prefix string, q *state.Quiz) (*state.Quiz, string, error) {
	quizID := fmt.Sprintf("%s-%d", prefix, time.Now().Unix())
	quizPath, err := config.Path("quizzes", class, quizID+".yaml")
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(filepath.Dir(quizPath), 0755); err != nil {
		return nil, "", fmt.Errorf("create quiz dir: %w", err)
	}
	data, err := yaml.Marshal(q)
	if err != nil {
		return nil, "", fmt.Errorf("marshal quiz: %w", err)
	}
	if err := os.WriteFile(quizPath, data, 0644); err != nil {
		return nil, "", fmt.Errorf("write quiz: %w", err)
	}
	sfqPath := strings.TrimSuffix(quizPath, ".yaml") + ".sfq"
	if err := os.WriteFile(sfqPath, quizToSFQ(q), 0644); err != nil {
		return nil, "", fmt.Errorf("write sfq file: %w", err)
	}
	return q, quizPath, nil
}

// quizToSFQ converts a Quiz to the .sfq block-delimiter format expected by
// the sfq binary. Each QuizSection becomes a short-answer question block.
func quizToSFQ(q *state.Quiz) []byte {
	var b strings.Builder
	if q.Title != "" {
		fmt.Fprintf(&b, "# %s\n\n", q.Title)
	}
	for _, sec := range q.Sections {
		b.WriteString("---\n")
		if sec.ID != "" {
			fmt.Fprintf(&b, "id: %s\n", sec.ID)
		}
		if sec.Hint != "" {
			fmt.Fprintf(&b, "hint: %q\n", sec.Hint)
		}
		if len(sec.Tags) > 0 {
			fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(sec.Tags, ", "))
		}
		fmt.Fprintf(&b, "? %s\n", sec.Question)
		if sec.Answer != "" {
			fmt.Fprintf(&b, "answer: %q\n", sec.Answer)
		}
		if sec.Reasoning != "" {
			fmt.Fprintf(&b, "explanation: %s\n", sec.Reasoning)
		}
		b.WriteString("\n")
	}
	b.WriteString("---\n")
	return []byte(b.String())
}

func filterNotes(notes []state.Note, class string, tags []string) []state.Note {
	var out []state.Note
	for _, n := range notes {
		if class != "" && !strings.EqualFold(n.Class, class) {
			continue
		}
		if len(tags) > 0 && !hasAnyTag(n.Tags, tags) {
			continue
		}
		out = append(out, n)
	}
	return out
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

// weakAreasForClass returns tags that the student has answered incorrectly
// at least twice across all stored results for class.
func weakAreasForClass(class string) []string {
	dir, err := config.Path("quizzes", class)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Count wrong answers per question ID across all result files.
	wrongByQID := make(map[string]int)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "-results.json") {
			continue
		}
		quizID := strings.TrimSuffix(e.Name(), "-results.json")
		results, err := state.LoadQuizResults(class, quizID)
		if err != nil {
			continue
		}
		for _, r := range results.Results {
			if !r.Correct {
				wrongByQID[r.QuestionID]++
			}
		}
	}

	// Map wrong question IDs → their tags via saved quiz YAML files.
	tagCounts := make(map[string]int)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		q, err := LoadQuiz(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, s := range q.Sections {
			if count := wrongByQID[s.ID]; count > 0 {
				for _, t := range s.Tags {
					tagCounts[t] += count
				}
			}
		}
	}

	var weak []string
	for tag, count := range tagCounts {
		if count >= 2 {
			weak = append(weak, tag)
		}
	}
	return weak
}

// pastQuestionsForClass collects the question text from every quiz for class.
func pastQuestionsForClass(class string) []string {
	dir, err := config.Path("quizzes", class)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var questions []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		q, err := LoadQuiz(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, s := range q.Sections {
			questions = append(questions, s.Question)
		}
	}
	return questions
}

// knowledgeContextForClass builds a list of context lines from the knowledge
// graph for class, including section and component IDs for provenance tagging.
func knowledgeContextForClass(class string) []string {
	secIdx, err := state.LoadSectionIndex()
	if err != nil {
		return nil
	}
	cmpIdx, err := state.LoadComponentIndex()
	if err != nil {
		return nil
	}
	var lines []string
	for _, s := range secIdx.Sections {
		if s.Class != class {
			continue
		}
		summary := s.Summary
		if len(summary) > 120 {
			summary = summary[:117] + "..."
		}
		lines = append(lines, fmt.Sprintf("Section %s: %q — %s", s.ID, s.Title, summary))
		for _, c := range cmpIdx.Components {
			if c.SectionID != s.ID {
				continue
			}
			content := c.Content
			if len(content) > 100 {
				content = content[:97] + "..."
			}
			lines = append(lines, fmt.Sprintf("  Component %s (%s): %s", c.ID, c.Kind, content))
		}
	}
	return lines
}
