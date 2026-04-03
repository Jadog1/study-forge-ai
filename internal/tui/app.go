// Package tui provides the interactive terminal UI for study-agent.
// app.go wires together the model, tab components, and overlay components.
package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
)

// Launch starts the StudyForge TUI. The UI opens even when the AI provider is
// not yet configured; the user can set it up inside the Settings tab.
func Launch() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	orc := orchestrator.NewFallback(cfg)
	m := newModel(cfg, orc)
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// ── tea.Model ────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Window resize is always handled before overlays.
	if wm, ok := msg.(tea.WindowSizeMsg); ok {
		m = m.resize(wm.Width, wm.Height)
		return m, nil
	}

	// Command palette overlay steals all input when visible.
	if m.palette.visible {
		var action string
		var cmd tea.Cmd
		m.palette, action, cmd = m.palette.Update(msg)
		if action != "" {
			m, nextCmd := m.handlePaletteAction(action)
			return m, tea.Batch(cmd, nextCmd)
		}
		return m, cmd
	}

	// Workflow overlay steals all input when visible.
	if m.workflow.visible {
		var workflowBusy bool
		var status string
		var cmd tea.Cmd
		m.workflow, workflowBusy, status, cmd = m.workflow.Update(msg, m.orc, m.cfg)
		if status != "" {
			m.status = status
		}
		m.busy = workflowBusy
		return m, cmd
	}

	// Cross-cutting async results are routed to the right component
	// regardless of which tab is currently active so responses are never lost.
	switch msg := msg.(type) {
	case aiStreamMsg:
		if msg.actionLabel != "" {
			if msg.actionDone {
				m.chat = m.chat.finishAction(msg.actionLabel, msg.actionInfo, msg.err)
				if msg.err != nil {
					m.status = "Agent action failed"
				} else {
					m.status = "Agent action complete"
				}
			} else {
				m.chat = m.chat.startAction(msg.actionLabel, msg.actionInfo)
				m.status = "Agent action running…"
			}
			return m, waitForAIStreamCmd(msg.stream)
		}
		if msg.err != nil {
			m.busy = false
			m.status = "Chat request failed"
			m.chat = m.chat.addError(msg.err.Error())
			return m, nil
		}
		if msg.part != "" {
			m.chat = m.chat.appendAIChunk(msg.part)
			m.status = "Streaming response…"
		}
		if msg.done {
			m.busy = false
			m.status = "Ready"
			return m, nil
		}
		return m, waitForAIStreamCmd(msg.stream)

	case workflowDoneMsg:
		// Handled by workflow overlay above; this catches any that arrive late.
		m.busy = false
		if msg.err != nil {
			m.status = "Workflow failed: " + msg.err.Error()
		} else {
			m.status = msg.summary
		}
		return m, nil

	case usageLoadedMsg:
		m.usage = m.usage.receiveWithConfig(msg.totals, msg.cfg, msg.err)
		if msg.err != nil {
			m.status = "Usage load failed"
		} else {
			m.status = "Usage refreshed"
		}
		return m, nil

	case knowledgeLoadedMsg:
		m.knowledge = m.knowledge.receive(msg.sections, msg.components, msg.err)
		if msg.err != nil {
			m.status = "Knowledge load failed"
		} else {
			m.status = "Knowledge refreshed"
		}
		return m, nil

	case quizDashboardLoadedMsg:
		var status string
		m.quizDashboard, status = m.quizDashboard.receive(msg.snapshot, msg.err)
		if status != "" {
			m.status = status
		}
		return m, nil

	case trackedSyncDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "Tracked sync failed: " + msg.err.Error()
			return m, nil
		}
		m.status = "Tracked sync complete: imported " + strconv.Itoa(msg.report.ImportedSessions) + ", backfilled " + strconv.Itoa(msg.report.BackfilledSessions) + ", pending " + strconv.Itoa(msg.report.PendingQuizzes)
		if msg.report.UnmappedAnswers > 0 {
			m.status += ", unmapped answers " + strconv.Itoa(msg.report.UnmappedAnswers)
		}
		if m.activeTab == tabQuizDashboard {
			m.quizDashboard = m.quizDashboard.startLoading()
			return m, loadQuizDashboardCmd()
		}
		return m, nil
	}

	// Global key bindings.
	if km, ok := msg.(tea.KeyMsg); ok {
		s := km.String()
		if (s == "left" || s == "right") && m.activeTab == tabSettings && m.settings.shouldConsumeHorizontalArrows() {
			return m.routeToActiveTab(msg)
		}
		switch s {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+p":
			m.palette = m.palette.Open()
			return m, nil
		case "q":
			if !m.isEditing(true) {
				return m, tea.Quit
			}
		case "tab", "right":
			if !m.isEditing(false) {
				m.activeTab = (m.activeTab + 1) % tabCount
				if m.activeTab == tabSettings {
					m.settings = m.settings.onTabEnter()
				}
				if m.activeTab == tabKnowledge {
					m.knowledge = m.knowledge.startLoading()
					m.status = "Loading knowledge..."
					return m, loadKnowledgeCmd()
				}
				if m.activeTab == tabQuizDashboard {
					m.quizDashboard = m.quizDashboard.startLoading()
					m.status = "Loading quiz dashboard..."
					return m, loadQuizDashboardCmd()
				}
				if m.activeTab == tabUsage && !m.usage.loaded && !m.usage.loading {
					m.usage = m.usage.startLoading()
					m.status = "Loading usage..."
					return m, loadUsageCmd(m.cfg)
				}
				m.status = "Switched tab"
				return m, nil
			}
		case "shift+tab", "left":
			if !m.isEditing(false) {
				m.activeTab = (m.activeTab + tabCount - 1) % tabCount
				if m.activeTab == tabSettings {
					m.settings = m.settings.onTabEnter()
				}
				if m.activeTab == tabKnowledge {
					m.knowledge = m.knowledge.startLoading()
					m.status = "Loading knowledge..."
					return m, loadKnowledgeCmd()
				}
				if m.activeTab == tabQuizDashboard {
					m.quizDashboard = m.quizDashboard.startLoading()
					m.status = "Loading quiz dashboard..."
					return m, loadQuizDashboardCmd()
				}
				if m.activeTab == tabUsage && !m.usage.loaded && !m.usage.loading {
					m.usage = m.usage.startLoading()
					m.status = "Loading usage..."
					return m, loadUsageCmd(m.cfg)
				}
				m.status = "Switched tab"
				return m, nil
			}
		case "esc":
			m = m.resetFocus()
			return m, nil
		}
	}

	return m.routeToActiveTab(msg)
}

// routeToActiveTab passes the message to the currently focused tab.
func (m model) routeToActiveTab(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.activeTab {
	case tabChat:
		var prompt string
		var cmd tea.Cmd
		m.chat, prompt, cmd = m.chat.updateInput(msg, m.busy)
		if prompt != "" {
			m.busy = true
			m.status = "Contacting model…"
			return m, askAICmd(m.orc, m.cfg, m.classes.SelectedClass(), prompt)
		}
		return m, cmd

	case tabClasses:
		var status string
		var cmd tea.Cmd
		m.classes, status, cmd = m.classes.update(msg)
		if status != "" {
			m.status = status
		}
		return m, cmd

	case tabKnowledge:
		var cmd tea.Cmd
		m.knowledge, cmd = m.knowledge.update(msg)
		if cmd != nil {
			m.status = "Loading knowledge..."
		}
		return m, cmd

	case tabSettings:
		var newOrc *orchestrator.Orchestrator
		var status string
		var cmd tea.Cmd
		m.settings, newOrc, status, cmd = m.settings.update(msg, m.cfg)
		if newOrc != nil {
			m.orc = newOrc
			m.savedCfg = cloneConfig(m.cfg)
		}
		if status != "" {
			m.status = status
		}
		return m, cmd

	case tabQuizDashboard:
		var status string
		var cmd tea.Cmd
		var busy bool
		m.quizDashboard, status, cmd, busy = m.quizDashboard.update(msg)
		if status != "" {
			m.status = status
		}
		m.busy = busy
		return m, cmd

	case tabUsage:
		var cmd tea.Cmd
		m.usage, cmd = m.usage.update(msg)
		if cmd != nil {
			m.status = "Refreshing usage..."
		}
		return m, cmd
	}
	return m, nil
}

// handlePaletteAction responds to a command palette selection.
func (m model) handlePaletteAction(action string) (model, tea.Cmd) {
	class := m.classes.SelectedClass()
	switch action {
	case "ingest":
		m.workflow = m.workflow.Open(WorkflowIngest, class)
	case "generate":
		m.workflow = m.workflow.Open(WorkflowGenerate, class)
	case "export-knowledge":
		m.workflow = m.workflow.Open(WorkflowExport, class)
	case "new-class":
		m.activeTab = tabClasses
		m.classes = m.classes.EnterNewClassMode()
		m.status = "Enter new class name, then press Enter"
	case "add-context":
		if class == "" {
			m.status = "Select a class first"
		} else {
			m.activeTab = tabClasses
			m.classes = m.classes.EnterAddContextMode()
			m.status = "Enter context file path, then press Enter"
		}
	case "quiz-dashboard":
		m.activeTab = tabQuizDashboard
		m.quizDashboard = m.quizDashboard.startLoading()
		m.status = "Loading quiz dashboard..."
		return m, loadQuizDashboardCmd()
	case "sync-tracked":
		m.busy = true
		m.status = "Syncing tracked quiz sessions..."
		return m, syncTrackedSessionsCmd()
	case "usage":
		m.activeTab = tabUsage
		m.usage = m.usage.startLoading()
		m.status = "Loading usage..."
		return m, loadUsageCmd(m.cfg)
	case "settings":
		m.activeTab = tabSettings
		m.settings = m.settings.onTabEnter()
		m.status = "Settings"
	case "provider-openai":
		m.cfg.Provider = "openai"
		m.orc = orchestrator.NewFallback(m.cfg)
		m.status = "Provider set to OpenAI for this session. Press s in Settings to save."
	case "provider-claude":
		m.cfg.Provider = "claude"
		m.orc = orchestrator.NewFallback(m.cfg)
		m.status = "Provider set to Claude for this session. Press s in Settings to save."
	case "provider-local":
		m.cfg.Provider = "local"
		m.orc = orchestrator.NewFallback(m.cfg)
		m.status = "Provider set to Local/Ollama for this session. Press s in Settings to save."
	}
	return m, nil
}

// isEditing returns true when the user is actively filling in a form field,
// used to suppress global shortcuts like q and tab.
func (m model) isEditing(checkChatTyping bool) bool {
	switch m.activeTab {
	case tabChat:
		return m.chat.input.Focused() && checkChatTyping
	case tabClasses:
		return m.classes.mode != ""
	case tabSettings:
		return m.settings.mode != ""
	}
	return false
}

// resetFocus exits any active edit modes and blurs all inputs.
func (m model) resetFocus() model {
	m.classes.mode = ""
	m.classes.classInput.Blur()
	m.classes.contextInput.Blur()
	m.settings.mode = ""
	m.settings.input.Blur()
	m.status = "Editing cancelled"
	return m
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading StudyForge UI..."
	}
	if m.width < 60 || m.height < 18 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			warnBannerStyle.Render("Terminal is too small. Resize to at least 60x18."))
	}

	availableDocWidth := max(52, m.width-8)
	labels, showPaletteHint := adaptiveTabLabels(availableDocWidth-headerBarStyle.GetHorizontalFrameSize(), m.activeTab)
	tabParts := renderTabParts(labels, m.activeTab)
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Bottom, tabParts...)
	headerContent := tabsRow
	if showPaletteHint {
		headerContent = lipgloss.JoinHorizontal(lipgloss.Bottom, tabsRow, dimStyle.Render("  Ctrl+P actions"))
	}

	docWidth := clamp(m.width-8, 52, 116)
	headerNeededWidth := lipgloss.Width(headerContent) + headerBarStyle.GetHorizontalFrameSize()
	if headerNeededWidth > docWidth {
		docWidth = min(availableDocWidth, headerNeededWidth)
	}

	bodyInnerWidth := clamp(docWidth-bodyPanelStyle.GetHorizontalFrameSize(), 24, docWidth)
	footerWidth := clamp(docWidth-footerBarStyle.GetHorizontalFrameSize(), 20, docWidth)
	headerWidth := clamp(docWidth-headerBarStyle.GetHorizontalFrameSize(), 20, docWidth)
	header := headerBarStyle.Width(headerWidth).Render(headerContent)

	// Active tab body
	// Total vertical chrome: appStyle padding=2, header=lipgloss.Height(header),
	// bodyPanelStyle borders+padding=4, footer=4  → subtract 10 beyond header height.
	var body string
	bodyHeight := clamp(m.height-lipgloss.Height(header)-10, 4, m.height-14)
	switch m.activeTab {
	case tabChat:
		body = m.chat.view(bodyInnerWidth, bodyHeight, m.orc.Provider.Name(), m.orc.Provider.Disabled(), m.classes.SelectedClass(), m.busy)
	case tabClasses:
		body = m.classes.view(bodyInnerWidth, bodyHeight)
	case tabKnowledge:
		body = m.knowledge.view(bodyInnerWidth, bodyHeight, m.classes.SelectedClass())
	case tabSettings:
		body = m.settings.view(bodyInnerWidth, bodyHeight, m.cfg, m.savedCfg)
	case tabQuizDashboard:
		body = m.quizDashboard.view(bodyInnerWidth, bodyHeight, m.classes.SelectedClass())
	case tabUsage:
		body = m.usage.view(bodyInnerWidth, bodyHeight, m.cfg)
	}
	body = lipgloss.NewStyle().Width(bodyInnerWidth).Height(bodyHeight).MaxHeight(bodyHeight).MaxWidth(bodyInnerWidth).Render(body)
	body = bodyPanelStyle.Render(body)

	footerStatusRaw := "Status: " + m.status
	footerHints := dimStyle.Render("Tab/Shift+Tab switch  •  Ctrl+P actions  •  Esc cancel  •  q quit")
	footerTone := infoBannerStyle
	statusText := strings.ToLower(m.status)
	switch {
	case strings.Contains(statusText, "fail"), strings.Contains(statusText, "error"), strings.Contains(statusText, "cannot"):
		footerTone = errorBannerStyle
	case strings.Contains(statusText, "not saved"), strings.Contains(statusText, "cancel"), strings.Contains(statusText, "select"), strings.Contains(statusText, "required"):
		footerTone = warnBannerStyle
	}
	footer := footerBarStyle.Width(footerWidth).Render(
		footerTone.Render(truncateWidth(footerStatusRaw, footerWidth-2)) + "\n" +
			lipgloss.NewStyle().Width(footerWidth-2).MaxWidth(footerWidth-2).Render(footerHints),
	)

	mainView := appStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))

	// Overlays are rendered centered on top of the main view.
	if m.palette.visible {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.palette.View(m.width, m.height))
	}
	if m.workflow.visible {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.workflow.View(m.width, m.height))
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, mainView)
}

func renderTabParts(labels []string, activeTab int) []string {
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		if i == activeTab {
			parts = append(parts, activeTabStyle.Render(label))
			continue
		}
		parts = append(parts, inactiveTabStyle.Render(label))
	}
	return parts
}

func adaptiveTabLabels(availableWidth, activeTab int) ([]string, bool) {
	labelSets := [][]string{
		{"Chat", "Knowledge", "Quiz Dashboard", "Classes", "Usage", "Settings"},
		{"Chat", "Knowledge", "Quiz Dash", "Classes", "Usage", "Settings"},
		{"Chat", "Know", "Quiz", "Class", "Usage", "Settings"},
		{"Chat", "Know", "Quiz", "Class", "Use", "Set"},
	}
	hint := dimStyle.Render("  Ctrl+P actions")

	for _, labels := range labelSets {
		parts := renderTabParts(labels, activeTab)
		tabsWidth := lipgloss.Width(lipgloss.JoinHorizontal(lipgloss.Bottom, parts...))
		if tabsWidth+lipgloss.Width(hint) <= availableWidth {
			return labels, true
		}
		if tabsWidth <= availableWidth {
			return labels, false
		}
	}

	labels := labelSets[len(labelSets)-1]
	return labels, false
}

func appBodyDimensions(width, height, activeTab int) (int, int) {
	if width <= 0 || height <= 0 {
		return 0, 0
	}

	availableDocWidth := max(52, width-8)
	labels, showPaletteHint := adaptiveTabLabels(availableDocWidth-headerBarStyle.GetHorizontalFrameSize(), activeTab)
	tabParts := renderTabParts(labels, activeTab)
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Bottom, tabParts...)
	headerContent := tabsRow
	if showPaletteHint {
		headerContent = lipgloss.JoinHorizontal(lipgloss.Bottom, tabsRow, dimStyle.Render("  Ctrl+P actions"))
	}

	docWidth := clamp(width-8, 52, 116)
	headerNeededWidth := lipgloss.Width(headerContent) + headerBarStyle.GetHorizontalFrameSize()
	if headerNeededWidth > docWidth {
		docWidth = min(availableDocWidth, headerNeededWidth)
	}

	bodyInnerWidth := clamp(docWidth-bodyPanelStyle.GetHorizontalFrameSize(), 24, docWidth)
	headerWidth := clamp(docWidth-headerBarStyle.GetHorizontalFrameSize(), 20, docWidth)
	header := headerBarStyle.Width(headerWidth).Render(headerContent)
	bodyHeight := clamp(height-lipgloss.Height(header)-10, 4, height-14)
	return bodyInnerWidth, bodyHeight
}
