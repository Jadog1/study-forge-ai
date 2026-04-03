package server

import "net/http"

// registerRoutes wires every API endpoint to its handler.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Config
	mux.HandleFunc("/api/config", s.routeConfig)

	// Chat (SSE streaming)
	mux.HandleFunc("/api/chat", s.handleChat)

	// Knowledge
	mux.HandleFunc("/api/knowledge/sections", s.handleListSections)
	mux.HandleFunc("/api/knowledge/components", s.handleListComponents)

	// Quiz
	mux.HandleFunc("/api/quiz/dashboard", s.handleQuizDashboard)
	mux.HandleFunc("/api/quiz/generate", s.handleGenerateQuiz)
	mux.HandleFunc("/api/quiz/sync", s.handleSyncTrackedSessions)

	// Classes
	mux.HandleFunc("/api/classes/profiles", s.handleContextProfiles)
	mux.HandleFunc("/api/classes", s.routeClasses)
	mux.HandleFunc("/api/classes/", s.routeClassByName)

	// Ingest
	mux.HandleFunc("/api/ingest", s.handleIngest)

	// Usage
	mux.HandleFunc("/api/usage/ledger", s.handleUsageLedger)
	mux.HandleFunc("/api/usage", s.handleUsageTotals)

	// Export
	mux.HandleFunc("/api/export", s.handleExport)

	// File browsing (local filesystem)
	mux.HandleFunc("/api/browse", s.handleBrowse)

	// SFQ meta
	mux.HandleFunc("/api/sfq/question-types", s.handleQuestionTypes)
}

// routeConfig dispatches GET / PUT to the config handlers.
func (s *Server) routeConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPut:
		s.handleUpdateConfig(w, r)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// routeClasses dispatches GET / POST on the /api/classes root (no trailing slash).
func (s *Server) routeClasses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListClasses(w, r)
	case http.MethodPost:
		s.handleCreateClass(w, r)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// routeClassByName handles all /api/classes/{name}... sub-routes using path
// prefix matching and manual segment parsing (Go 1.21 ServeMux has no params).
func (s *Server) routeClassByName(w http.ResponseWriter, r *http.Request) {
	// After "/api/classes/" we expect: {name} or {name}/sub/...
	rest := pathParam(r, "/api/classes/")
	if rest == "" {
		jsonError(w, http.StatusBadRequest, "class name is required")
		return
	}

	parts := splitPath(rest)
	className := parts[0]

	// /api/classes/{name}
	if len(parts) == 1 {
		if r.Method == http.MethodGet {
			s.handleGetClass(w, r, className)
		} else {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	sub := parts[1]
	switch sub {
	case "context":
		// PUT /api/classes/{name}/context
		if r.Method == http.MethodPut {
			s.handleUpdateClassContext(w, r, className)
		} else {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case "profile":
		// PUT /api/classes/{name}/profile/{kind}
		if len(parts) < 3 {
			jsonError(w, http.StatusBadRequest, "profile kind is required")
			return
		}
		profileKind := parts[2]
		if r.Method == http.MethodPut {
			s.handleUpdateProfileContext(w, r, className, profileKind)
		} else {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case "roster":
		s.routeClassRoster(w, r, className, parts[2:])

	case "coverage":
		if len(parts) < 3 {
			jsonError(w, http.StatusBadRequest, "coverage kind is required")
			return
		}
		coverageKind := parts[2]
		switch r.Method {
		case http.MethodGet:
			s.handleGetCoverage(w, r, className, coverageKind)
		case http.MethodPut:
			s.handleUpdateCoverage(w, r, className, coverageKind)
		default:
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	default:
		jsonError(w, http.StatusNotFound, "not found")
	}
}

// routeClassRoster handles /api/classes/{name}/roster[/{label}].
func (s *Server) routeClassRoster(w http.ResponseWriter, r *http.Request, className string, remaining []string) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetRoster(w, r, className)
	case http.MethodPut:
		s.handleUpdateRoster(w, r, className)
	case http.MethodPost:
		s.handleAddRosterEntry(w, r, className)
	case http.MethodDelete:
		if len(remaining) == 0 {
			jsonError(w, http.StatusBadRequest, "roster label is required for DELETE")
			return
		}
		s.handleRemoveRosterEntry(w, r, className, remaining[0])
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// splitPath splits a cleaned URL path on "/" and returns non-empty segments.
func splitPath(path string) []string {
	var parts []string
	for _, p := range splitString(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitString(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
