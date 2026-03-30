package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
	mode         string // "", "new-class", "add-context"
	classInput   textinput.Model
	contextInput textinput.Model
}

type classContextEditedMsg struct {
	className string
	profile   string
	path      string
	err       error
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

func (c ClassesTab) resize(width int) ClassesTab {
	c.classInput.Width = clamp(width-6, 18, width)
	c.contextInput.Width = clamp(width-6, 18, width)
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
	if edited, ok := msg.(classContextEditedMsg); ok {
		if edited.err != nil {
			return c, "Edit context failed: " + edited.err.Error(), nil
		}
		return c, fmt.Sprintf("Saved %s context: %s", edited.profile, edited.path), nil
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
		case "r":
			c.classes, _ = classpkg.List()
			return c, "Class list refreshed", nil
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

	hint := dimStyle.Render("↑/↓ select  •  ←/→ profile  •  e edit context  •  n new  •  a add file  •  r refresh  •  esc cancel")
	if c.mode == "new-class" {
		hint = "New class\n" + c.classInput.View() + "\n" + dimStyle.Render("Press Enter to create the class.")
	} else if c.mode == "add-context" {
		hint = "Context file path\n" + c.contextInput.View() + "\n" + dimStyle.Render("Press Enter to attach the file to the selected class.")
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

	return lipgloss.JoinVertical(lipgloss.Left,
		renderSection("Classes", classesBody, width),
		renderSection(contextTitle, contextBody, width),
		renderSection("Actions", hint, width),
	)
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
