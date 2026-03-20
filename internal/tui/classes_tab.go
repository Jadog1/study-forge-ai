package tui

import (
	"fmt"
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
	mode         string // "", "new-class", "add-context"
	classInput   textinput.Model
	contextInput textinput.Model
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
		case "n":
			return c.EnterNewClassMode(), "Enter new class name, then press Enter", nil
		case "a":
			if c.SelectedClass() == "" {
				return c, "Create or select a class first", nil
			}
			return c.EnterAddContextMode(), "Enter context file path, then press Enter", nil
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
	ctxLines := []string{dimStyle.Render("(none)")}
	contextTitle := "Context files"
	if className != "" {
		ctx, err := classpkg.LoadContext(className)
		if err != nil {
			ctxLines = []string{errorStyle.Render(err.Error())}
			contextTitle = "Context files (load error)"
		} else if len(ctx.ContextFiles) > 0 {
			ctxLines = ctx.ContextFiles
		}
	}

	hint := dimStyle.Render("↑/↓ select  •  n new  •  a add context  •  r refresh  •  esc cancel")
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
	contextBody := lipgloss.NewStyle().Width(width - 6).Render(strings.Join(ctxLines, "\n"))
	contextBody = selectedBody + "\n\n" + clipLines(contextBody, clamp(height/3, 5, 10))

	return lipgloss.JoinVertical(lipgloss.Left,
		renderSection("Classes", classesBody, width),
		renderSection(contextTitle, contextBody, width),
		renderSection("Actions", hint, width),
	)
}
