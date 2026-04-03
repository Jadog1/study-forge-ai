package server

import (
	"net/http"

	"github.com/studyforge/study-agent/internal/chat"
	"github.com/studyforge/study-agent/internal/orchestrator"
)

type chatRequest struct {
	Message string `json:"message"`
	Class   string `json:"class"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req chatRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Message == "" {
		jsonError(w, http.StatusBadRequest, "message is required")
		return
	}

	cfg := s.Config()
	provider := orchestrator.BuildProviderForRole("chat", cfg)

	flush := sseSetup(w)
	flush()

	err := chat.AskStream(provider, cfg, req.Class, req.Message, func(event chat.StreamEvent) error {
		payload := map[string]string{}
		switch event.Kind {
		case chat.StreamEventChunk:
			payload["type"] = "chunk"
			payload["text"] = event.Text
		case chat.StreamEventActionStart:
			payload["type"] = "action-start"
			payload["label"] = event.Label
			payload["detail"] = event.Detail
		case chat.StreamEventActionDone:
			payload["type"] = "action-done"
			payload["label"] = event.Label
			payload["detail"] = event.Detail
			if event.Err != nil {
				payload["error"] = event.Err.Error()
			}
		}
		sseEvent(w, flush, payload)
		return nil
	})

	if err != nil {
		sseEvent(w, flush, map[string]string{"type": "error", "error": err.Error()})
		return
	}
	sseEvent(w, flush, map[string]string{"type": "done"})
}
