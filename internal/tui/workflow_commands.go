package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/state"
)

func (w WorkflowModel) startWorkflow(orc *orchestrator.Orchestrator, cfg *config.Config) (WorkflowModel, bool, string, tea.Cmd) {
	class := strings.TrimSpace(w.classInput.Value())

	switch w.kind {
	case WorkflowIngest:
		path := strings.TrimSpace(w.pathInput.Value())
		if len(w.selectedFiles) > 0 {
			if orc.EmbeddingProvider.Disabled() {
				w.step = stepConfirm
				w.confirmType = "embeddings_disabled"
				w.confirmMsg = fmt.Sprintf(
					"Embeddings are not configured (provider: %s).\nDeduplication and semantic consolidation will NOT happen.\n\nContinue anyway?",
					orc.EmbeddingProvider.Name(),
				)
				return w, false, "", nil
			}
			if w.cleanBeforeIngest {
				if err := state.ClearIngestedData(); err != nil {
					return w, false, fmt.Sprintf("Failed to clear ingestion data: %v", err), nil
				}
			}
			w.step = stepRunning
			return w, true, "Running " + w.title() + "…", runIngestFilesCmd(w.selectedFiles, class, orc, cfg)
		}
		return w.runIngestWorkflow(path, class, orc, cfg, true)

	case WorkflowGenerate:
		if class == "" {
			return w, false, "Class name is required", nil
		}
		opts := quiz.QuizOptions{
			AssessmentKind:  w.selectedAssessmentKind(),
			Count:           parseQuizCount(w.countInput.Value()),
			TypePreference:  w.selectedQuestionPreference(),
			FocusedSections: parseSectionsList(w.sectionsInput.Value()),
		}
		w.step = stepRunning
		return w, true, "Running " + w.title() + "…", runQuizCmd(class, opts, orc, cfg)

	case WorkflowExport:
		path := strings.TrimSpace(w.pathInput.Value())
		if path == "" {
			return w, false, "Output file path is required", nil
		}
		w.step = stepRunning
		return w, true, "Running " + w.title() + "…", runExportKnowledgeCmd(path, class, w.includeEmbeddings)
	}

	return w, false, "", nil
}

func (w WorkflowModel) runIngestWorkflow(path, class string, orc *orchestrator.Orchestrator, cfg *config.Config, requireEmbeddingsConfirm bool) (WorkflowModel, bool, string, tea.Cmd) {
	if path == "" {
		return w, false, "Folder path is required", nil
	}

	if requireEmbeddingsConfirm && orc.EmbeddingProvider.Disabled() {
		w.step = stepConfirm
		w.confirmType = "embeddings_disabled"
		w.confirmMsg = fmt.Sprintf(
			"Embeddings are not configured (provider: %s).\nDeduplication and semantic consolidation will NOT happen.\n\nContinue anyway?",
			orc.EmbeddingProvider.Name(),
		)
		return w, false, "", nil
	}

	if w.cleanBeforeIngest {
		if err := state.ClearIngestedData(); err != nil {
			return w, false, fmt.Sprintf("Failed to clear ingestion data: %v", err), nil
		}
	}

	w.step = stepRunning
	return w, true, "Running " + w.title() + "…", runIngestCmd(path, class, orc, cfg)
}
