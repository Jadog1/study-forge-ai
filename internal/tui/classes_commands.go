package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/state"
	"gopkg.in/yaml.v3"
)

type classContextEditedMsg struct {
	className string
	profile   string
	path      string
	err       error
}

type classConfigEditedMsg struct {
	className string
	kind      string
	label     string
	path      string
	err       error
}

type bootstrapPathsMsg struct {
	className string
	paths     []string
	err       error
}

// loadBootstrapPathsCmd asynchronously discovers ingested source paths for a class.
func loadBootstrapPathsCmd(className string) tea.Cmd {
	return func() tea.Msg {
		idx, err := state.LoadSectionIndex()
		if err != nil {
			return bootstrapPathsMsg{className: className, err: err}
		}
		seen := make(map[string]bool)
		var paths []string
		for _, section := range idx.Sections {
			if !strings.EqualFold(strings.TrimSpace(section.Class), className) {
				continue
			}
			for _, sp := range section.SourcePaths {
				norm := strings.TrimSpace(sp)
				if norm == "" {
					continue
				}
				key := strings.ToLower(norm)
				if seen[key] {
					continue
				}
				seen[key] = true
				paths = append(paths, norm)
			}
		}
		sort.Strings(paths)
		return bootstrapPathsMsg{className: className, paths: paths}
	}
}

// openContextEditorCmd emits an openEditorMsg for a class context profile.
func openContextEditorCmd(className, profileKind, profileLabel string) tea.Cmd {
	return func() tea.Msg {
		path, err := classpkg.ContextProfilePath(className, profileKind)
		if err != nil {
			return classContextEditedMsg{className: className, profile: profileKind, err: err}
		}
		text, err := classpkg.LoadProfileContextText(className, profileKind)
		if err != nil {
			return classContextEditedMsg{className: className, profile: profileKind, err: err}
		}
		return openEditorMsg{
			title:    fmt.Sprintf("%s Context — %s", profileLabel, className),
			filePath: path,
			content:  text,
			onSave: func(content string) error {
				return classpkg.SaveProfileContextText(className, profileKind, content)
			},
		}
	}
}

// openRosterEditorCmd emits an openEditorMsg for a class note roster YAML.
func openRosterEditorCmd(className string) tea.Cmd {
	return func() tea.Msg {
		path, err := classpkg.NoteRosterPath(className)
		if err != nil {
			return classConfigEditedMsg{className: className, label: "note roster", err: err}
		}
		roster, loadErr := classpkg.LoadNoteRoster(className)
		if loadErr == nil {
			_ = classpkg.SaveNoteRoster(className, roster)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				data = []byte("")
			} else {
				return classConfigEditedMsg{className: className, label: "note roster", err: err}
			}
		}
		return openEditorMsg{
			title:    fmt.Sprintf("Note Roster — %s", className),
			filePath: path,
			content:  string(data),
			onSave:   yamlFileSaver(path),
		}
	}
}

// openCoverageEditorCmd emits an openEditorMsg for a class coverage scope YAML.
func openCoverageEditorCmd(className, profileKind, profileLabel string) tea.Cmd {
	return func() tea.Msg {
		kind := classpkg.NormalizeContextProfile(profileKind)
		path, err := classpkg.CoverageScopePath(className, kind)
		if err != nil {
			return classConfigEditedMsg{className: className, kind: kind, label: "coverage scope", err: err}
		}
		scope, loadErr := classpkg.LoadCoverageScope(className, kind)
		if loadErr == nil && scope == nil {
			scope = &classpkg.CoverageScope{Class: className, Kind: kind, Groups: []classpkg.ScopeGroup{}}
			_ = classpkg.SaveCoverageScope(className, kind, scope)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				data = []byte("")
			} else {
				return classConfigEditedMsg{className: className, kind: kind, label: "coverage scope", err: err}
			}
		}
		return openEditorMsg{
			title:    fmt.Sprintf("%s Coverage Scope — %s", profileLabel, className),
			filePath: path,
			content:  string(data),
			onSave:   yamlFileSaver(path),
		}
	}
}

// yamlFileSaver returns an onSave callback that validates YAML before writing.
func yamlFileSaver(path string) func(string) error {
	return func(content string) error {
		if strings.TrimSpace(content) != "" {
			var probe interface{}
			if err := yaml.Unmarshal([]byte(content), &probe); err != nil {
				return fmt.Errorf("invalid YAML: %w", err)
			}
		}
		return os.WriteFile(path, []byte(content), 0644)
	}
}
