package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SFQTab holds state for the SFQ Search tab.
type SFQTab struct {
	input  textinput.Model
	output string
}

func newSFQTab() SFQTab {
	input := textinput.New()
	input.Placeholder = "search term"
	input.Focus()
	input.CharLimit = 400
	return SFQTab{input: input}
}

func (s SFQTab) resize(width int) SFQTab {
	s.input.Width = clamp(width-4, 18, width)
	return s
}

// updateInput handles key events in the SFQ tab.
// Returns (updated tab, status string, tea.Cmd).
// busy should be true while a search is running; enter is suppressed.
func (s SFQTab) updateInput(msg tea.Msg, sfqCommand string, busy bool) (SFQTab, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" && !busy {
		q := strings.TrimSpace(s.input.Value())
		if q == "" {
			return s, "", nil
		}
		if strings.TrimSpace(sfqCommand) == "" {
			return s, "Set sfq.command in Settings before searching", nil
		}
		return s, "Running sfq search…", runSFQCmd(sfqCommand, q, false)
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, "", cmd
}

// receiveResult stores search results from the sfq plugin.
func (s SFQTab) receiveResult(text string, err error) (SFQTab, string) {
	if err != nil {
		s.output = errorStyle.Render(err.Error())
		return s, "SFQ search failed"
	}
	s.output = text
	return s, "SFQ search complete"
}

func (s SFQTab) view(width, height int) string {
	out := s.output
	if out == "" {
		out = dimStyle.Render("No results yet.")
	}
	hint := dimStyle.Render("Type a term and press Enter to search via the sfq plugin.")
	resultsBody := lipgloss.NewStyle().Width(width - 6).Render(out)
	resultsBody = clipLines(resultsBody, clamp(height-8, 6, 18))
	return lipgloss.JoinVertical(lipgloss.Left,
		renderSection("Query", s.input.View()+"\n"+hint, width),
		renderSection("Output", resultsBody, width),
	)
}
