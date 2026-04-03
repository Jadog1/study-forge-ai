package tui

import "github.com/studyforge/study-agent/internal/config"

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
