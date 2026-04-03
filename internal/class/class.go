// Package class manages class directories, syllabi, and rule files under
// ~/.study-forge-ai/classes/<name>/.
package class

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/studyforge/study-agent/internal/config"
	"gopkg.in/yaml.v3"
)

// Syllabus lists the weekly/daily topics for a class.
type Syllabus struct {
	Class  string  `yaml:"class"`
	Topics []Topic `yaml:"topics"`
}

// Topic is a single entry in the syllabus.
type Topic struct {
	Week        int      `yaml:"week,omitempty"`
	Day         string   `yaml:"day,omitempty"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

// Rules captures exam expectations and question style preferences.
type Rules struct {
	Class          string   `yaml:"class"`
	ExamExpect     string   `yaml:"exam_expectations,omitempty"`
	QuestionStyles []string `yaml:"question_styles,omitempty"`
	Notes          string   `yaml:"notes,omitempty"`
}

// Context tracks external file paths used as class-level AI context.
type Context struct {
	Class        string   `yaml:"class"`
	ContextFiles []string `yaml:"context_files"`
}

// NoteRosterEntry maps a user-visible label to source-matching hints and a
// stable order so users can explicitly define course progression.
type NoteRosterEntry struct {
	Label         string   `yaml:"label"`
	SourcePattern string   `yaml:"source_pattern,omitempty"`
	Tags          []string `yaml:"tags,omitempty"`
	Week          int      `yaml:"week,omitempty"`
	Order         int      `yaml:"order,omitempty"`
}

// NoteRoster stores class note ordering preferences.
type NoteRoster struct {
	Class   string            `yaml:"class"`
	Entries []NoteRosterEntry `yaml:"entries"`
}

// ScopeGroup defines one weighted material bucket for an assessment scope.
type ScopeGroup struct {
	Labels         []string `yaml:"labels,omitempty"`
	SourcePatterns []string `yaml:"source_patterns,omitempty"`
	Tags           []string `yaml:"tags,omitempty"`
	Weight         float64  `yaml:"weight"`
}

// CoverageScope configures weighted material selection for a profile kind.
type CoverageScope struct {
	Class            string       `yaml:"class"`
	Kind             string       `yaml:"kind"`
	ExcludeUnmatched bool         `yaml:"exclude_unmatched,omitempty"`
	Groups           []ScopeGroup `yaml:"groups"`
}

const noteRosterFileName = "roster.yaml"

// ContextProfile describes one assessment-specific class context profile.
// Add new entries to defaultContextProfiles to extend supported profile kinds.
type ContextProfile struct {
	Kind                string
	Label               string
	FileName            string
	DefaultQuestionType string
}

var defaultContextProfiles = []ContextProfile{
	{
		Kind:                "quiz",
		Label:               "Quiz",
		FileName:            "context.quiz.md",
		DefaultQuestionType: "multiple-choice",
	},
	{
		Kind:                "exam",
		Label:               "Exam",
		FileName:            "context.exam.md",
		DefaultQuestionType: "short-answer",
	},
	{
		Kind:                "focused",
		Label:               "Focused",
		FileName:            "context.focused.md",
		DefaultQuestionType: "multiple-choice",
	},
}

func classDir(name string) (string, error) {
	return config.Path("classes", name)
}

func classFilePath(name, fileName string) (string, error) {
	dir, err := classDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// Create scaffolds a new class directory with default syllabus and rules files.
func Create(name string) error {
	dir, err := classDir(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create class dir: %w", err)
	}

	syllabus := Syllabus{
		Class: name,
		Topics: []Topic{
			{Week: 1, Title: "Introduction", Description: "Edit this topic."},
		},
	}
	if err := writeYAML(filepath.Join(dir, "syllabus.yaml"), syllabus); err != nil {
		return fmt.Errorf("write syllabus: %w", err)
	}

	rules := Rules{
		Class:          name,
		ExamExpect:     "Conceptual understanding with worked examples",
		QuestionStyles: []string{"open-ended", "multiple-choice"},
	}
	if err := writeYAML(filepath.Join(dir, "rules.yaml"), rules); err != nil {
		return fmt.Errorf("write rules: %w", err)
	}

	ctx := Context{Class: name, ContextFiles: []string{}}
	if err := writeYAML(filepath.Join(dir, "context.yaml"), ctx); err != nil {
		return fmt.Errorf("write context: %w", err)
	}

	for _, profile := range defaultContextProfiles {
		if err := ensureProfileContextFile(name, profile); err != nil {
			return fmt.Errorf("write %s context: %w", profile.Kind, err)
		}
	}

	return nil
}

// ContextProfiles returns all supported context profiles.
func ContextProfiles() []ContextProfile {
	out := make([]ContextProfile, len(defaultContextProfiles))
	copy(out, defaultContextProfiles)
	return out
}

// NormalizeContextProfile coerces unknown/empty values to the default profile.
func NormalizeContextProfile(kind string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	if _, ok := contextProfileForKind(k); ok {
		return k
	}
	return defaultContextProfiles[0].Kind
}

// DefaultContextProfile returns the default context profile key.
func DefaultContextProfile() string {
	return defaultContextProfiles[0].Kind
}

// ContextProfilePath returns the on-disk file path for a class profile context.
func ContextProfilePath(className, profileKind string) (string, error) {
	profile, ok := contextProfileForKind(profileKind)
	if !ok {
		return "", fmt.Errorf("unknown context profile %q", profileKind)
	}
	return classFilePath(className, profile.FileName)
}

// NoteRosterPath returns the on-disk file path for class note roster config.
func NoteRosterPath(className string) (string, error) {
	return classFilePath(className, noteRosterFileName)
}

// CoverageScopePath returns the on-disk file path for class coverage scope.
func CoverageScopePath(className, kind string) (string, error) {
	return classFilePath(className, coverageScopeFileName(kind))
}

// LoadProfileContextText loads profile context text and ensures defaults exist.
func LoadProfileContextText(className, profileKind string) (string, error) {
	profile, ok := contextProfileForKind(profileKind)
	if !ok {
		return "", fmt.Errorf("unknown context profile %q", profileKind)
	}
	if err := ensureProfileContextFile(className, profile); err != nil {
		return "", err
	}
	path, err := classFilePath(className, profile.FileName)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// SaveProfileContextText writes assessment profile context text for class.
func SaveProfileContextText(className, profileKind, text string) error {
	profile, ok := contextProfileForKind(profileKind)
	if !ok {
		return fmt.Errorf("unknown context profile %q", profileKind)
	}
	path, err := classFilePath(className, profile.FileName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		trimmed = strings.TrimSpace(defaultProfileContextTemplate(profile))
	}
	if !strings.HasSuffix(trimmed, "\n") {
		trimmed += "\n"
	}
	if err := os.WriteFile(path, []byte(trimmed), 0644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

// ResolveProfileDefaultQuestionType extracts default_question_type from profile
// text; falls back to profile default and then fallback when missing.
func ResolveProfileDefaultQuestionType(className, profileKind, fallback string) string {
	profileKind = NormalizeContextProfile(profileKind)
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		fallback = "multiple-choice"
	}
	text, err := LoadProfileContextText(className, profileKind)
	if err != nil {
		if profile, ok := contextProfileForKind(profileKind); ok && strings.TrimSpace(profile.DefaultQuestionType) != "" {
			return profile.DefaultQuestionType
		}
		return fallback
	}
	if parsed := parseDefaultQuestionType(text); parsed != "" {
		return parsed
	}
	if profile, ok := contextProfileForKind(profileKind); ok && strings.TrimSpace(profile.DefaultQuestionType) != "" {
		return profile.DefaultQuestionType
	}
	return fallback
}

// LoadSyllabus reads the syllabus for the named class.
func LoadSyllabus(name string) (*Syllabus, error) {
	var s Syllabus
	path, err := classFilePath(name, "syllabus.yaml")
	if err != nil {
		return nil, err
	}
	if err := readYAML(path, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// LoadRules reads the rules for the named class.
func LoadRules(name string) (*Rules, error) {
	var r Rules
	path, err := classFilePath(name, "rules.yaml")
	if err != nil {
		return nil, err
	}
	if err := readYAML(path, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// List returns the names of all classes that have been created.
func List() ([]string, error) {
	dir, err := config.Path("classes")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list classes: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// LoadContext reads class context.yaml. If missing, returns an empty context.
func LoadContext(name string) (*Context, error) {
	path, err := classFilePath(name, "context.yaml")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Context{Class: name, ContextFiles: []string{}}, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var c Context
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	if c.Class == "" {
		c.Class = name
	}
	return &c, nil
}

// SaveContext writes class context.yaml.
func SaveContext(name string, c *Context) error {
	if c == nil {
		return fmt.Errorf("context is nil")
	}
	if c.Class == "" {
		c.Class = name
	}
	path, err := classFilePath(name, "context.yaml")
	if err != nil {
		return err
	}
	return writeYAML(path, c)
}

// AddContextFile appends a file path to class context if not present.
func AddContextFile(name, filePath string) error {
	c, err := LoadContext(name)
	if err != nil {
		return err
	}
	for _, p := range c.ContextFiles {
		if p == filePath {
			return nil
		}
	}
	c.ContextFiles = append(c.ContextFiles, filePath)
	return SaveContext(name, c)
}

// LoadNoteRoster reads class roster.yaml and returns an empty roster if absent.
func LoadNoteRoster(name string) (*NoteRoster, error) {
	path, err := classFilePath(name, noteRosterFileName)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &NoteRoster{Class: name, Entries: []NoteRosterEntry{}}, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var roster NoteRoster
	if err := yaml.Unmarshal(data, &roster); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	if roster.Class == "" {
		roster.Class = name
	}
	roster.Entries = normalizeRosterEntries(roster.Entries)
	return &roster, nil
}

// SaveNoteRoster writes class roster.yaml.
func SaveNoteRoster(name string, roster *NoteRoster) error {
	if roster == nil {
		return fmt.Errorf("roster is nil")
	}
	if roster.Class == "" {
		roster.Class = name
	}
	roster.Entries = normalizeRosterEntries(roster.Entries)
	path, err := classFilePath(name, noteRosterFileName)
	if err != nil {
		return err
	}
	return writeYAML(path, roster)
}

// UpsertNoteRosterEntry inserts or replaces a roster entry by label.
func UpsertNoteRosterEntry(name string, entry NoteRosterEntry) (*NoteRoster, error) {
	roster, err := LoadNoteRoster(name)
	if err != nil {
		return nil, err
	}
	entry.Label = strings.TrimSpace(entry.Label)
	entry.SourcePattern = strings.TrimSpace(entry.SourcePattern)
	entry.Tags = dedupeStrings(entry.Tags)
	if entry.Label == "" {
		return nil, fmt.Errorf("entry label is required")
	}
	if entry.Order <= 0 {
		entry.Order = nextRosterOrder(roster.Entries)
	}
	updated := false
	for i := range roster.Entries {
		if strings.EqualFold(strings.TrimSpace(roster.Entries[i].Label), entry.Label) {
			roster.Entries[i] = entry
			updated = true
			break
		}
	}
	if !updated {
		roster.Entries = append(roster.Entries, entry)
	}
	roster.Entries = normalizeRosterEntries(roster.Entries)
	if err := SaveNoteRoster(name, roster); err != nil {
		return nil, err
	}
	return roster, nil
}

// RemoveNoteRosterEntry removes a roster entry by label.
func RemoveNoteRosterEntry(name, label string) (*NoteRoster, error) {
	roster, err := LoadNoteRoster(name)
	if err != nil {
		return nil, err
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return nil, fmt.Errorf("entry label is required")
	}
	filtered := make([]NoteRosterEntry, 0, len(roster.Entries))
	removed := false
	for _, entry := range roster.Entries {
		if strings.EqualFold(strings.TrimSpace(entry.Label), label) {
			removed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !removed {
		return nil, fmt.Errorf("roster entry %q not found", label)
	}
	roster.Entries = normalizeRosterEntries(filtered)
	if err := SaveNoteRoster(name, roster); err != nil {
		return nil, err
	}
	return roster, nil
}

// ReorderNoteRosterEntries rewrites the order field from the provided labels.
// Any entries omitted from labels are appended in their current relative order.
func ReorderNoteRosterEntries(name string, labels []string) (*NoteRoster, error) {
	roster, err := LoadNoteRoster(name)
	if err != nil {
		return nil, err
	}
	if len(roster.Entries) == 0 {
		return roster, nil
	}
	roster.Entries = normalizeRosterEntries(roster.Entries)
	entryByLabel := make(map[string]NoteRosterEntry, len(roster.Entries))
	for _, entry := range roster.Entries {
		entryByLabel[strings.ToLower(strings.TrimSpace(entry.Label))] = entry
	}
	ordered := make([]NoteRosterEntry, 0, len(roster.Entries))
	seen := make(map[string]bool, len(roster.Entries))
	for _, label := range labels {
		key := strings.ToLower(strings.TrimSpace(label))
		if key == "" || seen[key] {
			continue
		}
		entry, ok := entryByLabel[key]
		if !ok {
			return nil, fmt.Errorf("roster entry %q not found", strings.TrimSpace(label))
		}
		ordered = append(ordered, entry)
		seen[key] = true
	}
	for _, entry := range roster.Entries {
		key := strings.ToLower(strings.TrimSpace(entry.Label))
		if seen[key] {
			continue
		}
		ordered = append(ordered, entry)
	}
	for i := range ordered {
		ordered[i].Order = i + 1
	}
	roster.Entries = normalizeRosterEntries(ordered)
	if err := SaveNoteRoster(name, roster); err != nil {
		return nil, err
	}
	return roster, nil
}

// LoadCoverageScope loads class coverage.<kind>.yaml. Missing file is nil,nil.
func LoadCoverageScope(name, kind string) (*CoverageScope, error) {
	profileKind := NormalizeContextProfile(kind)
	path, err := classFilePath(name, coverageScopeFileName(profileKind))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var scope CoverageScope
	if err := yaml.Unmarshal(data, &scope); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	if scope.Class == "" {
		scope.Class = name
	}
	scope.Kind = NormalizeContextProfile(scope.Kind)
	scope.Groups = normalizeScopeGroups(scope.Groups)
	if len(scope.Groups) == 0 {
		return nil, nil
	}
	return &scope, nil
}

// SaveCoverageScope writes class coverage.<kind>.yaml.
func SaveCoverageScope(name, kind string, scope *CoverageScope) error {
	if scope == nil {
		return fmt.Errorf("coverage scope is nil")
	}
	profileKind := NormalizeContextProfile(kind)
	if scope.Class == "" {
		scope.Class = name
	}
	scope.Kind = profileKind
	scope.Groups = normalizeScopeGroups(scope.Groups)
	path, err := classFilePath(name, coverageScopeFileName(profileKind))
	if err != nil {
		return err
	}
	return writeYAML(path, scope)
}

// ResolveGroupPatterns expands roster labels to source patterns and merges
// inline source patterns.
func ResolveGroupPatterns(group ScopeGroup, roster *NoteRoster) []string {
	patterns := make([]string, 0, len(group.SourcePatterns)+len(group.Labels))
	patterns = append(patterns, group.SourcePatterns...)
	if roster != nil {
		for _, label := range group.Labels {
			entry, ok := rosterEntryByLabel(roster.Entries, label)
			if !ok {
				continue
			}
			if strings.TrimSpace(entry.SourcePattern) != "" {
				patterns = append(patterns, entry.SourcePattern)
			}
		}
	}
	return dedupeStrings(patterns)
}

func contextProfileForKind(kind string) (ContextProfile, bool) {
	k := strings.ToLower(strings.TrimSpace(kind))
	for _, profile := range defaultContextProfiles {
		if profile.Kind == k {
			return profile, true
		}
	}
	return ContextProfile{}, false
}

func ensureProfileContextFile(className string, profile ContextProfile) error {
	path, err := classFilePath(className, profile.FileName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %q: %w", path, err)
	}
	return os.WriteFile(path, []byte(defaultProfileContextTemplate(profile)), 0644)
}

func defaultProfileContextTemplate(profile ContextProfile) string {
	return fmt.Sprintf(`# %s Context

default_question_type: %s

Paste or write context that should influence %s generation.
Examples:
- expected question style
- exam/quiz format hints
- emphasized topics and prohibited topics
- time pressure, depth, and scoring preferences
`, profile.Label, profile.DefaultQuestionType, strings.ToLower(profile.Label))
}

func parseDefaultQuestionType(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		if key != "default_question_type" {
			continue
		}
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeRosterEntries(entries []NoteRosterEntry) []NoteRosterEntry {
	if len(entries) == 0 {
		return []NoteRosterEntry{}
	}
	normalized := make([]NoteRosterEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Label = strings.TrimSpace(entry.Label)
		entry.SourcePattern = strings.TrimSpace(entry.SourcePattern)
		entry.Tags = dedupeStrings(entry.Tags)
		if entry.Label == "" {
			continue
		}
		normalized = append(normalized, entry)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		left := normalized[i]
		right := normalized[j]
		if left.Order <= 0 && right.Order <= 0 {
			return strings.ToLower(left.Label) < strings.ToLower(right.Label)
		}
		if left.Order <= 0 {
			return false
		}
		if right.Order <= 0 {
			return true
		}
		if left.Order == right.Order {
			return strings.ToLower(left.Label) < strings.ToLower(right.Label)
		}
		return left.Order < right.Order
	})
	for i := range normalized {
		normalized[i].Order = i + 1
	}
	return normalized
}

func normalizeScopeGroups(groups []ScopeGroup) []ScopeGroup {
	normalized := make([]ScopeGroup, 0, len(groups))
	for _, group := range groups {
		group.Labels = dedupeStrings(group.Labels)
		group.SourcePatterns = dedupeStrings(group.SourcePatterns)
		group.Tags = dedupeStrings(group.Tags)
		if group.Weight < 0 {
			continue
		}
		normalized = append(normalized, group)
	}
	return normalized
}

func coverageScopeFileName(kind string) string {
	return fmt.Sprintf("coverage.%s.yaml", NormalizeContextProfile(kind))
}

func rosterEntryByLabel(entries []NoteRosterEntry, label string) (NoteRosterEntry, bool) {
	needle := strings.ToLower(strings.TrimSpace(label))
	for _, entry := range entries {
		if strings.ToLower(strings.TrimSpace(entry.Label)) == needle {
			return entry, true
		}
	}
	return NoteRosterEntry{}, false
}

func nextRosterOrder(entries []NoteRosterEntry) int {
	maxOrder := 0
	for _, entry := range entries {
		if entry.Order > maxOrder {
			maxOrder = entry.Order
		}
	}
	if maxOrder <= 0 {
		return len(entries) + 1
	}
	return maxOrder + 1
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeYAML(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func readYAML(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}
	return yaml.Unmarshal(data, v)
}
