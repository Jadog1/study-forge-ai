package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
)

// ── SettingsTab ───────────────────────────────────────────────────────────

// SettingsTab holds state for the Settings tab.
type SettingsTab struct {
	index int    // selected row in itemDefs
	mode  string // "" | "edit"
	nav   bool   // true when settings list navigation is explicitly engaged
	input textinput.Model
}

func newSettingsTab() SettingsTab {
	input := textinput.New()
	input.CharLimit = 2000
	return SettingsTab{input: input, nav: false}
}

func (s SettingsTab) resize(width int) SettingsTab {
	s.input.Width = clamp(width-6, 18, width)
	return s
}

// onTabEnter puts Settings into a passive mode so global arrows continue to
// switch tabs until the user intentionally engages settings navigation.
func (s SettingsTab) onTabEnter() SettingsTab {
	s.nav = false
	s.mode = ""
	s.input.Blur()
	return s
}

// shouldConsumeHorizontalArrows reports whether left/right arrows should be
// handled by the settings list instead of global tab navigation.
func (s SettingsTab) shouldConsumeHorizontalArrows() bool {
	if !s.nav {
		return false
	}
	if s.mode != "" {
		return true
	}
	if s.index < 0 || s.index >= len(itemDefs) {
		return false
	}
	return itemDefs[s.index].kind == skCycle
}

// update handles all messages for the Settings tab.
// Returns: updated tab, new *Orchestrator when settings are saved (else nil),
// status string, and any tea.Cmd.
func (s SettingsTab) update(msg tea.Msg, cfg *config.Config) (SettingsTab, *orchestrator.Orchestrator, string, tea.Cmd) {
	// Passive mode: wait for an intentional action before capturing arrows.
	if !s.nav {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "down", "j":
				s.nav = true
				s.index = 0
				return s, nil, "Settings focused", nil
			case "up", "k":
				s.nav = true
				s.index = len(itemDefs) - 1
				return s, nil, "Settings focused", nil
			case "enter":
				s.nav = true
				s.index = clamp(s.index, 0, len(itemDefs)-1)
				return s, nil, "Settings focused", nil
			case "e", "pgup", "pgdown":
				s.nav = true
				s.index = clamp(s.index, 0, len(itemDefs)-1)
			default:
				return s, nil, "", nil
			}
		}
	}

	// ── Text-edit mode ──────────────────────────────────────────────────────
	if s.mode == "edit" {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "enter":
				itemSet(s.index, strings.TrimSpace(s.input.Value()), cfg)
				s.mode = ""
				s.input.Blur()
				return s, nil, "Value staged. Press s to save to config.yaml.", nil
			case "esc":
				s.mode = ""
				s.input.Blur()
				return s, nil, "", nil
			}
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, nil, "", cmd
	}

	// ── Normal navigation ───────────────────────────────────────────────────
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			if s.index > 0 {
				s.index--
			}
		case "down", "j":
			if s.index < len(itemDefs)-1 {
				s.index++
			}
		case "pgup":
			s.index = clamp(s.index-5, 0, len(itemDefs)-1)
		case "pgdown":
			s.index = clamp(s.index+5, 0, len(itemDefs)-1)

		case "left":
			choices := itemChoices(s.index, cfg)
			if len(choices) > 0 {
				itemSet(s.index, cycleBackward(choices, itemGet(s.index, cfg)), cfg)
			}
		case "right":
			choices := itemChoices(s.index, cfg)
			if len(choices) > 0 {
				itemSet(s.index, cycleForward(choices, itemGet(s.index, cfg)), cfg)
			}

		case "e":
			s.input.SetValue(itemGet(s.index, cfg))
			s.input.Focus()
			s.mode = "edit"
			return s, nil, "Editing: " + itemDefs[s.index].label, nil

		case "r":
			if itemDefs[s.index].isRole {
				resetRoleItem(s.index, cfg)
				return s, nil, itemDefs[s.index].label + " reset to auto", nil
			}

		case "R":
			resetAllRoles(cfg)
			return s, nil, "All role overrides reset to auto", nil

		case "s":
			if err := config.Save(cfg); err != nil {
				return s, nil, "Save failed: " + err.Error(), nil
			}
			orc := orchestrator.NewFallback(cfg)
			return s, orc, "Settings saved (API keys are never written to disk)", nil
		}
	}
	return s, nil, "", nil
}

func (s SettingsTab) helpKeys() []KeyBinding {
	if s.mode == "edit" {
		return []KeyBinding{
			{Key: "Enter", Desc: "confirm"},
			{Key: "Esc", Desc: "cancel"},
		}
	}
	return []KeyBinding{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "←/→", Desc: "cycle"},
		{Key: "e", Desc: "edit"},
		{Key: "s", Desc: "save"},
	}
}

// ── View ──────────────────────────────────────────────────────────────────

// buildSettingsLines renders all settings rows into a flat string slice,
// interleaving section headings. selectedLine receives the line index of the
// currently selected item so callers can implement scroll-to-visible.
func buildSettingsLines(cfg, savedCfg *config.Config, selectedIdx, labelW, valW int) (lines []string, selectedLine int) {
	seenSection := ""
	for i, def := range itemDefs {
		if def.section != "" && def.section != seenSection {
			seenSection = def.section
			if len(lines) > 0 {
				lines = append(lines, "") // blank separator
			}
			lines = append(lines, sectionTitleStyle.Render("── "+def.section+" ──"))
		}
		if i == selectedIdx {
			selectedLine = len(lines)
		}
		lines = append(lines, renderSettingRow(i, def, cfg, savedCfg, i == selectedIdx, labelW, valW))
	}
	return
}

func buildSettingsLinesPassive(cfg, savedCfg *config.Config, labelW, valW int) (lines []string) {
	seenSection := ""
	for i, def := range itemDefs {
		if def.section != "" && def.section != seenSection {
			seenSection = def.section
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, sectionTitleStyle.Render("── "+def.section+" ──"))
		}
		lines = append(lines, renderSettingRow(i, def, cfg, savedCfg, false, labelW, valW))
	}
	return
}

func renderSettingRow(idx int, def settingItemDef, cfg, savedCfg *config.Config, selected bool, labelW, valW int) string {
	cursor := "  "
	if selected {
		cursor = selectedStyle.Render("▸ ")
	}

	label := padRightWidth(def.label, labelW)
	val := itemGet(idx, cfg)

	var valueStr string
	switch def.kind {
	case skCycle:
		choices := itemChoices(idx, cfg)
		display := val
		if display == "" {
			display = dimStyle.Render("(unset)")
		}
		hint := resolvedAutoHint(idx, cfg)
		hintStr := ""
		if hint != "" {
			hintStr = dimStyle.Render("  →" + hint)
		}
		if selected && len(choices) > 0 {
			valueStr = dimStyle.Render("← ") + selectedStyle.Render(truncateWidth(display, valW)) + dimStyle.Render(" →") + hintStr
		} else {
			valueStr = truncateWidth(display, valW) + hintStr
		}
		status := itemStatus(idx, cfg)
		if status != "" {
			valueStr += "  " + status
		}
		if selected && def.isModel {
			valueStr += "  " + dimStyle.Render("e=custom")
		}
		if selected && def.isRole && val != "auto" {
			valueStr += "  " + dimStyle.Render("r=reset")
		}

	case skText:
		display := compactSettingValue(val)
		if display == "" {
			display = dimStyle.Render("(empty)")
		} else {
			display = truncateWidth(display, valW)
		}
		dirty := ""
		if savedCfg != nil && itemGet(idx, savedCfg) != val {
			dirty = " " + warnStyle.Render("unsaved")
		}
		editHint := ""
		if selected {
			editHint = "  " + dimStyle.Render("e=edit")
		}
		valueStr = display + dirty + editHint
	}

	return cursor + labelStyle.Render(label) + " " + valueStr
}

func (s SettingsTab) view(width, height int, cfg, savedCfg *config.Config) string {
	unsaved := !configsEqual(cfg, savedCfg)
	bannerText := successStyle.Render("Saved  " + config.DisplayRootDir() + "/config.yaml")
	if unsaved {
		bannerText = warnBannerStyle.Render("Unsaved changes — press s to save to config.yaml")
	}
	if !s.nav {
		bannerText = infoBannerStyle.Render("Passive mode: arrows switch tabs. Press ↓ to focus settings.")
	}

	labelW := clamp(width/3, 20, 28)
	valW := clamp(width-labelW-16, 14, 44)

	lines := []string{}
	selectedLine := 0
	if s.nav {
		lines, selectedLine = buildSettingsLines(cfg, savedCfg, s.index, labelW, valW)
	} else {
		lines = buildSettingsLinesPassive(cfg, savedCfg, labelW, valW)
	}

	editRows := 0
	if s.mode == "edit" {
		editRows = 4
	}
	// Reserve space for: banner + section chrome + hint bar + edit panel
	availRows := clamp(height-7-editRows, 6, height)
	start, end := visibleWindow(len(lines), selectedLine, availRows)

	window := make([]string, end-start)
	copy(window, lines[start:end])
	if start > 0 {
		window[0] = dimStyle.Render("  ↑ more")
	}
	if end < len(lines) {
		window[len(window)-1] = dimStyle.Render("  ↓ more")
	}

	scrollHint := dimStyle.Render(fmt.Sprintf("  %d/%d", s.index+1, len(itemDefs)))
	if !s.nav {
		scrollHint = dimStyle.Render("  passive")
	}
	body := strings.Join(window, "\n") + "\n" + scrollHint

	var editSection string
	if s.mode == "edit" {
		prompt := dimStyle.Render(itemDefs[s.index].label + ": ")
		editSection = "\n" + renderSection("Edit", prompt+s.input.View()+"\n"+dimStyle.Render("Enter=apply  Esc=cancel"), width)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		bannerText,
		renderSection("Settings", body, width)+editSection,
	)
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Height(height).MaxHeight(height).Render(content)
}

// compactSettingValue normalizes multiline values for stable single-line display.
func compactSettingValue(val string) string {
	val = strings.ReplaceAll(val, "\r\n", "\\n")
	val = strings.ReplaceAll(val, "\n", "\\n")
	return strings.TrimSpace(val)
}

// visibleWindow computes a scrolling window that keeps the selected index in view.
func visibleWindow(total, selected, rows int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	rows = clamp(rows, 1, total)
	start := selected - rows/2
	if start < 0 {
		start = 0
	}
	if start > total-rows {
		start = total - rows
	}
	end := start + rows
	if end > total {
		end = total
	}
	return start, end
}
