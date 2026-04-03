package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	classpkg "github.com/studyforge/study-agent/internal/class"
)

func (c ClassesTab) updateNewClassMode(msg tea.Msg) (ClassesTab, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" {
		name := strings.TrimSpace(c.classInput.Value())
		if name == "" {
			return c, "Class name cannot be empty", nil
		}
		if err := classpkg.Create(name); err != nil {
			return c, "Create class failed: " + err.Error(), nil
		}
		c.classes, _ = classpkg.List()
		for i, cl := range c.classes {
			if cl == name {
				c.index = i
				break
			}
		}
		c.classInput.SetValue("")
		c.classInput.Blur()
		c.mode = ""
		return c, "Class created: " + name, nil
	}
	var cmd tea.Cmd
	c.classInput, cmd = c.classInput.Update(msg)
	return c, "", cmd
}

func (c ClassesTab) updateAddContextMode(msg tea.Msg) (ClassesTab, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" {
		path := strings.TrimSpace(c.contextInput.Value())
		if path == "" {
			return c, "Context path cannot be empty", nil
		}
		className := c.SelectedClass()
		if className == "" {
			return c, "No class selected", nil
		}
		if err := classpkg.AddContextFile(className, path); err != nil {
			return c, "Add context failed: " + err.Error(), nil
		}
		c.contextInput.SetValue("")
		c.contextInput.Blur()
		c.mode = ""
		return c, "Context file added", nil
	}
	var cmd tea.Cmd
	c.contextInput, cmd = c.contextInput.Update(msg)
	return c, "", cmd
}

func (c ClassesTab) updateReorderRosterMode(msg tea.Msg) (ClassesTab, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			c.mode = ""
			c.rosterDraft = nil
			c.rosterCursor = 0
			c.rosterScroll = 0
			return c, "Roster reorder cancelled", nil
		case "up":
			if c.rosterCursor > 0 {
				c.rosterCursor--
			}
			c.rosterScroll = scrollToView(c.rosterScroll, c.rosterCursor, clamp(c.height/3, 6, 12))
			return c, "", nil
		case "down":
			if c.rosterCursor < len(c.rosterDraft)-1 {
				c.rosterCursor++
			}
			c.rosterScroll = scrollToView(c.rosterScroll, c.rosterCursor, clamp(c.height/3, 6, 12))
			return c, "", nil
		case "u":
			if c.rosterCursor <= 0 || c.rosterCursor >= len(c.rosterDraft) {
				return c, "", nil
			}
			c.rosterDraft[c.rosterCursor-1], c.rosterDraft[c.rosterCursor] = c.rosterDraft[c.rosterCursor], c.rosterDraft[c.rosterCursor-1]
			c.rosterCursor--
			c.rosterScroll = scrollToView(c.rosterScroll, c.rosterCursor, clamp(c.height/3, 6, 12))
			return c, "", nil
		case "d":
			if c.rosterCursor < 0 || c.rosterCursor >= len(c.rosterDraft)-1 {
				return c, "", nil
			}
			c.rosterDraft[c.rosterCursor], c.rosterDraft[c.rosterCursor+1] = c.rosterDraft[c.rosterCursor+1], c.rosterDraft[c.rosterCursor]
			c.rosterCursor++
			c.rosterScroll = scrollToView(c.rosterScroll, c.rosterCursor, clamp(c.height/3, 6, 12))
			return c, "", nil
		case "enter":
			className := c.SelectedClass()
			if className == "" {
				return c, "No class selected", nil
			}
			labels := make([]string, 0, len(c.rosterDraft))
			for _, entry := range c.rosterDraft {
				labels = append(labels, entry.Label)
			}
			if _, err := classpkg.ReorderNoteRosterEntries(className, labels); err != nil {
				return c, "Save roster order failed: " + err.Error(), nil
			}
			c.mode = ""
			c.rosterDraft = nil
			c.rosterCursor = 0
			c.rosterScroll = 0
			return c, "Roster order saved", nil
		}
	}
	return c, "", nil
}

func (c ClassesTab) updateAssignCoverageMode(msg tea.Msg) (ClassesTab, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			c.mode = ""
			c.coverageCursor = 0
			c.coverageScroll = 0
			c.coverageLabels = nil
			c.coveragePick = nil
			c.coverageExclude = false
			return c, "Coverage assignment cancelled", nil
		case "up":
			if c.coverageCursor > 0 {
				c.coverageCursor--
			}
			c.coverageScroll = scrollToView(c.coverageScroll, c.coverageCursor, clamp(c.height/2, 8, 15))
			return c, "", nil
		case "down":
			if c.coverageCursor < len(c.coverageLabels)-1 {
				c.coverageCursor++
			}
			c.coverageScroll = scrollToView(c.coverageScroll, c.coverageCursor, clamp(c.height/2, 8, 15))
			return c, "", nil
		case "1":
			if label := c.coverageLabelAtCursor(); label != "" {
				c.coveragePick[label] = 1
			}
			return c, "", nil
		case "2":
			if label := c.coverageLabelAtCursor(); label != "" {
				c.coveragePick[label] = 2
			}
			return c, "", nil
		case "0", "backspace", "delete":
			if label := c.coverageLabelAtCursor(); label != "" {
				c.coveragePick[label] = 0
			}
			return c, "", nil
		case "x":
			c.coverageExclude = !c.coverageExclude
			return c, "", nil
		case "enter":
			className := c.SelectedClass()
			if className == "" {
				return c, "No class selected", nil
			}
			profile := c.selectedProfile()
			primary := make([]string, 0)
			secondary := make([]string, 0)
			for _, label := range c.coverageLabels {
				switch c.coveragePick[label] {
				case 1:
					primary = append(primary, label)
				case 2:
					secondary = append(secondary, label)
				}
			}
			groups := make([]classpkg.ScopeGroup, 0, 2)
			if len(primary) > 0 {
				groups = append(groups, classpkg.ScopeGroup{Labels: primary, Weight: 1.0})
			}
			if len(secondary) > 0 {
				groups = append(groups, classpkg.ScopeGroup{Labels: secondary, Weight: 0.30})
			}
			scope := &classpkg.CoverageScope{
				Class:            className,
				Kind:             profile.Kind,
				ExcludeUnmatched: c.coverageExclude,
				Groups:           groups,
			}
			if err := classpkg.SaveCoverageScope(className, profile.Kind, scope); err != nil {
				return c, "Save coverage failed: " + err.Error(), nil
			}
			c.mode = ""
			c.coverageCursor = 0
			c.coverageScroll = 0
			c.coverageLabels = nil
			c.coveragePick = nil
			c.coverageExclude = false
			return c, fmt.Sprintf("Saved %s coverage assignments", profile.Label), nil
		}
	}
	return c, "", nil
}

func (c ClassesTab) updateBootstrapRosterMode(msg tea.Msg) (ClassesTab, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if c.bootstrapEditing {
			switch k.String() {
			case "enter":
				newLabel := strings.TrimSpace(c.contextInput.Value())
				if newLabel != "" {
					c.bootstrapLabels[c.bootstrapCursor] = newLabel
				}
				c.contextInput.SetValue("")
				c.contextInput.Blur()
				c.bootstrapEditing = false
				return c, "", nil
			case "esc":
				c.contextInput.SetValue("")
				c.contextInput.Blur()
				c.bootstrapEditing = false
				return c, "", nil
			default:
				var cmd tea.Cmd
				c.contextInput, cmd = c.contextInput.Update(msg)
				return c, "", cmd
			}
		}
		switch k.String() {
		case "esc":
			c.mode = ""
			c.bootstrapPaths = nil
			c.bootstrapIncluded = nil
			c.bootstrapLabels = nil
			c.bootstrapCursor = 0
			c.bootstrapScroll = 0
			return c, "Bootstrap cancelled", nil
		case "up":
			if c.bootstrapCursor > 0 {
				c.bootstrapCursor--
			}
			c.bootstrapScroll = scrollToView(c.bootstrapScroll, c.bootstrapCursor, clamp(c.height/2, 8, 15))
			return c, "", nil
		case "down":
			if c.bootstrapCursor < len(c.bootstrapPaths)-1 {
				c.bootstrapCursor++
			}
			c.bootstrapScroll = scrollToView(c.bootstrapScroll, c.bootstrapCursor, clamp(c.height/2, 8, 15))
			return c, "", nil
		case " ":
			c.bootstrapIncluded[c.bootstrapCursor] = !c.bootstrapIncluded[c.bootstrapCursor]
			return c, "", nil
		case "enter":
			c.bootstrapEditing = true
			c.contextInput.SetValue(c.bootstrapLabels[c.bootstrapCursor])
			c.contextInput.Placeholder = "label for this entry"
			c.contextInput.Focus()
			return c, "Edit label, press Enter to confirm or Esc to cancel", nil
		case "A":
			className := c.SelectedClass()
			if className == "" {
				return c, "No class selected", nil
			}
			count := 0
			for i, path := range c.bootstrapPaths {
				if !c.bootstrapIncluded[i] {
					continue
				}
				label := strings.TrimSpace(c.bootstrapLabels[i])
				if label == "" {
					label = deriveLabelFromPath(path)
				}
				entry := classpkg.NoteRosterEntry{Label: label, SourcePattern: path}
				if _, err := classpkg.UpsertNoteRosterEntry(className, entry); err != nil {
					return c, "Save roster entry failed: " + err.Error(), nil
				}
				count++
			}
			c.mode = ""
			c.bootstrapPaths = nil
			c.bootstrapIncluded = nil
			c.bootstrapLabels = nil
			c.bootstrapCursor = 0
			c.bootstrapScroll = 0
			if count == 0 {
				return c, "No entries included; nothing saved", nil
			}
			// Immediately chain into assign-coverage mode.
			roster, err := classpkg.LoadNoteRoster(className)
			if err != nil || len(roster.Entries) == 0 {
				return c, fmt.Sprintf("Saved %d roster entries", count), nil
			}
			profile := c.selectedProfile()
			scope, _ := classpkg.LoadCoverageScope(className, profile.Kind)
			labels := make([]string, 0, len(roster.Entries))
			pick := make(map[string]int, len(roster.Entries))
			for _, entry := range roster.Entries {
				lbl := strings.TrimSpace(entry.Label)
				if lbl == "" {
					continue
				}
				labels = append(labels, lbl)
				pick[lbl] = 0
			}
			exclude := false
			if scope != nil {
				exclude = scope.ExcludeUnmatched
				for _, group := range scope.Groups {
					gs := 0
					switch {
					case group.Weight >= 0.999:
						gs = 1
					case group.Weight > 0:
						gs = 2
					}
					for _, lbl := range group.Labels {
						if _, ok := pick[lbl]; ok {
							pick[lbl] = gs
						}
					}
				}
			}
			c.mode = "assign-coverage"
			c.coverageCursor = 0
			c.coverageScroll = 0
			c.coverageLabels = labels
			c.coveragePick = pick
			c.coverageExclude = exclude
			return c, fmt.Sprintf("Saved %d roster entries — now assign coverage: 1 primary, 2 secondary, 0 clear, Enter save", count), nil
		}
	}
	return c, "", nil
}

func (c ClassesTab) coverageLabelAtCursor() string {
	if c.coverageCursor < 0 || c.coverageCursor >= len(c.coverageLabels) {
		return ""
	}
	return c.coverageLabels[c.coverageCursor]
}
