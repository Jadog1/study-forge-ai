package server

import (
	"net/http"

	"github.com/studyforge/study-agent/internal/ingestion"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/state"
)

type ingestRequest struct {
	Path  string   `json:"path"`
	Class string   `json:"class"`
	Files []string `json:"files"`
	Clean bool     `json:"clean"`
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ingestRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Path == "" && len(req.Files) == 0 {
		jsonError(w, http.StatusBadRequest, "path or files is required")
		return
	}

	if req.Clean {
		if err := state.ClearIngestedData(); err != nil {
			jsonError(w, http.StatusInternalServerError, "clear data: "+err.Error())
			return
		}
	}

	cfg := s.Config()
	orch := s.Orchestrator()
	provider := orchestrator.BuildProviderForRole("ingestion", cfg)

	flush := sseSetup(w)
	flush()

	onProgress := func(event ingestion.ProgressEvent) {
		payload := map[string]string{
			"type":   "progress",
			"label":  event.Label,
			"detail": event.Detail,
		}
		if event.Done {
			payload["done"] = "true"
		}
		if event.Err != nil {
			payload["error"] = event.Err.Error()
		}
		sseEvent(w, flush, payload)
	}

	var result ingestion.KnowledgeIngestResult
	var err error

	if len(req.Files) > 0 {
		result, err = ingestion.IngestKnowledgeFilesStream(
			req.Files, req.Class, provider, orch.EmbeddingProvider, cfg, onProgress,
		)
	} else {
		result, err = ingestion.IngestKnowledgeFolderStream(
			req.Path, req.Class, provider, orch.EmbeddingProvider, cfg, onProgress,
		)
	}

	if err != nil {
		sseEvent(w, flush, map[string]string{"type": "error", "error": err.Error()})
		return
	}

	notesIdx, loadErr := state.LoadNotesIndex()
	if loadErr == nil {
		for _, note := range result.Notes {
			notesIdx.AddOrUpdate(note)
		}
		_ = state.SaveNotesIndex(notesIdx)
	}

	sseEvent(w, flush, map[string]any{
		"type":             "done",
		"notes":            len(result.Notes),
		"sections_added":   result.SectionsAdded,
		"components_added": result.ComponentsAdded,
		"usage_events":     result.UsageEvents,
	})
}
