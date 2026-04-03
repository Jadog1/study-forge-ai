package tui

import "github.com/charmbracelet/lipgloss"

var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTextBright).
			Background(colorPrimary).
			Border(lipgloss.RoundedBorder(), true, true, false, true).
			BorderForeground(colorSecondary).
			Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorSurface).
			Border(lipgloss.RoundedBorder(), true, true, false, true).
			BorderForeground(colorBorder).
			Padding(0, 1)
	statusBarStyle   = lipgloss.NewStyle().Foreground(colorText)
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(colorTextBright)
	errorStyle       = lipgloss.NewStyle().Foreground(colorError)
	successStyle     = lipgloss.NewStyle().Foreground(colorSuccess)
	dimStyle         = lipgloss.NewStyle().Foreground(colorMuted)
	selectedStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	warnStyle        = lipgloss.NewStyle().Foreground(colorWarning)
	labelStyle       = lipgloss.NewStyle().Foreground(colorSecondary)

	appStyle = lipgloss.NewStyle().Padding(0, 1)

	headerBarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	bodyPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	footerBarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDim).
			Padding(0, 1)

	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSecondary).
			Padding(1, 2)

	workflowStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSuccess).
			Padding(1, 2)

	sectionStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	userBubbleStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Padding(0, 1)

	assistantBubbleStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Padding(0, 1)

	systemBubbleStyle = lipgloss.NewStyle().
				Foreground(colorError).
				Padding(0, 1)

	inputPanelStyle = lipgloss.NewStyle().
			Padding(0, 2)

	toolRunningStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorWarning).
				Padding(0, 1)

	toolDoneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSuccess).
			Padding(0, 1)

	toolErrorStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorError).
			Padding(0, 1)

	errorBannerStyle = lipgloss.NewStyle().Foreground(colorTextBright).Background(colorError).Padding(0, 1)
	warnBannerStyle  = lipgloss.NewStyle().Foreground(colorSurface).Background(colorWarning).Padding(0, 1)
	infoBannerStyle  = lipgloss.NewStyle().Foreground(colorTextBright).Background(colorPrimary).Padding(0, 1)

	paletteSelectedRowStyle = lipgloss.NewStyle().Foreground(colorTextBright).Background(colorPrimary)
)
