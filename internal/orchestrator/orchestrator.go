// Package orchestrator is the central coordination layer. It loads the
// workspace configuration and wires the correct AI provider to the rest of
// the system. All CLI commands go through an Orchestrator instance.
package orchestrator

import (
	"fmt"
	"strings"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/plugins"
	claudeprovider "github.com/studyforge/study-agent/plugins/claude"
	localprovider "github.com/studyforge/study-agent/plugins/local"
	openaiprovider "github.com/studyforge/study-agent/plugins/openai"
	voyageprovider "github.com/studyforge/study-agent/plugins/voyage"
)

// Orchestrator holds the active configuration and AI provider for a session.
type Orchestrator struct {
	Config            *config.Config
	Provider          plugins.AIProvider
	EmbeddingProvider plugins.EmbeddingProvider
}

// New builds an Orchestrator and returns an error if the selected provider is
// not available (e.g. missing API key). Used by CLI commands that require a
// working provider before proceeding.
func New(cfg *config.Config) (*Orchestrator, error) {
	p := buildProvider(cfg)
	if p.Disabled() {
		return nil, fmt.Errorf("provider %q is not configured — edit %s/%s", cfg.Provider, config.DisplayRootDir(), config.ConfigFile)
	}
	return &Orchestrator{Config: cfg, Provider: p, EmbeddingProvider: buildEmbeddingProvider(cfg)}, nil
}

// NewFallback builds an Orchestrator even when the selected provider is
// disabled or unconfigured. Used by the TUI so the UI opens regardless of
// provider state — the user can configure it in the Settings tab.
func NewFallback(cfg *config.Config) *Orchestrator {
	return &Orchestrator{Config: cfg, Provider: buildProvider(cfg), EmbeddingProvider: buildEmbeddingProvider(cfg)}
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

func buildEmbeddingProvider(cfg *config.Config) plugins.EmbeddingProvider {
	provider := strings.TrimSpace(strings.ToLower(cfg.Embeddings.Provider))
	model := strings.TrimSpace(cfg.Embeddings.Model)

	switch provider {
	case "", "openai":
		if model == "" {
			model = "text-embedding-3-small"
		}
		return openaiprovider.New(cfg.OpenAI.APIKey, model)
	case "voyage", "voyageai":
		if model == "" {
			model = cfg.Voyage.Model
		}
		return voyageprovider.New(cfg.Voyage.APIKey, model)
	case "local":
		endpoint := strings.TrimSpace(cfg.Local.EmbeddingsEndpoint)
		if endpoint == "" {
			endpoint = strings.TrimSpace(cfg.Local.Endpoint)
		}
		if model == "" {
			model = cfg.Local.Model
		}
		return localprovider.NewEmbeddingProvider(endpoint, model)
	default:
		return &unknownEmbeddingProvider{name: provider}
	}
}

// unknownProvider satisfies plugins.AIProvider for unrecognised provider names.
type unknownProvider struct{ name string }

func (u *unknownProvider) Name() string   { return u.name }
func (u *unknownProvider) Disabled() bool { return true }
func (u *unknownProvider) Generate(_ string) (string, error) {
	return "", fmt.Errorf("unknown provider %q — valid values: openai, claude, local", u.name)
}
func (u *unknownProvider) Model() string { return "unknown" }

type unknownEmbeddingProvider struct{ name string }

func (u *unknownEmbeddingProvider) Name() string   { return u.name }
func (u *unknownEmbeddingProvider) Disabled() bool { return true }
func (u *unknownEmbeddingProvider) Embed(_ []string) ([][]float64, error) {
	return nil, fmt.Errorf("unknown embeddings provider %q — valid values: openai, voyage, local", u.name)
}
func (u *unknownEmbeddingProvider) Model() string { return "unknown" }
