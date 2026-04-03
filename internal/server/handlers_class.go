package server

import (
	"net/http"

	"github.com/studyforge/study-agent/internal/class"
)

func (s *Server) handleListClasses(w http.ResponseWriter, r *http.Request) {
	names, err := class.List()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "list classes: "+err.Error())
		return
	}
	if names == nil {
		names = []string{}
	}
	jsonResponse(w, http.StatusOK, names)
}

type createClassRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateClass(w http.ResponseWriter, r *http.Request) {
	var req createClassRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := class.Create(req.Name); err != nil {
		jsonError(w, http.StatusInternalServerError, "create class: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]string{"name": req.Name})
}

type classDetail struct {
	Name     string                  `json:"name"`
	Syllabus *class.Syllabus         `json:"syllabus"`
	Rules    *class.Rules            `json:"rules"`
	Context  *class.Context          `json:"context"`
	Profiles map[string]string       `json:"profiles"`
	Roster   *class.NoteRoster       `json:"roster"`
	Coverage map[string]*class.CoverageScope `json:"coverage"`
}

func (s *Server) handleGetClass(w http.ResponseWriter, r *http.Request, className string) {
	syllabus, _ := class.LoadSyllabus(className)
	rules, _ := class.LoadRules(className)
	ctx, _ := class.LoadContext(className)
	roster, _ := class.LoadNoteRoster(className)

	profiles := make(map[string]string)
	for _, p := range class.ContextProfiles() {
		text, err := class.LoadProfileContextText(className, p.Kind)
		if err == nil {
			profiles[p.Kind] = text
		}
	}

	coverage := make(map[string]*class.CoverageScope)
	for _, p := range class.ContextProfiles() {
		scope, err := class.LoadCoverageScope(className, p.Kind)
		if err == nil && scope != nil {
			coverage[p.Kind] = scope
		}
	}

	jsonResponse(w, http.StatusOK, classDetail{
		Name:     className,
		Syllabus: syllabus,
		Rules:    rules,
		Context:  ctx,
		Profiles: profiles,
		Roster:   roster,
		Coverage: coverage,
	})
}

type updateContextRequest struct {
	ContextFiles []string `json:"context_files"`
}

func (s *Server) handleUpdateClassContext(w http.ResponseWriter, r *http.Request, className string) {
	var req updateContextRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	ctx := &class.Context{
		Class:        className,
		ContextFiles: req.ContextFiles,
	}
	if err := class.SaveContext(className, ctx); err != nil {
		jsonError(w, http.StatusInternalServerError, "save context: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ctx)
}

type updateProfileRequest struct {
	Text string `json:"text"`
}

func (s *Server) handleUpdateProfileContext(w http.ResponseWriter, r *http.Request, className, profileKind string) {
	var req updateProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := class.SaveProfileContextText(className, profileKind, req.Text); err != nil {
		jsonError(w, http.StatusInternalServerError, "save profile context: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"kind": profileKind, "text": req.Text})
}

func (s *Server) handleGetRoster(w http.ResponseWriter, r *http.Request, className string) {
	roster, err := class.LoadNoteRoster(className)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load roster: "+err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, roster)
}

type updateRosterRequest struct {
	Entries []class.NoteRosterEntry `json:"entries"`
	Labels  []string                `json:"labels"`
}

func (s *Server) handleUpdateRoster(w http.ResponseWriter, r *http.Request, className string) {
	var req updateRosterRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if len(req.Labels) > 0 {
		roster, err := class.ReorderNoteRosterEntries(className, req.Labels)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "reorder roster: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, roster)
		return
	}

	roster := &class.NoteRoster{
		Class:   className,
		Entries: req.Entries,
	}
	if err := class.SaveNoteRoster(className, roster); err != nil {
		jsonError(w, http.StatusInternalServerError, "save roster: "+err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, roster)
}

func (s *Server) handleAddRosterEntry(w http.ResponseWriter, r *http.Request, className string) {
	var entry class.NoteRosterEntry
	if err := decodeJSON(r, &entry); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	roster, err := class.UpsertNoteRosterEntry(className, entry)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "upsert roster entry: "+err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, roster)
}

func (s *Server) handleRemoveRosterEntry(w http.ResponseWriter, r *http.Request, className, label string) {
	roster, err := class.RemoveNoteRosterEntry(className, label)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "remove roster entry: "+err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, roster)
}

func (s *Server) handleGetCoverage(w http.ResponseWriter, r *http.Request, className, kind string) {
	scope, err := class.LoadCoverageScope(className, kind)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load coverage: "+err.Error())
		return
	}
	if scope == nil {
		jsonResponse(w, http.StatusOK, map[string]any{
			"class":  className,
			"kind":   kind,
			"groups": []any{},
		})
		return
	}
	jsonResponse(w, http.StatusOK, scope)
}

func (s *Server) handleUpdateCoverage(w http.ResponseWriter, r *http.Request, className, kind string) {
	var scope class.CoverageScope
	if err := decodeJSON(r, &scope); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := class.SaveCoverageScope(className, kind, &scope); err != nil {
		jsonError(w, http.StatusInternalServerError, "save coverage: "+err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, &scope)
}
