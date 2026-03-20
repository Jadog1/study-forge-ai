package state

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
)

// Section is a grouped learning unit composed from one or more sources.
type Section struct {
	ID             string    `json:"id" yaml:"id"`
	Class          string    `json:"class" yaml:"class"`
	Title          string    `json:"title" yaml:"title"`
	Summary        string    `json:"summary" yaml:"summary"`
	Tags           []string  `json:"tags,omitempty" yaml:"tags,omitempty"`
	Concepts       []string  `json:"concepts,omitempty" yaml:"concepts,omitempty"`
	SourcePaths    []string  `json:"source_paths,omitempty" yaml:"source_paths,omitempty"`
	SourceTags     []string  `json:"source_tags,omitempty" yaml:"source_tags,omitempty"`
	ComponentIDs   []string  `json:"component_ids,omitempty" yaml:"component_ids,omitempty"`
	Embedding      []float64 `json:"embedding,omitempty" yaml:"embedding,omitempty"`
	EmbeddingModel string    `json:"embedding_model,omitempty" yaml:"embedding_model,omitempty"`
	CreatedAt      time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" yaml:"updated_at"`
}

// Component is a granular learning unit, linked to a section.
type Component struct {
	ID             string    `json:"id" yaml:"id"`
	SectionID      string    `json:"section_id" yaml:"section_id"`
	Class          string    `json:"class" yaml:"class"`
	Kind           string    `json:"kind" yaml:"kind"`
	Content        string    `json:"content" yaml:"content"`
	Tags           []string  `json:"tags,omitempty" yaml:"tags,omitempty"`
	Concepts       []string  `json:"concepts,omitempty" yaml:"concepts,omitempty"`
	SourcePaths    []string  `json:"source_paths,omitempty" yaml:"source_paths,omitempty"`
	SourceTags     []string  `json:"source_tags,omitempty" yaml:"source_tags,omitempty"`
	Embedding      []float64 `json:"embedding,omitempty" yaml:"embedding,omitempty"`
	EmbeddingModel string    `json:"embedding_model,omitempty" yaml:"embedding_model,omitempty"`
	CreatedAt      time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" yaml:"updated_at"`
}

// SectionIndex stores all known sections.
type SectionIndex struct {
	SchemaVersion int       `json:"schema_version"`
	Sections      []Section `json:"sections"`
}

// ComponentIndex stores all known components.
type ComponentIndex struct {
	SchemaVersion int         `json:"schema_version"`
	Components    []Component `json:"components"`
}

// UsageEvent captures per-call model usage for cost accounting.
type UsageEvent struct {
	ID           string    `json:"id"`
	Operation    string    `json:"operation"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	RequestID    string    `json:"request_id,omitempty"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	CostUSD      float64   `json:"cost_usd,omitempty"`
	Class        string    `json:"class,omitempty"`
	SourcePath   string    `json:"source_path,omitempty"`
	IngestRunID  string    `json:"ingest_run_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// UsageLedger stores all usage events.
type UsageLedger struct {
	Events []UsageEvent `json:"events"`
}

// UsageTotals stores aggregate totals by provider+model.
type UsageTotals struct {
	TotalInputTokens  int               `json:"total_input_tokens"`
	TotalOutputTokens int               `json:"total_output_tokens"`
	TotalTokens       int               `json:"total_tokens"`
	TotalCostUSD      float64           `json:"total_cost_usd"`
	ByModel           map[string]Totals `json:"by_model"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type Totals struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

func sectionIndexPath() (string, error) {
	return config.Path("knowledge", "sections", "index.json")
}

func componentIndexPath() (string, error) {
	return config.Path("knowledge", "components", "index.json")
}

func usageLedgerPath() (string, error) {
	return config.Path("usage", "ledger.json")
}

func usageTotalsPath() (string, error) {
	return config.Path("usage", "totals.json")
}

func LoadSectionIndex() (*SectionIndex, error) {
	path, err := sectionIndexPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SectionIndex{SchemaVersion: 1}, nil
		}
		return nil, fmt.Errorf("read section index: %w", err)
	}
	var idx SectionIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse section index: %w", err)
	}
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = 1
	}
	return &idx, nil
}

func SaveSectionIndex(idx *SectionIndex) error {
	if idx == nil {
		return fmt.Errorf("section index is nil")
	}
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = 1
	}
	path, err := sectionIndexPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create section index dir: %w", err)
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal section index: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func LoadComponentIndex() (*ComponentIndex, error) {
	path, err := componentIndexPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ComponentIndex{SchemaVersion: 1}, nil
		}
		return nil, fmt.Errorf("read component index: %w", err)
	}
	var idx ComponentIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse component index: %w", err)
	}
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = 1
	}
	return &idx, nil
}

func SaveComponentIndex(idx *ComponentIndex) error {
	if idx == nil {
		return fmt.Errorf("component index is nil")
	}
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = 1
	}
	path, err := componentIndexPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create component index dir: %w", err)
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal component index: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func (idx *SectionIndex) AddOrUpdate(section Section) {
	section = normalizeSection(section)
	for i := range idx.Sections {
		if idx.Sections[i].ID == section.ID {
			idx.Sections[i] = mergeSection(idx.Sections[i], section)
			return
		}
	}
	idx.Sections = append(idx.Sections, section)
}

func (idx *ComponentIndex) AddOrUpdate(component Component) {
	component = normalizeComponent(component)
	for i := range idx.Components {
		if idx.Components[i].ID == component.ID {
			idx.Components[i] = mergeComponent(idx.Components[i], component)
			return
		}
	}
	idx.Components = append(idx.Components, component)
}

func normalizeSection(section Section) Section {
	section.Title = strings.TrimSpace(section.Title)
	section.Summary = strings.TrimSpace(section.Summary)
	section.Tags = dedupe(section.Tags)
	section.Concepts = dedupe(section.Concepts)
	section.SourcePaths = dedupe(section.SourcePaths)
	section.SourceTags = dedupe(section.SourceTags)
	section.ComponentIDs = dedupe(section.ComponentIDs)
	if section.CreatedAt.IsZero() {
		section.CreatedAt = time.Now().UTC()
	}
	section.UpdatedAt = time.Now().UTC()
	return section
}

func normalizeComponent(component Component) Component {
	component.Kind = strings.TrimSpace(component.Kind)
	component.Content = strings.TrimSpace(component.Content)
	component.Tags = dedupe(component.Tags)
	component.Concepts = dedupe(component.Concepts)
	component.SourcePaths = dedupe(component.SourcePaths)
	component.SourceTags = dedupe(component.SourceTags)
	if component.CreatedAt.IsZero() {
		component.CreatedAt = time.Now().UTC()
	}
	component.UpdatedAt = time.Now().UTC()
	return component
}

func mergeSection(existing, incoming Section) Section {
	merged := existing
	if incoming.Title != "" {
		merged.Title = incoming.Title
	}
	if incoming.Summary != "" {
		merged.Summary = incoming.Summary
	}
	if incoming.Class != "" {
		merged.Class = incoming.Class
	}
	merged.Tags = dedupe(append(existing.Tags, incoming.Tags...))
	merged.Concepts = dedupe(append(existing.Concepts, incoming.Concepts...))
	merged.SourcePaths = dedupe(append(existing.SourcePaths, incoming.SourcePaths...))
	merged.SourceTags = dedupe(append(existing.SourceTags, incoming.SourceTags...))
	merged.ComponentIDs = dedupe(append(existing.ComponentIDs, incoming.ComponentIDs...))
	if len(incoming.Embedding) > 0 {
		merged.Embedding = incoming.Embedding
		merged.EmbeddingModel = incoming.EmbeddingModel
	}
	if merged.CreatedAt.IsZero() {
		merged.CreatedAt = time.Now().UTC()
	}
	merged.UpdatedAt = time.Now().UTC()
	return merged
}

func mergeComponent(existing, incoming Component) Component {
	merged := existing
	if incoming.SectionID != "" {
		merged.SectionID = incoming.SectionID
	}
	if incoming.Class != "" {
		merged.Class = incoming.Class
	}
	if incoming.Kind != "" {
		merged.Kind = incoming.Kind
	}
	if incoming.Content != "" {
		merged.Content = incoming.Content
	}
	merged.Tags = dedupe(append(existing.Tags, incoming.Tags...))
	merged.Concepts = dedupe(append(existing.Concepts, incoming.Concepts...))
	merged.SourcePaths = dedupe(append(existing.SourcePaths, incoming.SourcePaths...))
	merged.SourceTags = dedupe(append(existing.SourceTags, incoming.SourceTags...))
	if len(incoming.Embedding) > 0 {
		merged.Embedding = incoming.Embedding
		merged.EmbeddingModel = incoming.EmbeddingModel
	}
	if merged.CreatedAt.IsZero() {
		merged.CreatedAt = time.Now().UTC()
	}
	merged.UpdatedAt = time.Now().UTC()
	return merged
}

func AppendUsageEvent(event UsageEvent) error {
	ledger, err := LoadUsageLedger()
	if err != nil {
		return err
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("usage-%d", time.Now().UnixNano())
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	ledger.Events = append(ledger.Events, event)
	if err := SaveUsageLedger(ledger); err != nil {
		return err
	}
	return updateUsageTotals(ledger)
}

func LoadUsageLedger() (*UsageLedger, error) {
	path, err := usageLedgerPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UsageLedger{}, nil
		}
		return nil, fmt.Errorf("read usage ledger: %w", err)
	}
	var ledger UsageLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return nil, fmt.Errorf("parse usage ledger: %w", err)
	}
	return &ledger, nil
}

func SaveUsageLedger(ledger *UsageLedger) error {
	if ledger == nil {
		return fmt.Errorf("usage ledger is nil")
	}
	path, err := usageLedgerPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create usage dir: %w", err)
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal usage ledger: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func updateUsageTotals(ledger *UsageLedger) error {
	totals := UsageTotals{ByModel: make(map[string]Totals)}
	for _, event := range ledger.Events {
		totals.TotalInputTokens += event.InputTokens
		totals.TotalOutputTokens += event.OutputTokens
		totals.TotalTokens += event.TotalTokens
		totals.TotalCostUSD += event.CostUSD
		key := strings.TrimSpace(event.Provider + ":" + event.Model)
		modelTotals := totals.ByModel[key]
		modelTotals.InputTokens += event.InputTokens
		modelTotals.OutputTokens += event.OutputTokens
		modelTotals.TotalTokens += event.TotalTokens
		modelTotals.CostUSD += event.CostUSD
		totals.ByModel[key] = modelTotals
	}
	totals.TotalCostUSD = roundFloat(totals.TotalCostUSD)
	for key, m := range totals.ByModel {
		m.CostUSD = roundFloat(m.CostUSD)
		totals.ByModel[key] = m
	}
	totals.UpdatedAt = time.Now().UTC()

	path, err := usageTotalsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create usage totals dir: %w", err)
	}
	data, err := json.MarshalIndent(totals, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal usage totals: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func roundFloat(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

// SearchSectionsByEmbedding finds nearest sections by cosine similarity.
func SearchSectionsByEmbedding(index *SectionIndex, embedding []float64, topK int) []Section {
	if index == nil || len(embedding) == 0 || topK <= 0 {
		return nil
	}
	type scored struct {
		section Section
		score   float64
	}
	var scoredSections []scored
	for _, section := range index.Sections {
		score := CosineSimilarity(embedding, section.Embedding)
		if score <= 0 {
			continue
		}
		scoredSections = append(scoredSections, scored{section: section, score: score})
	}
	sort.Slice(scoredSections, func(i, j int) bool { return scoredSections[i].score > scoredSections[j].score })
	if len(scoredSections) > topK {
		scoredSections = scoredSections[:topK]
	}
	results := make([]Section, 0, len(scoredSections))
	for _, row := range scoredSections {
		results = append(results, row.section)
	}
	return results
}

// SearchComponentsByEmbedding finds nearest components by cosine similarity.
func SearchComponentsByEmbedding(index *ComponentIndex, embedding []float64, topK int) []Component {
	if index == nil || len(embedding) == 0 || topK <= 0 {
		return nil
	}
	type scored struct {
		component Component
		score     float64
	}
	var scoredComponents []scored
	for _, component := range index.Components {
		score := CosineSimilarity(embedding, component.Embedding)
		if score <= 0 {
			continue
		}
		scoredComponents = append(scoredComponents, scored{component: component, score: score})
	}
	sort.Slice(scoredComponents, func(i, j int) bool { return scoredComponents[i].score > scoredComponents[j].score })
	if len(scoredComponents) > topK {
		scoredComponents = scoredComponents[:topK]
	}
	results := make([]Component, 0, len(scoredComponents))
	for _, row := range scoredComponents {
		results = append(results, row.component)
	}
	return results
}

// CosineSimilarity computes cosine similarity for equal-length vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
