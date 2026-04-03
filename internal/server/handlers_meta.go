package server

import (
	"net/http"

	"github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/sfq"
)

func (s *Server) handleQuestionTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	jsonResponse(w, http.StatusOK, sfq.SupportedQuestionTypes())
}

func (s *Server) handleContextProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	jsonResponse(w, http.StatusOK, class.ContextProfiles())
}
