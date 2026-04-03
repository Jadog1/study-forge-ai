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
	pathInput         textinput.Model // folder path (ingest only)
	classInput        textinput.Model // class name (ingest/generate)
	countInput        textinput.Model // quiz question count (generate)
	assessmentOptions []string        // quiz/exam context profile options
	assessmentIdx     int
	questionTypeOpts  []string // context-default + supported sfq types
	questionTypeIdx   int
	fieldIdx          int // focused field index for current workflow kind

	// Ingest-specific options.
	cleanBeforeIngest bool // if true, delete all previous ingestion data before starting

	// Export-specific options.
	includeEmbeddings bool // if true, include embeddings in exported JSON

	sectionsInput textinput.Model // section filter for focused assessment mode

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

	sectionsInput := textinput.New()
	sectionsInput.Placeholder = "section titles or IDs, comma-separated (optional)"
	sectionsInput.CharLimit = 500
	sectionsInput.Width = 40

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
		sectionsInput:     sectionsInput,
		assessmentOptions: assessmentOptions,
		questionTypeOpts:  questionTypeOpts,
		filePicker:        newFilePicker(),
	}
}

func (w WorkflowModel) resize(width, height int) WorkflowModel {
	w.pathInput.Width = clamp(width-6, 18, width)
	w.classInput.Width = clamp(width-6, 18, width)
	w.countInput.Width = clamp(width-30, 10, 18)
	w.sectionsInput.Width = clamp(width-6, 18, width)

	panelHeight := clamp(height-6, 12, 26)
	innerHeight := clamp(panelHeight-workflowStyle.GetVerticalFrameSize(), 8, panelHeight)
	w.actionRows = clamp(innerHeight-6, 3, innerHeight-2)
	w.doneRows = clamp(innerHeight-8, 3, innerHeight-3)
	w.clampActionOffset()
	w.clampDoneOffset()
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
	w.sectionsInput.SetValue("")
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
		case 4:
			w.sectionsInput, cmd = w.sectionsInput.Update(msg)
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
	w.sectionsInput.Blur()
	switch w.fieldIdx {
	case 0:
		w.classInput.Focus()
	case 1:
		w.countInput.Focus()
	case 2, 3:
		// Selection rows use highlighted styling only.
	case 4:
		w.sectionsInput.Focus()
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
		if w.selectedAssessmentKind() == "focused" {
			return 5 // class, count, assessment, question preference, sections
		}
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
					AssessmentKind:  w.selectedAssessmentKind(),
					Count:           parseQuizCount(w.countInput.Value()),
					TypePreference:  w.selectedQuestionPreference(),
					FocusedSections: parseSectionsList(w.sectionsInput.Value()),
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

func (w *WorkflowModel) clampDoneOffset() {
	if w.doneOffset < 0 {
		w.doneOffset = 0
	}
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
			w.clampDoneOffset()
			return w, false, "", nil
		case "down", "j":
			w.doneOffset++
			w.clampDoneOffset()
			return w, false, "", nil
		case "pgup", "b":
			w.doneOffset -= w.doneRows
			w.clampDoneOffset()
			return w, false, "", nil
		case "pgdown", "f":
			w.doneOffset += w.doneRows
			w.clampDoneOffset()
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

func parseSectionsList(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	var result []string
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
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
