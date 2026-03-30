package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
)

// WorkflowKind identifies which in-app workflow operation is active.
type WorkflowKind int

const (
	WorkflowNone     WorkflowKind = iota
	WorkflowIngest                // process notes from a folder
	WorkflowGenerate              // generate a quiz for a class
	WorkflowExport                // export section/component dataset as JSON
)

// workflowStep tracks which stage of a workflow is running.
type workflowStep int

const (
	stepInput   workflowStep = iota // collecting user input
	stepConfirm                     // confirming critical settings (e.g., embeddings disabled)
	stepRunning                     // async operation in progress
	stepDone                        // finished; waiting for dismissal
)

// WorkflowModel manages multi-step in-app workflows as a modal overlay.
type WorkflowModel struct {
	kind    WorkflowKind
	step    workflowStep
	visible bool

	// Input fields shared or used per workflow kind.
	pathInput  textinput.Model // folder path (ingest only)
	classInput textinput.Model // class name (ingest/generate)
	countInput textinput.Model // quiz question count (generate)
	assessmentOptions []string // quiz/exam context profile options
	assessmentIdx     int
	questionTypeOpts  []string // context-default + supported sfq types
	questionTypeIdx   int
	fieldIdx   int             // focused field index for current workflow kind

	// Ingest-specific options.
	cleanBeforeIngest bool // if true, delete all previous ingestion data before starting

	// Export-specific options.
	includeEmbeddings bool // if true, include embeddings in exported JSON

	// Live agent-step display populated during stepRunning for streaming workflows.
	actionLines   []string // one entry per tool action invoked so far
	actionOffset  int      // top line offset for scrollable running log
	actionRows    int      // viewport line count for running log
	followActions bool     // auto-follow new updates only while already at bottom
	pendingResult string   // accumulated text parts arriving before done
	doneOffset    int      // top line offset for wrapped done/error content
	doneRows      int      // viewport line count for wrapped done/error content

	// Confirmation step display.
	confirmType string // what we're confirming (e.g., "embeddings_disabled")
	confirmMsg  string // message to show user

	// Outcome display.
	result string
	errMsg string

	// File picker sub-overlay for selecting individual files.
	filePicker    FilePickerModel
	selectedFiles []string // files chosen via the picker (overrides folder path for ingest)
}

func newWorkflow() WorkflowModel {
	pathInput := textinput.New()
	pathInput.Placeholder = "folder path  (e.g. ./notes)"
	pathInput.CharLimit = 500
	pathInput.Width = 40

	classInput := textinput.New()
	classInput.Placeholder = "class name"
	classInput.CharLimit = 120
	classInput.Width = 40

	countInput := textinput.New()
	countInput.Placeholder = "10"
	countInput.CharLimit = 4
	countInput.Width = 16

	assessmentOptions := make([]string, 0, len(classpkg.ContextProfiles()))
	for _, profile := range classpkg.ContextProfiles() {
		assessmentOptions = append(assessmentOptions, profile.Kind)
	}
	if len(assessmentOptions) == 0 {
		assessmentOptions = []string{"quiz"}
	}

	questionTypeOpts := []string{"context-default"}
	questionTypeOpts = append(questionTypeOpts, sfq.SupportedQuestionTypes()...)

	return WorkflowModel{
		pathInput:         pathInput,
		classInput:        classInput,
		countInput:        countInput,
		assessmentOptions: assessmentOptions,
		questionTypeOpts:  questionTypeOpts,
		filePicker:        newFilePicker(),
	}
}

func (w WorkflowModel) resize(width int) WorkflowModel {
	w.pathInput.Width = clamp(width-6, 18, width)
	w.classInput.Width = clamp(width-6, 18, width)
	w.countInput.Width = clamp(width-30, 10, 18)
	return w
}

// Open initialises and shows the workflow overlay for the given kind.
func (w WorkflowModel) Open(kind WorkflowKind, defaultClass string) WorkflowModel {
	w.kind = kind
	w.step = stepInput
	w.visible = true
	w.fieldIdx = 0
	w.result = ""
	w.errMsg = ""
	w.actionLines = nil
	w.actionOffset = 0
	w.actionRows = 8
	w.followActions = true
	w.pendingResult = ""
	w.doneOffset = 0
	w.doneRows = 8
	w.pathInput.SetValue("")
	w.classInput.SetValue(defaultClass)
	w.countInput.SetValue("10")
	w.pathInput.Placeholder = "folder path  (e.g. ./notes)"
	w.classInput.Placeholder = "class name"
	w.assessmentIdx = 0
	w.questionTypeIdx = 0
	w.cleanBeforeIngest = false
	w.includeEmbeddings = false
	w.selectedFiles = nil
	w.filePicker = w.filePicker.Open("")
	w.filePicker.visible = false // closed by default until user activates it

	switch kind {
	case WorkflowIngest:
		w.classInput.Placeholder = "class (optional)"
		w.pathInput.Focus()
		w.classInput.Blur()
	case WorkflowGenerate:
		w.classInput.Placeholder = "class name (required)"
		w.fieldIdx = 0
		w.updateGenerateFieldFocus()
	case WorkflowExport:
		w.classInput.Placeholder = "class (optional)"
		w.pathInput.Placeholder = "output file (e.g. ./knowledge-export.json)"
		w.pathInput.SetValue(filepath.Join(".", fmt.Sprintf("knowledge-export-%s.json", time.Now().UTC().Format("20060102-150405"))))
		w.fieldIdx = 0
		w.updateExportFieldFocus()
	}
	return w
}

func (w WorkflowModel) title() string {
	switch w.kind {
	case WorkflowIngest:
		return "Ingest Notes"
	case WorkflowGenerate:
		return "Generate Quiz"
	case WorkflowExport:
		return "Export Knowledge"
	}
	return "Workflow"
}

// Update handles all messages while the workflow overlay is visible.
// Returns (updated model, busy flag, status string, tea.Cmd).
func (w WorkflowModel) Update(msg tea.Msg, orc *orchestrator.Orchestrator, cfg *config.Config) (WorkflowModel, bool, string, tea.Cmd) {
	if !w.visible {
		return w, false, "", nil
	}

	// Delegate to file picker when it is open.
	if w.filePicker.Visible() {
		var cmd tea.Cmd
		w.filePicker, cmd = w.filePicker.Update(msg)
		if !w.filePicker.Visible() {
			if w.filePicker.Done() {
				w.selectedFiles = w.filePicker.SelectedFiles()
				if len(w.selectedFiles) > 0 {
					return w, false, fmt.Sprintf("%d file(s) selected", len(w.selectedFiles)), cmd
				}
				return w, false, "No files selected", cmd
			}
			// Cancelled — keep previous selection.
			return w, false, "", cmd
		}
		return w, false, "", cmd
	}

	switch w.step {
	case stepInput:
		return w.updateInput(msg, orc, cfg)
	case stepConfirm:
		return w.updateConfirm(msg, orc, cfg)
	case stepRunning:
		return w.updateRunning(msg)
	case stepDone:
		return w.updateDone(msg)
	}
	return w, false, "", nil
}

func (w WorkflowModel) updateInput(msg tea.Msg, orc *orchestrator.Orchestrator, cfg *config.Config) (WorkflowModel, bool, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			w.visible = false
			return w, false, "Workflow cancelled", nil
		case "tab":
			count := w.fieldCount()
			if count > 0 {
				w.fieldIdx = (w.fieldIdx + 1) % count
				w.updateFieldFocus()
			}
			return w, false, "", nil
		case "shift+tab":
			count := w.fieldCount()
			if count > 0 {
				w.fieldIdx = (w.fieldIdx - 1 + count) % count
				w.updateFieldFocus()
			}
			return w, false, "", nil
		case "up":
			count := w.fieldCount()
			if count > 0 {
				w.fieldIdx = (w.fieldIdx - 1 + count) % count
				w.updateFieldFocus()
			}
			return w, false, "", nil
		case "down":
			count := w.fieldCount()
			if count > 0 {
				w.fieldIdx = (w.fieldIdx + 1) % count
				w.updateFieldFocus()
			}
			return w, false, "", nil
		case "left", "right":
			if w.kind == WorkflowGenerate && w.fieldIdx == 2 {
				w.assessmentIdx = cycleIndex(w.assessmentIdx, len(w.assessmentOptions), k.String() == "right")
				return w, false, "", nil
			}
			if w.kind == WorkflowGenerate && w.fieldIdx == 3 {
				w.questionTypeIdx = cycleIndex(w.questionTypeIdx, len(w.questionTypeOpts), k.String() == "right")
				return w, false, "", nil
			}
			// Toggle checkbox on left/right when on clean field
			if w.kind == WorkflowIngest && w.fieldIdx == 2 {
				w.cleanBeforeIngest = !w.cleanBeforeIngest
			}
			if w.kind == WorkflowExport && w.fieldIdx == 2 {
				w.includeEmbeddings = !w.includeEmbeddings
			}
			return w, false, "", nil
		case "space":
			if w.kind == WorkflowGenerate && w.fieldIdx == 2 {
				w.assessmentIdx = cycleIndex(w.assessmentIdx, len(w.assessmentOptions), true)
				return w, false, "", nil
			}
			if w.kind == WorkflowGenerate && w.fieldIdx == 3 {
				w.questionTypeIdx = cycleIndex(w.questionTypeIdx, len(w.questionTypeOpts), true)
				return w, false, "", nil
			}
			// Toggle checkbox on space when on clean field
			if w.kind == WorkflowIngest && w.fieldIdx == 2 {
				w.cleanBeforeIngest = !w.cleanBeforeIngest
				return w, false, "", nil
			}
			if w.kind == WorkflowExport && w.fieldIdx == 2 {
				w.includeEmbeddings = !w.includeEmbeddings
				return w, false, "", nil
			}
			// Otherwise, pass through to current field
		case "enter":
			// Field 3 on WorkflowIngest opens the file picker.
			if w.kind == WorkflowIngest && w.fieldIdx == 3 {
				startDir := strings.TrimSpace(w.pathInput.Value())
				w.filePicker = w.filePicker.Open(startDir)
				return w, false, "Browse files…", nil
			}
			return w.startWorkflow(orc, cfg)
		}
	}

	// Route input to the focused field.
	var cmd tea.Cmd
	if (w.kind == WorkflowIngest || w.kind == WorkflowExport) && w.fieldIdx == 0 {
		w.pathInput, cmd = w.pathInput.Update(msg)
	} else if w.kind == WorkflowGenerate {
		switch w.fieldIdx {
		case 0:
			w.classInput, cmd = w.classInput.Update(msg)
		case 1:
			w.countInput, cmd = w.countInput.Update(msg)
		default:
			cmd = nil
		}
	} else if w.kind == WorkflowExport {
		w.classInput, cmd = w.classInput.Update(msg)
	} else {
		w.classInput, cmd = w.classInput.Update(msg)
	}
	return w, false, "", cmd

}

func (w *WorkflowModel) updateIngestFieldFocus() {
	w.pathInput.Blur()
	w.classInput.Blur()
	w.countInput.Blur()
	switch w.fieldIdx {
	case 0:
		w.pathInput.Focus()
	case 1:
		w.classInput.Focus()
	case 2, 3:
		// Checkbox and browse button rows — no text input focus.
	}
}

func (w *WorkflowModel) updateGenerateFieldFocus() {
	w.pathInput.Blur()
	w.classInput.Blur()
	w.countInput.Blur()
	switch w.fieldIdx {
	case 0:
		w.classInput.Focus()
	case 1:
		w.countInput.Focus()
	case 2, 3:
		// Selection rows use highlighted styling only.
	}
}

func (w *WorkflowModel) updateExportFieldFocus() {
	w.pathInput.Blur()
	w.classInput.Blur()
	w.countInput.Blur()
	switch w.fieldIdx {
	case 0:
		w.pathInput.Focus()
	case 1:
		w.classInput.Focus()
	case 2:
		// Checkbox row uses highlighted styling only.
	}
}

func (w WorkflowModel) fieldCount() int {
	switch w.kind {
	case WorkflowIngest:
		return 4 // path, class, clean checkbox, browse
	case WorkflowGenerate:
		return 4 // class, count, assessment, question preference
	case WorkflowExport:
		return 3
	default:
		return 0
	}
}

func (w *WorkflowModel) updateFieldFocus() {
	switch w.kind {
	case WorkflowIngest:
		w.updateIngestFieldFocus()
	case WorkflowGenerate:
		w.updateGenerateFieldFocus()
	case WorkflowExport:
		w.updateExportFieldFocus()
	}
}

func (w WorkflowModel) startWorkflow(orc *orchestrator.Orchestrator, cfg *config.Config) (WorkflowModel, bool, string, tea.Cmd) {
	class := strings.TrimSpace(w.classInput.Value())

	switch w.kind {
	case WorkflowIngest:
		path := strings.TrimSpace(w.pathInput.Value())
		if len(w.selectedFiles) > 0 {
			// Individual-file mode: bypass the embeddings confirmation (same logic applies).
			if orc.EmbeddingProvider.Disabled() {
				w.step = stepConfirm
				w.confirmType = "embeddings_disabled"
				w.confirmMsg = fmt.Sprintf(
					"Embeddings are not configured (provider: %s).\nDeduplication and semantic consolidation will NOT happen.\n\nContinue anyway?",
					orc.EmbeddingProvider.Name(),
				)
				return w, false, "", nil
			}
			if w.cleanBeforeIngest {
				if err := state.ClearIngestedData(); err != nil {
					return w, false, fmt.Sprintf("Failed to clear ingestion data: %v", err), nil
				}
			}
			w.step = stepRunning
			return w, true, "Running " + w.title() + "…", runIngestFilesCmd(w.selectedFiles, class, orc, cfg)
		}
		return w.runIngestWorkflow(path, class, orc, cfg, true)

	case WorkflowGenerate:
		if class == "" {
			return w, false, "Class name is required", nil
		}
		opts := quiz.QuizOptions{
			AssessmentKind: w.selectedAssessmentKind(),
			Count:          parseQuizCount(w.countInput.Value()),
			TypePreference: w.selectedQuestionPreference(),
		}
		w.step = stepRunning
		return w, true, "Running " + w.title() + "…", runQuizCmd(class, opts, orc, cfg)

	case WorkflowExport:
		path := strings.TrimSpace(w.pathInput.Value())
		if path == "" {
			return w, false, "Output file path is required", nil
		}
		w.step = stepRunning
		return w, true, "Running " + w.title() + "…", runExportKnowledgeCmd(path, class, w.includeEmbeddings)
	}

	return w, false, "", nil
}

func (w WorkflowModel) runIngestWorkflow(path, class string, orc *orchestrator.Orchestrator, cfg *config.Config, requireEmbeddingsConfirm bool) (WorkflowModel, bool, string, tea.Cmd) {
	if path == "" {
		return w, false, "Folder path is required", nil
	}

	if requireEmbeddingsConfirm && orc.EmbeddingProvider.Disabled() {
		w.step = stepConfirm
		w.confirmType = "embeddings_disabled"
		w.confirmMsg = fmt.Sprintf(
			"Embeddings are not configured (provider: %s).\nDeduplication and semantic consolidation will NOT happen.\n\nContinue anyway?",
			orc.EmbeddingProvider.Name(),
		)
		return w, false, "", nil
	}

	if w.cleanBeforeIngest {
		if err := state.ClearIngestedData(); err != nil {
			return w, false, fmt.Sprintf("Failed to clear ingestion data: %v", err), nil
		}
	}

	w.step = stepRunning
	return w, true, "Running " + w.title() + "…", runIngestCmd(path, class, orc, cfg)
}

func (w WorkflowModel) updateConfirm(msg tea.Msg, orc *orchestrator.Orchestrator, cfg *config.Config) (WorkflowModel, bool, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "enter":
			// User confirmed; proceed with workflow
			var cmd tea.Cmd
			class := strings.TrimSpace(w.classInput.Value())
			path := strings.TrimSpace(w.pathInput.Value())

			switch w.kind {
			case WorkflowIngest:
				if len(w.selectedFiles) > 0 {
					if w.cleanBeforeIngest {
						if err := state.ClearIngestedData(); err != nil {
							return w, false, fmt.Sprintf("Failed to clear ingestion data: %v", err), nil
						}
					}
					w.step = stepRunning
					cmd = runIngestFilesCmd(w.selectedFiles, class, orc, cfg)
				} else {
					return w.runIngestWorkflow(path, class, orc, cfg, false)
				}
			case WorkflowGenerate:
				opts := quiz.QuizOptions{
					AssessmentKind: w.selectedAssessmentKind(),
					Count:          parseQuizCount(w.countInput.Value()),
					TypePreference: w.selectedQuestionPreference(),
				}
				w.step = stepRunning
				cmd = runQuizCmd(class, opts, orc, cfg)
			}
			return w, true, "Running " + w.title() + "…", cmd

		case "esc":
			// User cancelled; go back to input
			w.step = stepInput
			w.confirmType = ""
			w.confirmMsg = ""
			return w, false, "Cancelled", nil
		}
	}
	return w, false, "", nil
}

func (w WorkflowModel) updateRunning(msg tea.Msg) (WorkflowModel, bool, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			w.actionOffset--
			w.clampActionOffset()
			w.followActions = w.isAtBottom()
			return w, true, "", nil
		case "down", "j":
			w.actionOffset++
			w.clampActionOffset()
			w.followActions = w.isAtBottom()
			return w, true, "", nil
		case "pgup", "b":
			w.actionOffset -= w.actionRows
			w.clampActionOffset()
			w.followActions = w.isAtBottom()
			return w, true, "", nil
		case "pgdown", "f":
			w.actionOffset += w.actionRows
			w.clampActionOffset()
			w.followActions = w.isAtBottom()
			return w, true, "", nil
		case "home", "g":
			w.actionOffset = 0
			w.clampActionOffset()
			w.followActions = false
			return w, true, "", nil
		case "end", "G":
			w.scrollActionsToBottom()
			w.followActions = true
			return w, true, "", nil
		}
	}

	switch msg := msg.(type) {
	case workflowDoneMsg:
		w.step = stepDone
		w.doneOffset = 0
		if msg.err != nil {
			w.errMsg = msg.err.Error()
			return w, false, w.title() + " failed", nil
		}
		w.result = msg.summary
		return w, false, w.title() + " complete! Press Enter to close", nil
	case aiStreamMsg:
		if msg.err != nil {
			w.step = stepDone
			w.doneOffset = 0
			w.errMsg = msg.err.Error()
			return w, false, w.title() + " failed", nil
		}
		if msg.done {
			w.step = stepDone
			w.doneOffset = 0
			if w.pendingResult != "" {
				w.result = strings.TrimSpace(w.pendingResult)
			} else {
				w.result = "Done"
			}
			return w, false, w.title() + " complete! Press Enter to close", nil
		}
		if msg.part != "" {
			w.pendingResult += msg.part
		}
		if msg.actionLabel != "" {
			wasAtBottom := w.isAtBottom()
			w.actionLines = updateActionLines(w.actionLines, msg.actionLabel, msg.actionInfo, msg.actionDone, msg.err)
			if wasAtBottom || w.followActions {
				w.scrollActionsToBottom()
			}
			w.followActions = w.isAtBottom()
		}
		return w, true, "", waitForAIStreamCmd(msg.stream)
	}
	return w, true, "", nil
}

func (w *WorkflowModel) clampActionOffset() {
	maxOffset := w.maxActionOffset()
	if w.actionOffset < 0 {
		w.actionOffset = 0
	}
	if w.actionOffset > maxOffset {
		w.actionOffset = maxOffset
	}
}

func (w WorkflowModel) maxActionOffset() int {
	rows := w.actionRows
	if rows <= 0 {
		rows = 1
	}
	maxOffset := len(w.actionLines) - rows
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func (w WorkflowModel) isAtBottom() bool {
	return w.actionOffset >= w.maxActionOffset()
}

func (w *WorkflowModel) scrollActionsToBottom() {
	w.actionOffset = w.maxActionOffset()
}

func (w WorkflowModel) visibleActionLines() []string {
	if len(w.actionLines) == 0 {
		return nil
	}
	rows := w.actionRows
	if rows <= 0 {
		rows = 1
	}
	start := w.actionOffset
	if start < 0 {
		start = 0
	}
	if start > len(w.actionLines) {
		start = len(w.actionLines)
	}
	end := start + rows
	if end > len(w.actionLines) {
		end = len(w.actionLines)
	}
	return w.actionLines[start:end]
}

// updateActionLines adds or updates an action entry in the action log.
func updateActionLines(lines []string, label, detail string, done bool, err error) []string {
	var suffix string
	switch {
	case !done:
		suffix = "…"
	case err != nil:
		suffix = " ✗"
	default:
		suffix = " ✓"
	}
	entry := label
	if detail != "" {
		entry += ": " + detail
	}
	entry += suffix
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], label) {
			lines[i] = entry
			return lines
		}
	}
	return append(lines, entry)
}

func (w WorkflowModel) updateDone(msg tea.Msg) (WorkflowModel, bool, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			w.doneOffset--
			if w.doneOffset < 0 {
				w.doneOffset = 0
			}
			return w, false, "", nil
		case "down", "j":
			w.doneOffset++
			return w, false, "", nil
		case "pgup", "b":
			w.doneOffset -= w.doneRows
			if w.doneOffset < 0 {
				w.doneOffset = 0
			}
			return w, false, "", nil
		case "pgdown", "f":
			w.doneOffset += w.doneRows
			return w, false, "", nil
		case "home", "g":
			w.doneOffset = 0
			return w, false, "", nil
		case "end", "G":
			w.doneOffset = 1 << 30
			return w, false, "", nil
		case "c":
			payload := strings.TrimSpace(w.doneContent())
			if payload == "" {
				return w, false, "Nothing to copy", nil
			}
			if err := clipboard.WriteAll(payload); err != nil {
				return w, false, "Copy failed: " + err.Error(), nil
			}
			return w, false, "Copied workflow output", nil
		case "enter", "esc", "q":
			w.visible = false
			return w, false, "Workflow closed", nil
		}
	}
	return w, false, "", nil
}

// View renders the workflow modal overlay string.
func (w WorkflowModel) View(width, height int) string {
	panelWidth := clamp(width-10, 42, 78)
	panelHeight := clamp(height-6, 12, 26)
	innerWidth := clamp(panelWidth-workflowStyle.GetHorizontalFrameSize(), 24, panelWidth)
	innerHeight := clamp(panelHeight-workflowStyle.GetVerticalFrameSize(), 8, panelHeight)

	if w.step == stepRunning {
		// Reserve room for title, subtitle, status, and scroll hint/footer.
		rows := clamp(innerHeight-6, 3, innerHeight-2)
		w.actionRows = rows
		w.clampActionOffset()
	}
	if w.step == stepDone {
		// Reserve room for title, subtitle, section title, and footer hints.
		w.doneRows = clamp(innerHeight-8, 3, innerHeight-3)
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(w.title()) + "\n")
	b.WriteString(dimStyle.Render("Guided workflow modal") + "\n\n")

	// When the file picker is active, render it in place of the normal content.
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
		if w.doneOffset > maxOffset {
			w.doneOffset = maxOffset
		}
		if w.doneOffset < 0 {
			w.doneOffset = 0
		}

		end := w.doneOffset + w.doneRows
		if end > len(allLines) {
			end = len(allLines)
		}
		visible := allLines[w.doneOffset:end]

		b.WriteString(sectionTitleStyle.Render(title) + "\n")
		for _, line := range visible {
			b.WriteString(lineStyle.Render(line) + "\n")
		}
		if len(allLines) > 0 {
			start := w.doneOffset + 1
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
		// Browse-files button row.
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
			b.WriteString(dimStyle.Render("Available: "+truncate(strings.Join(classes, ", "), 120)) + "\n")
		}
		b.WriteString(w.classInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Question count:") + "\n")
		b.WriteString(w.countInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Assessment type:") + "\n")
		b.WriteString(w.renderSelectionRow(w.selectedAssessmentKind(), w.fieldIdx == 2) + "\n\n")
		b.WriteString(labelStyle.Render("Question preference:") + "\n")
		b.WriteString(w.renderSelectionRow(w.selectedQuestionPreference(), w.fieldIdx == 3) + "\n")
		b.WriteString(dimStyle.Render("context-default uses default_question_type from class context file") + "\n\n")
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

func (w WorkflowModel) selectedAssessmentKind() string {
	if len(w.assessmentOptions) == 0 {
		return classpkg.DefaultContextProfile()
	}
	idx := w.assessmentIdx
	if idx < 0 || idx >= len(w.assessmentOptions) {
		idx = 0
	}
	return classpkg.NormalizeContextProfile(w.assessmentOptions[idx])
}

func (w WorkflowModel) selectedQuestionPreference() string {
	if len(w.questionTypeOpts) == 0 {
		return "context-default"
	}
	idx := w.questionTypeIdx
	if idx < 0 || idx >= len(w.questionTypeOpts) {
		idx = 0
	}
	value := strings.TrimSpace(w.questionTypeOpts[idx])
	if value == "" {
		return "context-default"
	}
	return value
}

func (w WorkflowModel) renderSelectionRow(value string, focused bool) string {
	row := "< " + value + " >"
	if focused {
		return warnStyle.Render(row)
	}
	return dimStyle.Render(row)
}

func cycleIndex(current, size int, forward bool) int {
	if size <= 0 {
		return 0
	}
	if current < 0 || current >= size {
		current = 0
	}
	if forward {
		return (current + 1) % size
	}
	return (current - 1 + size) % size
}

func (w WorkflowModel) doneContent() string {
	if w.errMsg != "" {
		return w.errMsg
	}
	if w.result != "" {
		return w.result
	}
	return "Done"
}

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

func parseQuizCount(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 10
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 10
	}
	if n > 100 {
		return 100
	}
	return n
}
