// Package chat assembles class context and sends chat prompts through the AI
// provider selected by the orchestrator.
package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/search"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/internal/tracking"
	"github.com/studyforge/study-agent/plugins"
)

const toolCallStartTag = "<tool_call>"

// Ask sends a prompt to the model with optional class context files included.
func Ask(provider plugins.AIProvider, cfg *config.Config, className, prompt string) (string, error) {
	fullPrompt, err := buildPrompt(cfg, className, prompt)
	if err != nil {
		return "", err
	}

	resp, err := runAgent(provider, cfg, className, fullPrompt, nil)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func buildPrompt(cfg *config.Config, className, prompt string) (string, error) {
	sections := []string{agentInstructions(className)}
	if className != "" {
		sections = append(sections, "Selected class:\n"+className)
		ctxText, err := buildClassContext(className)
		if err != nil {
			return "", err
		}
		if ctxText != "" {
			sections = append(sections, "Class context:\n"+ctxText)
		}
	}

	noteText, err := buildNoteContext(className, prompt)
	if err != nil {
		return "", err
	}
	if noteText != "" {
		sections = append(sections, "Relevant ingested notes:\n"+noteText)
	}

	sections = append(sections, "User request:\n"+prompt)

	if cfg.CustomPromptContext != "" {
		sections = append(sections, "Additional instructions:\n"+cfg.CustomPromptContext)
	}

	return strings.Join(sections, "\n\n"), nil
}

func buildClassContext(className string) (string, error) {
	ctx, err := classpkg.LoadContext(className)
	if err != nil {
		return "", err
	}
	if len(ctx.ContextFiles) == 0 {
		return "", nil
	}

	var b strings.Builder
	for _, p := range ctx.ContextFiles {
		data, err := os.ReadFile(p)
		if err != nil {
			b.WriteString(fmt.Sprintf("\n--- %s (unreadable: %v) ---\n", p, err))
			continue
		}
		content := string(data)
		if len(content) > 4000 {
			content = content[:4000]
		}
		b.WriteString("\n--- ")
		b.WriteString(p)
		b.WriteString(" ---\n")
		b.WriteString(content)
		b.WriteString("\n")
	}
	return b.String(), nil
}

func buildNoteContext(className, prompt string) (string, error) {
	knowledgeResults, knowledgeErr := search.ByKnowledgeQuery(prompt, className, 6)
	if knowledgeErr == nil && len(knowledgeResults) > 0 {
		var b strings.Builder
		for i, result := range knowledgeResults {
			if result.Kind == "section" {
				section := result.Section
				fmt.Fprintf(&b, "[%d] section id=%s class=%s score=%d\n", i+1, section.ID, section.Class, result.Score)
				fmt.Fprintf(&b, "title: %s\n", section.Title)
				if len(section.Tags) > 0 {
					fmt.Fprintf(&b, "tags: %s\n", strings.Join(section.Tags, ", "))
				}
				if len(section.Concepts) > 0 {
					fmt.Fprintf(&b, "concepts: %s\n", strings.Join(section.Concepts, ", "))
				}
				fmt.Fprintf(&b, "summary: %s\n\n", section.Summary)
				continue
			}
			component := result.Component
			fmt.Fprintf(&b, "[%d] component id=%s class=%s score=%d\n", i+1, component.ID, component.Class, result.Score)
			fmt.Fprintf(&b, "kind: %s\n", component.Kind)
			if len(component.Tags) > 0 {
				fmt.Fprintf(&b, "tags: %s\n", strings.Join(component.Tags, ", "))
			}
			if len(component.Concepts) > 0 {
				fmt.Fprintf(&b, "concepts: %s\n", strings.Join(component.Concepts, ", "))
			}
			fmt.Fprintf(&b, "content: %s\n\n", component.Content)
		}
		return strings.TrimSpace(b.String()), nil
	}

	results, err := search.ByQuery(prompt, className, 4)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}

	var b strings.Builder
	for i, result := range results {
		note := result.Note
		fmt.Fprintf(&b, "[%d] id=%s class=%s source=%s score=%d\n", i+1, note.ID, note.Class, note.Source, result.Score)
		if len(note.Tags) > 0 {
			fmt.Fprintf(&b, "tags: %s\n", strings.Join(note.Tags, ", "))
		}
		if len(note.Concepts) > 0 {
			fmt.Fprintf(&b, "concepts: %s\n", strings.Join(note.Concepts, ", "))
		}
		fmt.Fprintf(&b, "summary: %s\n\n", note.Summary)
	}
	return strings.TrimSpace(b.String()), nil
}

const availableTools = `Available tools:
- search_notes: search ingested notes by natural-language query. Arguments: query (string), optional class (string), optional limit (int).
- search_knowledge: search AI-processed knowledge sections and components by natural-language query. Returns richer extracted knowledge than search_notes. Arguments: query (string), optional class (string), optional kind ("section" or "component", leave empty for both), optional limit (int).
- get_class_context: fetch the registered class context files for a class. Arguments: optional class (string).
- sfq_schema: fetch the quiz YAML schema for strict formatting guidance when generating quizzes. No arguments.
- generate_quiz: generate and save a new quiz for a class from its ingested notes. Arguments: class (string), optional count (int), optional type (string), optional tags (array of strings), optional directives (array of objects with component_id, optional section_id, optional section_title, optional question_count, optional question_types, optional angle). Use directives when you already chose the exact component(s) and want to bypass the quiz orchestrator.
- list_classes: list all registered classes. No arguments.
- list_tools: show this list of available tools and their descriptions. No arguments.`

func agentInstructions(className string) string {
	base := `You are Study Forge AI, a study assistant with access to note-search, class-context, and quiz tools.
Use any provided class context and relevant ingested note summaries if the user asks questions about classes.
If you need more note context, do not claim you cannot search notes. Use a tool.
If the user asks you to generate a quiz, use the generate_quiz or adapt_quiz tool.
Preserve explicit user constraints when calling quiz tools. If the user asks for a specific number of questions, a demo quiz, a simple quiz, or a narrow topic, pass those constraints explicitly.
For narrowly scoped quiz requests, prefer using search_knowledge first and then call generate_quiz with explicit directives so you control the plan instead of delegating selection to the orchestrator.

` + availableTools + `

When you want a tool, respond with ONLY this XML block and nothing else:
<tool_call>
{"name":"search_notes","arguments":{"query":"your query","class":"optional class","limit":5}}
</tool_call>

After a tool result is returned, either call another tool the same way or answer the user normally.
Keep answers grounded in the available notes and class context.`
	if className == "" {
		return base
	}
	return base + "\nThe currently selected class is: " + className
}

func runAgent(provider plugins.AIProvider, cfg *config.Config, className, prompt string, onEvent func(StreamEvent) error) (string, error) {
	transcript := prompt
	for step := 0; step < 4; step++ {
		resp, streamed, usage, err := generateAgentResponse(provider, transcript, onEvent)
		if err != nil {
			return "", fmt.Errorf("chat generate: %w", err)
		}
		if usage.Metadata.Provider != "" {
			_ = state.AppendUsageEvent(state.UsageEvent{
				Operation:    "chat",
				Provider:     usage.Metadata.Provider,
				Model:        usage.Metadata.Model,
				RequestID:    usage.Metadata.RequestID,
				InputTokens:  usage.Usage.InputTokens,
				OutputTokens: usage.Usage.OutputTokens,
				TotalTokens:  usage.Usage.TotalTokens,
				CostUSD:      config.CostForTokens(usage.Metadata.Model, usage.Usage.InputTokens, usage.Usage.OutputTokens, cfg),
				Class:        className,
			})
		}

		call, found, err := extractToolCall(resp)
		if err != nil {
			return "", fmt.Errorf("chat tool call: %w", err)
		}
		if !found {
			if onEvent != nil && !streamed {
				if err := emitChunked(resp, 256, onEvent); err != nil {
					return "", err
				}
			}
			return resp, nil
		}

		if onEvent != nil {
			if err := onEvent(StreamEvent{
				Kind:   StreamEventActionStart,
				Label:  formatActionLabel(call),
				Detail: describeToolCall(className, call),
			}); err != nil {
				return "", fmt.Errorf("chat stream callback: %w", err)
			}
		}

		result, toolErr := executeToolCall(provider, cfg, className, call, onEvent)
		if toolErr != nil {
			result = "Tool error: " + toolErr.Error()
		}
		if onEvent != nil {
			if err := onEvent(StreamEvent{
				Kind:   StreamEventActionDone,
				Label:  formatActionLabel(call),
				Detail: summarizeToolResult(result),
				Err:    toolErr,
			}); err != nil {
				return "", fmt.Errorf("chat stream callback: %w", err)
			}
		}

		transcript += "\n\nAssistant requested tool:\n" + strings.TrimSpace(resp)
		transcript += "\n\nTool result:\n" + result
		transcript += "\n\nUse the tool result above. If more information is needed, call another tool. Otherwise answer the user directly."
	}
	return "", fmt.Errorf("chat agent exceeded tool-call limit")
}

func formatActionLabel(call *toolCall) string {
	if call == nil || strings.TrimSpace(call.Name) == "" {
		return "Agent action"
	}
	return strings.ReplaceAll(call.Name, "_", " ")
}

func describeToolCall(className string, call *toolCall) string {
	if call == nil {
		return "Preparing agent action"
	}
	switch call.Name {
	case "search_notes":
		query := toolString(call.Arguments, "query")
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		if targetClass != "" {
			return fmt.Sprintf("Searching notes for %q in %s", query, targetClass)
		}
		return fmt.Sprintf("Searching notes for %q", query)
	case "search_knowledge":
		query := toolString(call.Arguments, "query")
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		kind := toolString(call.Arguments, "kind")
		var suffix string
		switch kind {
		case "section":
			suffix = " (sections)"
		case "component":
			suffix = " (components)"
		}
		if targetClass != "" {
			return fmt.Sprintf("Searching knowledge%s for %q in %s", suffix, query, targetClass)
		}
		return fmt.Sprintf("Searching knowledge%s for %q", suffix, query)
	case "get_class_context":
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		if targetClass == "" {
			return "Loading class context"
		}
		return fmt.Sprintf("Loading class context for %s", targetClass)
	case "sfq_schema":
		return "Fetching quiz YAML schema"
	case "generate_quiz":
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		count := toolInt(call.Arguments, "count", 0)
		typePref := toolString(call.Arguments, "type")
		tags := toolStringSlice(call.Arguments, "tags")
		directives, err := toolQuizDirectives(call.Arguments, count, typePref)
		if err != nil {
			return err.Error()
		}
		if targetClass != "" {
			return fmt.Sprintf("Generating quiz for %s%s", targetClass, describeQuizRequestSuffix(count, typePref, tags, len(directives) > 0))
		}
		return "Generating quiz" + describeQuizRequestSuffix(count, typePref, tags, len(directives) > 0)
	case "list_classes":
		return "Listing registered classes"
	case "list_tools", "print_list_tools":
		return "Listing available tools"
	default:
		return "Running agent action"
	}
}

func summarizeToolResult(result string) string {
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return "Action completed"
	}
	line := strings.Split(trimmed, "\n")[0]
	return truncateText(line, 96)
}

func truncateText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return text[:limit-3] + "..."
}

type toolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func extractToolCall(resp string) (*toolCall, bool, error) {
	start := strings.Index(resp, toolCallStartTag)
	if start == -1 {
		return nil, false, nil
	}
	end := strings.Index(resp, "</tool_call>")
	if end == -1 || end < start {
		return nil, false, fmt.Errorf("unterminated tool_call block")
	}
	jsonBlock := strings.TrimSpace(resp[start+len(toolCallStartTag) : end])
	jsonBlock = strings.TrimPrefix(jsonBlock, "```json")
	jsonBlock = strings.TrimPrefix(jsonBlock, "```")
	jsonBlock = strings.TrimSuffix(jsonBlock, "```")
	jsonBlock = strings.TrimSpace(jsonBlock)

	var call toolCall
	if err := json.Unmarshal([]byte(jsonBlock), &call); err != nil {
		return nil, false, fmt.Errorf("parse tool JSON: %w", err)
	}
	if call.Name == "" {
		return nil, false, fmt.Errorf("tool name is required")
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	return &call, true, nil
}

func executeToolCall(provider plugins.AIProvider, cfg *config.Config, className string, call *toolCall, onEvent func(StreamEvent) error) (string, error) {
	switch call.Name {
	case "search_notes":
		query := toolString(call.Arguments, "query")
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		limit := toolInt(call.Arguments, "limit", 5)
		results, err := search.ByQuery(query, targetClass, limit)
		if err != nil {
			return "", err
		}
		return formatToolSearchResults(query, targetClass, results), nil
	case "search_knowledge":
		query := toolString(call.Arguments, "query")
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		kind := toolString(call.Arguments, "kind")
		limit := toolInt(call.Arguments, "limit", 8)
		results, err := search.ByKnowledgeQuery(query, targetClass, limit)
		if err != nil {
			return "", err
		}
		return formatKnowledgeResults(query, targetClass, kind, results), nil
	case "get_class_context":
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		if targetClass == "" {
			return "", fmt.Errorf("class is required when no class is selected")
		}
		ctxText, err := buildClassContext(targetClass)
		if err != nil {
			return "", err
		}
		if ctxText == "" {
			return fmt.Sprintf("No class context files registered for %q.", targetClass), nil
		}
		return ctxText, nil
	case "sfq_schema":
		return sfq.Schema(cfg.SFQ.Command), nil
	case "generate_quiz":
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		if targetClass == "" {
			return "", fmt.Errorf("class is required when no class is selected")
		}
		count := toolInt(call.Arguments, "count", 10)
		typePref := toolString(call.Arguments, "type")
		if typePref == "" {
			typePref = "multiple-choice"
		}
		tags := toolStringSlice(call.Arguments, "tags")
		directives, err := toolQuizDirectives(call.Arguments, count, typePref)
		if err != nil {
			return "", err
		}
		opts := quiz.QuizOptions{Count: count, TypePreference: typePref, Tags: tags, Directives: directives}
		q, path, err := quiz.NewQuizStream(targetClass, opts, provider, cfg, func(progress quiz.ProgressEvent) {
			_ = emitQuizProgressEvent(progress, onEvent)
		})
		if err != nil {
			return "", err
		}
		quizID := strings.TrimSuffix(filepath.Base(path), ".yaml")
		sfqPath := strings.TrimSuffix(path, ".yaml") + ".sfq"
		if _, cacheErr := state.RegisterTrackedQuiz(targetClass, path, sfqPath); cacheErr != nil {
			return fmt.Sprintf("Quiz generated and saved to %s\nQuiz ID: %s\nTitle: %s\nQuestions: %d\nTracked cache warning: %v", path, quizID, q.Title, len(q.Sections), cacheErr), nil
		}
		report, syncErr := tracking.SyncTrackedQuizSessions()
		sfqErr := sfq.Track(sfqPath)
		syncSummary := ""
		if syncErr != nil {
			syncSummary = "\nSession sync warning: " + syncErr.Error()
		} else {
			syncSummary = fmt.Sprintf("\nImported sessions: %d\nPending tracked quizzes: %d", report.ImportedSessions, report.PendingQuizzes)
			if report.UnmappedAnswers > 0 {
				syncSummary += fmt.Sprintf("\nUnmapped answers: %d", report.UnmappedAnswers)
			}
		}
		if sfqErr != nil {
			return fmt.Sprintf("Quiz generated and saved to %s\nQuiz ID: %s\nTitle: %s\nQuestions: %d\nTracked session could not start: %v%s", path, quizID, q.Title, len(q.Sections), sfqErr, syncSummary), nil
		}
		return fmt.Sprintf("Quiz generated and saved to %s\nQuiz ID: %s\nTitle: %s\nQuestions: %d\nTracked quiz session started in browser.%s", path, quizID, q.Title, len(q.Sections), syncSummary), nil
	case "adapt_quiz":
		targetClass := toolString(call.Arguments, "class")
		if targetClass == "" {
			targetClass = className
		}
		if targetClass == "" {
			return "", fmt.Errorf("class is required when no class is selected")
		}
		count := toolInt(call.Arguments, "count", 10)
		typePref := toolString(call.Arguments, "type")
		if typePref == "" {
			typePref = "multiple-choice"
		}
		opts := quiz.QuizOptions{Count: count, TypePreference: typePref}
		q, path, err := quiz.NewQuizStream(targetClass, opts, provider, cfg, func(progress quiz.ProgressEvent) {
			_ = emitQuizProgressEvent(progress, onEvent)
		})
		if err != nil {
			return "", err
		}
		quizID := strings.TrimSuffix(filepath.Base(path), ".yaml")
		sfqPath := strings.TrimSuffix(path, ".yaml") + ".sfq"
		if _, cacheErr := state.RegisterTrackedQuiz(targetClass, path, sfqPath); cacheErr != nil {
			return fmt.Sprintf("Adaptive quiz generated and saved to %s\nQuiz ID: %s\nTitle: %s\nQuestions: %d\nTracked cache warning: %v", path, quizID, q.Title, len(q.Sections), cacheErr), nil
		}
		report, syncErr := tracking.SyncTrackedQuizSessions()
		sfqErr := sfq.Track(sfqPath)
		syncSummary := ""
		if syncErr != nil {
			syncSummary = "\nSession sync warning: " + syncErr.Error()
		} else {
			syncSummary = fmt.Sprintf("\nImported sessions: %d\nPending tracked quizzes: %d", report.ImportedSessions, report.PendingQuizzes)
			if report.UnmappedAnswers > 0 {
				syncSummary += fmt.Sprintf("\nUnmapped answers: %d", report.UnmappedAnswers)
			}
		}
		if sfqErr != nil {
			return fmt.Sprintf("Adaptive quiz generated and saved to %s\nQuiz ID: %s\nTitle: %s\nQuestions: %d\nTracked session could not start: %v%s", path, quizID, q.Title, len(q.Sections), sfqErr, syncSummary), nil
		}
		return fmt.Sprintf("Adaptive quiz generated and saved to %s\nQuiz ID: %s\nTitle: %s\nQuestions: %d\nTracked quiz session started in browser.%s", path, quizID, q.Title, len(q.Sections), syncSummary), nil
	case "list_classes":
		names, err := classpkg.List()
		if err != nil {
			return "", err
		}
		if len(names) == 0 {
			return "No classes registered yet. Use 'sfa class new <name>' to create one.", nil
		}
		return "Registered classes:\n" + strings.Join(names, "\n"), nil
	case "list_tools":
		return availableTools, nil
	default:
		return "", fmt.Errorf("unknown tool %q", call.Name)
	}
}

func emitQuizProgressEvent(progress quiz.ProgressEvent, onEvent func(StreamEvent) error) error {
	if onEvent == nil {
		return nil
	}
	kind := StreamEventActionStart
	if progress.Done {
		kind = StreamEventActionDone
	}
	return onEvent(StreamEvent{
		Kind:   kind,
		Label:  progress.Label,
		Detail: progress.Detail,
		Err:    progress.Err,
	})
}

func formatKnowledgeResults(query, className, kind string, results []search.KnowledgeResult) string {
	if kind == "section" || kind == "component" {
		filtered := results[:0]
		for _, r := range results {
			if r.Kind == kind {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	if len(results) == 0 {
		if className != "" {
			return fmt.Sprintf("No knowledge sections or components matched query %q for class %q.", query, className)
		}
		return fmt.Sprintf("No knowledge sections or components matched query %q.", query)
	}

	var b strings.Builder
	if className != "" {
		fmt.Fprintf(&b, "Knowledge results for query %q in class %q:\n\n", query, className)
	} else {
		fmt.Fprintf(&b, "Knowledge results for query %q:\n\n", query)
	}
	for i, result := range results {
		if result.Kind == "section" {
			s := result.Section
			fmt.Fprintf(&b, "%d. [section] id=%s class=%s score=%d\n", i+1, s.ID, s.Class, result.Score)
			fmt.Fprintf(&b, "   title: %s\n", s.Title)
			if len(s.Tags) > 0 {
				fmt.Fprintf(&b, "   tags: %s\n", strings.Join(s.Tags, ", "))
			}
			if len(s.Concepts) > 0 {
				fmt.Fprintf(&b, "   concepts: %s\n", strings.Join(s.Concepts, ", "))
			}
			fmt.Fprintf(&b, "   summary: %s\n\n", s.Summary)
		} else {
			c := result.Component
			fmt.Fprintf(&b, "%d. [component] id=%s class=%s score=%d\n", i+1, c.ID, c.Class, result.Score)
			fmt.Fprintf(&b, "   kind: %s\n", c.Kind)
			if len(c.Tags) > 0 {
				fmt.Fprintf(&b, "   tags: %s\n", strings.Join(c.Tags, ", "))
			}
			if len(c.Concepts) > 0 {
				fmt.Fprintf(&b, "   concepts: %s\n", strings.Join(c.Concepts, ", "))
			}
			fmt.Fprintf(&b, "   content: %s\n\n", c.Content)
		}
	}
	return strings.TrimSpace(b.String())
}

func formatToolSearchResults(query, className string, results []search.Result) string {
	if len(results) == 0 {
		if className != "" {
			return fmt.Sprintf("No ingested notes matched query %q for class %q.", query, className)
		}
		return fmt.Sprintf("No ingested notes matched query %q.", query)
	}

	var b strings.Builder
	if className != "" {
		fmt.Fprintf(&b, "Matched ingested notes for query %q in class %q:\n\n", query, className)
	} else {
		fmt.Fprintf(&b, "Matched ingested notes for query %q:\n\n", query)
	}
	for i, result := range results {
		note := result.Note
		fmt.Fprintf(&b, "%d. id=%s class=%s source=%s score=%d\n", i+1, note.ID, note.Class, note.Source, result.Score)
		if len(note.Tags) > 0 {
			fmt.Fprintf(&b, "   tags: %s\n", strings.Join(note.Tags, ", "))
		}
		if len(note.Concepts) > 0 {
			fmt.Fprintf(&b, "   concepts: %s\n", strings.Join(note.Concepts, ", "))
		}
		fmt.Fprintf(&b, "   summary: %s\n\n", note.Summary)
	}
	return strings.TrimSpace(b.String())
}

func toolStringSlice(args map[string]any, key string) []string {
	value, ok := args[key]
	if !ok {
		return nil
	}
	arr, ok := value.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, v := range arr {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			result = append(result, strings.TrimSpace(s))
		}
	}
	return result
}

func toolQuizDirectives(args map[string]any, totalCount int, typePreference string) ([]quiz.OrchestratorDirective, error) {
	value, ok := args["directives"]
	if !ok {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("directives must be an array")
	}
	directives := make([]quiz.OrchestratorDirective, 0, len(items))
	for index, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("directive %d must be an object", index+1)
		}
		directive := quiz.OrchestratorDirective{
			ComponentID:   toolString(entry, "component_id"),
			SectionID:     toolString(entry, "section_id"),
			SectionTitle:  toolString(entry, "section_title"),
			QuestionCount: toolInt(entry, "question_count", 0),
			QuestionTypes: toolStringSlice(entry, "question_types"),
			Angle:         toolString(entry, "angle"),
		}
		if len(directive.QuestionTypes) == 0 {
			if singleType := toolString(entry, "question_type"); singleType != "" {
				directive.QuestionTypes = []string{singleType}
			}
		}
		if len(directive.QuestionTypes) == 0 && strings.TrimSpace(typePreference) != "" {
			directive.QuestionTypes = []string{strings.TrimSpace(typePreference)}
		}
		directives = append(directives, directive)
	}
	if len(directives) == 1 && directives[0].QuestionCount == 0 && totalCount > 0 {
		directives[0].QuestionCount = totalCount
	}
	return directives, nil
}

func toolString(args map[string]any, key string) string {
	value, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func toolInt(args map[string]any, key string, defaultValue int) int {
	value, ok := args[key]
	if !ok {
		return defaultValue
	}
	switch v := value.(type) {
	case float64:
		if v < 1 {
			return defaultValue
		}
		return int(v)
	case int:
		if v < 1 {
			return defaultValue
		}
		return v
	default:
		return defaultValue
	}
}

func describeQuizRequestSuffix(count int, typePreference string, tags []string, hasDirectives bool) string {
	parts := make([]string, 0, 4)
	if count > 0 {
		parts = append(parts, fmt.Sprintf("%d question(s)", count))
	}
	if strings.TrimSpace(typePreference) != "" {
		parts = append(parts, strings.TrimSpace(typePreference))
	}
	if len(tags) > 0 {
		parts = append(parts, "tags: "+strings.Join(tags, ", "))
	}
	if hasDirectives {
		parts = append(parts, "manual plan")
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, "; ") + ")"
}
