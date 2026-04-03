package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/state"
)

// ClassesTab holds state for the Classes tab.
type ClassesTab struct {
	classes      []string
	index        int
	profileIdx   int
	mode         string // "", "new-class", "add-context", "reorder-roster", "assign-coverage", "bootstrap-roster", "edit-context"
	classInput   textinput.Model
	contextInput textinput.Model
	contextArea  textarea.Model

	rosterCursor    int
	rosterDraft     []classpkg.NoteRosterEntry
	coverageCursor  int
	coverageLabels  []string
	coveragePick    map[string]int // 0 none, 1 primary, 2 secondary
	coverageExclude bool

	bootstrapPaths    []string
	bootstrapCursor   int
	bootstrapIncluded map[int]bool
	bootstrapLabels   map[int]string
	bootstrapEditing  bool

	height          int
	rosterScroll    int
	coverageScroll  int
	bootstrapScroll int
}

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

func newClassesTab(classes []string) ClassesTab {
	classInput := textinput.New()
	classInput.Placeholder = "new class name"
	classInput.CharLimit = 120

	contextInput := textinput.New()
	contextInput.Placeholder = "path to context file"
	contextInput.CharLimit = 1000

	ctxArea := textarea.New()
	ctxArea.Placeholder = "Enter context text here..."
	ctxArea.ShowLineNumbers = false

	return ClassesTab{
		classes:      classes,
		classInput:   classInput,
		contextInput: contextInput,
		contextArea:  ctxArea,
	}
}

func (c ClassesTab) resize(width, height int) ClassesTab {
	c.classInput.Width = clamp(width-6, 18, width)
	c.contextInput.Width = clamp(width-6, 18, width)
	c.contextArea.SetWidth(clamp(width-6, 40, width))
	c.contextArea.SetHeight(12)
	c.height = height
	return c
}

// SelectedClass returns the name of the highlighted class, or "".
func (c ClassesTab) SelectedClass() string {
	if len(c.classes) == 0 || c.index < 0 || c.index >= len(c.classes) {
		return ""
	}
	return c.classes[c.index]
}

func (c ClassesTab) selectedProfile() classpkg.ContextProfile {
	profiles := classpkg.ContextProfiles()
	if len(profiles) == 0 {
		return classpkg.ContextProfile{Kind: "quiz", Label: "Quiz", FileName: "context.quiz.md", DefaultQuestionType: "multiple-choice"}
	}
	idx := c.profileIdx
	if idx < 0 || idx >= len(profiles) {
		idx = 0
	}
	return profiles[idx]
}

// EnterNewClassMode activates the new-class input sub-mode.
func (c ClassesTab) EnterNewClassMode() ClassesTab {
	c.mode = "new-class"
	c.classInput.Focus()
	return c
}

// EnterAddContextMode activates the add-context input sub-mode.
func (c ClassesTab) EnterAddContextMode() ClassesTab {
	c.mode = "add-context"
	c.contextInput.Focus()
	return c
}

// update processes all messages for the Classes tab.
// Returns (updated tab, status string, tea.Cmd).
func (c ClassesTab) update(msg tea.Msg) (ClassesTab, string, tea.Cmd) {
	if boot, ok := msg.(bootstrapPathsMsg); ok {
		if boot.err != nil {
			return c, "Failed to discover source paths: " + boot.err.Error(), nil
		}
		if len(boot.paths) == 0 {
			return c, "No source paths found for this class. Run 'sfa ingest' first.", nil
		}
		c.mode = "bootstrap-roster"
		c.bootstrapPaths = boot.paths
		c.bootstrapCursor = 0
		c.bootstrapScroll = 0
		c.bootstrapIncluded = make(map[int]bool, len(boot.paths))
		c.bootstrapLabels = make(map[int]string, len(boot.paths))
		for i, path := range boot.paths {
			c.bootstrapIncluded[i] = true
			c.bootstrapLabels[i] = deriveLabelFromPath(path)
		}
		c.bootstrapEditing = false
		return c, fmt.Sprintf("Bootstrap: %d source(s) found — ↑/↓ select  •  space toggle  •  Enter rename  •  A accept all  •  Esc cancel", len(boot.paths)), nil
	}

	if edited, ok := msg.(classContextEditedMsg); ok {
		if edited.err != nil {
			return c, "Edit context failed: " + edited.err.Error(), nil
		}
		return c, fmt.Sprintf("Saved %s context: %s", edited.profile, edited.path), nil
	}
	if edited, ok := msg.(classConfigEditedMsg); ok {
		if edited.err != nil {
			return c, "Edit " + edited.label + " failed: " + edited.err.Error(), nil
		}
		if edited.kind != "" {
			return c, fmt.Sprintf("Saved %s for %s: %s", edited.label, edited.kind, edited.path), nil
		}
		return c, fmt.Sprintf("Saved %s: %s", edited.label, edited.path), nil
	}

	switch c.mode {
	case "new-class":
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

	case "add-context":
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

	case "reorder-roster":
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

	case "assign-coverage":
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

	case "bootstrap-roster":
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

	case "edit-context":
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "ctrl+s":
				className := c.SelectedClass()
				if className == "" {
					return c, "No class selected", nil
				}
				profile := c.selectedProfile()
				text := c.contextArea.Value()
				if err := classpkg.SaveProfileContextText(className, profile.Kind, text); err != nil {
					return c, "Save context failed: " + err.Error(), nil
				}
				c.contextArea.Blur()
				c.mode = ""
				return c, fmt.Sprintf("Saved %s context", profile.Label), nil
			case "esc":
				c.contextArea.Blur()
				c.mode = ""
				return c, "Context edit cancelled", nil
			}
		}
		var cmd tea.Cmd
		c.contextArea, cmd = c.contextArea.Update(msg)
		return c, "", cmd
	}

	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up":
			if c.index > 0 {
				c.index--
			}
		case "down":
			if c.index < len(c.classes)-1 {
				c.index++
			}
		case "left":
			profiles := classpkg.ContextProfiles()
			if len(profiles) > 0 {
				c.profileIdx = (c.profileIdx - 1 + len(profiles)) % len(profiles)
			}
		case "right":
			profiles := classpkg.ContextProfiles()
			if len(profiles) > 0 {
				c.profileIdx = (c.profileIdx + 1) % len(profiles)
			}
		case "n":
			return c.EnterNewClassMode(), "Enter new class name, then press Enter", nil
		case "a":
			if c.SelectedClass() == "" {
				return c, "Create or select a class first", nil
			}
			return c.EnterAddContextMode(), "Enter context file path, then press Enter", nil
		case "e":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			profile := c.selectedProfile()
			return c, fmt.Sprintf("Opening %s context in editor...", profile.Label), openClassContextEditorCmd(className, profile.Kind)
		case "i":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			profile := c.selectedProfile()
			text, err := classpkg.LoadProfileContextText(className, profile.Kind)
			if err != nil {
				return c, "Load context failed: " + err.Error(), nil
			}
			c.contextArea.SetValue(text)
			c.contextArea.Focus()
			c.mode = "edit-context"
			return c, fmt.Sprintf("Inline editing %s context — Ctrl+S save  •  Esc cancel", profile.Label), nil
		case "o":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			return c, "Opening note roster in editor...", openNoteRosterEditorCmd(className)
		case "c":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			profile := c.selectedProfile()
			return c, fmt.Sprintf("Opening %s coverage scope in editor...", profile.Label), openCoverageScopeEditorCmd(className, profile.Kind)
		case "r":
			c.classes, _ = classpkg.List()
			return c, "Class list refreshed", nil
		case "R":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			roster, err := classpkg.LoadNoteRoster(className)
			if err != nil {
				return c, "Load roster failed: " + err.Error(), nil
			}
			if len(roster.Entries) == 0 {
				return c, "No roster entries to reorder yet", nil
			}
			c.mode = "reorder-roster"
			c.rosterCursor = 0
			c.rosterScroll = 0
			c.rosterDraft = append([]classpkg.NoteRosterEntry(nil), roster.Entries...)
			return c, "Roster reorder mode: ↑/↓ select, u/d move, Enter save, Esc cancel", nil
		case "C":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			roster, err := classpkg.LoadNoteRoster(className)
			if err != nil {
				return c, "Load roster failed: " + err.Error(), nil
			}
			if len(roster.Entries) == 0 {
				// No roster yet — bootstrap from ingested source paths.
				return c, "Discovering source paths...", loadBootstrapPathsCmd(className)
			}
			profile := c.selectedProfile()
			scope, err := classpkg.LoadCoverageScope(className, profile.Kind)
			if err != nil {
				return c, "Load coverage failed: " + err.Error(), nil
			}
			labels := make([]string, 0, len(roster.Entries))
			pick := make(map[string]int, len(roster.Entries))
			for _, entry := range roster.Entries {
				label := strings.TrimSpace(entry.Label)
				if label == "" {
					continue
				}
				labels = append(labels, label)
				pick[label] = 0
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
					for _, label := range group.Labels {
						if _, ok := pick[label]; ok {
							pick[label] = gs
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
			return c, fmt.Sprintf("%s coverage mode: 1 primary, 2 secondary, 0 clear, x toggle exclude, Enter save", profile.Label), nil
		}
	}
	return c, "", nil
}

func (c ClassesTab) view(width, height int) string {
	if len(c.classes) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			renderSection("Classes", "No classes yet.", width),
			renderSection("Next step", dimStyle.Render("Press n to create one, or open Ctrl+P and choose New Class."), width),
		)
	}

	var lines []string
	for i, cl := range c.classes {
		label := truncateWidth(cl, width-10)
		if i == c.index {
			lines = append(lines, selectedStyle.Render("▸ "+label))
		} else {
			lines = append(lines, "  "+label)
		}
	}

	className := c.SelectedClass()
	profile := c.selectedProfile()
	ctxLines := []string{dimStyle.Render("(none)")}
	contextTitle := fmt.Sprintf("%s context", profile.Label)
	if className != "" {
		text, err := classpkg.LoadProfileContextText(className, profile.Kind)
		if err != nil {
			ctxLines = []string{errorStyle.Render(err.Error())}
			contextTitle += " (load error)"
		} else if strings.TrimSpace(text) != "" {
			ctxLines = strings.Split(strings.TrimSpace(text), "\n")
			if len(ctxLines) > 8 {
				ctxLines = append(ctxLines[:8], "...")
			}
		}
	}

	attachedLines := []string{dimStyle.Render("(none)")}
	if className != "" {
		ctx, err := classpkg.LoadContext(className)
		if err != nil {
			attachedLines = []string{errorStyle.Render(err.Error())}
		} else if len(ctx.ContextFiles) > 0 {
			attachedLines = ctx.ContextFiles
		}
	}

	rosterLines := []string{dimStyle.Render("(none configured)")}
	if className != "" {
		roster, err := classpkg.LoadNoteRoster(className)
		if err != nil {
			rosterLines = []string{errorStyle.Render(err.Error())}
		} else if len(roster.Entries) > 0 {
			rosterLines = make([]string, 0, len(roster.Entries)+1)
			for _, entry := range roster.Entries {
				line := fmt.Sprintf("%d. %s", entry.Order, entry.Label)
				if strings.TrimSpace(entry.SourcePattern) != "" {
					line += " -> " + entry.SourcePattern
				}
				rosterLines = append(rosterLines, line)
			}
			if len(rosterLines) > 6 {
				rosterLines = append(rosterLines[:6], "...")
			}
		}
	}

	coverageLines := []string{dimStyle.Render("(none configured)")}
	if className != "" {
		scope, err := classpkg.LoadCoverageScope(className, profile.Kind)
		if err != nil {
			coverageLines = []string{errorStyle.Render(err.Error())}
		} else if scope != nil && len(scope.Groups) > 0 {
			coverageLines = make([]string, 0, len(scope.Groups)+2)
			for i, group := range scope.Groups {
				line := fmt.Sprintf("group %d weight %.2f", i+1, group.Weight)
				if len(group.Labels) > 0 {
					line += " labels=" + strings.Join(group.Labels, ",")
				}
				coverageLines = append(coverageLines, line)
			}
			if scope.ExcludeUnmatched {
				coverageLines = append(coverageLines, "unmatched material excluded")
			}
		}
	}

	hint := dimStyle.Render("↑/↓ select  •  ←/→ profile  •  i inline edit context  •  e editor context  •  o edit roster file  •  c edit coverage file  •  R reorder roster  •  C assign/bootstrap coverage  •  n new  •  a add file  •  r refresh")
	switch c.mode {
	case "new-class":
		hint = "New class\n" + c.classInput.View() + "\n" + dimStyle.Render("Press Enter to create the class.")
	case "add-context":
		hint = "Context file path\n" + c.contextInput.View() + "\n" + dimStyle.Render("Press Enter to attach the file to the selected class.")
	case "reorder-roster":
		hint = dimStyle.Render("Reorder mode: ↑/↓ select  •  u/d move  •  Enter save  •  Esc cancel")
	case "assign-coverage":
		hint = dimStyle.Render("Coverage mode: ↑/↓ select  •  1 primary  •  2 secondary  •  0 clear  •  x exclude toggle  •  Enter save  •  Esc cancel")
	case "bootstrap-roster":
		if c.bootstrapEditing {
			hint = "Label: " + c.contextInput.View() + "\n" + dimStyle.Render("Enter to confirm  •  Esc to cancel")
		} else {
			hint = dimStyle.Render("Bootstrap: ↑/↓ select  •  space toggle  •  Enter rename label  •  A accept all  •  Esc cancel")
		}
	case "edit-context":
		hint = dimStyle.Render("Inline edit: Ctrl+S save  •  Esc cancel")
	}

	listHeight := clamp(height/3, 6, 12)
	classesBody := clipLines(strings.Join(lines, "\n"), listHeight)
	selectedBody := fmt.Sprintf("Selected class: %s", dimStyle.Render("none"))
	if className != "" {
		selectedBody = fmt.Sprintf("Selected class: %s", selectedStyle.Render(className))
	}
	profileLine := dimStyle.Render(fmt.Sprintf("Profile: %s (%s)", profile.Kind, profile.FileName))
	contextBody := lipgloss.NewStyle().Width(width - 6).Render(strings.Join(ctxLines, "\n"))
	attachedBody := lipgloss.NewStyle().Width(width - 6).Render(strings.Join(attachedLines, "\n"))
	contextBody = selectedBody + "\n" + profileLine + "\n\n" + clipLines(contextBody, clamp(height/3, 4, 8)) + "\n\n" +
		dimStyle.Render("Attached context files:") + "\n" + clipLines(attachedBody, clamp(height/4, 3, 6))

	rosterBody := clipLines(strings.Join(rosterLines, "\n"), clamp(height/4, 4, 8))
	coverageBody := clipLines(strings.Join(coverageLines, "\n"), clamp(height/4, 3, 7))
	settingsBody := dimStyle.Render("Note roster:") + "\n" + rosterBody + "\n\n" +
		dimStyle.Render(fmt.Sprintf("%s coverage:", profile.Label)) + "\n" + coverageBody

	if c.mode == "reorder-roster" {
		draftLines := make([]string, 0, len(c.rosterDraft))
		for i, entry := range c.rosterDraft {
			line := fmt.Sprintf("%d. %s", i+1, entry.Label)
			if i == c.rosterCursor {
				draftLines = append(draftLines, selectedStyle.Render("▸ "+line))
			} else {
				draftLines = append(draftLines, "  "+line)
			}
		}
		settingsBody = dimStyle.Render("Reorder note roster (draft):") + "\n" + windowLines(draftLines, c.rosterScroll, clamp(height/3, 6, 12))
	}

	if c.mode == "assign-coverage" {
		draftLines := make([]string, 0, len(c.coverageLabels)+2)
		for i, label := range c.coverageLabels {
			marker := "[ ]"
			switch c.coveragePick[label] {
			case 1:
				marker = "[P]"
			case 2:
				marker = "[S]"
			}
			line := fmt.Sprintf("%s %s", marker, label)
			if i == c.coverageCursor {
				draftLines = append(draftLines, selectedStyle.Render("▸ "+line))
			} else {
				draftLines = append(draftLines, "  "+line)
			}
		}
		draftLines = append(draftLines, "")
		draftLines = append(draftLines, fmt.Sprintf("exclude unmatched: %t", c.coverageExclude))
		settingsBody = dimStyle.Render(fmt.Sprintf("Assign %s coverage (draft):", profile.Label)) + "\n" + windowLines(draftLines, c.coverageScroll, clamp(height/2, 8, 15))
	}

	if c.mode == "bootstrap-roster" {
		draftLines := make([]string, 0, len(c.bootstrapPaths))
		for i, path := range c.bootstrapPaths {
			check := "[ ]"
			if c.bootstrapIncluded[i] {
				check = "[✓]"
			}
			label := c.bootstrapLabels[i]
			pathShort := truncateWidth(filepath.Base(path), width/3)
			line := fmt.Sprintf("%s %-24s %s", check, label, dimStyle.Render("← "+pathShort))
			if i == c.bootstrapCursor {
				draftLines = append(draftLines, selectedStyle.Render("▸ "+line))
			} else {
				draftLines = append(draftLines, "  "+line)
			}
		}
		settingsBody = dimStyle.Render("Bootstrap note roster from discovered sources:") + "\n" +
			windowLines(draftLines, c.bootstrapScroll, clamp(height/2, 8, 15))
	}

	var contextSectionBody string
	if c.mode == "edit-context" {
		contextSectionBody = selectedBody + "\n" + profileLine + "\n\n" + c.contextArea.View()
	} else {
		contextSectionBody = contextBody
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		renderSection("Classes", classesBody, width),
		renderSection(contextTitle, contextSectionBody, width),
		renderSection("Ordering And Coverage", settingsBody, width),
		renderSection("Actions", hint, width),
	)
}

func (c ClassesTab) coverageLabelAtCursor() string {
	if c.coverageCursor < 0 || c.coverageCursor >= len(c.coverageLabels) {
		return ""
	}
	return c.coverageLabels[c.coverageCursor]
}

// deriveLabelFromPath produces a human-readable label from a file path.
func deriveLabelFromPath(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	// Replace underscores/hyphens with spaces for readability.
	base = strings.NewReplacer("_", " ", "-", " ").Replace(base)
	return base
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

func openClassContextEditorCmd(className, profileKind string) tea.Cmd {
	path, err := classpkg.ContextProfilePath(className, profileKind)
	if err != nil {
		return func() tea.Msg {
			return classContextEditedMsg{className: className, profile: profileKind, err: err}
		}
	}
	if _, err := classpkg.LoadProfileContextText(className, profileKind); err != nil {
		return func() tea.Msg {
			return classContextEditedMsg{className: className, profile: profileKind, err: err}
		}
	}

	editor := resolveTerminalEditor()
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return func() tea.Msg {
			return classContextEditedMsg{className: className, profile: profileKind, err: fmt.Errorf("no editor configured")}
		}
	}

	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(runErr error) tea.Msg {
		return classContextEditedMsg{className: className, profile: profileKind, path: path, err: runErr}
	})
}

func openNoteRosterEditorCmd(className string) tea.Cmd {
	path, err := classpkg.NoteRosterPath(className)
	if err != nil {
		return func() tea.Msg {
			return classConfigEditedMsg{className: className, label: "note roster", err: err}
		}
	}
	roster, loadErr := classpkg.LoadNoteRoster(className)
	if loadErr == nil {
		_ = classpkg.SaveNoteRoster(className, roster)
	}
	return openClassFileEditor(path, func(runErr error) tea.Msg {
		return classConfigEditedMsg{className: className, label: "note roster", path: path, err: runErr}
	})
}

func openCoverageScopeEditorCmd(className, profileKind string) tea.Cmd {
	kind := classpkg.NormalizeContextProfile(profileKind)
	path, err := classpkg.CoverageScopePath(className, kind)
	if err != nil {
		return func() tea.Msg {
			return classConfigEditedMsg{className: className, kind: kind, label: "coverage scope", err: err}
		}
	}
	scope, loadErr := classpkg.LoadCoverageScope(className, kind)
	if loadErr == nil && scope == nil {
		scope = &classpkg.CoverageScope{Class: className, Kind: kind, Groups: []classpkg.ScopeGroup{}}
		_ = classpkg.SaveCoverageScope(className, kind, scope)
	}
	return openClassFileEditor(path, func(runErr error) tea.Msg {
		return classConfigEditedMsg{className: className, kind: kind, label: "coverage scope", path: path, err: runErr}
	})
}

func openClassFileEditor(path string, done func(error) tea.Msg) tea.Cmd {
	editor := resolveTerminalEditor()
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return func() tea.Msg {
			return done(fmt.Errorf("no editor configured"))
		}
	}

	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(runErr error) tea.Msg {
		return done(runErr)
	})
}

func resolveTerminalEditor() string {
	for _, key := range []string{"SFA_EDITOR", "VISUAL", "EDITOR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}
