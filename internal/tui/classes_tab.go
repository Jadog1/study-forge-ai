package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	classpkg "github.com/studyforge/study-agent/internal/class"
)

// ClassesTab holds state for the Classes tab.
type ClassesTab struct {
	classes      []string
	index        int
	profileIdx   int
	mode         string // "", "new-class", "add-context", "reorder-roster", "assign-coverage", "bootstrap-roster"
	classInput   textinput.Model
	contextInput textinput.Model

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

func newClassesTab(classes []string) ClassesTab {
	classInput := textinput.New()
	classInput.Placeholder = "new class name"
	classInput.CharLimit = 120

	contextInput := textinput.New()
	contextInput.Placeholder = "path to context file"
	contextInput.CharLimit = 1000

	return ClassesTab{
		classes:      classes,
		classInput:   classInput,
		contextInput: contextInput,
	}
}

func (c ClassesTab) resize(width, height int) ClassesTab {
	c.classInput.Width = clamp(width-6, 18, width)
	c.contextInput.Width = clamp(width-6, 18, width)
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
		return c.updateNewClassMode(msg)
	case "add-context":
		return c.updateAddContextMode(msg)
	case "reorder-roster":
		return c.updateReorderRosterMode(msg)
	case "assign-coverage":
		return c.updateAssignCoverageMode(msg)
	case "bootstrap-roster":
		return c.updateBootstrapRosterMode(msg)
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
		case "e", "i":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			profile := c.selectedProfile()
			return c, fmt.Sprintf("Opening %s context editor...", profile.Label), openContextEditorCmd(className, profile.Kind, profile.Label)
		case "o":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			return c, "Opening note roster editor...", openRosterEditorCmd(className)
		case "c":
			className := c.SelectedClass()
			if className == "" {
				return c, "Create or select a class first", nil
			}
			profile := c.selectedProfile()
			return c, fmt.Sprintf("Opening %s coverage editor...", profile.Label), openCoverageEditorCmd(className, profile.Kind, profile.Label)
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

	var hint string
	switch c.mode {
	case "new-class":
		hint = "New class\n" + c.classInput.View()
	case "add-context":
		hint = "Context file path\n" + c.contextInput.View()
	case "bootstrap-roster":
		if c.bootstrapEditing {
			hint = "Label: " + c.contextInput.View()
		}
	}

	listHeight := clamp(height/3, 6, 12)
	classesBody := clipLines(strings.Join(lines, "\n"), listHeight)
	selectedBody := fmt.Sprintf("Selected class: %s", dimStyle.Render("none"))
	if className != "" {
		selectedBody = fmt.Sprintf("Selected class: %s", selectedStyle.Render(className))
	}
	profileLine := dimStyle.Render(fmt.Sprintf("Profile: %s (%s)", profile.Kind, profile.FileName))
	contextBody := lipgloss.NewStyle().Width(width - 6).MaxWidth(width - 6).Render(strings.Join(ctxLines, "\n"))
	attachedBody := lipgloss.NewStyle().Width(width - 6).MaxWidth(width - 6).Render(strings.Join(attachedLines, "\n"))
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

	contextSectionBody := contextBody

	defs := []SectionDef{
		{Title: "Classes", Body: classesBody},
		{Title: contextTitle, Body: contextSectionBody, Flex: true},
		{Title: "Ordering And Coverage", Body: settingsBody},
	}
	if hint != "" {
		defs = append(defs, SectionDef{Title: "Input", Body: hint})
	}
	return renderSections(defs, width, height)
}

func (c ClassesTab) helpKeys() []KeyBinding {
	switch c.mode {
	case "new-class":
		return []KeyBinding{
			{Key: "Enter", Desc: "create"},
			{Key: "Esc", Desc: "cancel"},
		}
	case "add-context":
		return []KeyBinding{
			{Key: "Enter", Desc: "attach"},
			{Key: "Esc", Desc: "cancel"},
		}
	case "reorder-roster":
		return []KeyBinding{
			{Key: "↑/↓", Desc: "select"},
			{Key: "u/d", Desc: "move"},
			{Key: "Enter", Desc: "save"},
			{Key: "Esc", Desc: "cancel"},
		}
	case "assign-coverage":
		return []KeyBinding{
			{Key: "↑/↓", Desc: "select"},
			{Key: "1/2/0", Desc: "assign"},
			{Key: "x", Desc: "exclude"},
			{Key: "Enter", Desc: "save"},
			{Key: "Esc", Desc: "cancel"},
		}
	case "bootstrap-roster":
		if c.bootstrapEditing {
			return []KeyBinding{
				{Key: "Enter", Desc: "confirm"},
				{Key: "Esc", Desc: "cancel"},
			}
		}
		return []KeyBinding{
			{Key: "↑/↓", Desc: "select"},
			{Key: "Space", Desc: "toggle"},
			{Key: "Enter", Desc: "rename"},
			{Key: "A", Desc: "accept all"},
			{Key: "Esc", Desc: "cancel"},
		}
	}
	return []KeyBinding{
		{Key: "↑/↓", Desc: "select"},
		{Key: "←/→", Desc: "profile"},
		{Key: "e/i", Desc: "edit context"},
		{Key: "o", Desc: "edit roster"},
		{Key: "c", Desc: "edit coverage"},
		{Key: "n", Desc: "new class"},
		{Key: "a", Desc: "add file"},
		{Key: "r", Desc: "refresh"},
	}
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

