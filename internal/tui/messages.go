package tui

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

// sfqDoneMsg carries the result of an SFQ plugin search.
// autoSFQ is true when this was a background lookup triggered from the chat tab.
type sfqDoneMsg struct {
	text    string
	err     error
	autoSFQ bool
}

// workflowDoneMsg signals that an ingest / generate / adapt operation finished.
type workflowDoneMsg struct {
	summary string
	err     error
}
