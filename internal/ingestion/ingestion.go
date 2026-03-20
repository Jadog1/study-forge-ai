// Package ingestion walks a folder of raw notes, calls the AI provider to
// extract metadata, and saves processed notes under ~/.study-forge-ai/notes/processed/.
package ingestion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/prompts"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/plugins"
	"gopkg.in/yaml.v3"
)

// supportedExts lists file extensions that can be ingested.
var supportedExts = map[string]bool{
	".md":  true,
	".txt": true,
	".rst": true,
}

// ProgressEvent reports ingestion stage updates for UI/CLI consumers.
type ProgressEvent struct {
	Label  string
	Detail string
	Done   bool
	Err    error
}

// IngestFolder processes every supported file inside folderPath using the
// supplied AI provider. Notes are associated with class when not empty.
// Caller is responsible for persisting the returned notes to the index.
func IngestFolder(folderPath, class string, provider plugins.AIProvider, cfg *config.Config) ([]state.Note, error) {
	return IngestFolderStream(folderPath, class, provider, cfg, nil)
}

// IngestFolderStream behaves like IngestFolder but emits incremental progress
// events via onProgress when provided.
func IngestFolderStream(folderPath, class string, provider plugins.AIProvider, cfg *config.Config, onProgress func(ProgressEvent)) ([]state.Note, error) {
	emit := func(event ProgressEvent) {
		if onProgress != nil {
			onProgress(event)
		}
	}

	emit(ProgressEvent{Label: "Discover files", Detail: folderPath})
	files, err := collectFiles(folderPath)
	if err != nil {
		emit(ProgressEvent{Label: "Discover files", Detail: folderPath, Done: true, Err: err})
		return nil, fmt.Errorf("collect files from %q: %w", folderPath, err)
	}
	emit(ProgressEvent{Label: "Discover files", Detail: fmt.Sprintf("%d file(s)", len(files)), Done: true})
	if len(files) == 0 {
		return nil, fmt.Errorf("no supported files found in %q", folderPath)
	}

	var notes []state.Note
	for _, f := range files {
		emit(ProgressEvent{Label: "Process file", Detail: f})
		fmt.Printf("  → processing %s\n", f)
		note, err := processFile(f, class, provider, cfg.CustomPromptContext)
		if err != nil {
			emit(ProgressEvent{Label: "Process file", Detail: f, Done: true, Err: err})
			fmt.Printf("  ⚠  skipping %s: %v\n", f, err)
			continue
		}
		emit(ProgressEvent{Label: "Process file", Detail: f, Done: true})
		emit(ProgressEvent{Label: "Persist note", Detail: note.ID})
		if err := saveProcessedNote(note); err != nil {
			emit(ProgressEvent{Label: "Persist note", Detail: note.ID, Done: true, Err: err})
			fmt.Printf("  ⚠  could not save %s: %v\n", note.ID, err)
		} else {
			emit(ProgressEvent{Label: "Persist note", Detail: note.ID, Done: true})
		}
		notes = append(notes, note)
	}
	return notes, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func collectFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if supportedExts[strings.ToLower(filepath.Ext(path))] {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// rawNoteResult is what the AI is asked to return — unmarshalled from YAML.
type rawNoteResult struct {
	ID       string   `yaml:"id"`
	Summary  string   `yaml:"summary"`
	Tags     []string `yaml:"tags"`
	Concepts []string `yaml:"concepts"`
}

func processFile(path, class string, provider plugins.AIProvider, customCtx string) (state.Note, error) {
	note, _, err := processFileWithMetadata(path, class, provider, customCtx)
	return note, err
}

func processFileWithMetadata(path, class string, provider plugins.AIProvider, customCtx string) (state.Note, plugins.GenerateResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return state.Note{}, plugins.GenerateResult{}, fmt.Errorf("read %q: %w", path, err)
	}

	prompt := prompts.SummarizeNote(string(content), class, customCtx)
	response, usage, err := generateWithMetadata(provider, prompt)
	if err != nil {
		return state.Note{}, plugins.GenerateResult{}, fmt.Errorf("AI call: %w", err)
	}
	response = sanitizeAIYAML(response)

	var result rawNoteResult
	if err := yaml.Unmarshal([]byte(response), &result); err != nil {
		return state.Note{}, usage, fmt.Errorf("parse AI YAML response: %w\nResponse was:\n%s", err, response)
	}

	// Fallback slug if the model didn't supply one.
	if result.ID == "" {
		base := filepath.Base(path)
		result.ID = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return state.Note{
		ID:        result.ID,
		Source:    path,
		SourceTag: sourceTagFromPath(path),
		Sources:   []string{path},
		Class:     class,
		Summary:   result.Summary,
		Tags:      result.Tags,
		Concepts:  result.Concepts,
		CreatedAt: time.Now(),
	}, usage, nil
}

func sourceTagFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		return "markdown"
	case ".txt":
		return "text"
	case ".rst":
		return "restructuredtext"
	default:
		return "unknown"
	}
}

func saveProcessedNote(note state.Note) error {
	dir, err := config.Path("notes", "processed")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(note)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, note.ID+".yaml"), data, 0644)
}
