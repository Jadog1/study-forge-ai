package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// PaletteAction represents one entry in the command palette.
type PaletteAction struct {
	Label  string // display label
	Desc   string // short description shown to the right
	Action string // identifier returned to the parent on selection
}

// PaletteModel is the Ctrl+P command palette overlay.
type PaletteModel struct {
	input    textinput.Model
	all      []PaletteAction
	filtered []PaletteAction
	index    int
	visible  bool
}

var paletteActions = []PaletteAction{
	{"Ingest Notes", "Process a folder of notes with AI", "ingest"},
	{"Generate Quiz", "Create a quiz from ingested notes", "generate"},
	{"New Class", "Create a new study class", "new-class"},
	{"Add Context File", "Attach a context file to the active class", "add-context"},
	{"Search Notes (SFQ)", "Open SFQ Search tab", "sfq-search"},
	{"Sync Tracked Quiz Sessions", "Import sfq tracked results into knowledge history", "sync-tracked"},
	{"Usage History", "Review historical AI usage and cost", "usage"},
	{"Toggle Auto-SFQ", "Auto-lookup related notes after AI response", "toggle-auto-sfq"},
	{"Settings", "Configure providers and preferences", "settings"},
	{"Use OpenAI", "Switch active provider to OpenAI", "provider-openai"},
	{"Use Claude", "Switch active provider to Anthropic Claude", "provider-claude"},
	{"Use Local (Ollama)", "Switch active provider to local Ollama", "provider-local"},
}

func newPalette() PaletteModel {
	input := textinput.New()
	input.Placeholder = "Type to filter commands…"
	input.CharLimit = 100
	return PaletteModel{
		all:      paletteActions,
		filtered: paletteActions,
		input:    input,
	}
}

func (p PaletteModel) resize(width int) PaletteModel {
	p.input.Width = clamp(width-6, 18, width)
	return p
}

// Open shows the palette, resetting filter and selection.
func (p PaletteModel) Open() PaletteModel {
	p.visible = true
	p.index = 0
	p.input.SetValue("")
	p.input.Focus()
	p.filtered = p.all
	return p
}

// Close hides the palette.
func (p PaletteModel) Close() PaletteModel {
	p.visible = false
	p.input.Blur()
	return p
}

// Update processes input while the palette is visible.
// Returns (updated palette, selected action identifier or "", tea.Cmd).
func (p PaletteModel) Update(msg tea.Msg) (PaletteModel, string, tea.Cmd) {
	if !p.visible {
		return p, "", nil
	}

	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return p.Close(), "", nil
		case "enter":
			if len(p.filtered) > 0 {
				action := p.filtered[p.index].Action
				return p.Close(), action, nil
			}
			return p.Close(), "", nil
		case "up":
			if p.index > 0 {
				p.index--
			}
			return p, "", nil
		case "down":
			if p.index < len(p.filtered)-1 {
				p.index++
			}
			return p, "", nil
		}
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)

	// Re-filter on every keystroke.
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	if q == "" {
		p.filtered = p.all
	} else {
		p.filtered = nil
		for _, a := range p.all {
			if strings.Contains(strings.ToLower(a.Label), q) ||
				strings.Contains(strings.ToLower(a.Desc), q) {
				p.filtered = append(p.filtered, a)
			}
		}
	}
	if p.index >= len(p.filtered) {
		p.index = 0
	}
	return p, "", cmd
}

// View renders the command palette centered overlay string.
func (p PaletteModel) View(width, _ int) string {
	panelWidth := clamp(width-10, 44, 82)
	innerWidth := clamp(panelWidth-overlayStyle.GetHorizontalFrameSize(), 24, panelWidth)
	labelWidth := clamp(innerWidth/2-2, 16, 28)
	descWidth := clamp(innerWidth-labelWidth-4, 14, 42)

	var b strings.Builder
	b.WriteString(headerStyle.Render("Command Palette") + "\n")
	b.WriteString(dimStyle.Render("Filter actions without changing the menu height.") + "\n\n")
	b.WriteString(p.input.View() + "\n\n")

	if len(p.filtered) == 0 {
		b.WriteString(dimStyle.Render("No matching commands") + "\n")
	} else {
		maxRows := min(8, len(p.filtered))
		start := 0
		if p.index >= maxRows {
			start = p.index - maxRows + 1
		}
		for i := start; i < start+maxRows; i++ {
			a := p.filtered[i]
			cursor := "  "
			label := padRightWidth(a.Label, labelWidth)
			desc := truncateWidth(a.Desc, descWidth)
			row := cursor + label + "  " + desc
			if i == p.index {
				row = paletteSelectedRowStyle.Render("▸ " + label + "  " + desc)
			} else {
				row = dimStyle.Render(row)
			}
			b.WriteString(row + "\n")
		}
		if len(p.filtered) > maxRows {
			b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Showing %d of %d matches", maxRows, len(p.filtered))))
		}
	}
	b.WriteString("\n\n" + dimStyle.Render("↑/↓ navigate  •  Enter select  •  Esc close"))

	return overlayStyle.Width(innerWidth).Render(b.String())
}
