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
		sectionIDFromTag, componentIDFromTag := extractProvenanceFromTags(q.Sections[i].Tags)
		if strings.TrimSpace(q.Sections[i].SectionID) == "" {
			q.Sections[i].SectionID = sectionIDFromTag
		}
		if strings.TrimSpace(q.Sections[i].ComponentID) == "" {
			q.Sections[i].ComponentID = componentIDFromTag
		}
		if q.Sections[i].SectionID != "" && !hasPrefixedTag(q.Sections[i].Tags, provenanceTagSectionPrefix, q.Sections[i].SectionID) {
			q.Sections[i].Tags = append(q.Sections[i].Tags, provenanceTagSectionPrefix+q.Sections[i].SectionID)
		}
		if q.Sections[i].ComponentID != "" && !hasPrefixedTag(q.Sections[i].Tags, provenanceTagComponentPrefix, q.Sections[i].ComponentID) {
			q.Sections[i].Tags = append(q.Sections[i].Tags, provenanceTagComponentPrefix+q.Sections[i].ComponentID)
		}
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

func hasPrefixedTag(tags []string, prefix, value string) bool {
	needle := strings.ToLower(strings.TrimSpace(prefix + value))
	for _, tag := range tags {
		if strings.ToLower(strings.TrimSpace(tag)) == needle {
			return true
		}
	}
	return false
}
