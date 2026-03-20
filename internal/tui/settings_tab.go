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

// SettingsTab holds state for the Settings tab.
type SettingsTab struct {
	index int
	mode  string // "", "edit-setting"
	input textinput.Model
}

func newSettingsTab() SettingsTab {
	input := textinput.New()
	input.CharLimit = 2000
	return SettingsTab{input: input}
}

func (s SettingsTab) resize(width int) SettingsTab {
	s.input.Width = clamp(width-6, 18, width)
	return s
}

var settingKeyList = []string{
	"provider",
	"embeddings.provider",
	"embeddings.model",
	"openai.api_key",
	"openai.model",
	"claude.api_key",
	"claude.model",
	"voyage.api_key",
	"voyage.model",
	"local.endpoint",
	"local.embeddings_endpoint",
	"local.model",
	"sfq.command",
	"custom_prompt_context",
}

func (s SettingsTab) currentKey() string {
	if s.index < 0 || s.index >= len(settingKeyList) {
		return settingKeyList[0]
	}
	return settingKeyList[s.index]
}

func settingValue(key string, cfg *config.Config) string {
	switch key {
	case "provider":
		return cfg.Provider
	case "embeddings.provider":
		return cfg.Embeddings.Provider
	case "embeddings.model":
		return cfg.Embeddings.Model
	case "openai.api_key":
		return cfg.OpenAI.APIKey
	case "openai.model":
		return cfg.OpenAI.Model
	case "claude.api_key":
		return cfg.Claude.APIKey
	case "claude.model":
		return cfg.Claude.Model
	case "voyage.api_key":
		return cfg.Voyage.APIKey
	case "voyage.model":
		return cfg.Voyage.Model
	case "local.endpoint":
		return cfg.Local.Endpoint
	case "local.embeddings_endpoint":
		return cfg.Local.EmbeddingsEndpoint
	case "local.model":
		return cfg.Local.Model
	case "sfq.command":
		return cfg.SFQ.Command
	case "custom_prompt_context":
		return cfg.CustomPromptContext
	default:
		return ""
	}
}

func applySetting(key, value string, cfg *config.Config) {
	switch key {
	case "provider":
		cfg.Provider = value
	case "embeddings.provider":
		cfg.Embeddings.Provider = value
	case "embeddings.model":
		cfg.Embeddings.Model = value
	case "openai.api_key":
		cfg.OpenAI.APIKey = value
	case "openai.model":
		cfg.OpenAI.Model = value
	case "claude.api_key":
		cfg.Claude.APIKey = value
	case "claude.model":
		cfg.Claude.Model = value
	case "voyage.api_key":
		cfg.Voyage.APIKey = value
	case "voyage.model":
		cfg.Voyage.Model = value
	case "local.endpoint":
		cfg.Local.Endpoint = value
	case "local.embeddings_endpoint":
		cfg.Local.EmbeddingsEndpoint = value
	case "local.model":
		cfg.Local.Model = value
	case "sfq.command":
		cfg.SFQ.Command = value
	case "custom_prompt_context":
		cfg.CustomPromptContext = value
	}
}

// isKeyConfigured returns true when the setting has a meaningful value set.
func isKeyConfigured(key, val string) bool {
	switch key {
	case "openai.api_key":
		return val != ""
	case "claude.api_key":
		return val != ""
	case "voyage.api_key":
		return val != ""
	}
	return true
}

// redactAPIKey partially hides API key values for display.
func redactAPIKey(key, val string) string {
	if (key == "openai.api_key" || key == "claude.api_key" || key == "voyage.api_key") && len(val) > 10 {
		return val[:7] + strings.Repeat("*", len(val)-7)
	}
	return val
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

// renderSettingsWindow returns only the visible slice of setting rows plus
// top/bottom overflow affordances so users know the list can scroll.
func renderSettingsWindow(lines []string, selected, rows int) string {
	if len(lines) == 0 {
		return ""
	}
	start, end := visibleWindow(len(lines), selected, rows)
	window := append([]string{}, lines[start:end]...)
	if start > 0 {
		window[0] = dimStyle.Render("↑ more")
	}
	if end < len(lines) {
		window[len(window)-1] = dimStyle.Render("↓ more")
	}
	return strings.Join(window, "\n")
}

// update handles all messages for the Settings tab.
// Returns: updated tab, new *Orchestrator when settings are saved (else nil),
// status string, and any tea.Cmd.
// isAPIKeyField reports whether the setting key is an API key field that is
// managed exclusively via environment variables and must not be edited.
func isAPIKeyField(key string) bool {
	return key == "openai.api_key" || key == "claude.api_key" || key == "voyage.api_key"
}

func (s SettingsTab) update(msg tea.Msg, cfg *config.Config) (SettingsTab, *orchestrator.Orchestrator, string, tea.Cmd) {
	if s.mode == "edit-setting" {
		if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" {
			applySetting(s.currentKey(), strings.TrimSpace(s.input.Value()), cfg)
			s.mode = ""
			s.input.Blur()
			return s, nil, "Setting updated in memory. Press s to save it to config.yaml.", nil
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, nil, "", cmd
	}

	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up":
			if s.index > 0 {
				s.index--
			}
		case "down":
			if s.index < len(settingKeyList)-1 {
				s.index++
			}
		case "pgup":
			s.index = clamp(s.index-5, 0, len(settingKeyList)-1)
		case "pgdown":
			s.index = clamp(s.index+5, 0, len(settingKeyList)-1)
		case "e":
			if isAPIKeyField(s.currentKey()) {
				envVar := config.EnvOpenAIKey
				if s.currentKey() == "claude.api_key" {
					envVar = config.EnvClaudeKey
				} else if s.currentKey() == "voyage.api_key" {
					envVar = config.EnvVoyageKey
				}
				return s, nil, fmt.Sprintf("API keys are read-only here — set %s in your environment.", envVar), nil
			}
			s.input.SetValue(settingValue(s.currentKey(), cfg))
			s.input.Focus()
			s.mode = "edit-setting"
			return s, nil, "Editing: " + s.currentKey(), nil
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

func (s SettingsTab) view(width, height int, cfg, savedCfg *config.Config) string {
	var lines []string
	unsaved := !configsEqual(cfg, savedCfg)
	lineValueWidth := clamp(width-36, 12, 48)
	for i, k := range settingKeyList {
		prefix := "  "
		if i == s.index {
			prefix = selectedStyle.Render("▸ ")
		}
		val := settingValue(k, cfg)
		displayVal := compactSettingValue(redactAPIKey(k, val))
		dirty := ""
		if savedCfg != nil && settingValue(k, savedCfg) != val {
			dirty = " " + warnStyle.Render("unsaved")
		}

		indicator := ""
		switch {
		case k == "openai.api_key" || k == "claude.api_key" || k == "voyage.api_key":
			envVar := config.EnvOpenAIKey
			if k == "claude.api_key" {
				envVar = config.EnvClaudeKey
			} else if k == "voyage.api_key" {
				envVar = config.EnvVoyageKey
			}
			if isKeyConfigured(k, val) {
				indicator = " " + successStyle.Render("✓") + dimStyle.Render(" (env: "+envVar+")")
			} else {
				indicator = " " + warnStyle.Render("⚠ set "+envVar+" env var")
			}
		case k == "provider":
			indicator = " " + dimStyle.Render("(active)")
		}

		lines = append(lines, fmt.Sprintf("%s%s: %s%s%s",
			prefix, labelStyle.Render(k), truncateWidth(displayVal, lineValueWidth), indicator, dirty))
	}

	bannerText := successStyle.Render("Saved to ~/.study-forge-ai/config.yaml")
	if unsaved {
		bannerText = warnBannerStyle.Render("Unsaved changes: runtime config differs from ~/.study-forge-ai/config.yaml")
	}

	currentKey := s.currentKey()
	detailValueWidth := clamp(width-26, 12, 72)
	currentValue := truncateWidth(compactSettingValue(redactAPIKey(currentKey, settingValue(currentKey, cfg))), detailValueWidth)
	savedValue := ""
	if savedCfg != nil {
		savedValue = truncateWidth(compactSettingValue(redactAPIKey(currentKey, settingValue(currentKey, savedCfg))), detailValueWidth)
	}
	detailBody := fmt.Sprintf("Current key: %s\nCurrent value: %s", labelStyle.Render(currentKey), currentValue)
	if savedCfg != nil {
		detailBody += "\nSaved value: " + savedValue
	}

	listRows := clamp(height/3, 4, 8)
	detailRows := clamp(height/4, 3, 5)
	actionRows := 2
	editRows := 1
	if s.mode == "edit-setting" {
		editRows = 2
	}

	settingsSection := ""
	selectedSection := ""
	actionsSection := ""
	editSection := ""

	for i := 0; i < 6; i++ {
		listBody := renderSettingsWindow(lines, s.index, listRows)
		scrollHint := dimStyle.Render(fmt.Sprintf("%d/%d", s.index+1, len(settingKeyList)))
		settingsSection = renderSection("Settings", listBody+"\n"+scrollHint, width)

		selectedSection = renderSection("Selected setting", clipLines(detailBody, detailRows), width)

		hint := dimStyle.Render("↑/↓ select  •  PgUp/PgDn jump  •  e edit  •  s save  •  esc cancel\nAPI keys (read-only): set " + config.EnvOpenAIKey + " / " + config.EnvClaudeKey + " / " + config.EnvVoyageKey + " in your environment")
		actionsSection = renderSection("Actions", clipLines(hint, actionRows), width)

		if s.mode == "edit-setting" {
			editSection = renderSection("Edit", clipLines(s.input.View()+"\n"+dimStyle.Render("Press Enter to stage the new value in memory."), editRows), width)
		} else {
			editSection = renderSection("Edit", clipLines(dimStyle.Render("Press e to edit the highlighted setting."), editRows), width)
		}

		content := lipgloss.JoinVertical(lipgloss.Left, bannerText, settingsSection, selectedSection, actionsSection, editSection)
		if lipgloss.Height(content) <= height {
			break
		}

		if listRows > 4 {
			listRows--
			continue
		}
		if detailRows > 2 {
			detailRows--
			continue
		}
		if actionRows > 1 {
			actionRows--
			continue
		}
		if editRows > 1 {
			editRows--
			continue
		}
		break
	}
	view := lipgloss.JoinVertical(lipgloss.Left,
		bannerText,
		settingsSection,
		selectedSection,
		actionsSection,
		editSection,
	)
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Height(height).MaxHeight(height).Render(view)
}
