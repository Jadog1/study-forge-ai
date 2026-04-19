package quiz

import (
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/repository"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/plugins"
)

// Service provides quiz generation operations with injected dependencies.
type Service struct {
	store repository.Store
}

// NewService constructs a quiz service.
func NewService(store repository.Store) *Service {
	return &Service{store: resolveStore(store)}
}

// NewQuiz generates and saves a quiz.
func (s *Service) NewQuiz(class string, opts QuizOptions, provider plugins.AIProvider, cfg *config.Config) (*state.Quiz, string, error) {
	return NewQuizWithStore(class, opts, provider, cfg, s.store)
}

// NewQuizStream generates and saves a quiz while emitting progress.
func (s *Service) NewQuizStream(class string, opts QuizOptions, provider plugins.AIProvider, cfg *config.Config, onProgress func(ProgressEvent)) (*state.Quiz, string, error) {
	return NewQuizStreamWithStore(class, opts, provider, cfg, s.store, onProgress)
}
