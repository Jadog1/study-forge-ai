package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/studyforge/study-agent/internal/chat"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/ingestion"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/internal/tracking"
)

type aiStreamEvent struct {
	part        string
	actionLabel string
	actionInfo  string
	actionDone  bool
	err         error
	done        bool
}

// askAICmd fires a streaming chat request against the configured AI provider.
func askAICmd(orc *orchestrator.Orchestrator, cfg *config.Config, className, prompt string) tea.Cmd {
	stream := make(chan aiStreamEvent, 32)
	go func() {
		defer close(stream)
		err := chat.AskStream(orc.Provider, cfg, className, prompt, func(event chat.StreamEvent) error {
			switch event.Kind {
			case chat.StreamEventChunk:
				if event.Text == "" {
					return nil
				}
				stream <- aiStreamEvent{part: event.Text}
			case chat.StreamEventActionStart:
				stream <- aiStreamEvent{actionLabel: event.Label, actionInfo: event.Detail}
			case chat.StreamEventActionDone:
				stream <- aiStreamEvent{actionLabel: event.Label, actionInfo: event.Detail, actionDone: true, err: event.Err}
			}
			return nil
		})
		if err != nil {
			stream <- aiStreamEvent{err: err}
			return
		}
		stream <- aiStreamEvent{done: true}
	}()

	return waitForAIStreamCmd(stream)
}

func waitForAIStreamCmd(stream <-chan aiStreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-stream
		if !ok {
			return aiStreamMsg{done: true}
		}
		return aiStreamMsg{
			stream:      stream,
			part:        event.part,
			actionLabel: event.actionLabel,
			actionInfo:  event.actionInfo,
			actionDone:  event.actionDone,
			err:         event.err,
			done:        event.done,
		}
	}
}

// runSFQCmd executes the configured sfq plugin search command.
// When auto is true the result is tagged as a background auto-lookup.
func runSFQCmd(command, query string, auto bool) tea.Cmd {
	return func() tea.Msg {
		out, err := sfq.Search(command, query)
		return sfqDoneMsg{text: out, err: err, autoSFQ: auto}
	}
}

// runIngestCmd processes a folder of notes with the AI provider and updates
// the notes index.
func runIngestCmd(folderPath, class string, orc *orchestrator.Orchestrator, cfg *config.Config) tea.Cmd {
	stream := make(chan aiStreamEvent, 32)
	go func() {
		defer close(stream)

		knowledge, err := ingestion.IngestKnowledgeFolderStream(folderPath, class, orc.Provider, orc.EmbeddingProvider, cfg, func(e ingestion.ProgressEvent) {
			stream <- aiStreamEvent{
				actionLabel: e.Label,
				actionInfo:  e.Detail,
				actionDone:  e.Done,
				err:         e.Err,
			}
		})
		if err != nil {
			stream <- aiStreamEvent{err: err, done: true}
			return
		}

		stream <- aiStreamEvent{actionLabel: "Update index", actionInfo: "loading", actionDone: false}
		idx, idxErr := state.LoadNotesIndex()
		if idxErr != nil {
			stream <- aiStreamEvent{err: fmt.Errorf("load notes index: %w", idxErr), done: true}
			return
		}
		for _, n := range knowledge.Notes {
			idx.AddOrUpdate(n)
		}
		if saveErr := state.SaveNotesIndex(idx); saveErr != nil {
			stream <- aiStreamEvent{err: fmt.Errorf("save notes index: %w", saveErr), done: true}
			return
		}
		stream <- aiStreamEvent{actionLabel: "Update index", actionInfo: fmt.Sprintf("%d note(s)", len(knowledge.Notes)), actionDone: true}

		stream <- aiStreamEvent{
			part: fmt.Sprintf("Ingested %d note(s) from %q\nSections: %d\nComponents: %d\nUsage events: %d", len(knowledge.Notes), folderPath, knowledge.SectionsAdded, knowledge.ComponentsAdded, knowledge.UsageEvents),
			done: true,
		}
	}()

	return waitForAIStreamCmd(stream)
}

// runQuizCmd generates a unified adaptive quiz for the given class, streaming
// agent tool-call events back to the TUI as aiStreamMsgs.
func runQuizCmd(class string, opts quiz.QuizOptions, orc *orchestrator.Orchestrator, cfg *config.Config) tea.Cmd {
	stream := make(chan aiStreamEvent, 32)
	go func() {
		defer close(stream)
		q, path, err := quiz.NewQuizStream(class, opts, orc.Provider, cfg, func(e quiz.ProgressEvent) {
			stream <- aiStreamEvent{
				actionLabel: e.Label,
				actionInfo:  e.Detail,
				actionDone:  e.Done,
				err:         e.Err,
			}
		})
		if err != nil {
			stream <- aiStreamEvent{err: err, done: true}
			return
		}
		quizID := strings.TrimSuffix(filepath.Base(path), ".yaml")
		sfqPath := strings.TrimSuffix(path, ".yaml") + ".sfq"
		_, cacheErr := state.RegisterTrackedQuiz(class, path, sfqPath)
		report, syncErr := tracking.SyncTrackedQuizSessions()
		sfqErr := sfq.Track(sfqPath)
		var sfqNote string
		if cacheErr != nil {
			sfqNote = fmt.Sprintf("\n  (could not register tracked quiz cache: %s)", cacheErr)
		} else if sfqErr != nil {
			sfqNote = fmt.Sprintf("\n  (could not start tracked quiz server: %s)", sfqErr)
		} else {
			sfqNote = "\n  Started tracked quiz session in browser..."
		}
		if syncErr != nil {
			sfqNote += fmt.Sprintf("\n  Session sync warning: %s", syncErr)
		} else {
			sfqNote += fmt.Sprintf("\n  Imported sessions: %d  Pending tracked quizzes: %d", report.ImportedSessions, report.PendingQuizzes)
		}
		stream <- aiStreamEvent{part: fmt.Sprintf("Quiz saved: %s\n  Quiz ID: %s\n  Title: %s\n  Questions: %d%s", path, quizID, q.Title, len(q.Sections), sfqNote), done: true}
	}()
	return waitForAIStreamCmd(stream)
}

// loadUsageCmd loads usage totals using the current pricing config.
func loadUsageCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		totals, err := state.LoadUsageTotalsWithPricing(cfg, state.UsageFilter{})
		return usageLoadedMsg{totals: totals, cfg: cfg, err: err}
	}
}

// loadLedgerCmd loads the usage ledger for display in the ledger view.
func loadLedgerCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ledger, err := state.LoadUsageLedger()
		return usageLedgerLoadedMsg{ledger: ledger, err: err}
	}
}

// loadKnowledgeCmd loads the learned sections and components for the Knowledge tab.
func loadKnowledgeCmd() tea.Cmd {
	return func() tea.Msg {
		sections, sectionErr := state.LoadSectionIndex()
		if sectionErr != nil {
			return knowledgeLoadedMsg{err: sectionErr}
		}
		components, componentErr := state.LoadComponentIndex()
		if componentErr != nil {
			return knowledgeLoadedMsg{err: componentErr}
		}
		return knowledgeLoadedMsg{sections: sections, components: components}
	}
}

// syncTrackedSessionsCmd imports unseen tracked sfq sessions into quiz results
// and section/component history.
func syncTrackedSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		report, err := tracking.SyncTrackedQuizSessions()
		return trackedSyncDoneMsg{report: report, err: err}
	}
}

// runExportKnowledgeCmd writes a shareable knowledge dataset JSON export.
func runExportKnowledgeCmd(outputPath, class string, includeEmbeddings bool) tea.Cmd {
	return func() tea.Msg {
		result, err := state.ExportKnowledgeDataset(outputPath, state.KnowledgeExportOptions{
			Class:             class,
			IncludeEmbeddings: includeEmbeddings,
		})
		if err != nil {
			return workflowDoneMsg{err: err}
		}

		summary := fmt.Sprintf("Exported knowledge dataset\nPath: %s\nSections: %d\nComponents: %d\nEmbeddings: %s", result.OutputPath, result.Sections, result.Components, ternaryText(result.IncludeEmbeddings, "included", "excluded"))
		if strings.TrimSpace(result.Class) != "" {
			summary += "\nClass filter: " + result.Class
		}
		return workflowDoneMsg{summary: summary}
	}
}

func ternaryText(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}
