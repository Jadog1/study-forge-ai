package repository

import (
	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/state"
)

// Store groups all storage repositories used by application services.
type Store interface {
	Knowledge() KnowledgeRepository
	Notes() NotesRepository
	Chat() ChatRepository
	Usage() UsageRepository
	QuizAttempts() QuizAttemptRepository
	Maintenance() MaintenanceRepository
	Classes() ClassRepository
	Config() ConfigRepository
	Export() ExportRepository
}

// KnowledgeRepository persists section/component indices.
type KnowledgeRepository interface {
	LoadSectionIndex() (*state.SectionIndex, error)
	SaveSectionIndex(idx *state.SectionIndex) error
	LoadComponentIndex() (*state.ComponentIndex, error)
	SaveComponentIndex(idx *state.ComponentIndex) error
	SearchSectionsByEmbedding(idx *state.SectionIndex, embedding []float64, topK int) []state.Section
	SearchComponentsByEmbedding(idx *state.ComponentIndex, embedding []float64, topK int) []state.Component
}

// NotesRepository persists processed note indexes.
type NotesRepository interface {
	LoadNotesIndex() (*state.NotesIndex, error)
	SaveNotesIndex(idx *state.NotesIndex) error
}

// ChatRepository persists the latest chat session for resume-on-refresh UX.
type ChatRepository interface {
	LoadLatestChatSession() (*state.ChatSession, error)
	SaveLatestChatSession(session *state.ChatSession) error
	ClearLatestChatSession() error
}

// UsageRepository persists usage ledgers and computed totals.
type UsageRepository interface {
	AppendUsageEvent(event state.UsageEvent) error
	LoadUsageLedger() (*state.UsageLedger, error)
	LoadUsageTotalsWithPricing(cfg *config.Config, filter state.UsageFilter) (*state.UsageTotals, error)
}

// QuizAttemptRepository persists tracked quiz cache and session results.
type QuizAttemptRepository interface {
	LoadTrackedQuizCache() (*state.TrackedQuizCache, error)
	SaveTrackedQuizCache(cache *state.TrackedQuizCache) error
	RegisterTrackedQuiz(class, quizPath, sfqPath string) (*state.TrackedQuizRecord, error)
	SaveQuizResults(results *state.QuizResults, class, quizID string) error
	AppendQuizQuestionHistory(class string, quiz state.Quiz, results state.QuizResults) error
}

// MaintenanceRepository groups destructive/reset operations.
type MaintenanceRepository interface {
	ClearIngestedData() error
}

// ClassRepository persists class metadata, context, roster, and coverage.
type ClassRepository interface {
	ListClasses() ([]string, error)
	CreateClass(name string) error
	LoadSyllabus(name string) (*classpkg.Syllabus, error)
	LoadRules(name string) (*classpkg.Rules, error)
	LoadContext(name string) (*classpkg.Context, error)
	SaveContext(name string, ctx *classpkg.Context) error
	LoadProfileContextText(className, profileKind string) (string, error)
	SaveProfileContextText(className, profileKind, text string) error
	LoadNoteRoster(name string) (*classpkg.NoteRoster, error)
	SaveNoteRoster(name string, roster *classpkg.NoteRoster) error
	UpsertNoteRosterEntry(name string, entry classpkg.NoteRosterEntry) (*classpkg.NoteRoster, error)
	RemoveNoteRosterEntry(name, label string) (*classpkg.NoteRoster, error)
	ReorderNoteRosterEntries(name string, labels []string) (*classpkg.NoteRoster, error)
	LoadCoverageScope(name, kind string) (*classpkg.CoverageScope, error)
	SaveCoverageScope(name, kind string, scope *classpkg.CoverageScope) error
	ContextProfiles() []classpkg.ContextProfile
}

// ConfigRepository persists application configuration.
type ConfigRepository interface {
	LoadConfig() (*config.Config, error)
	SaveConfig(cfg *config.Config) error
	EnsureInitialized() (*config.InitResult, error)
}

// ExportRepository handles dataset export persistence.
type ExportRepository interface {
	ExportKnowledgeDataset(outputPath string, opts state.KnowledgeExportOptions) (state.KnowledgeExportResult, error)
}
