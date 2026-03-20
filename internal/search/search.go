// Package search provides tag-based and class-based note lookup.
package search

import (
	"fmt"
	"sort"
	"strings"

	"github.com/studyforge/study-agent/internal/state"
)

// Result pairs a matched note with a relevance score.
type Result struct {
	Note  state.Note
	Score int // number of matching tags
}

// KnowledgeResult is a unified match over sections/components.
type KnowledgeResult struct {
	Kind      string
	Section   state.Section
	Component state.Component
	Score     int
}

// ByTags returns notes that match any of the supplied tags, ranked by hit count.
func ByTags(tags []string) ([]Result, error) {
	idx, err := state.LoadNotesIndex()
	if err != nil {
		return nil, fmt.Errorf("load notes index: %w", err)
	}

	var results []Result
	for _, n := range idx.Notes {
		if s := matchScore(n.Tags, tags); s > 0 {
			results = append(results, Result{Note: n, Score: s})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results, nil
}

// ByClass returns all notes associated with the given class.
func ByClass(class string) ([]Result, error) {
	idx, err := state.LoadNotesIndex()
	if err != nil {
		return nil, fmt.Errorf("load notes index: %w", err)
	}

	var results []Result
	for _, n := range idx.Notes {
		if strings.EqualFold(n.Class, class) {
			results = append(results, Result{Note: n, Score: 1})
		}
	}
	return results, nil
}

// ByQuery returns notes ranked by simple text relevance across summary, tags,
// concepts, class, and source fields. When query is empty and class is set, it
// falls back to all notes for that class.
func ByQuery(query, class string, limit int) ([]Result, error) {
	idx, err := state.LoadNotesIndex()
	if err != nil {
		return nil, fmt.Errorf("load notes index: %w", err)
	}

	query = strings.TrimSpace(query)
	class = strings.TrimSpace(class)
	tokens := tokenize(query)

	var results []Result
	for _, note := range idx.Notes {
		score := queryScore(note, query, tokens, class)
		if score > 0 {
			results = append(results, Result{Note: note, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return strings.ToLower(results[i].Note.ID) < strings.ToLower(results[j].Note.ID)
		}
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// ByKnowledgeQuery returns sections/components ranked by lexical relevance.
func ByKnowledgeQuery(query, class string, limit int) ([]KnowledgeResult, error) {
	sectionIdx, err := state.LoadSectionIndex()
	if err != nil {
		return nil, fmt.Errorf("load section index: %w", err)
	}
	componentIdx, err := state.LoadComponentIndex()
	if err != nil {
		return nil, fmt.Errorf("load component index: %w", err)
	}

	query = strings.TrimSpace(query)
	class = strings.TrimSpace(class)
	tokens := tokenize(query)

	var results []KnowledgeResult
	for _, section := range sectionIdx.Sections {
		score := sectionQueryScore(section, query, tokens, class)
		if score > 0 {
			results = append(results, KnowledgeResult{Kind: "section", Section: section, Score: score})
		}
	}
	for _, component := range componentIdx.Components {
		score := componentQueryScore(component, query, tokens, class)
		if score > 0 {
			results = append(results, KnowledgeResult{Kind: "component", Component: component, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			left := strings.ToLower(results[i].Kind + ":" + knowledgeResultID(results[i]))
			right := strings.ToLower(results[j].Kind + ":" + knowledgeResultID(results[j]))
			return left < right
		}
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// matchScore returns the count of filterTags found in noteTags.
func matchScore(noteTags, filterTags []string) int {
	set := make(map[string]bool, len(noteTags))
	for _, t := range noteTags {
		set[strings.ToLower(t)] = true
	}
	count := 0
	for _, t := range filterTags {
		if set[strings.ToLower(t)] {
			count++
		}
	}
	return count
}

func queryScore(note state.Note, rawQuery string, tokens []string, class string) int {
	summary := strings.ToLower(note.Summary)
	source := strings.ToLower(note.Source)
	noteClass := strings.ToLower(note.Class)
	id := strings.ToLower(note.ID)

	tagSet := make(map[string]bool, len(note.Tags))
	for _, tag := range note.Tags {
		tagSet[strings.ToLower(tag)] = true
	}

	conceptSet := make(map[string]bool, len(note.Concepts))
	for _, concept := range note.Concepts {
		conceptSet[strings.ToLower(concept)] = true
	}

	score := 0
	if class != "" && strings.EqualFold(note.Class, class) {
		score += 4
	}

	if rawQuery == "" {
		return score
	}

	q := strings.ToLower(rawQuery)
	if strings.Contains(summary, q) {
		score += 6
	}
	if strings.Contains(source, q) || strings.Contains(noteClass, q) || strings.Contains(id, q) {
		score += 2
	}

	for _, token := range tokens {
		switch {
		case tagSet[token]:
			score += 4
		case conceptSet[token]:
			score += 4
		case strings.Contains(summary, token):
			score += 3
		case strings.Contains(source, token), strings.Contains(noteClass, token), strings.Contains(id, token):
			score += 1
		}
	}

	return score
}

func tokenize(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	seen := make(map[string]bool, len(fields))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		tokens = append(tokens, field)
	}
	return tokens
}

func sectionQueryScore(section state.Section, rawQuery string, tokens []string, class string) int {
	name := strings.ToLower(section.Title)
	summary := strings.ToLower(section.Summary)
	score := 0

	if class != "" && strings.EqualFold(section.Class, class) {
		score += 4
	}
	if rawQuery == "" {
		return score
	}

	q := strings.ToLower(rawQuery)
	if strings.Contains(name, q) || strings.Contains(summary, q) {
		score += 6
	}

	tags := make(map[string]bool, len(section.Tags))
	for _, tag := range section.Tags {
		tags[strings.ToLower(tag)] = true
	}
	concepts := make(map[string]bool, len(section.Concepts))
	for _, concept := range section.Concepts {
		concepts[strings.ToLower(concept)] = true
	}

	for _, token := range tokens {
		switch {
		case tags[token], concepts[token]:
			score += 4
		case strings.Contains(name, token), strings.Contains(summary, token):
			score += 2
		}
	}
	return score
}

func componentQueryScore(component state.Component, rawQuery string, tokens []string, class string) int {
	body := strings.ToLower(component.Content)
	kind := strings.ToLower(component.Kind)
	score := 0

	if class != "" && strings.EqualFold(component.Class, class) {
		score += 4
	}
	if rawQuery == "" {
		return score
	}

	q := strings.ToLower(rawQuery)
	if strings.Contains(body, q) || strings.Contains(kind, q) {
		score += 5
	}

	tags := make(map[string]bool, len(component.Tags))
	for _, tag := range component.Tags {
		tags[strings.ToLower(tag)] = true
	}
	concepts := make(map[string]bool, len(component.Concepts))
	for _, concept := range component.Concepts {
		concepts[strings.ToLower(concept)] = true
	}

	for _, token := range tokens {
		switch {
		case tags[token], concepts[token]:
			score += 4
		case strings.Contains(body, token), strings.Contains(kind, token):
			score += 2
		}
	}
	return score
}

func knowledgeResultID(result KnowledgeResult) string {
	if result.Kind == "component" {
		return result.Component.ID
	}
	return result.Section.ID
}
