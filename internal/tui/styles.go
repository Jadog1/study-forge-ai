package tui

import "github.com/charmbracelet/lipgloss"

var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Border(lipgloss.RoundedBorder(), true, true, false, true).
			BorderForeground(lipgloss.Color("117")).
			Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Background(lipgloss.Color("236")).
			Border(lipgloss.RoundedBorder(), true, true, false, true).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
	statusBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	successStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	dimStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	selectedStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
	warnStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
	labelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))

	appStyle = lipgloss.NewStyle().Padding(1, 2)

	headerBarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	bodyPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	footerBarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)

	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("223"))

	// overlayStyle is used for the command palette.
	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("117")).
			Padding(1, 2)

	// workflowStyle is used for in-app ingest/generate/adapt panels.
	workflowStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("78")).
			Padding(1, 2)

	// sectionStyle draws bordered cards for embedded sections.
	sectionStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)

	userBubbleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")).
			Padding(0, 1)

	assistantBubbleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	systemBubbleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Padding(0, 1)

	inputPanelStyle = lipgloss.NewStyle().
			Padding(0, 1)

	toolRunningStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("221")).
			Padding(0, 1)

	toolDoneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("78")).
			Padding(0, 1)

	toolErrorStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("160")).
			Padding(0, 1)

	errorBannerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("160")).Padding(0, 1)
	warnBannerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("221")).Padding(0, 1)
	infoBannerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1)

	paletteSelectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
)
