// Package state manages all persistent data stored under ~/.study-forge-ai/.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/studyforge/study-agent/internal/config"
)

// ── Note ─────────────────────────────────────────────────────────────────────

// Note is the processed representation of a single source file.
type Note struct {
	ID        string    `yaml:"id"         json:"id"`
	Source    string    `yaml:"source"     json:"source"`
	Class     string    `yaml:"class"      json:"class"`
	Summary   string    `yaml:"summary"    json:"summary"`
	Tags      []string  `yaml:"tags"       json:"tags"`
	Concepts  []string  `yaml:"concepts"   json:"concepts"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// ── Quiz ─────────────────────────────────────────────────────────────────────

// QuizSection is a single question inside a quiz YAML document.
// This is also the format consumed by studyforge.
type QuizSection struct {
	Type      string   `yaml:"type"      json:"type"`
	ID        string   `yaml:"id"        json:"id"`
	Question  string   `yaml:"question"  json:"question"`
	Hint      string   `yaml:"hint"      json:"hint"`
	Answer    string   `yaml:"answer"    json:"answer"`
	Reasoning string   `yaml:"reasoning" json:"reasoning"`
	Tags      []string `yaml:"tags"      json:"tags"`
}

// Quiz is a complete quiz document ready to be written to disk and handed
// to studyforge for HTML rendering.
type Quiz struct {
	Title    string        `yaml:"title"    json:"title"`
	Class    string        `yaml:"class"    json:"class"`
	Tags     []string      `yaml:"tags"     json:"tags"`
	Sections []QuizSection `yaml:"sections" json:"sections"`
}

// ── Results ──────────────────────────────────────────────────────────────────

// QuizResult records whether the user answered a single question correctly.
type QuizResult struct {
	QuestionID string `json:"question_id"`
	Correct    bool   `json:"correct"`
	TimeSpent  int    `json:"time_spent"` // seconds
}

// QuizResults is the full result set for one completed quiz session.
type QuizResults struct {
	QuizID      string       `json:"quiz_id"`
	CompletedAt time.Time    `json:"completed_at"`
	Results     []QuizResult `json:"results"`
}

// ── Notes index ──────────────────────────────────────────────────────────────

// NotesIndex is the flat index of every processed note kept at
// ~/.study-forge-ai/notes/processed/index.json.
type NotesIndex struct {
	Notes []Note `json:"notes"`
}

func notesIndexPath() (string, error) {
	return config.Path("notes", "processed", "index.json")
}

// LoadNotesIndex reads the notes index from disk. Returns an empty index if
// the file does not exist yet.
func LoadNotesIndex() (*NotesIndex, error) {
	path, err := notesIndexPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &NotesIndex{}, nil
		}
		return nil, fmt.Errorf("read notes index: %w", err)
	}
	var idx NotesIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse notes index: %w", err)
	}
	return &idx, nil
}

// SaveNotesIndex writes the notes index to disk, creating directories as needed.
func SaveNotesIndex(idx *NotesIndex) error {
	path, err := notesIndexPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create notes index dir: %w", err)
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal notes index: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// AddOrUpdate replaces an existing note with the same ID or appends a new one.
func (idx *NotesIndex) AddOrUpdate(note Note) {
	for i, n := range idx.Notes {
		if n.ID == note.ID {
			idx.Notes[i] = note
			return
		}
	}
	idx.Notes = append(idx.Notes, note)
}

// ── Quiz results ─────────────────────────────────────────────────────────────

func quizResultsPath(class, quizID string) (string, error) {
	return config.Path("quizzes", class, quizID+"-results.json")
}

// SaveQuizResults persists quiz results under ~/.study-forge-ai/quizzes/<class>/.
func SaveQuizResults(results *QuizResults, class, quizID string) error {
	path, err := quizResultsPath(class, quizID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create results dir: %w", err)
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadQuizResults reads previously saved quiz results from disk.
func LoadQuizResults(class, quizID string) (*QuizResults, error) {
	path, err := quizResultsPath(class, quizID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read quiz results: %w", err)
	}
	var results QuizResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse quiz results: %w", err)
	}
	return &results, nil
}
