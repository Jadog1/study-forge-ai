package server

import (
	"net/http"

	"github.com/studyforge/study-agent/internal/state"
)

func (s *Server) handleListSections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idx, err := s.Store().Knowledge().LoadSectionIndex()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load sections: "+err.Error())
		return
	}

	for i := range idx.Sections {
		idx.Sections[i].Embedding = nil
	}
	if idx.Sections == nil {
		idx.Sections = []state.Section{}
	}

	jsonResponse(w, http.StatusOK, idx)
}

func (s *Server) handleListComponents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idx, err := s.Store().Knowledge().LoadComponentIndex()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load components: "+err.Error())
		return
	}

	for i := range idx.Components {
		idx.Components[i].Embedding = nil
	}
	if idx.Components == nil {
		idx.Components = []state.Component{}
	}

	jsonResponse(w, http.StatusOK, idx)
}
