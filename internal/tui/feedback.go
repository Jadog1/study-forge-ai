package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Spinner ──────────────────────────────────────────────────────────────────

type SpinnerModel struct {
	spinner spinner.Model
	active  bool
	label   string
}

func newSpinner() SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(colorSecondary)
	return SpinnerModel{spinner: s}
}

func (s SpinnerModel) Start(label string) SpinnerModel {
	s.active = true
	s.label = label
	return s
}

func (s SpinnerModel) Stop() SpinnerModel {
	s.active = false
	s.label = ""
	return s
}

func (s SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	if !s.active {
		return s, nil
	}
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

func (s SpinnerModel) View() string {
	if !s.active {
		return ""
	}
	return s.spinner.View() + " " + s.label
}

// ── Toast ────────────────────────────────────────────────────────────────────

type toastTickMsg struct{ id int }

type ToastModel struct {
	message string
	style   lipgloss.Style
	visible bool
	tickID  int
}

func showToast(message string, style lipgloss.Style) (ToastModel, tea.Cmd) {
	t := ToastModel{
		message: message,
		style:   style,
		visible: true,
		tickID:  int(time.Now().UnixNano() & 0x7FFFFFFF),
	}
	id := t.tickID
	cmd := tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
		return toastTickMsg{id: id}
	})
	return t, cmd
}

func (t ToastModel) Update(msg tea.Msg) ToastModel {
	if tick, ok := msg.(toastTickMsg); ok {
		if tick.id == t.tickID {
			t.visible = false
			t.message = ""
		}
	}
	return t
}

func (t ToastModel) View(width int) string {
	if !t.visible || t.message == "" {
		return ""
	}
	banner := t.style.Width(width).MaxWidth(width).Render(truncateWidth(t.message, width-2))
	return banner
}

// ── Skeleton ─────────────────────────────────────────────────────────────────

func renderSkeleton(width, lines int) string {
	if width <= 0 || lines <= 0 {
		return ""
	}
	fractions := []float64{0.80, 0.60, 0.90, 0.70, 0.50, 0.85, 0.65, 0.75}
	style := dimStyle
	rows := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		frac := fractions[i%len(fractions)]
		lineWidth := int(float64(width) * frac)
		if lineWidth < 1 {
			lineWidth = 1
		}
		rows = append(rows, style.Render(strings.Repeat("▒", lineWidth)))
	}
	return strings.Join(rows, "\n")
}
