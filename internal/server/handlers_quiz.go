package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/internal/tracking"
)

type quizDocSummary struct {
	ID            string    `json:"id"`
	Class         string    `json:"class"`
	Title         string    `json:"title"`
	Path          string    `json:"path"`
	QuestionCount int       `json:"question_count"`
	GeneratedAt   time.Time `json:"generated_at"`
}

type quizDashboard struct {
	Sections   []state.Section         `json:"sections"`
	Components []state.Component       `json:"components"`
	Tracked    *state.TrackedQuizCache `json:"tracked"`
	Quizzes    []quizDocSummary        `json:"quizzes"`
	LoadedAt   time.Time               `json:"loaded_at"`
}

func (s *Server) handleQuizDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	secIdx, err := s.Store().Knowledge().LoadSectionIndex()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load sections: "+err.Error())
		return
	}
	for i := range secIdx.Sections {
		secIdx.Sections[i].Embedding = nil
	}

	cmpIdx, err := s.Store().Knowledge().LoadComponentIndex()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load components: "+err.Error())
		return
	}
	for i := range cmpIdx.Components {
		cmpIdx.Components[i].Embedding = nil
	}

	cache, err := s.Store().QuizAttempts().LoadTrackedQuizCache()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load tracked cache: "+err.Error())
		return
	}

	docs, err := loadQuizDocs()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "load quiz docs: "+err.Error())
		return
	}

	sections := secIdx.Sections
	if sections == nil {
		sections = []state.Section{}
	}
	components := cmpIdx.Components
	if components == nil {
		components = []state.Component{}
	}
	if docs == nil {
		docs = []quizDocSummary{}
	}

	jsonResponse(w, http.StatusOK, quizDashboard{
		Sections:   sections,
		Components: components,
		Tracked:    cache,
		Quizzes:    docs,
		LoadedAt:   time.Now().UTC(),
	})
}

func loadQuizDocs() ([]quizDocSummary, error) {
	quizRoot, err := config.Path("quizzes")
	if err != nil {
		return nil, err
	}

	var docs []quizDocSummary
	walkErr := filepath.WalkDir(quizRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(d.Name()) != ".yaml" {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		quizDoc, loadErr := quiz.LoadQuiz(path)
		if loadErr != nil || quizDoc == nil {
			return nil
		}
		docs = append(docs, quizDocSummary{
			ID:            strings.TrimSuffix(filepath.Base(path), ".yaml"),
			Class:         strings.TrimSpace(quizDoc.Class),
			Title:         strings.TrimSpace(quizDoc.Title),
			Path:          path,
			QuestionCount: len(quizDoc.Sections),
			GeneratedAt:   info.ModTime().UTC(),
		})
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk quiz directory: %w", walkErr)
	}
	return docs, nil
}

type generateQuizRequest struct {
	Class           string                       `json:"class"`
	Count           int                          `json:"count"`
	Type            string                       `json:"type"`
	AssessmentType  string                       `json:"assessment_type"`
	QuestionType    string                       `json:"question_type"`
	FocusedSections []string                     `json:"focused_sections"`
	Tags            []string                     `json:"tags"`
	UseOrchestrator bool                         `json:"use_orchestrator"`
	CandidateIDs    []string                     `json:"candidate_component_ids"`
	Directives      []quiz.OrchestratorDirective `json:"directives"`
}

func (s *Server) handleGenerateQuiz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req generateQuizRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Class == "" {
		jsonError(w, http.StatusBadRequest, "class is required")
		return
	}

	cfg := s.Config()
	provider := orchestrator.BuildProviderForRole("quiz_orchestrator", cfg)

	typePreference := req.Type
	if typePreference == "" {
		typePreference = req.QuestionType
	}

	opts := quiz.QuizOptions{
		AssessmentKind:        req.AssessmentType,
		Count:                 req.Count,
		TypePreference:        typePreference,
		FocusedSections:       req.FocusedSections,
		Tags:                  req.Tags,
		UseOrchestrator:       req.UseOrchestrator,
		CandidateComponentIDs: req.CandidateIDs,
		Directives:            req.Directives,
		ProviderOverrides: &quiz.QuizProviderOverrides{
			Orchestrator: provider,
			Component:    orchestrator.BuildProviderForRole("quiz_component", cfg),
		},
	}

	flush := sseSetup(w)
	flush()

	q, quizPath, err := s.QuizService().NewQuizStream(req.Class, opts, provider, cfg, func(progress quiz.ProgressEvent) {
		payload := map[string]string{
			"type":   "progress",
			"label":  progress.Label,
			"detail": progress.Detail,
		}
		if progress.Done {
			payload["done"] = "true"
		}
		if progress.Err != nil {
			payload["error"] = progress.Err.Error()
		}
		sseEvent(w, flush, payload)
	})
	if err != nil {
		sseEvent(w, flush, map[string]string{"type": "error", "error": err.Error()})
		return
	}

	quizID := strings.TrimSuffix(filepath.Base(quizPath), ".yaml")
	sfqPath := strings.TrimSuffix(quizPath, ".yaml") + ".sfq"

	_, cacheErr := s.Store().QuizAttempts().RegisterTrackedQuiz(req.Class, quizPath, sfqPath)
	if cacheErr != nil {
		sseEvent(w, flush, map[string]string{
			"type":    "warning",
			"message": "tracked cache: " + cacheErr.Error(),
		})
	}

	_ = sfq.Track(sfqPath)
	_, _ = s.SyncService().SyncTrackedQuizSessions()

	sseEvent(w, flush, map[string]any{
		"type":           "done",
		"quiz_id":        quizID,
		"title":          q.Title,
		"path":           quizPath,
		"question_count": len(q.Sections),
	})
}

func (s *Server) handleSyncTrackedSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	report, err := s.SyncService().SyncTrackedQuizSessionsWithOptions(tracking.SyncOptions{BackfillImported: true})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "sync failed: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, report)
}
