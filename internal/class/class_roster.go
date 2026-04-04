package class

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// NoteRosterEntry maps a user-visible label to source-matching hints and a
// stable order so users can explicitly define course progression.
type NoteRosterEntry struct {
	Label         string   `json:"label" yaml:"label"`
	SourcePattern string   `json:"source_pattern,omitempty" yaml:"source_pattern,omitempty"`
	Tags          []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Week          int      `json:"week,omitempty" yaml:"week,omitempty"`
	Order         int      `json:"order,omitempty" yaml:"order,omitempty"`
}

// NoteRoster stores class note ordering preferences.
type NoteRoster struct {
	Class   string            `json:"class" yaml:"class"`
	Entries []NoteRosterEntry `json:"entries" yaml:"entries"`
}

// ScopeGroup defines one weighted material bucket for an assessment scope.
type ScopeGroup struct {
	Labels         []string `json:"labels,omitempty" yaml:"labels,omitempty"`
	SourcePatterns []string `json:"source_patterns,omitempty" yaml:"source_patterns,omitempty"`
	Tags           []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Weight         float64  `json:"weight" yaml:"weight"`
}

// CoverageScope configures weighted material selection for a profile kind.
type CoverageScope struct {
	Class            string       `json:"class" yaml:"class"`
	Kind             string       `json:"kind" yaml:"kind"`
	ExcludeUnmatched bool         `json:"exclude_unmatched,omitempty" yaml:"exclude_unmatched,omitempty"`
	Groups           []ScopeGroup `json:"groups" yaml:"groups"`
}

const noteRosterFileName = "roster.yaml"

// NoteRosterPath returns the on-disk file path for class note roster config.
func NoteRosterPath(className string) (string, error) {
	return classFilePath(className, noteRosterFileName)
}

// CoverageScopePath returns the on-disk file path for class coverage scope.
func CoverageScopePath(className, kind string) (string, error) {
	return classFilePath(className, coverageScopeFileName(kind))
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
