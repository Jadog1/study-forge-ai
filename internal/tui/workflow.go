package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
)

// WorkflowKind identifies which in-app workflow operation is active.
type WorkflowKind int

const (
	WorkflowNone     WorkflowKind = iota
	WorkflowIngest                // process notes from a folder
	WorkflowGenerate              // generate a quiz for a class
	WorkflowAdapt                 // adaptive quiz from performance history
)

// workflowStep tracks which stage of a workflow is running.
type workflowStep int

const (
	stepInput   workflowStep = iota // collecting user input
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
	classInput textinput.Model // class name (ingest/generate/adapt)
	fieldIdx   int             // focused field index within ingest (0=path, 1=class)

	// Live agent-step display populated during stepRunning for streaming workflows.
	actionLines   []string // one entry per tool action invoked so far
	pendingResult string   // accumulated text parts arriving before done

	// Outcome display.
	result string
	errMsg string
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

	return WorkflowModel{pathInput: pathInput, classInput: classInput}
}

func (w WorkflowModel) resize(width int) WorkflowModel {
	w.pathInput.Width = clamp(width-6, 18, width)
	w.classInput.Width = clamp(width-6, 18, width)
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
	w.pendingResult = ""
	w.pathInput.SetValue("")
	w.classInput.SetValue(defaultClass)

	switch kind {
	case WorkflowIngest:
		w.classInput.Placeholder = "class (optional)"
		w.pathInput.Focus()
		w.classInput.Blur()
	case WorkflowGenerate:
		w.classInput.Placeholder = "class name (required)"
		w.classInput.Focus()
	case WorkflowAdapt:
		w.classInput.Placeholder = "class name (required)"
		w.classInput.Focus()
	}
	return w
}

func (w WorkflowModel) title() string {
	switch w.kind {
	case WorkflowIngest:
		return "Ingest Notes"
	case WorkflowGenerate:
		return "Generate Quiz"
	case WorkflowAdapt:
		return "Adapt Content"
	}
	return "Workflow"
}

// Update handles all messages while the workflow overlay is visible.
// Returns (updated model, busy flag, status string, tea.Cmd).
func (w WorkflowModel) Update(msg tea.Msg, orc *orchestrator.Orchestrator, cfg *config.Config) (WorkflowModel, bool, string, tea.Cmd) {
	if !w.visible {
		return w, false, "", nil
	}
	switch w.step {
	case stepInput:
		return w.updateInput(msg, orc, cfg)
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
			// Ingest has two fields; cycle focus between them.
			if w.kind == WorkflowIngest {
				if w.fieldIdx == 0 {
					w.fieldIdx = 1
					w.pathInput.Blur()
					w.classInput.Focus()
				} else {
					w.fieldIdx = 0
					w.classInput.Blur()
					w.pathInput.Focus()
				}
			}
			return w, false, "", nil
		case "enter":
			return w.startWorkflow(orc, cfg)
		}
	}

	// Route input to the focused field.
	var cmd tea.Cmd
	if w.kind == WorkflowIngest && w.fieldIdx == 0 {
		w.pathInput, cmd = w.pathInput.Update(msg)
	} else {
		w.classInput, cmd = w.classInput.Update(msg)
	}
	return w, false, "", cmd
}

func (w WorkflowModel) startWorkflow(orc *orchestrator.Orchestrator, cfg *config.Config) (WorkflowModel, bool, string, tea.Cmd) {
	class := strings.TrimSpace(w.classInput.Value())
	var cmd tea.Cmd

	switch w.kind {
	case WorkflowIngest:
		path := strings.TrimSpace(w.pathInput.Value())
		if path == "" {
			return w, false, "Folder path is required", nil
		}
		w.step = stepRunning
		cmd = runIngestCmd(path, class, orc, cfg)

	case WorkflowGenerate:
		if class == "" {
			return w, false, "Class name is required", nil
		}
		w.step = stepRunning
		cmd = runGenerateCmd(class, nil, orc, cfg)

	case WorkflowAdapt:
		if class == "" {
			return w, false, "Class name is required", nil
		}
		w.step = stepRunning
		cmd = runAdaptCmd(class, orc, cfg)
	}

	return w, true, "Running " + w.title() + "…", cmd
}

func (w WorkflowModel) updateRunning(msg tea.Msg) (WorkflowModel, bool, string, tea.Cmd) {
	switch msg := msg.(type) {
	case workflowDoneMsg:
		w.step = stepDone
		if msg.err != nil {
			w.errMsg = msg.err.Error()
			return w, false, w.title() + " failed", nil
		}
		w.result = msg.summary
		return w, false, w.title() + " complete! Press Enter to close", nil
	case aiStreamMsg:
		if msg.err != nil {
			w.step = stepDone
			w.errMsg = msg.err.Error()
			return w, false, w.title() + " failed", nil
		}
		if msg.done {
			w.step = stepDone
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
			w.actionLines = updateActionLines(w.actionLines, msg.actionLabel, msg.actionInfo, msg.actionDone, msg.err)
		}
		return w, true, "", waitForAIStreamCmd(msg.stream)
	}
	return w, true, "", nil
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
		case "enter", "esc", "q":
			w.visible = false
			return w, false, "Workflow closed", nil
		}
	}
	return w, false, "", nil
}

// View renders the workflow modal overlay string.
func (w WorkflowModel) View(width, _ int) string {
	panelWidth := clamp(width-10, 42, 78)
	innerWidth := clamp(panelWidth-workflowStyle.GetHorizontalFrameSize(), 24, panelWidth)

	var b strings.Builder
	b.WriteString(headerStyle.Render(w.title()) + "\n")
	b.WriteString(dimStyle.Render("Guided workflow modal") + "\n\n")

	switch w.step {
	case stepInput:
		b.WriteString(w.viewInputStep())
	case stepRunning:
		b.WriteString(warnStyle.Render("Running, please wait…") + "\n")
		for _, line := range w.actionLines {
			b.WriteString(dimStyle.Render("  "+line) + "\n")
		}
	case stepDone:
		if w.errMsg != "" {
			b.WriteString(renderSection("Error", errorStyle.Render(w.errMsg), innerWidth) + "\n")
		} else {
			resultBody := lipgloss.NewStyle().Width(innerWidth - 6).Render(w.result)
			b.WriteString(renderSection("Result", clipLines(resultBody, 8), innerWidth) + "\n")
		}
		b.WriteString("\n" + dimStyle.Render("Press Enter or Esc to close"))
	}

	return workflowStyle.Width(innerWidth).Render(b.String())
}

func (w WorkflowModel) viewInputStep() string {
	var b strings.Builder
	switch w.kind {
	case WorkflowIngest:
		b.WriteString(labelStyle.Render("Folder path:") + "\n")
		b.WriteString(w.pathInput.View() + "\n\n")
		b.WriteString(labelStyle.Render("Class (optional):") + "\n")
		b.WriteString(w.classInput.View() + "\n\n")
		b.WriteString(dimStyle.Render("Tab switches fields  •  Enter to start  •  Esc to cancel"))

	case WorkflowGenerate, WorkflowAdapt:
		b.WriteString(labelStyle.Render("Class name:") + "\n")
		if classes, _ := classpkg.List(); len(classes) > 0 {
			b.WriteString(dimStyle.Render("Available: "+truncate(strings.Join(classes, ", "), 120)) + "\n")
		}
		b.WriteString(w.classInput.View() + "\n\n")
		verb := map[WorkflowKind]string{WorkflowGenerate: "generate", WorkflowAdapt: "adapt"}[w.kind]
		b.WriteString(dimStyle.Render(fmt.Sprintf("Enter to %s  •  Esc to cancel", verb)))
	}
	return b.String()
}
