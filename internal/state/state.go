// Package state manages all persistent data stored under ~/.study-forge-ai/.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
)

// ── Note ─────────────────────────────────────────────────────────────────────

// Note is the processed representation of a single source file.
type Note struct {
	ID        string    `yaml:"id"         json:"id"`
	Source    string    `yaml:"source"     json:"source"`
	SourceTag string    `yaml:"source_tag" json:"source_tag,omitempty"`
	Sources   []string  `yaml:"sources"    json:"sources,omitempty"`
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
	Type        string   `yaml:"type"                       json:"type"`
	ID          string   `yaml:"id"                         json:"id"`
	Question    string   `yaml:"question"                   json:"question"`
	Hint        string   `yaml:"hint"                       json:"hint"`
	Answer      string   `yaml:"answer"                     json:"answer"`
	Reasoning   string   `yaml:"reasoning"                  json:"reasoning"`
	SectionID   string   `yaml:"section_id,omitempty"   json:"section_id,omitempty"`
	ComponentID string   `yaml:"component_id,omitempty" json:"component_id,omitempty"`
	Tags        []string `yaml:"tags"                       json:"tags"`
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
	QuestionID  string    `json:"question_id"`
	Correct     bool      `json:"correct"`
	TimeSpent   int       `json:"time_spent"` // seconds
	UserAnswer  string    `json:"user_answer,omitempty"`
	AnsweredAt  time.Time `json:"answered_at,omitempty"`
	SectionID   string    `json:"section_id,omitempty"`
	ComponentID string    `json:"component_id,omitempty"`
}

// QuizResults is the full result set for one completed quiz session.
type QuizResults struct {
	QuizID      string       `json:"quiz_id"`
	CompletedAt time.Time    `json:"completed_at"`
	Results     []QuizResult `json:"results"`
}

// TrackedQuizRecord stores generated quiz metadata for sfq tracked-session sync.
type TrackedQuizRecord struct {
	QuizID         string    `json:"quiz_id"`
	Class          string    `json:"class"`
	QuizPath       string    `json:"quiz_path"`
	SFQPath        string    `json:"sfq_path"`
	RegisteredAt   time.Time `json:"registered_at"`
	LastSessionID  string    `json:"last_session_id,omitempty"`
	LastImportedAt time.Time `json:"last_imported_at,omitempty"`
}

// TrackedQuizCache tracks generated quizzes and imported sfq session IDs.
type TrackedQuizCache struct {
	SchemaVersion      int                 `json:"schema_version"`
	Quizzes            []TrackedQuizRecord `json:"quizzes"`
	ImportedSessionIDs []string            `json:"imported_session_ids,omitempty"`
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
	note = normalizeSources(note)

	for i, n := range idx.Notes {
		if sameSource(n, note) {
			idx.Notes[i] = mergeNotes(n, note)
			return
		}
	}

	for i, n := range idx.Notes {
		if n.ID == note.ID {
			idx.Notes[i] = mergeNotes(n, note)
			return
		}
	}
	idx.Notes = append(idx.Notes, note)
}

func sameSource(a, b Note) bool {
	if a.Source == "" || b.Source == "" {
		return false
	}
	return a.Source == b.Source
}

func normalizeSources(note Note) Note {
	if note.Source != "" {
		note.Sources = appendUnique(note.Sources, note.Source)
	}
	if note.SourceTag != "" {
		note.Tags = appendUnique(note.Tags, "source:"+note.SourceTag)
	}
	note.Tags = dedupe(note.Tags)
	note.Concepts = dedupe(note.Concepts)
	note.Sources = dedupe(note.Sources)
	return note
}

func mergeNotes(existing, incoming Note) Note {
	merged := existing

	if incoming.Summary != "" {
		merged.Summary = incoming.Summary
	}
	if incoming.Class != "" {
		merged.Class = incoming.Class
	}
	if incoming.Source != "" {
		merged.Source = incoming.Source
	}
	if incoming.SourceTag != "" {
		merged.SourceTag = incoming.SourceTag
	}
	if !incoming.CreatedAt.IsZero() {
		merged.CreatedAt = incoming.CreatedAt
	}

	merged.Tags = dedupe(append(existing.Tags, incoming.Tags...))
	merged.Concepts = dedupe(append(existing.Concepts, incoming.Concepts...))
	merged.Sources = dedupe(append(existing.Sources, incoming.Sources...))

	if merged.Source != "" {
		merged.Sources = appendUnique(merged.Sources, merged.Source)
	}
	if merged.SourceTag != "" {
		merged.Tags = appendUnique(merged.Tags, "source:"+merged.SourceTag)
	}

	return merged
}

func appendUnique(items []string, value string) []string {
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func dedupe(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

// ── Quiz results ─────────────────────────────────────────────────────────────

func quizResultsPath(class, quizID string) (string, error) {
	return config.Path("quizzes", class, quizID+"-results.json")
}

func trackedQuizCachePath() (string, error) {
	return config.Path("quizzes", "tracked_cache.json")
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

// LoadTrackedQuizCache reads tracked quiz cache data from disk.
func LoadTrackedQuizCache() (*TrackedQuizCache, error) {
	path, err := trackedQuizCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TrackedQuizCache{SchemaVersion: 1}, nil
		}
		return nil, fmt.Errorf("read tracked quiz cache: %w", err)
	}
	var cache TrackedQuizCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parse tracked quiz cache: %w", err)
	}
	if cache.SchemaVersion == 0 {
		cache.SchemaVersion = 1
	}
	cache.ImportedSessionIDs = dedupe(cache.ImportedSessionIDs)
	return &cache, nil
}

// SaveTrackedQuizCache writes tracked quiz cache data to disk.
func SaveTrackedQuizCache(cache *TrackedQuizCache) error {
	if cache == nil {
		return fmt.Errorf("tracked quiz cache is nil")
	}
	if cache.SchemaVersion == 0 {
		cache.SchemaVersion = 1
	}
	cache.ImportedSessionIDs = dedupe(cache.ImportedSessionIDs)

	path, err := trackedQuizCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create tracked quiz cache dir: %w", err)
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tracked quiz cache: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// RegisterTrackedQuiz records a generated quiz as eligible for sfq session sync.
func RegisterTrackedQuiz(class, quizPath, sfqPath string) (*TrackedQuizRecord, error) {
	cache, err := LoadTrackedQuizCache()
	if err != nil {
		return nil, err
	}
	quizID := strings.TrimSuffix(filepath.Base(quizPath), ".yaml")
	record := TrackedQuizRecord{
		QuizID:       quizID,
		Class:        strings.TrimSpace(class),
		QuizPath:     strings.TrimSpace(quizPath),
		SFQPath:      strings.TrimSpace(sfqPath),
		RegisteredAt: time.Now().UTC(),
	}

	for i := range cache.Quizzes {
		if cache.Quizzes[i].QuizPath == record.QuizPath {
			existing := cache.Quizzes[i]
			record.LastSessionID = existing.LastSessionID
			record.LastImportedAt = existing.LastImportedAt
			if existing.RegisteredAt.IsZero() {
				record.RegisteredAt = time.Now().UTC()
			} else {
				record.RegisteredAt = existing.RegisteredAt
			}
			cache.Quizzes[i] = record
			if err := SaveTrackedQuizCache(cache); err != nil {
				return nil, err
			}
			return &record, nil
		}
	}

	cache.Quizzes = append(cache.Quizzes, record)
	if err := SaveTrackedQuizCache(cache); err != nil {
		return nil, err
	}
	return &record, nil
}

// IsSessionImported returns true when a session ID has already been synced.
func (cache *TrackedQuizCache) IsSessionImported(sessionID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || cache == nil {
		return false
	}
	for _, importedID := range cache.ImportedSessionIDs {
		if importedID == sessionID {
			return true
		}
	}
	return false
}

// MarkSessionImported marks sessionID as synced and updates quiz import metadata.
func (cache *TrackedQuizCache) MarkSessionImported(sessionID, quizPath string, importedAt time.Time) {
	if cache == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	quizPath = strings.TrimSpace(quizPath)
	if sessionID != "" && !cache.IsSessionImported(sessionID) {
		cache.ImportedSessionIDs = append(cache.ImportedSessionIDs, sessionID)
	}
	if importedAt.IsZero() {
		importedAt = time.Now().UTC()
	}
	for i := range cache.Quizzes {
		if cache.Quizzes[i].QuizPath != quizPath {
			continue
		}
		cache.Quizzes[i].LastSessionID = sessionID
		cache.Quizzes[i].LastImportedAt = importedAt.UTC()
	}
}
