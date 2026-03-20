// Package orchestrator is the central coordination layer. It loads the
// workspace configuration and wires the correct AI provider to the rest of
// the system. All CLI commands go through an Orchestrator instance.
package orchestrator

import (
	"fmt"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/plugins"
	claudeprovider "github.com/studyforge/study-agent/plugins/claude"
	localprovider "github.com/studyforge/study-agent/plugins/local"
	openaiprovider "github.com/studyforge/study-agent/plugins/openai"
)

// Orchestrator holds the active configuration and AI provider for a session.
type Orchestrator struct {
	Config   *config.Config
	Provider plugins.AIProvider
}

// New builds an Orchestrator and returns an error if the selected provider is
// not available (e.g. missing API key). Used by CLI commands that require a
// working provider before proceeding.
func New(cfg *config.Config) (*Orchestrator, error) {
	p := buildProvider(cfg)
	if p.Disabled() {
		return nil, fmt.Errorf("provider %q is not configured — edit %s/%s", cfg.Provider, config.DisplayRootDir(), config.ConfigFile)
	}
	return &Orchestrator{Config: cfg, Provider: p}, nil
}

// NewFallback builds an Orchestrator even when the selected provider is
// disabled or unconfigured. Used by the TUI so the UI opens regardless of
// provider state — the user can configure it in the Settings tab.
func NewFallback(cfg *config.Config) *Orchestrator {
	return &Orchestrator{Config: cfg, Provider: buildProvider(cfg)}
}

// buildProvider always returns a valid AIProvider (which may report Disabled()).
func buildProvider(cfg *config.Config) plugins.AIProvider {
	switch cfg.Provider {
	case "openai":
		return openaiprovider.New(cfg.OpenAI.APIKey, cfg.OpenAI.Model)
	case "claude":
		return claudeprovider.New(cfg.Claude.APIKey, cfg.Claude.Model)
	case "local":
		return localprovider.New(cfg.Local.Endpoint, cfg.Local.Model)
	default:
		return &unknownProvider{name: cfg.Provider}
	}
}

// unknownProvider satisfies plugins.AIProvider for unrecognised provider names.
type unknownProvider struct{ name string }

func (u *unknownProvider) Name() string     { return u.name }
func (u *unknownProvider) Disabled() bool   { return true }
func (u *unknownProvider) Generate(_ string) (string, error) {
	return "", fmt.Errorf("unknown provider %q — valid values: openai, claude, local", u.name)
}
