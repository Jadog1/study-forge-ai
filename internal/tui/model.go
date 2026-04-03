// Package tui provides the interactive terminal UI for study-agent.
// The UI is split into modular component files, one per tab/overlay.
package tui

import (
	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
)

const (
	tabChat = iota
	tabKnowledge
	tabQuizDashboard
	tabClasses
	tabUsage
	tabSettings
	tabCount
)

// model is the root Bubble Tea model. It owns all application state and
// delegates rendering and message handling to per-tab and overlay components.
type model struct {
	cfg      *config.Config
	orc      *orchestrator.Orchestrator
	savedCfg *config.Config

	width  int
	height int
	status string
	busy   bool

	activeTab int

	// Tab components — one per tab.
	chat          ChatTab
	classes       ClassesTab
	knowledge     KnowledgeTab
	settings      SettingsTab
	quizDashboard QuizDashboardTab
	usage         UsageTab

	// Feedback components — spinner, toast.
	spin  SpinnerModel
	toast ToastModel

	// Overlay components — rendered on top when visible.
	palette  PaletteModel
	workflow WorkflowModel
	editor   EditorModel
}

func newModel(cfg *config.Config, orc *orchestrator.Orchestrator) model {
	classes, _ := classpkg.List()
	return model{
		cfg:           cfg,
		orc:           orc,
		savedCfg:      cloneConfig(cfg),
		activeTab:     tabChat,
		status:        "Ready",
		chat:          newChatTab(),
		classes:       newClassesTab(classes),
		knowledge:     newKnowledgeTab(),
		settings:      newSettingsTab(),
		quizDashboard: newQuizDashboardTab(),
		usage:         newUsageTab(),
		spin:          newSpinner(),
		palette:       newPalette(),
		workflow:      newWorkflow(),
		editor:        newEditor(),
	}
}

func (m model) resize(width, height int) model {
	m.width = width
	m.height = height

	innerWidth, bodyHeight := appBodyDimensions(width, height, m.activeTab)
	if innerWidth == 0 {
		innerWidth = clamp(width-10, 20, width)
	}
	if bodyHeight == 0 {
		bodyHeight = clamp(height-14, 4, height)
	}

	m.chat = m.chat.resize(innerWidth)
	m.classes = m.classes.resize(innerWidth, bodyHeight)
	m.knowledge = m.knowledge.resize(innerWidth, bodyHeight)
	m.settings = m.settings.resize(innerWidth)
	m.quizDashboard = m.quizDashboard.resize(innerWidth, bodyHeight)
	m.usage = m.usage.resize(innerWidth)
	m.palette = m.palette.resize(clamp(innerWidth, 36, 76))
	m.workflow = m.workflow.resize(clamp(innerWidth, 28, 72), height)
	m.editor = m.editor.resize(innerWidth, bodyHeight)
	return m
}
