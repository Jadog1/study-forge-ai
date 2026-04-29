package quiz

import (
	"strings"

	"github.com/studyforge/study-agent/internal/state"
)

const (
	provenanceTagSectionPrefix   = "src_section:"
	provenanceTagComponentPrefix = "src_component:"
)

func normalizeQuizProvenance(q *state.Quiz) {
	if q == nil {
		return
	}
	for i := range q.Sections {
		q.Sections[i].SectionID = strings.TrimSpace(q.Sections[i].SectionID)
		q.Sections[i].ComponentID = strings.TrimSpace(q.Sections[i].ComponentID)
		sectionIDFromTag, componentIDFromTag := extractProvenanceFromTags(q.Sections[i].Tags)
		if strings.TrimSpace(q.Sections[i].SectionID) == "" {
			q.Sections[i].SectionID = sectionIDFromTag
		}
		if strings.TrimSpace(q.Sections[i].ComponentID) == "" {
			q.Sections[i].ComponentID = componentIDFromTag
		}
		q.Sections[i].Tags = canonicalizeProvenanceTags(q.Sections[i].Tags, q.Sections[i].SectionID, q.Sections[i].ComponentID)
	}
}

func extractProvenanceFromTags(tags []string) (sectionID, componentID string) {
	for _, tag := range tags {
		normalized := strings.TrimSpace(tag)
		if sectionID == "" && strings.HasPrefix(strings.ToLower(normalized), provenanceTagSectionPrefix) {
			sectionID = strings.TrimSpace(normalized[len(provenanceTagSectionPrefix):])
		}
		if componentID == "" && strings.HasPrefix(strings.ToLower(normalized), provenanceTagComponentPrefix) {
			componentID = strings.TrimSpace(normalized[len(provenanceTagComponentPrefix):])
		}
	}
	return sectionID, componentID
}

func canonicalizeProvenanceTags(tags []string, sectionID, componentID string) []string {
	sectionID = strings.TrimSpace(sectionID)
	componentID = strings.TrimSpace(componentID)
	filtered := make([]string, 0, len(tags)+2)
	for _, tag := range tags {
		normalized := strings.TrimSpace(tag)
		lowered := strings.ToLower(normalized)
		if strings.HasPrefix(lowered, provenanceTagSectionPrefix) || strings.HasPrefix(lowered, provenanceTagComponentPrefix) {
			continue
		}
		filtered = append(filtered, normalized)
	}
	if sectionID != "" {
		filtered = append(filtered, provenanceTagSectionPrefix+sectionID)
	}
	if componentID != "" {
		filtered = append(filtered, provenanceTagComponentPrefix+componentID)
	}
	return filtered
}
