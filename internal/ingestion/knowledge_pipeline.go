package ingestion

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/prompts"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/plugins"
	"gopkg.in/yaml.v3"
)

// KnowledgeIngestResult summarizes ingestion outcomes.
type KnowledgeIngestResult struct {
	Notes           []state.Note
	SectionsAdded   int
	ComponentsAdded int
	UsageEvents     int
	IngestRunID     string
}

// IngestKnowledgeFolderStream runs the full compose/embed/consolidate pipeline
// and persists note, section, component, and usage data.
func IngestKnowledgeFolderStream(folderPath, class string, provider plugins.AIProvider, embeddingProvider plugins.EmbeddingProvider, cfg *config.Config, onProgress func(ProgressEvent)) (KnowledgeIngestResult, error) {
	emit := func(event ProgressEvent) {
		if onProgress != nil {
			onProgress(event)
		}
	}

	result := KnowledgeIngestResult{IngestRunID: fmt.Sprintf("ingest-%d", time.Now().UnixNano())}

	// Warn if embeddings are disabled
	if embeddingProvider == nil || embeddingProvider.Disabled() {
		emit(ProgressEvent{
			Label:  "Configure embeddings",
			Detail: "Embeddings disabled - deduplication will not happen",
			Done:   true,
			Err:    nil,
		})
	}

	emit(ProgressEvent{Label: "Discover files", Detail: folderPath})
	files, err := collectFiles(folderPath)
	if err != nil {
		emit(ProgressEvent{Label: "Discover files", Detail: folderPath, Done: true, Err: err})
		return result, fmt.Errorf("collect files from %q: %w", folderPath, err)
	}
	emit(ProgressEvent{Label: "Discover files", Detail: fmt.Sprintf("%d file(s)", len(files)), Done: true})
	if len(files) == 0 {
		return result, fmt.Errorf("no supported files found in %q", folderPath)
	}

	sectionIndex, err := state.LoadSectionIndex()
	if err != nil {
		return result, fmt.Errorf("load section index: %w", err)
	}
	componentIndex, err := state.LoadComponentIndex()
	if err != nil {
		return result, fmt.Errorf("load component index: %w", err)
	}

	for _, filePath := range files {
		emit(ProgressEvent{Label: "Process file", Detail: filePath})
		note, summarizeUsage, noteErr := processFileWithMetadata(filePath, class, provider, cfg.CustomPromptContext)
		if noteErr != nil {
			emit(ProgressEvent{Label: "Process file", Detail: filePath, Done: true, Err: noteErr})
			continue
		}
		emit(ProgressEvent{Label: "Process file", Detail: filePath, Done: true})
		result.Notes = append(result.Notes, note)
		if summarizeUsage.Metadata.Provider != "" {
			_ = state.AppendUsageEvent(state.UsageEvent{
				Operation:    "ingest.summarize",
				Provider:     summarizeUsage.Metadata.Provider,
				Model:        summarizeUsage.Metadata.Model,
				RequestID:    summarizeUsage.Metadata.RequestID,
				InputTokens:  summarizeUsage.Usage.InputTokens,
				OutputTokens: summarizeUsage.Usage.OutputTokens,
				TotalTokens:  summarizeUsage.Usage.TotalTokens,
				CostUSD:      costForGenResult(summarizeUsage, cfg),
				Class:        class,
				SourcePath:   note.Source,
				IngestRunID:  result.IngestRunID,
			})
			result.UsageEvents++
		}

		emit(ProgressEvent{Label: "Persist note", Detail: note.ID})
		if saveErr := saveProcessedNote(note); saveErr != nil {
			emit(ProgressEvent{Label: "Persist note", Detail: note.ID, Done: true, Err: saveErr})
		} else {
			emit(ProgressEvent{Label: "Persist note", Detail: note.ID, Done: true})
		}

		emit(ProgressEvent{Label: "Compose sections", Detail: note.ID})
		sections, usage, composeErr := composeKnowledge(note, provider, cfg)
		if composeErr != nil {
			emit(ProgressEvent{Label: "Compose sections", Detail: note.ID, Done: true, Err: composeErr})
			continue
		}
		emit(ProgressEvent{Label: "Compose sections", Detail: fmt.Sprintf("%s (%d section(s))", note.ID, len(sections.Sections)), Done: true})

		if usage.Metadata.Provider != "" {
			_ = state.AppendUsageEvent(state.UsageEvent{
				Operation:    "ingest.compose",
				Provider:     usage.Metadata.Provider,
				Model:        usage.Metadata.Model,
				RequestID:    usage.Metadata.RequestID,
				InputTokens:  usage.Usage.InputTokens,
				OutputTokens: usage.Usage.OutputTokens,
				TotalTokens:  usage.Usage.TotalTokens,
				CostUSD:      costForGenResult(usage, cfg),
				Class:        class,
				SourcePath:   note.Source,
				IngestRunID:  result.IngestRunID,
			})
			result.UsageEvents++
		}

		for _, section := range sections.Sections {
			normalizedSection, components := materializeKnowledgeUnits(section, note, class)
			components = attachEmbeddings(&normalizedSection, components, embeddingProvider, emit, result.IngestRunID, cfg)

			candidates := state.SearchSectionsByEmbedding(sectionIndex, normalizedSection.Embedding, 3)
			merged := false
			for _, candidate := range candidates {
				if strings.TrimSpace(candidate.ID) == "" || candidate.ID == normalizedSection.ID {
					continue
				}
				emit(ProgressEvent{Label: "Review consolidation", Detail: normalizedSection.Title})
				decision, reviewUsage, reviewErr := reviewMergeDecision(provider, normalizedSection, candidate, cfg)
				if reviewErr != nil {
					emit(ProgressEvent{Label: "Review consolidation", Detail: normalizedSection.Title, Done: true, Err: reviewErr})
					continue
				}
				if reviewUsage.Metadata.Provider != "" {
					_ = state.AppendUsageEvent(state.UsageEvent{
						Operation:    "ingest.review",
						Provider:     reviewUsage.Metadata.Provider,
						Model:        reviewUsage.Metadata.Model,
						RequestID:    reviewUsage.Metadata.RequestID,
						InputTokens:  reviewUsage.Usage.InputTokens,
						OutputTokens: reviewUsage.Usage.OutputTokens,
						TotalTokens:  reviewUsage.Usage.TotalTokens,
						CostUSD:      costForGenResult(reviewUsage, cfg),
						Class:        class,
						SourcePath:   note.Source,
						IngestRunID:  result.IngestRunID,
					})
					result.UsageEvents++
				}
				emit(ProgressEvent{Label: "Review consolidation", Detail: fmt.Sprintf("%s -> %s", normalizedSection.Title, decision), Done: true})
				if decision == "merge" {
					normalizedSection.ID = candidate.ID
					merged = true
					break
				}
			}

			sectionIndex.AddOrUpdate(normalizedSection)
			if !merged {
				result.SectionsAdded++
			}

			for _, component := range components {
				component.SectionID = normalizedSection.ID
				componentIndex.AddOrUpdate(component)
				result.ComponentsAdded++
			}
		}
	}

	emit(ProgressEvent{Label: "Persist knowledge", Detail: "sections index"})
	if err := state.SaveSectionIndex(sectionIndex); err != nil {
		emit(ProgressEvent{Label: "Persist knowledge", Detail: "sections index", Done: true, Err: err})
		return result, fmt.Errorf("save section index: %w", err)
	}
	emit(ProgressEvent{Label: "Persist knowledge", Detail: "sections index", Done: true})

	emit(ProgressEvent{Label: "Persist knowledge", Detail: "components index"})
	if err := state.SaveComponentIndex(componentIndex); err != nil {
		emit(ProgressEvent{Label: "Persist knowledge", Detail: "components index", Done: true, Err: err})
		return result, fmt.Errorf("save component index: %w", err)
	}
	emit(ProgressEvent{Label: "Persist knowledge", Detail: "components index", Done: true})

	return result, nil
}

type composedKnowledge struct {
	Sections []composedSection `yaml:"sections"`
}

type composedSection struct {
	Title      string              `yaml:"title"`
	Summary    string              `yaml:"summary"`
	Tags       []string            `yaml:"tags"`
	Concepts   []string            `yaml:"concepts"`
	Components []composedComponent `yaml:"components"`
}

type composedComponent struct {
	Kind     string   `yaml:"kind"`
	Content  string   `yaml:"content"`
	Tags     []string `yaml:"tags"`
	Concepts []string `yaml:"concepts"`
}

func composeKnowledge(note state.Note, provider plugins.AIProvider, cfg *config.Config) (composedKnowledge, plugins.GenerateResult, error) {
	noteContent := note.Summary
	if data, err := os.ReadFile(note.Source); err == nil {
		noteContent = string(data)
		if len(noteContent) > 8000 {
			noteContent = noteContent[:8000]
		}
	}
	prompt := prompts.ComposeKnowledge(note.Summary, noteContent, note.Class, note.Source, cfg.CustomPromptContext)
	response, usage, err := generateWithMetadata(provider, prompt)
	if err != nil {
		return composedKnowledge{}, plugins.GenerateResult{}, err
	}
	response = sanitizeComposeYAML(response)
	parsed, err := parseComposedKnowledge(response)
	if err != nil {
		return composedKnowledge{}, usage, fmt.Errorf("parse compose YAML: %w\nResponse was:\n%s", err, response)
	}
	if len(parsed.Sections) == 0 {
		return composedKnowledge{}, usage, fmt.Errorf("compose returned no sections")
	}
	return parsed, usage, nil
}

func parseComposedKnowledge(response string) (composedKnowledge, error) {
	var parsed composedKnowledge
	firstErr := yaml.Unmarshal([]byte(response), &parsed)
	if firstErr == nil {
		return parsed, nil
	}

	normalized := normalizeComposeYAMLPlainScalars(response)
	if normalized == response {
		return composedKnowledge{}, firstErr
	}
	if err := yaml.Unmarshal([]byte(normalized), &parsed); err != nil {
		return composedKnowledge{}, err
	}
	return parsed, nil
}

func sanitizeComposeYAML(response string) string {
	cleaned := sanitizeAIYAML(response)
	if cleaned == "" {
		return cleaned
	}

	if idx := indexOfTopLevelSections(cleaned); idx > 0 {
		cleaned = strings.TrimSpace(cleaned[idx:])
	}

	return cleaned
}

// sanitizeAIYAML removes common LLM formatting artifacts (BOM, fenced code
// blocks, and preamble prose) before YAML parsing.
func sanitizeAIYAML(response string) string {
	cleaned := strings.TrimSpace(strings.TrimPrefix(response, "\ufeff"))
	if cleaned == "" {
		return cleaned
	}

	if fenced := extractFirstFencedBlock(cleaned); fenced != "" {
		cleaned = fenced
	}

	if idx := indexOfFirstTopLevelKey(cleaned); idx > 0 {
		cleaned = strings.TrimSpace(cleaned[idx:])
	}

	return cleaned
}

func indexOfFirstTopLevelKey(text string) int {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	if startsWithTopLevelKey(normalized) {
		return 0
	}
	lines := strings.Split(normalized, "\n")
	offset := 0
	for _, line := range lines {
		if startsWithTopLevelKey(line) {
			return offset
		}
		offset += len(line) + 1
	}
	return -1
}

func startsWithTopLevelKey(line string) bool {
	if line == "" {
		return false
	}
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "-") {
		return false
	}
	colon := strings.Index(line, ":")
	if colon <= 0 {
		return false
	}
	key := strings.TrimSpace(line[:colon])
	if key == "" {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func extractFirstFencedBlock(text string) string {
	start := strings.Index(text, "```")
	if start == -1 {
		return ""
	}
	afterStart := text[start+3:]
	newline := strings.Index(afterStart, "\n")
	if newline == -1 {
		return ""
	}
	bodyStart := start + 3 + newline + 1
	rest := text[bodyStart:]
	end := strings.Index(rest, "```")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func indexOfTopLevelSections(text string) int {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.HasPrefix(normalized, "sections:") {
		return 0
	}
	needle := "\nsections:"
	idx := strings.Index(normalized, needle)
	if idx == -1 {
		return -1
	}
	return idx + 1
}

func normalizeComposeYAMLPlainScalars(text string) string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	changed := false
	stringFields := map[string]struct{}{
		"title":   {},
		"summary": {},
		"kind":    {},
		"content": {},
	}

	for i, line := range lines {
		prefixLen := len(line) - len(strings.TrimLeft(line, " \t"))
		indent := line[:prefixLen]
		rest := strings.TrimSpace(line)

		listPrefix := ""
		if strings.HasPrefix(rest, "- ") {
			listPrefix = "- "
			rest = strings.TrimSpace(rest[2:])
		}

		colon := strings.Index(rest, ":")
		if colon <= 0 {
			continue
		}

		key := strings.TrimSpace(rest[:colon])
		if _, ok := stringFields[key]; !ok {
			continue
		}

		value := strings.TrimSpace(rest[colon+1:])
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") || strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">") {
			continue
		}

		quoted := strconv.Quote(value)
		lines[i] = indent + listPrefix + key + ": " + quoted
		changed = true
	}

	if !changed {
		return text
	}
	return strings.Join(lines, "\n")
}

func materializeKnowledgeUnits(sectionData composedSection, note state.Note, class string) (state.Section, []state.Component) {
	title := strings.TrimSpace(sectionData.Title)
	if title == "" {
		title = note.ID
	}
	sectionID := stableID("sec", class, title, sectionData.Summary)
	section := state.Section{
		ID:          sectionID,
		Class:       class,
		Title:       title,
		Summary:     strings.TrimSpace(sectionData.Summary),
		Tags:        append([]string{}, sectionData.Tags...),
		Concepts:    append([]string{}, sectionData.Concepts...),
		SourcePaths: []string{note.Source},
		SourceTags:  []string{note.SourceTag},
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	components := make([]state.Component, 0, len(sectionData.Components))
	for _, c := range sectionData.Components {
		content := strings.TrimSpace(c.Content)
		if content == "" {
			continue
		}
		componentID := stableID("cmp", class, title, c.Kind, content)
		components = append(components, state.Component{
			ID:          componentID,
			SectionID:   sectionID,
			Class:       class,
			Kind:        strings.TrimSpace(c.Kind),
			Content:     content,
			Tags:        append([]string{}, c.Tags...),
			Concepts:    append([]string{}, c.Concepts...),
			SourcePaths: []string{note.Source},
			SourceTags:  []string{note.SourceTag},
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		})
		section.ComponentIDs = append(section.ComponentIDs, componentID)
	}
	return section, components
}

func attachEmbeddings(section *state.Section, components []state.Component, provider plugins.EmbeddingProvider, emit func(ProgressEvent), ingestRunID string, cfg *config.Config) []state.Component {
	if section == nil || provider == nil || provider.Disabled() {
		emit(ProgressEvent{
			Label:  "Embed section",
			Detail: fmt.Sprintf("%s (skipped - embeddings disabled)", section.Title),
			Done:   true,
			Err:    nil,
		})
		return components
	}

	emit(ProgressEvent{Label: "Embed section", Detail: section.Title})
	vectors, usage, err := embedWithMetadata(provider, []string{section.Title + "\n" + section.Summary})
	if err != nil {
		emit(ProgressEvent{Label: "Embed section", Detail: section.Title, Done: true, Err: err})
		return components
	}
	if len(vectors) > 0 {
		section.Embedding = vectors[0]
		section.EmbeddingModel = usage.Metadata.Model
	}
	emit(ProgressEvent{Label: "Embed section", Detail: section.Title, Done: true})

	_ = state.AppendUsageEvent(state.UsageEvent{
		Operation:    "ingest.embed.section",
		Provider:     usage.Metadata.Provider,
		Model:        usage.Metadata.Model,
		RequestID:    usage.Metadata.RequestID,
		InputTokens:  usage.Usage.InputTokens,
		OutputTokens: usage.Usage.OutputTokens,
		TotalTokens:  usage.Usage.TotalTokens,
		CostUSD:      costForEmbedResult(usage, cfg),
		Class:        section.Class,
		SourcePath:   firstSource(section.SourcePaths),
		IngestRunID:  ingestRunID,
	})

	if len(components) == 0 {
		return components
	}

	componentTexts := make([]string, 0, len(components))
	for _, component := range components {
		componentTexts = append(componentTexts, strings.TrimSpace(component.Kind+": "+component.Content))
	}
	emit(ProgressEvent{Label: "Embed components", Detail: fmt.Sprintf("%d item(s)", len(componentTexts))})
	componentVectors, componentUsage, componentErr := embedWithMetadata(provider, componentTexts)
	if componentErr != nil {
		emit(ProgressEvent{Label: "Embed components", Detail: "failed", Done: true, Err: componentErr})
		return components
	}
	for i := range components {
		if i < len(componentVectors) {
			components[i].Embedding = componentVectors[i]
			components[i].EmbeddingModel = componentUsage.Metadata.Model
		}
	}
	emit(ProgressEvent{Label: "Embed components", Detail: fmt.Sprintf("%d item(s)", len(componentTexts)), Done: true})

	_ = state.AppendUsageEvent(state.UsageEvent{
		Operation:    "ingest.embed.component",
		Provider:     componentUsage.Metadata.Provider,
		Model:        componentUsage.Metadata.Model,
		RequestID:    componentUsage.Metadata.RequestID,
		InputTokens:  componentUsage.Usage.InputTokens,
		OutputTokens: componentUsage.Usage.OutputTokens,
		TotalTokens:  componentUsage.Usage.TotalTokens,
		CostUSD:      costForEmbedResult(componentUsage, cfg),
		Class:        section.Class,
		SourcePath:   firstSource(section.SourcePaths),
		IngestRunID:  ingestRunID,
	})

	return components
}

func reviewMergeDecision(provider plugins.AIProvider, candidate, existing state.Section, cfg *config.Config) (string, plugins.GenerateResult, error) {
	prompt := prompts.ReviewConsolidation(candidate.Title, candidate.Summary, existing.Title, existing.Summary, cfg.CustomPromptContext)
	response, usage, err := generateWithMetadata(provider, prompt)
	if err != nil {
		return "", plugins.GenerateResult{}, err
	}
	response = sanitizeAIYAML(response)
	var parsed struct {
		Decision string `yaml:"decision"`
	}
	if err := yaml.Unmarshal([]byte(response), &parsed); err != nil {
		return "", usage, fmt.Errorf("parse review YAML: %w", err)
	}
	decision := strings.ToLower(strings.TrimSpace(parsed.Decision))
	if decision != "merge" {
		decision = "keep"
	}
	return decision, usage, nil
}

func stableID(prefix string, parts ...string) string {
	joined := strings.ToLower(strings.Join(parts, "|"))
	joined = strings.Join(strings.Fields(joined), " ")
	h := sha1.Sum([]byte(joined))
	return prefix + "-" + hex.EncodeToString(h[:8])
}

func firstSource(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// costForGenResult returns the CostUSD for a generate call, using the pricing
// config to compute it if the provider did not supply a cost.
func costForGenResult(usage plugins.GenerateResult, cfg *config.Config) float64 {
	if usage.Usage.CostUSD > 0 {
		return usage.Usage.CostUSD
	}
	return config.CostForTokens(usage.Metadata.Model, usage.Usage.InputTokens, usage.Usage.OutputTokens, cfg)
}

// costForEmbedResult returns the CostUSD for an embed call, using the pricing
// config to compute it if the provider did not supply a cost.
func costForEmbedResult(usage plugins.EmbedResult, cfg *config.Config) float64 {
	if usage.Usage.CostUSD > 0 {
		return usage.Usage.CostUSD
	}
	return config.CostForTokens(usage.Metadata.Model, usage.Usage.InputTokens, 0, cfg)
}

func generateWithMetadata(provider plugins.AIProvider, prompt string) (string, plugins.GenerateResult, error) {
	if usageAware, ok := provider.(plugins.UsageAwareAIProvider); ok {
		result, err := usageAware.GenerateWithMetadata(prompt)
		if err != nil {
			return "", plugins.GenerateResult{}, err
		}
		if result.Metadata.At.IsZero() {
			result.Metadata.At = time.Now().UTC()
		}
		if result.Metadata.Provider == "" {
			result.Metadata.Provider = provider.Name()
		}
		if result.Metadata.Model == "" {
			result.Metadata.Model = "unknown"
		}
		return result.Text, result, nil
	}

	text, err := provider.Generate(prompt)
	if err != nil {
		return "", plugins.GenerateResult{}, err
	}
	usage := estimateTokenUsage(prompt, text)
	return text, plugins.GenerateResult{
		Text:  text,
		Usage: usage,
		Metadata: plugins.CallMetadata{
			Provider: provider.Name(),
			Model:    "unknown",
			At:       time.Now().UTC(),
		},
	}, nil
}

func embedWithMetadata(provider plugins.EmbeddingProvider, input []string) ([][]float64, plugins.EmbedResult, error) {
	if usageAware, ok := provider.(plugins.UsageAwareEmbeddingProvider); ok {
		result, err := usageAware.EmbedWithMetadata(input)
		if err != nil {
			return nil, plugins.EmbedResult{}, err
		}
		if result.Metadata.At.IsZero() {
			result.Metadata.At = time.Now().UTC()
		}
		if result.Metadata.Provider == "" {
			result.Metadata.Provider = provider.Name()
		}
		if result.Metadata.Model == "" {
			result.Metadata.Model = "unknown"
		}
		return result.Vectors, result, nil
	}

	vectors, err := provider.Embed(input)
	if err != nil {
		return nil, plugins.EmbedResult{}, err
	}
	usage := estimateTokenUsage(strings.Join(input, "\n"), "")
	return vectors, plugins.EmbedResult{
		Vectors: vectors,
		Usage:   usage,
		Metadata: plugins.CallMetadata{
			Provider: provider.Name(),
			Model:    "unknown",
			At:       time.Now().UTC(),
		},
	}, nil
}

func estimateTokenUsage(input, output string) plugins.TokenUsage {
	inputTokens := len(strings.Fields(input))
	outputTokens := len(strings.Fields(output))
	return plugins.TokenUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	}
}
