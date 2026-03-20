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
	tabClasses
	tabSettings
	tabSFQ
	tabUsage
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
	chat     ChatTab
	classes  ClassesTab
	settings SettingsTab
	sfq      SFQTab
	usage    UsageTab

	// Overlay components — rendered on top when visible.
	palette  PaletteModel
	workflow WorkflowModel
}

func newModel(cfg *config.Config, orc *orchestrator.Orchestrator) model {
	classes, _ := classpkg.List()
	return model{
		cfg:       cfg,
		orc:       orc,
		savedCfg:  cloneConfig(cfg),
		activeTab: tabChat,
		status:    "Ready",
		chat:      newChatTab(),
		classes:   newClassesTab(classes),
		settings:  newSettingsTab(),
		sfq:       newSFQTab(),
		usage:     newUsageTab(),
		palette:   newPalette(),
		workflow:  newWorkflow(),
	}
}

func (m model) resize(width, height int) model {
	m.width = width
	m.height = height

	contentWidth := clamp(width-14, 34, 108)
	innerWidth := clamp(contentWidth-bodyPanelStyle.GetHorizontalFrameSize()-4, 20, 96)

	m.chat = m.chat.resize(innerWidth)
	m.classes = m.classes.resize(innerWidth)
	m.settings = m.settings.resize(innerWidth)
	m.sfq = m.sfq.resize(innerWidth)
	m.usage = m.usage.resize(innerWidth)
	m.palette = m.palette.resize(clamp(innerWidth, 36, 76))
	m.workflow = m.workflow.resize(clamp(innerWidth, 28, 72))
	return m
}
