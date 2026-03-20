package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/studyforge/study-agent/internal/chat"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/ingestion"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
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

// runGenerateCmd generates a new quiz for the given class, streaming agent
// tool-call events back to the TUI as aiStreamMsgs so the workflow overlay
// can display live progress.
func runGenerateCmd(class string, tags []string, orc *orchestrator.Orchestrator, cfg *config.Config) tea.Cmd {
	stream := make(chan aiStreamEvent, 32)
	go func() {
		defer close(stream)
		q, path, err := quiz.GenerateStream(class, tags, orc.Provider, cfg, func(e quiz.ProgressEvent) {
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
		sfqPath := strings.TrimSuffix(path, ".yaml") + ".sfq"
		sfqErr := sfq.Generate(sfqPath)
		var sfqNote string
		if sfqErr != nil {
			sfqNote = fmt.Sprintf("\n  (could not launch quiz: %s)", sfqErr)
		} else {
			sfqNote = "\n  Opening quiz in browser..."
		}
		stream <- aiStreamEvent{
			part: fmt.Sprintf("Quiz saved: %s\n  Title: %s\n  Questions: %d%s", path, q.Title, len(q.Sections), sfqNote),
			done: true,
		}
	}()
	return waitForAIStreamCmd(stream)
}

// runAdaptCmd generates an adaptive quiz for the given class based on
// prior performance results, streaming agent tool-call events back to the TUI.
func runAdaptCmd(class string, orc *orchestrator.Orchestrator, cfg *config.Config) tea.Cmd {
	stream := make(chan aiStreamEvent, 32)
	go func() {
		defer close(stream)
		q, path, err := quiz.AdaptStream(class, orc.Provider, cfg, func(e quiz.ProgressEvent) {
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
		sfqPath := strings.TrimSuffix(path, ".yaml") + ".sfq"
		sfqErr := sfq.Generate(sfqPath)
		var sfqNote string
		if sfqErr != nil {
			sfqNote = fmt.Sprintf("\n  (could not launch quiz: %s)", sfqErr)
		} else {
			sfqNote = "\n  Opening quiz in browser..."
		}
		stream <- aiStreamEvent{
			part: fmt.Sprintf("Adaptive quiz saved: %s\n  Title: %s\n  Questions: %d%s", path, q.Title, len(q.Sections), sfqNote),
			done: true,
		}
	}()
	return waitForAIStreamCmd(stream)
}
