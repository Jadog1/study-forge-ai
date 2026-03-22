package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/studyforge/study-agent/internal/config"
)

func TestExportKnowledgeDataset_ClassFilterAndNoEmbeddings(t *testing.T) {
	setupExportStateTest(t)

	secIdx := &SectionIndex{SchemaVersion: 1, Sections: []Section{
		{
			ID:             "sec-math-1",
			Class:          "math",
			Title:          "Derivatives",
			Summary:        "rate of change",
			Embedding:      []float64{0.1, 0.2},
			EmbeddingModel: "test-embed",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		},
		{
			ID:        "sec-history-1",
			Class:     "history",
			Title:     "Rome",
			Summary:   "empire",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}}
	cmpIdx := &ComponentIndex{SchemaVersion: 1, Components: []Component{
		{
			ID:             "cmp-math-1",
			SectionID:      "sec-math-1",
			Class:          "math",
			Kind:           "fact",
			Content:        "Product rule",
			Embedding:      []float64{0.4, 0.5},
			EmbeddingModel: "test-embed",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		},
		{
			ID:        "cmp-history-1",
			SectionID: "sec-history-1",
			Class:     "history",
			Kind:      "fact",
			Content:   "Pax Romana",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}}

	if err := SaveSectionIndex(secIdx); err != nil {
		t.Fatalf("save section index: %v", err)
	}
	if err := SaveComponentIndex(cmpIdx); err != nil {
		t.Fatalf("save component index: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "knowledge-math.json")
	res, err := ExportKnowledgeDataset(outputPath, KnowledgeExportOptions{Class: "math", IncludeEmbeddings: false})
	if err != nil {
		t.Fatalf("export knowledge dataset: %v", err)
	}
	if res.Sections != 1 || res.Components != 1 {
		t.Fatalf("unexpected export counts: %+v", res)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read export output: %v", err)
	}
	var bundle KnowledgeExportBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("parse export output: %v", err)
	}
	if bundle.Class != "math" {
		t.Fatalf("expected class filter to be recorded, got %q", bundle.Class)
	}
	if len(bundle.Sections) != 1 || bundle.Sections[0].Class != "math" {
		t.Fatalf("expected only math section, got %#v", bundle.Sections)
	}
	if len(bundle.Components) != 1 || bundle.Components[0].Class != "math" {
		t.Fatalf("expected only math component, got %#v", bundle.Components)
	}
	if len(bundle.Sections[0].Embedding) != 0 || bundle.Sections[0].EmbeddingModel != "" {
		t.Fatalf("expected section embeddings to be stripped, got %#v", bundle.Sections[0])
	}
	if len(bundle.Components[0].Embedding) != 0 || bundle.Components[0].EmbeddingModel != "" {
		t.Fatalf("expected component embeddings to be stripped, got %#v", bundle.Components[0])
	}
}

func TestExportKnowledgeDataset_IncludeEmbeddings(t *testing.T) {
	setupExportStateTest(t)

	secIdx := &SectionIndex{SchemaVersion: 1, Sections: []Section{{
		ID:             "sec-1",
		Class:          "science",
		Title:          "Atoms",
		Summary:        "matter basics",
		Embedding:      []float64{0.7, 0.8},
		EmbeddingModel: "model-x",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}}}
	cmpIdx := &ComponentIndex{SchemaVersion: 1, Components: []Component{{
		ID:             "cmp-1",
		SectionID:      "sec-1",
		Class:          "science",
		Kind:           "definition",
		Content:        "An atom is...",
		Embedding:      []float64{0.1, 0.2},
		EmbeddingModel: "model-x",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}}}

	if err := SaveSectionIndex(secIdx); err != nil {
		t.Fatalf("save section index: %v", err)
	}
	if err := SaveComponentIndex(cmpIdx); err != nil {
		t.Fatalf("save component index: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "knowledge-full.json")
	if _, err := ExportKnowledgeDataset(outputPath, KnowledgeExportOptions{IncludeEmbeddings: true}); err != nil {
		t.Fatalf("export knowledge dataset: %v", err)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read export output: %v", err)
	}
	var bundle KnowledgeExportBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("parse export output: %v", err)
	}
	if len(bundle.Sections) != 1 || len(bundle.Sections[0].Embedding) == 0 || bundle.Sections[0].EmbeddingModel == "" {
		t.Fatalf("expected section embeddings in export, got %#v", bundle.Sections)
	}
	if len(bundle.Components) != 1 || len(bundle.Components[0].Embedding) == 0 || bundle.Components[0].EmbeddingModel == "" {
		t.Fatalf("expected component embeddings in export, got %#v", bundle.Components)
	}
}

func setupExportStateTest(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	})
	_ = os.Setenv("HOME", tmp)
	_ = os.Setenv("USERPROFILE", tmp)

	if _, err := config.EnsureInitialized(); err != nil {
		t.Fatalf("ensure initialized: %v", err)
	}
}
