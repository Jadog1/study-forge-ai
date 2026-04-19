package search

import (
	"testing"

	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/repository"
	"github.com/studyforge/study-agent/internal/state"
)

type testStore struct {
	knowledge repository.KnowledgeRepository
	notes     repository.NotesRepository
}

func (s testStore) Knowledge() repository.KnowledgeRepository      { return s.knowledge }
func (s testStore) Notes() repository.NotesRepository              { return s.notes }
func (s testStore) Usage() repository.UsageRepository              { return noopUsageRepo{} }
func (s testStore) QuizAttempts() repository.QuizAttemptRepository { return noopQuizRepo{} }
func (s testStore) Maintenance() repository.MaintenanceRepository  { return noopMaintenanceRepo{} }
func (s testStore) Classes() repository.ClassRepository            { return noopClassRepo{} }
func (s testStore) Config() repository.ConfigRepository            { return noopConfigRepo{} }
func (s testStore) Export() repository.ExportRepository            { return noopExportRepo{} }

type testKnowledgeRepo struct {
	sections   state.SectionIndex
	components state.ComponentIndex
}

func (r testKnowledgeRepo) LoadSectionIndex() (*state.SectionIndex, error) { return &r.sections, nil }
func (r testKnowledgeRepo) SaveSectionIndex(idx *state.SectionIndex) error { return nil }
func (r testKnowledgeRepo) LoadComponentIndex() (*state.ComponentIndex, error) {
	return &r.components, nil
}
func (r testKnowledgeRepo) SaveComponentIndex(idx *state.ComponentIndex) error { return nil }
func (r testKnowledgeRepo) SearchSectionsByEmbedding(idx *state.SectionIndex, embedding []float64, topK int) []state.Section {
	return nil
}
func (r testKnowledgeRepo) SearchComponentsByEmbedding(idx *state.ComponentIndex, embedding []float64, topK int) []state.Component {
	return nil
}

type testNotesRepo struct{}

func (testNotesRepo) LoadNotesIndex() (*state.NotesIndex, error) { return &state.NotesIndex{}, nil }
func (testNotesRepo) SaveNotesIndex(idx *state.NotesIndex) error { return nil }

type noopUsageRepo struct{}

func (noopUsageRepo) AppendUsageEvent(event state.UsageEvent) error { return nil }
func (noopUsageRepo) LoadUsageLedger() (*state.UsageLedger, error)  { return &state.UsageLedger{}, nil }
func (noopUsageRepo) LoadUsageTotalsWithPricing(cfg *config.Config, filter state.UsageFilter) (*state.UsageTotals, error) {
	return &state.UsageTotals{}, nil
}

type noopQuizRepo struct{}

func (noopQuizRepo) LoadTrackedQuizCache() (*state.TrackedQuizCache, error) {
	return &state.TrackedQuizCache{}, nil
}
func (noopQuizRepo) SaveTrackedQuizCache(cache *state.TrackedQuizCache) error { return nil }
func (noopQuizRepo) RegisterTrackedQuiz(class, quizPath, sfqPath string) (*state.TrackedQuizRecord, error) {
	return &state.TrackedQuizRecord{}, nil
}
func (noopQuizRepo) SaveQuizResults(results *state.QuizResults, class, quizID string) error {
	return nil
}
func (noopQuizRepo) AppendQuizQuestionHistory(class string, quiz state.Quiz, results state.QuizResults) error {
	return nil
}

type noopMaintenanceRepo struct{}

func (noopMaintenanceRepo) ClearIngestedData() error { return nil }

type noopClassRepo struct{}

func (noopClassRepo) ListClasses() ([]string, error) { return nil, nil }
func (noopClassRepo) CreateClass(name string) error  { return nil }
func (noopClassRepo) LoadSyllabus(name string) (*classpkg.Syllabus, error) {
	return nil, nil
}
func (noopClassRepo) LoadRules(name string) (*classpkg.Rules, error) { return nil, nil }
func (noopClassRepo) LoadContext(name string) (*classpkg.Context, error) {
	return nil, nil
}
func (noopClassRepo) SaveContext(name string, ctx *classpkg.Context) error { return nil }
func (noopClassRepo) LoadProfileContextText(className, profileKind string) (string, error) {
	return "", nil
}
func (noopClassRepo) SaveProfileContextText(className, profileKind, text string) error { return nil }
func (noopClassRepo) LoadNoteRoster(name string) (*classpkg.NoteRoster, error) {
	return nil, nil
}
func (noopClassRepo) SaveNoteRoster(name string, roster *classpkg.NoteRoster) error { return nil }
func (noopClassRepo) UpsertNoteRosterEntry(name string, entry classpkg.NoteRosterEntry) (*classpkg.NoteRoster, error) {
	return nil, nil
}
func (noopClassRepo) RemoveNoteRosterEntry(name, label string) (*classpkg.NoteRoster, error) {
	return nil, nil
}
func (noopClassRepo) ReorderNoteRosterEntries(name string, labels []string) (*classpkg.NoteRoster, error) {
	return nil, nil
}
func (noopClassRepo) LoadCoverageScope(name, kind string) (*classpkg.CoverageScope, error) {
	return nil, nil
}
func (noopClassRepo) SaveCoverageScope(name, kind string, scope *classpkg.CoverageScope) error {
	return nil
}
func (noopClassRepo) ContextProfiles() []classpkg.ContextProfile { return nil }

type noopConfigRepo struct{}

func (noopConfigRepo) LoadConfig() (*config.Config, error) { return config.DefaultConfig(), nil }
func (noopConfigRepo) SaveConfig(cfg *config.Config) error { return nil }
func (noopConfigRepo) EnsureInitialized() (*config.InitResult, error) {
	return &config.InitResult{}, nil
}

type noopExportRepo struct{}

func (noopExportRepo) ExportKnowledgeDataset(outputPath string, opts state.KnowledgeExportOptions) (state.KnowledgeExportResult, error) {
	return state.KnowledgeExportResult{}, nil
}

func buildKnowledgeStore() repository.Store {
	return testStore{
		knowledge: testKnowledgeRepo{
			sections: state.SectionIndex{Sections: []state.Section{
				{ID: "sec-week10", Class: "biology", Title: "Week 10 Metabolism", SourcePaths: []string{"notes/week10.md"}},
				{ID: "sec-week9", Class: "biology", Title: "Week 9 DNA", SourcePaths: []string{"notes/week9.md"}},
			}},
			components: state.ComponentIndex{Components: []state.Component{
				{ID: "cmp-week10-a", SectionID: "sec-week10", Class: "biology", Kind: "concept", Content: "ATP cycle", SourcePaths: []string{"slides/week10/cell-cycle.md"}},
				{ID: "cmp-week9-a", SectionID: "sec-week9", Class: "biology", Kind: "definition", Content: "DNA replication", SourcePaths: []string{"slides/week9/dna.md"}},
			}},
		},
		notes: testNotesRepo{},
	}
}

func TestBySourcePathLooseWithStore_MatchesWeekTokenVariants(t *testing.T) {
	results, err := BySourcePathLooseWithStore("week 10", "biology", "", 10, buildKnowledgeStore())
	if err != nil {
		t.Fatalf("BySourcePathLooseWithStore returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (section + component), got %d", len(results))
	}
}

func TestBySourcePathLooseWithStore_KindFilter(t *testing.T) {
	results, err := BySourcePathLooseWithStore("week10", "biology", "section", 10, buildKnowledgeStore())
	if err != nil {
		t.Fatalf("BySourcePathLooseWithStore returned error: %v", err)
	}
	if len(results) != 1 || results[0].Kind != "section" {
		t.Fatalf("expected only one section result, got %#v", results)
	}
}

func TestBySectionIDWithStore_ReturnsSectionAndComponents(t *testing.T) {
	results, err := BySectionIDWithStore("sec-week10", "biology", 10, buildKnowledgeStore())
	if err != nil {
		t.Fatalf("BySectionIDWithStore returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected section plus one component, got %d", len(results))
	}
	if results[0].Kind != "section" {
		t.Fatalf("expected first result to be section, got %q", results[0].Kind)
	}
}

func TestByComponentIDWithStore_ReturnsSingleComponent(t *testing.T) {
	results, err := ByComponentIDWithStore("cmp-week10-a", "biology", buildKnowledgeStore())
	if err != nil {
		t.Fatalf("ByComponentIDWithStore returned error: %v", err)
	}
	if len(results) != 1 || results[0].Kind != "component" {
		t.Fatalf("expected one component result, got %#v", results)
	}
}
