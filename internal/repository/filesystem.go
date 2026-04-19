package repository

import (
	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/state"
)

// FilesystemStore is the default repository implementation backed by existing
// state package persistence under ~/.study-forge-ai.
type FilesystemStore struct{}

func NewFilesystemStore() *FilesystemStore {
	return &FilesystemStore{}
}

func (s *FilesystemStore) Knowledge() KnowledgeRepository { return filesystemKnowledgeRepo{} }
func (s *FilesystemStore) Notes() NotesRepository         { return filesystemNotesRepo{} }
func (s *FilesystemStore) Usage() UsageRepository         { return filesystemUsageRepo{} }
func (s *FilesystemStore) QuizAttempts() QuizAttemptRepository {
	return filesystemQuizAttemptRepo{}
}
func (s *FilesystemStore) Maintenance() MaintenanceRepository { return filesystemMaintenanceRepo{} }
func (s *FilesystemStore) Classes() ClassRepository           { return filesystemClassRepo{} }
func (s *FilesystemStore) Config() ConfigRepository           { return filesystemConfigRepo{} }
func (s *FilesystemStore) Export() ExportRepository           { return filesystemExportRepo{} }

type filesystemKnowledgeRepo struct{}

func (filesystemKnowledgeRepo) LoadSectionIndex() (*state.SectionIndex, error) {
	return state.LoadSectionIndex()
}

func (filesystemKnowledgeRepo) SaveSectionIndex(idx *state.SectionIndex) error {
	return state.SaveSectionIndex(idx)
}

func (filesystemKnowledgeRepo) LoadComponentIndex() (*state.ComponentIndex, error) {
	return state.LoadComponentIndex()
}

func (filesystemKnowledgeRepo) SaveComponentIndex(idx *state.ComponentIndex) error {
	return state.SaveComponentIndex(idx)
}

func (filesystemKnowledgeRepo) SearchSectionsByEmbedding(idx *state.SectionIndex, embedding []float64, topK int) []state.Section {
	return state.SearchSectionsByEmbedding(idx, embedding, topK)
}

func (filesystemKnowledgeRepo) SearchComponentsByEmbedding(idx *state.ComponentIndex, embedding []float64, topK int) []state.Component {
	return state.SearchComponentsByEmbedding(idx, embedding, topK)
}

type filesystemNotesRepo struct{}

func (filesystemNotesRepo) LoadNotesIndex() (*state.NotesIndex, error) {
	return state.LoadNotesIndex()
}

func (filesystemNotesRepo) SaveNotesIndex(idx *state.NotesIndex) error {
	return state.SaveNotesIndex(idx)
}

type filesystemUsageRepo struct{}

func (filesystemUsageRepo) AppendUsageEvent(event state.UsageEvent) error {
	return state.AppendUsageEvent(event)
}

func (filesystemUsageRepo) LoadUsageLedger() (*state.UsageLedger, error) {
	return state.LoadUsageLedger()
}

func (filesystemUsageRepo) LoadUsageTotalsWithPricing(cfg *config.Config, filter state.UsageFilter) (*state.UsageTotals, error) {
	return state.LoadUsageTotalsWithPricing(cfg, filter)
}

type filesystemQuizAttemptRepo struct{}

func (filesystemQuizAttemptRepo) LoadTrackedQuizCache() (*state.TrackedQuizCache, error) {
	return state.LoadTrackedQuizCache()
}

func (filesystemQuizAttemptRepo) SaveTrackedQuizCache(cache *state.TrackedQuizCache) error {
	return state.SaveTrackedQuizCache(cache)
}

func (filesystemQuizAttemptRepo) RegisterTrackedQuiz(class, quizPath, sfqPath string) (*state.TrackedQuizRecord, error) {
	return state.RegisterTrackedQuiz(class, quizPath, sfqPath)
}

func (filesystemQuizAttemptRepo) SaveQuizResults(results *state.QuizResults, class, quizID string) error {
	return state.SaveQuizResults(results, class, quizID)
}

func (filesystemQuizAttemptRepo) AppendQuizQuestionHistory(class string, quiz state.Quiz, results state.QuizResults) error {
	return state.AppendQuizQuestionHistory(class, quiz, results)
}

type filesystemMaintenanceRepo struct{}

func (filesystemMaintenanceRepo) ClearIngestedData() error {
	return state.ClearIngestedData()
}

type filesystemClassRepo struct{}

func (filesystemClassRepo) ListClasses() ([]string, error) { return classpkg.List() }
func (filesystemClassRepo) CreateClass(name string) error  { return classpkg.Create(name) }

func (filesystemClassRepo) LoadSyllabus(name string) (*classpkg.Syllabus, error) {
	return classpkg.LoadSyllabus(name)
}

func (filesystemClassRepo) LoadRules(name string) (*classpkg.Rules, error) {
	return classpkg.LoadRules(name)
}

func (filesystemClassRepo) LoadContext(name string) (*classpkg.Context, error) {
	return classpkg.LoadContext(name)
}

func (filesystemClassRepo) SaveContext(name string, ctx *classpkg.Context) error {
	return classpkg.SaveContext(name, ctx)
}

func (filesystemClassRepo) LoadProfileContextText(className, profileKind string) (string, error) {
	return classpkg.LoadProfileContextText(className, profileKind)
}

func (filesystemClassRepo) SaveProfileContextText(className, profileKind, text string) error {
	return classpkg.SaveProfileContextText(className, profileKind, text)
}

func (filesystemClassRepo) LoadNoteRoster(name string) (*classpkg.NoteRoster, error) {
	return classpkg.LoadNoteRoster(name)
}

func (filesystemClassRepo) SaveNoteRoster(name string, roster *classpkg.NoteRoster) error {
	return classpkg.SaveNoteRoster(name, roster)
}

func (filesystemClassRepo) UpsertNoteRosterEntry(name string, entry classpkg.NoteRosterEntry) (*classpkg.NoteRoster, error) {
	return classpkg.UpsertNoteRosterEntry(name, entry)
}

func (filesystemClassRepo) RemoveNoteRosterEntry(name, label string) (*classpkg.NoteRoster, error) {
	return classpkg.RemoveNoteRosterEntry(name, label)
}

func (filesystemClassRepo) ReorderNoteRosterEntries(name string, labels []string) (*classpkg.NoteRoster, error) {
	return classpkg.ReorderNoteRosterEntries(name, labels)
}

func (filesystemClassRepo) LoadCoverageScope(name, kind string) (*classpkg.CoverageScope, error) {
	return classpkg.LoadCoverageScope(name, kind)
}

func (filesystemClassRepo) SaveCoverageScope(name, kind string, scope *classpkg.CoverageScope) error {
	return classpkg.SaveCoverageScope(name, kind, scope)
}

func (filesystemClassRepo) ContextProfiles() []classpkg.ContextProfile {
	return classpkg.ContextProfiles()
}

type filesystemConfigRepo struct{}

func (filesystemConfigRepo) LoadConfig() (*config.Config, error) { return config.Load() }
func (filesystemConfigRepo) SaveConfig(cfg *config.Config) error { return config.Save(cfg) }

func (filesystemConfigRepo) EnsureInitialized() (*config.InitResult, error) {
	return config.EnsureInitialized()
}

type filesystemExportRepo struct{}

func (filesystemExportRepo) ExportKnowledgeDataset(outputPath string, opts state.KnowledgeExportOptions) (state.KnowledgeExportResult, error) {
	return state.ExportKnowledgeDataset(outputPath, opts)
}
