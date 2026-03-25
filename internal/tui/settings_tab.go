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

// ── Item catalogue ────────────────────────────────────────────────────────

// siKind distinguishes cycle-list items (navigate with ←/→) from free-text
// items edited with the e key.
type siKind int

const (
	skCycle siKind = iota // left/right arrows cycle through preset values
	skText                // e opens a text-input editor
)

// settingItemDef is a static descriptor for one row in the settings view.
type settingItemDef struct {
	label   string
	kind    siKind
	section string // non-empty → emit a section heading before this row
	isModel bool   // true → e always opens text edit (model names can be arbitrary)
	isRole  bool   // true → r/R can reset this to auto
}

// itemDefs is the complete, ordered list of settings rows.
var itemDefs = []settingItemDef{
	// ── Provider Setup ──────────────────────────────────────────────────────
	{label: "Active provider", kind: skCycle, section: "Provider Setup"},
	{label: "Active model", kind: skCycle, isModel: true},
	{label: "Embeddings provider", kind: skCycle},
	{label: "Embeddings model", kind: skCycle, isModel: true},
	// ── Role Overrides ──────────────────────────────────────────────────────
	{label: "Chat provider", kind: skCycle, section: "Role Overrides", isRole: true},
	{label: "Chat model", kind: skCycle, isModel: true, isRole: true},
	{label: "Ingestion provider", kind: skCycle, isRole: true},
	{label: "Ingestion model", kind: skCycle, isModel: true, isRole: true},
	{label: "Quiz Planner provider", kind: skCycle, isRole: true},
	{label: "Quiz Planner model", kind: skCycle, isModel: true, isRole: true},
	{label: "Quiz Questions provider", kind: skCycle, isRole: true},
	{label: "Quiz Questions model", kind: skCycle, isModel: true, isRole: true},
	// ── Advanced ────────────────────────────────────────────────────────────
	{label: "sfq.command", kind: skText, section: "Advanced"},
	{label: "local.endpoint", kind: skText},
	{label: "local.embeddings_endpoint", kind: skText},
	{label: "custom_prompt_context", kind: skText},
}

// Named indices into itemDefs for readability.
const (
	iActiveProvider   = 0
	iActiveModel      = 1
	iEmbedProvider    = 2
	iEmbedModel       = 3
	iChatProvider     = 4
	iChatModel        = 5
	iIngestProvider   = 6
	iIngestModel      = 7
	iPlannerProvider  = 8
	iPlannerModel     = 9
	iQuestProvider    = 10
	iQuestModel       = 11
	iSFQCommand       = 12
	iLocalEndpoint    = 13
	iLocalEmbEndpoint = 14
	iCustomPrompt     = 15
)

// ── Preset value lists ────────────────────────────────────────────────────

var (
	mainProviderChoices  = []string{"openai", "claude", "local"}
	embedProviderChoices = []string{"openai", "voyage", "local"}
	roleProvChoices      = []string{"auto", "openai", "claude", "local"}

	chatModelMap = map[string][]string{
		"claude": {
			"claude-opus-4-5", "claude-sonnet-4-5", "claude-haiku-4-5",
			"claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
		},
		"openai": {"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo", "o3-mini"},
		"local":  {},
	}

	embedModelMap = map[string][]string{
		"openai":   {"text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"},
		"voyage":   {"voyage-3", "voyage-3-large", "voyage-3-lite", "voyage-code-3"},
		"voyageai": {"voyage-3", "voyage-3-large", "voyage-3-lite", "voyage-code-3"},
		"local":    {},
	}
)

// ── Config accessors ──────────────────────────────────────────────────────

// itemGet returns the current config value for a settings row.
func itemGet(idx int, cfg *config.Config) string {
	switch idx {
	case iActiveProvider:
		return cfg.Provider
	case iActiveModel:
		return activeModelValue(cfg)
	case iEmbedProvider:
		return cfg.Embeddings.Provider
	case iEmbedModel:
		return cfg.Embeddings.Model
	case iChatProvider:
		return orAuto(cfg.AgentModels.Chat.Provider)
	case iChatModel:
		return orAuto(cfg.AgentModels.Chat.Model)
	case iIngestProvider:
		return orAuto(cfg.AgentModels.Ingestion.Provider)
	case iIngestModel:
		return orAuto(cfg.AgentModels.Ingestion.Model)
	case iPlannerProvider:
		return orAuto(cfg.AgentModels.QuizOrchestrator.Provider)
	case iPlannerModel:
		return orAuto(cfg.AgentModels.QuizOrchestrator.Model)
	case iQuestProvider:
		return orAuto(cfg.AgentModels.QuizComponent.Provider)
	case iQuestModel:
		return orAuto(cfg.AgentModels.QuizComponent.Model)
	case iSFQCommand:
		return cfg.SFQ.Command
	case iLocalEndpoint:
		return cfg.Local.Endpoint
	case iLocalEmbEndpoint:
		return cfg.Local.EmbeddingsEndpoint
	case iCustomPrompt:
		return cfg.CustomPromptContext
	}
	return ""
}

// itemSet applies a value to the appropriate config field for a settings row.
func itemSet(idx int, val string, cfg *config.Config) {
	switch idx {
	case iActiveProvider:
		cfg.Provider = val
	case iActiveModel:
		setActiveModel(val, cfg)
	case iEmbedProvider:
		cfg.Embeddings.Provider = val
	case iEmbedModel:
		cfg.Embeddings.Model = val
	case iChatProvider:
		cfg.AgentModels.Chat.Provider = zeroIfAuto(val)
	case iChatModel:
		cfg.AgentModels.Chat.Model = zeroIfAuto(val)
	case iIngestProvider:
		cfg.AgentModels.Ingestion.Provider = zeroIfAuto(val)
	case iIngestModel:
		cfg.AgentModels.Ingestion.Model = zeroIfAuto(val)
	case iPlannerProvider:
		cfg.AgentModels.QuizOrchestrator.Provider = zeroIfAuto(val)
	case iPlannerModel:
		cfg.AgentModels.QuizOrchestrator.Model = zeroIfAuto(val)
	case iQuestProvider:
		cfg.AgentModels.QuizComponent.Provider = zeroIfAuto(val)
	case iQuestModel:
		cfg.AgentModels.QuizComponent.Model = zeroIfAuto(val)
	case iSFQCommand:
		cfg.SFQ.Command = val
	case iLocalEndpoint:
		cfg.Local.Endpoint = val
	case iLocalEmbEndpoint:
		cfg.Local.EmbeddingsEndpoint = val
	case iCustomPrompt:
		cfg.CustomPromptContext = val
	}
}

func orAuto(v string) string {
	if v == "" {
		return "auto"
	}
	return v
}

func zeroIfAuto(v string) string {
	if v == "auto" {
		return ""
	}
	return v
}

func activeModelValue(cfg *config.Config) string {
	switch cfg.Provider {
	case "openai":
		return cfg.OpenAI.Model
	case "claude":
		return cfg.Claude.Model
	case "local":
		return cfg.Local.Model
	}
	return ""
}

func setActiveModel(val string, cfg *config.Config) {
	switch cfg.Provider {
	case "openai":
		cfg.OpenAI.Model = val
	case "claude":
		cfg.Claude.Model = val
	case "local":
		cfg.Local.Model = val
	}
}

// itemChoices returns the preset cycle list for a given row index.
// Returns nil for text-only rows or when there are no presets (e.g. local model).
func itemChoices(idx int, cfg *config.Config) []string {
	switch idx {
	case iActiveProvider:
		return mainProviderChoices
	case iActiveModel:
		return chatModelMap[cfg.Provider]
	case iEmbedProvider:
		return embedProviderChoices
	case iEmbedModel:
		return embedModelMap[cfg.Embeddings.Provider]
	case iChatProvider:
		return roleProvChoices
	case iChatModel:
		return append([]string{"auto"}, chatModelMap[resolveRoleProvider(cfg.AgentModels.Chat.Provider, cfg.Provider)]...)
	case iIngestProvider:
		return roleProvChoices
	case iIngestModel:
		return append([]string{"auto"}, chatModelMap[resolveRoleProvider(cfg.AgentModels.Ingestion.Provider, cfg.Provider)]...)
	case iPlannerProvider:
		return roleProvChoices
	case iPlannerModel:
		return append([]string{"auto"}, chatModelMap[resolveRoleProvider(cfg.AgentModels.QuizOrchestrator.Provider, cfg.Provider)]...)
	case iQuestProvider:
		return roleProvChoices
	case iQuestModel:
		return append([]string{"auto"}, chatModelMap[resolveRoleProvider(cfg.AgentModels.QuizComponent.Provider, cfg.Provider)]...)
	}
	return nil
}

// resolveRoleProvider returns the effective provider for a role, treating
// empty string and "auto" as inheriting the global provider.
func resolveRoleProvider(override, global string) string {
	if override == "" || override == "auto" {
		return global
	}
	return override
}

// resolvedAutoHint returns the effective value that "auto" resolves to,
// for display as a dim hint. Returns empty string when value is not "auto".
func resolvedAutoHint(idx int, cfg *config.Config) string {
	val := itemGet(idx, cfg)
	if val != "auto" {
		return ""
	}
	switch idx {
	case iChatProvider, iIngestProvider, iPlannerProvider, iQuestProvider:
		return cfg.Provider
	case iChatModel:
		return resolvedRoleModel(cfg.AgentModels.Chat.Provider, cfg)
	case iIngestModel:
		return resolvedRoleModel(cfg.AgentModels.Ingestion.Provider, cfg)
	case iPlannerModel:
		return resolvedRoleModel(cfg.AgentModels.QuizOrchestrator.Provider, cfg)
	case iQuestModel:
		return resolvedRoleModel(cfg.AgentModels.QuizComponent.Provider, cfg)
	}
	return ""
}

func resolvedRoleModel(roleProvider string, cfg *config.Config) string {
	p := resolveRoleProvider(roleProvider, cfg.Provider)
	switch p {
	case "openai":
		return cfg.OpenAI.Model
	case "claude":
		return cfg.Claude.Model
	case "local":
		return cfg.Local.Model
	}
	return ""
}

// itemStatus returns an optional right-side status string (e.g. API key check).
func itemStatus(idx int, cfg *config.Config) string {
	switch idx {
	case iActiveProvider:
		return providerKeyStatus(cfg.Provider, cfg)
	case iEmbedProvider:
		return providerKeyStatus(cfg.Embeddings.Provider, cfg)
	}
	return ""
}

func providerKeyStatus(provider string, cfg *config.Config) string {
	switch provider {
	case "openai":
		if cfg.OpenAI.APIKey != "" {
			return successStyle.Render("✓") + dimStyle.Render(" "+config.EnvOpenAIKey)
		}
		return warnStyle.Render("⚠ set " + config.EnvOpenAIKey)
	case "claude":
		if cfg.Claude.APIKey != "" {
			return successStyle.Render("✓") + dimStyle.Render(" "+config.EnvClaudeKey)
		}
		return warnStyle.Render("⚠ set " + config.EnvClaudeKey)
	case "voyage", "voyageai":
		if cfg.Voyage.APIKey != "" {
			return successStyle.Render("✓") + dimStyle.Render(" "+config.EnvVoyageKey)
		}
		return warnStyle.Render("⚠ set " + config.EnvVoyageKey)
	}
	return ""
}

// ── Cycle helpers ─────────────────────────────────────────────────────────

func cycleForward(choices []string, current string) string {
	if len(choices) == 0 {
		return current
	}
	for i, c := range choices {
		if c == current {
			return choices[(i+1)%len(choices)]
		}
	}
	return choices[0]
}

func cycleBackward(choices []string, current string) string {
	if len(choices) == 0 {
		return current
	}
	for i, c := range choices {
		if c == current {
			return choices[(i-1+len(choices))%len(choices)]
		}
	}
	return choices[len(choices)-1]
}

// resetRoleItem resets the override for a single role field to auto (empty).
func resetRoleItem(idx int, cfg *config.Config) {
	switch idx {
	case iChatProvider:
		cfg.AgentModels.Chat.Provider = ""
	case iChatModel:
		cfg.AgentModels.Chat.Model = ""
	case iIngestProvider:
		cfg.AgentModels.Ingestion.Provider = ""
	case iIngestModel:
		cfg.AgentModels.Ingestion.Model = ""
	case iPlannerProvider:
		cfg.AgentModels.QuizOrchestrator.Provider = ""
	case iPlannerModel:
		cfg.AgentModels.QuizOrchestrator.Model = ""
	case iQuestProvider:
		cfg.AgentModels.QuizComponent.Provider = ""
	case iQuestModel:
		cfg.AgentModels.QuizComponent.Model = ""
	}
}

// resetAllRoles clears every role override so all roles inherit the global provider.
func resetAllRoles(cfg *config.Config) {
	cfg.AgentModels = config.AgentModels{}
}

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

	hintLine1 := dimStyle.Render("↑/↓ select  ←/→ cycle value  e edit  s save")
	hintLine2 := dimStyle.Render("r reset role  R reset all roles  keys: " + config.EnvOpenAIKey + " / " + config.EnvClaudeKey + " / " + config.EnvVoyageKey)
	if !s.nav {
		hintLine1 = dimStyle.Render("Press ↓ to focus first setting  •  ←/→ switch tabs")
	}
	hint := hintLine1 + "\n" + hintLine2

	content := lipgloss.JoinVertical(lipgloss.Left,
		bannerText,
		renderSection("Settings", body, width)+editSection,
		renderSection("", hint, width),
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
