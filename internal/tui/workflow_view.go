package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	classpkg "github.com/studyforge/study-agent/internal/class"
)

func wrapTextLines(text string, width int) []string {
	if width <= 1 {
		width = 1
	}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(normalized, "\n")
	var lines []string
	for _, part := range parts {
		wrapped := wrapLine(part, width)
		if len(wrapped) == 0 {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapped...)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func wrapLine(line string, width int) []string {
	if line == "" {
		return []string{""}
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}

	var out []string
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		if lipgloss.Width(current) > width {
			out = append(out, splitLongWord(current, width)...)
		} else {
			out = append(out, current)
		}
		current = word
	}
	if lipgloss.Width(current) > width {
		out = append(out, splitLongWord(current, width)...)
	} else {
		out = append(out, current)
	}
	return out
}

func splitLongWord(word string, width int) []string {
	if width <= 1 {
		return strings.Split(word, "")
	}
	var lines []string
	var b strings.Builder
	for _, r := range word {
		candidate := b.String() + string(r)
		if lipgloss.Width(candidate) > width {
			lines = append(lines, b.String())
			b.Reset()
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	if len(lines) == 0 {
		return []string{word}
	}
	return lines
}

// View renders the workflow modal overlay string.
func (w WorkflowModel) View(width, height int) string {
	panelWidth := clamp(width-10, 42, 78)
	panelHeight := clamp(height-6, 12, 26)
	innerWidth := clamp(panelWidth-workflowStyle.GetHorizontalFrameSize(), 24, panelWidth)
	innerHeight := clamp(panelHeight-workflowStyle.GetVerticalFrameSize(), 8, panelHeight)

	// actionRows and doneRows are kept up to date via resize; no mutations here.

	var b strings.Builder
	b.WriteString(headerStyle.Render(w.title()) + "\n")
	b.WriteString(dimStyle.Render("Guided workflow modal") + "\n\n")

	if w.filePicker.Visible() {
		pickerContent := w.filePicker.View(innerWidth, innerHeight)
		content := lipgloss.NewStyle().Width(innerWidth).Height(innerHeight).MaxHeight(innerHeight).Render(pickerContent)
		return workflowStyle.Width(innerWidth).Height(innerHeight).Render(content)
	}

	switch w.step {
	case stepInput:
		b.WriteString(w.viewInputStep())
	case stepConfirm:
		b.WriteString(w.viewConfirmStep())
	case stepRunning:
		b.WriteString(warnStyle.Render("Running, please wait…") + "\n")
		visible := w.visibleActionLines()
		for _, line := range visible {
			b.WriteString(dimStyle.Render("  "+line) + "\n")
		}
		if len(w.actionLines) == 0 {
			b.WriteString(dimStyle.Render("  Waiting for updates...") + "\n")
		}
		if len(w.actionLines) > 0 {
			start := w.actionOffset + 1
			end := w.actionOffset + len(visible)
			b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Showing %d-%d of %d  •  ↑/↓ scroll  •  PgUp/PgDn jump  •  End follow", start, end, len(w.actionLines))))
		}
	case stepDone:
		title := "Result"
		lineStyle := dimStyle
		if w.errMsg != "" {
			title = "Error"
			lineStyle = errorStyle
		}
		wrapWidth := clamp(innerWidth-2, 12, innerWidth)
		allLines := wrapTextLines(w.doneContent(), wrapWidth)
		maxOffset := len(allLines) - w.doneRows
		if maxOffset < 0 {
			maxOffset = 0
		}
		doneOffset := w.doneOffset
		if doneOffset > maxOffset {
			doneOffset = maxOffset
		}
		if doneOffset < 0 {
			doneOffset = 0
		}

		end := doneOffset + w.doneRows
		if end > len(allLines) {
			end = len(allLines)
		}
		visible := allLines[doneOffset:end]

		b.WriteString(sectionTitleStyle.Render(title) + "\n")
		for _, line := range visible {
			b.WriteString(lineStyle.Render(line) + "\n")
		}
		if len(allLines) > 0 {
			start := doneOffset + 1
			b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Showing %d-%d of %d  •  ↑/↓ scroll  •  PgUp/PgDn jump  •  c copy", start, end, len(allLines))))
		}
		b.WriteString("\n" + dimStyle.Render("Enter/Esc close  •  Home/End jump"))
	}

	content := lipgloss.NewStyle().Width(innerWidth).Height(innerHeight).MaxHeight(innerHeight).Render(b.String())
	return workflowStyle.Width(innerWidth).Height(innerHeight).Render(content)
}

func (w WorkflowModel) viewConfirmStep() string {
	var b strings.Builder
	b.WriteString(warnStyle.Render("⚠ Confirm Settings") + "\n\n")
	b.WriteString(w.confirmMsg + "\n\n")
	b.WriteString(dimStyle.Render("Enter to continue  •  Esc to go back"))
	return b.String()
}

func (w WorkflowModel) viewInputStep() string {
	var b strings.Builder
	switch w.kind {
	case WorkflowIngest:
		b.WriteString(labelStyle.Render("Folder path:") + "\n")
		b.WriteString(w.pathInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Class (optional):") + "\n")
		b.WriteString(w.classInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Clean before ingest:") + "\n")
		checkboxStyle := dimStyle
		if w.fieldIdx == 2 {
			checkboxStyle = warnStyle
		}
		checkMark := "[ ]"
		if w.cleanBeforeIngest {
			checkMark = "[✓]"
		}
		b.WriteString(checkboxStyle.Render(checkMark + " Delete all previous ingestion data and start fresh\n\n"))
		browseStyle := dimStyle
		if w.fieldIdx == 3 {
			browseStyle = warnStyle
		}
		if len(w.selectedFiles) > 0 {
			b.WriteString(browseStyle.Render(fmt.Sprintf("[✓] %d file(s) selected  (Enter to browse again)\n\n", len(w.selectedFiles))))
		} else {
			b.WriteString(browseStyle.Render("[ ] Browse & select individual files  (Enter to open picker)\n\n"))
		}
		b.WriteString(dimStyle.Render("↑/↓ or Tab to navigate  •  Space to toggle  •  Enter to start or browse  •  Esc to cancel"))
	case WorkflowGenerate:
		b.WriteString(labelStyle.Render("Class name:") + "\n")
		if classes, _ := classpkg.List(); len(classes) > 0 {
			b.WriteString(dimStyle.Render("Available: "+truncateWidth(strings.Join(classes, ", "), 120)) + "\n")
		}
		b.WriteString(w.classInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Question count:") + "\n")
		b.WriteString(w.countInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Assessment type:") + "\n")
		b.WriteString(w.renderSelectionRow(w.selectedAssessmentKind(), w.fieldIdx == 2) + "\n\n")
		b.WriteString(labelStyle.Render("Question preference:") + "\n")
		b.WriteString(w.renderSelectionRow(w.selectedQuestionPreference(), w.fieldIdx == 3) + "\n")
		b.WriteString(dimStyle.Render("context-default uses default_question_type from class context file") + "\n\n")
		if w.selectedAssessmentKind() == "focused" {
			b.WriteString(labelStyle.Render("Sections to focus on:") + "\n")
			b.WriteString(dimStyle.Render("Comma-separated section titles or IDs (leave blank for all sections)") + "\n")
			b.WriteString(w.sectionsInput.View() + "\n\n")
		}
		b.WriteString(dimStyle.Render("Tab/Shift+Tab navigate  •  Left/Right or Space cycle  •  Enter to generate  •  Esc to cancel"))
	case WorkflowExport:
		b.WriteString(labelStyle.Render("Output file:") + "\n")
		b.WriteString(w.pathInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Class filter (optional):") + "\n")
		b.WriteString(w.classInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Include embeddings:") + "\n")
		checkboxStyle := dimStyle
		if w.fieldIdx == 2 {
			checkboxStyle = warnStyle
		}
		checkMark := "[ ]"
		if w.includeEmbeddings {
			checkMark = "[✓]"
		}
		b.WriteString(checkboxStyle.Render(checkMark + " Include section/component embedding vectors\n\n"))
		b.WriteString(dimStyle.Render("↑/↓ or Tab to navigate  •  Space to toggle  •  Enter to export  •  Esc to cancel"))
	}
	return b.String()
}

func (w WorkflowModel) renderSelectionRow(value string, focused bool) string {
	row := "< " + value + " >"
	if focused {
		return warnStyle.Render(row)
	}
	return dimStyle.Render(row)
}
