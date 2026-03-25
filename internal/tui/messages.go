package tui

import (
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/internal/tracking"
)

// aiStreamMsg carries a streaming AI response event from the provider.
type aiStreamMsg struct {
	stream      <-chan aiStreamEvent
	part        string
	actionLabel string
	actionInfo  string
	actionDone  bool
	err         error
	done        bool
}

// workflowDoneMsg signals that an ingest / generate / adapt operation finished.
type workflowDoneMsg struct {
	summary string
	err     error
}

// usageLoadedMsg carries loaded usage totals for the Usage tab.
type usageLoadedMsg struct {
	totals *state.UsageTotals
	cfg    *config.Config
	err    error
}

// usageLedgerLoadedMsg carries loaded usage ledger for the Usage tab ledger view.
type usageLedgerLoadedMsg struct {
	ledger *state.UsageLedger
	err    error
}

// knowledgeLoadedMsg carries loaded section/component knowledge for the Knowledge tab.
type knowledgeLoadedMsg struct {
	sections   *state.SectionIndex
	components *state.ComponentIndex
	err        error
}

// quizDashboardLoadedMsg carries quiz dashboard data for the dashboard tab.
type quizDashboardLoadedMsg struct {
	snapshot *quizDashboardSnapshot
	err      error
}

// trackedSyncDoneMsg carries completion status for manual/automatic tracked-session sync.
type trackedSyncDoneMsg struct {
	report tracking.SyncReport
	err    error
}
