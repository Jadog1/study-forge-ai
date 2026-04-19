package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/chat"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/state"
)

type chatRequest struct {
	Message string `json:"message"`
	Class   string `json:"class"`
	Mode    string `json:"mode,omitempty"`
}

type latestChatResponse struct {
	Class     string              `json:"class,omitempty"`
	Mode      string              `json:"mode,omitempty"`
	Messages  []state.ChatMessage `json:"messages"`
	UpdatedAt string              `json:"updated_at,omitempty"`
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
	mode := chat.NormalizeMode(req.Mode)

	// Load conversation history from previous session
	var history []state.ChatMessage
	session, sessionErr := s.Store().Chat().LoadLatestChatSession()
	if sessionErr == nil && session != nil && session.Class == req.Class && session.Mode == string(mode) {
		// Use existing history if class and mode match
		history = session.Messages
	}

	flush := sseSetup(w)
	flush()
	var assistantResponse strings.Builder

	err := s.ChatService().AskStreamWithHistory(provider, cfg, req.Class, req.Message, mode, history, func(event chat.StreamEvent) error {
		payload := map[string]string{}
		switch event.Kind {
		case chat.StreamEventChunk:
			assistantResponse.WriteString(event.Text)
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

	// Append new messages to history
	newMessages := append(history,
		state.ChatMessage{Role: "user", Content: req.Message},
		state.ChatMessage{Role: "assistant", Content: assistantResponse.String()},
	)

	_ = s.Store().Chat().SaveLatestChatSession(&state.ChatSession{
		Class:     strings.TrimSpace(req.Class),
		Mode:      string(mode),
		Messages:  newMessages,
		UpdatedAt: time.Now().UTC(),
	})

	sseEvent(w, flush, map[string]string{"type": "done"})
}

func (s *Server) handleGetLatestChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	session, err := s.Store().Chat().LoadLatestChatSession()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil {
		jsonResponse(w, http.StatusOK, latestChatResponse{Messages: []state.ChatMessage{}})
		return
	}
	resp := latestChatResponse{
		Class:    session.Class,
		Mode:     session.Mode,
		Messages: session.Messages,
	}
	if !session.UpdatedAt.IsZero() {
		resp.UpdatedAt = session.UpdatedAt.UTC().Format(time.RFC3339)
	}
	jsonResponse(w, http.StatusOK, resp)
}

func (s *Server) handleClearLatestChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.Store().Chat().ClearLatestChatSession(); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}
