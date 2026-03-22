package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// KnowledgeExportOptions controls how knowledge data is exported.
type KnowledgeExportOptions struct {
	Class             string
	IncludeEmbeddings bool
}

// KnowledgeExportBundle is the on-disk representation for shared knowledge exports.
type KnowledgeExportBundle struct {
	SchemaVersion int         `json:"schema_version"`
	ExportedAt    time.Time   `json:"exported_at"`
	Class         string      `json:"class,omitempty"`
	Sections      []Section   `json:"sections"`
	Components    []Component `json:"components"`
}

// KnowledgeExportResult summarizes an export operation.
type KnowledgeExportResult struct {
	OutputPath        string
	Class             string
	Sections          int
	Components        int
	IncludeEmbeddings bool
}

// ExportKnowledgeDataset writes sections/components to outputPath in JSON format.
func ExportKnowledgeDataset(outputPath string, opts KnowledgeExportOptions) (KnowledgeExportResult, error) {
	result := KnowledgeExportResult{}
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return result, fmt.Errorf("output path is required")
	}

	sectionIdx, err := LoadSectionIndex()
	if err != nil {
		return result, fmt.Errorf("load section index: %w", err)
	}
	componentIdx, err := LoadComponentIndex()
	if err != nil {
		return result, fmt.Errorf("load component index: %w", err)
	}

	className := strings.TrimSpace(opts.Class)
	sections := make([]Section, 0, len(sectionIdx.Sections))
	for _, section := range sectionIdx.Sections {
		if className != "" && !strings.EqualFold(strings.TrimSpace(section.Class), className) {
			continue
		}
		sections = append(sections, section)
	}

	components := make([]Component, 0, len(componentIdx.Components))
	for _, component := range componentIdx.Components {
		if className != "" && !strings.EqualFold(strings.TrimSpace(component.Class), className) {
			continue
		}
		components = append(components, component)
	}

	sort.Slice(sections, func(i, j int) bool {
		left := sections[i]
		right := sections[j]
		if !strings.EqualFold(left.Class, right.Class) {
			return strings.ToLower(left.Class) < strings.ToLower(right.Class)
		}
		if !strings.EqualFold(left.Title, right.Title) {
			return strings.ToLower(left.Title) < strings.ToLower(right.Title)
		}
		return strings.ToLower(left.ID) < strings.ToLower(right.ID)
	})

	sort.Slice(components, func(i, j int) bool {
		left := components[i]
		right := components[j]
		if !strings.EqualFold(left.Class, right.Class) {
			return strings.ToLower(left.Class) < strings.ToLower(right.Class)
		}
		if left.SectionID != right.SectionID {
			return strings.ToLower(left.SectionID) < strings.ToLower(right.SectionID)
		}
		if !strings.EqualFold(left.Kind, right.Kind) {
			return strings.ToLower(left.Kind) < strings.ToLower(right.Kind)
		}
		return strings.ToLower(left.ID) < strings.ToLower(right.ID)
	})

	if !opts.IncludeEmbeddings {
		for i := range sections {
			sections[i].Embedding = nil
			sections[i].EmbeddingModel = ""
		}
		for i := range components {
			components[i].Embedding = nil
			components[i].EmbeddingModel = ""
		}
	}

	bundle := KnowledgeExportBundle{
		SchemaVersion: 1,
		ExportedAt:    time.Now().UTC(),
		Class:         className,
		Sections:      sections,
		Components:    components,
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return result, fmt.Errorf("create export directory: %w", err)
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return result, fmt.Errorf("marshal knowledge export: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return result, fmt.Errorf("write knowledge export: %w", err)
	}

	absPath, absErr := filepath.Abs(outputPath)
	if absErr != nil {
		absPath = outputPath
	}

	result = KnowledgeExportResult{
		OutputPath:        absPath,
		Class:             className,
		Sections:          len(sections),
		Components:        len(components),
		IncludeEmbeddings: opts.IncludeEmbeddings,
	}
	return result, nil
}
