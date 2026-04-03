package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// editorClosedMsg is emitted when the inline editor is closed (save or discard).
type editorClosedMsg struct{}

// openEditorMsg requests the inline editor to open with the given content.
type openEditorMsg struct {
	title    string
	filePath string
	content  string
	onSave   func(string) error
}

// EditorModel is an inline text editor overlay built on bubbles/textarea.
type EditorModel struct {
	area       textarea.Model
	filePath   string
	title      string
	modified   bool
	visible    bool
	cursorLine int
	cursorCol  int
	onSave     func(content string) error
	saveErr    string
	lineCount  int
	prevValue  string
}

func newEditor() EditorModel {
	area := textarea.New()
	area.ShowLineNumbers = true
	area.Prompt = ""
	area.FocusedStyle.CursorLine = lipgloss.NewStyle()
	return EditorModel{area: area}
}

// Open loads content and activates the editor.
func (e EditorModel) Open(title, filePath, content string, onSave func(string) error) EditorModel {
	e.area.SetValue(content)
	e.area.Focus()
	e.title = title
	e.filePath = filePath
	e.onSave = onSave
	e.modified = false
	e.visible = true
	e.saveErr = ""
	e.prevValue = content
	e.lineCount = strings.Count(content, "\n") + 1
	e.cursorLine = 1
	e.cursorCol = 1
	return e
}

// Close resets the editor and hides it.
func (e EditorModel) Close() EditorModel {
	e.area.Blur()
	e.area.SetValue("")
	e.visible = false
	e.modified = false
	e.title = ""
	e.filePath = ""
	e.onSave = nil
	e.saveErr = ""
	e.prevValue = ""
	e.lineCount = 0
	e.cursorLine = 1
	e.cursorCol = 1
	return e
}

// Update handles keystrokes when the editor is visible.
func (e EditorModel) Update(msg tea.Msg) (EditorModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "ctrl+s":
			if e.onSave != nil {
				if err := e.onSave(e.area.Value()); err != nil {
					e.saveErr = err.Error()
					return e, nil
				}
			}
			e.modified = false
			e.saveErr = ""
			e = e.Close()
			return e, func() tea.Msg { return editorClosedMsg{} }

		case "esc":
			e = e.Close()
			return e, func() tea.Msg { return editorClosedMsg{} }
		}
	}

	var cmd tea.Cmd
	e.area, cmd = e.area.Update(msg)

	val := e.area.Value()
	if val != e.prevValue {
		e.modified = true
		e.prevValue = val
	}
	e.lineCount = strings.Count(val, "\n") + 1
	e.cursorLine = e.area.Line() + 1
	e.cursorCol = e.area.LineInfo().ColumnOffset + 1

	return e, cmd
}

// View renders the editor filling the given dimensions.
func (e EditorModel) View(width, height int) string {
	if !e.visible {
		return ""
	}

	titleText := e.title
	if e.modified {
		titleText += "  " + warnStyle.Render("[modified]")
	}
	titleBar := sectionTitleStyle.Render(titleText)

	statusParts := []string{
		dimStyle.Render(e.filePath),
		dimStyle.Render(fmt.Sprintf("Ln %d, Col %d", e.cursorLine, e.cursorCol)),
		dimStyle.Render(fmt.Sprintf("%d lines", e.lineCount)),
	}
	statusBar := dimStyle.Render(strings.Join(statusParts, "  │  "))

	helpBar := dimStyle.Render("Ctrl+S save  •  Esc close")

	var errLine string
	if e.saveErr != "" {
		errLine = errorStyle.Render("Save error: " + e.saveErr)
	}

	chrome := 1 + 1 + 1 // title + status + help
	if errLine != "" {
		chrome++
	}
	areaHeight := max(3, height-chrome)

	e.area.SetWidth(width)
	e.area.SetHeight(areaHeight)

	parts := []string{titleBar, e.area.View(), statusBar, helpBar}
	if errLine != "" {
		parts = append(parts, errLine)
	}

	return lipgloss.NewStyle().
		Width(width).MaxWidth(width).
		Height(height).MaxHeight(height).
		Render(strings.Join(parts, "\n"))
}

// resize updates the editor textarea dimensions.
func (e EditorModel) resize(width, height int) EditorModel {
	chrome := 4
	areaHeight := max(3, height-chrome)
	e.area.SetWidth(width)
	e.area.SetHeight(areaHeight)
	return e
}
