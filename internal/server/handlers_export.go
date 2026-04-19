package server

import (
	"net/http"

	"github.com/studyforge/study-agent/internal/state"
)

type exportRequest struct {
	OutputPath        string `json:"output_path"`
	Class             string `json:"class"`
	IncludeEmbeddings bool   `json:"include_embeddings"`
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req exportRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.OutputPath == "" {
		jsonError(w, http.StatusBadRequest, "output_path is required")
		return
	}

	result, err := s.Store().Export().ExportKnowledgeDataset(req.OutputPath, state.KnowledgeExportOptions{
		Class:             req.Class,
		IncludeEmbeddings: req.IncludeEmbeddings,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "export: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"output_path":        result.OutputPath,
		"class":              result.Class,
		"sections":           result.Sections,
		"components":         result.Components,
		"include_embeddings": result.IncludeEmbeddings,
	})
}
