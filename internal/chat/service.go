package chat

import (
	"strings"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/repository"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/plugins"
)

// ChatMessage represents a single message in the conversation.
type ChatMessage = state.ChatMessage

// Mode controls how the chat agent guides learning interactions.
type Mode string

const (
	ModeStandard    Mode = "standard"
	ModeSocratic    Mode = "socratic"
	ModeExplainBack Mode = "explain_back"
)

// NormalizeMode converts arbitrary user input into a supported mode.
func NormalizeMode(raw string) Mode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ModeSocratic):
		return ModeSocratic
	case string(ModeExplainBack), "explain-back", "explain back":
		return ModeExplainBack
	default:
		return ModeStandard
	}
}

// Service provides chat operations with injected dependencies.
type Service struct {
	store repository.Store
}

// NewService constructs a chat service.
func NewService(store repository.Store) *Service {
	if store == nil {
		store = repository.NewFilesystemStore()
	}
	return &Service{store: store}
}

// Ask sends a prompt and returns a single response.
func (s *Service) Ask(provider plugins.AIProvider, cfg *config.Config, className, prompt string) (string, error) {
	return s.AskWithMode(provider, cfg, className, prompt, ModeStandard)
}

// AskWithMode sends a prompt and returns a single response with mode-aware instructions.
func (s *Service) AskWithMode(provider plugins.AIProvider, cfg *config.Config, className, prompt string, mode Mode) (string, error) {
	return askWithStore(provider, cfg, className, prompt, NormalizeMode(string(mode)), s.store, nil)
}

// AskStream sends a prompt and streams response events.
func (s *Service) AskStream(provider plugins.AIProvider, cfg *config.Config, className, prompt string, onEvent func(StreamEvent) error) error {
	return s.AskStreamWithMode(provider, cfg, className, prompt, ModeStandard, onEvent)
}

// AskStreamWithMode sends a prompt and streams response events with mode-aware instructions.
func (s *Service) AskStreamWithMode(provider plugins.AIProvider, cfg *config.Config, className, prompt string, mode Mode, onEvent func(StreamEvent) error) error {
	return askStreamWithStore(provider, cfg, className, prompt, NormalizeMode(string(mode)), s.store, nil, onEvent)
}

// AskStreamWithHistory sends a prompt with conversation history and streams response events.
func (s *Service) AskStreamWithHistory(provider plugins.AIProvider, cfg *config.Config, className, prompt string, mode Mode, history []ChatMessage, onEvent func(StreamEvent) error) error {
	return askStreamWithStore(provider, cfg, className, prompt, NormalizeMode(string(mode)), s.store, history, onEvent)
}
