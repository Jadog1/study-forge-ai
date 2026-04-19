package server

import (
	"net/http"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
)

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	safe := sanitizeConfigForResponse(cfg)
	jsonResponse(w, http.StatusOK, safe)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var incoming config.Config
	if err := decodeJSON(r, &incoming); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	current := s.Config()

	if incoming.Provider != "" {
		current.Provider = incoming.Provider
	}
	if incoming.OpenAI.Model != "" {
		current.OpenAI.Model = incoming.OpenAI.Model
	}
	if incoming.Claude.Model != "" {
		current.Claude.Model = incoming.Claude.Model
	}
	if incoming.Voyage.Model != "" {
		current.Voyage.Model = incoming.Voyage.Model
	}
	if incoming.Local.Endpoint != "" {
		current.Local.Endpoint = incoming.Local.Endpoint
	}
	if incoming.Local.EmbeddingsEndpoint != "" {
		current.Local.EmbeddingsEndpoint = incoming.Local.EmbeddingsEndpoint
	}
	if incoming.Local.Model != "" {
		current.Local.Model = incoming.Local.Model
	}
	if incoming.SFQ.Command != "" {
		current.SFQ.Command = incoming.SFQ.Command
	}
	if incoming.Embeddings.Provider != "" {
		current.Embeddings.Provider = incoming.Embeddings.Provider
	}
	if incoming.Embeddings.Model != "" {
		current.Embeddings.Model = incoming.Embeddings.Model
	}
	current.AgentModels = incoming.AgentModels
	current.CustomPromptContext = incoming.CustomPromptContext
	if incoming.ModelPrices != nil {
		current.ModelPrices = incoming.ModelPrices
	}

	if err := s.Store().Config().SaveConfig(current); err != nil {
		jsonError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}

	reloaded, err := s.Store().Config().LoadConfig()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "reload config: "+err.Error())
		return
	}

	orch := orchestrator.NewFallback(reloaded)
	s.SetConfigAndOrchestrator(reloaded, orch)

	safe := sanitizeConfigForResponse(reloaded)
	jsonResponse(w, http.StatusOK, safe)
}

// sanitizeConfigForResponse zeroes API keys before returning config as JSON.
func sanitizeConfigForResponse(cfg *config.Config) *config.Config {
	out := *cfg
	out.OpenAI.APIKey = ""
	out.Claude.APIKey = ""
	out.Voyage.APIKey = ""
	return &out
}
