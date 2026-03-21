package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/state"
)

// UsageTab displays AI token usage and cost breakdown loaded from disk.
type UsageTab struct {
	totals         *state.UsageTotals
	baseCfg        *config.Config
	err            error
	loaded         bool
	loading        bool
	width          int
	filter         usageTimeFilter
	viewMode       usageViewMode
	ledger         *state.UsageLedger
	ledgerLoaded   bool
	ledgerErr      error
	scrollPos      int
	filteredEvents []state.UsageEvent
	cachedLines    []string
	cachedWidth    int
	lastHeight     int
}

type usageViewMode int

const (
	usageViewSummary usageViewMode = iota
	usageViewLedger
)

type usageTimeFilter int

const (
	usageFilterAll usageTimeFilter = iota
	usageFilterLast24Hours
	usageFilterLast7Days
	usageFilterLast30Days
)

func newUsageTab() UsageTab {
	return UsageTab{}
}

func (u UsageTab) resize(width int) UsageTab {
	// Invalidate cache if width changed significantly
	if u.width != width && u.cachedWidth != width {
		u.cachedWidth = width
		u.cachedLines = nil
	}
	u.width = width
	return u
}

func (u UsageTab) startLoading() UsageTab {
	u.loading = true
	return u
}

func (u UsageTab) receive(totals *state.UsageTotals, err error) UsageTab {
	u.totals = totals
	u.err = err
	u.loaded = true
	u.loading = false
	return u
}

func (u UsageTab) receiveWithConfig(totals *state.UsageTotals, cfg *config.Config, err error) UsageTab {
	u = u.receive(totals, err)
	u.baseCfg = cfg
	return u.applyFilter()
}

func (u UsageTab) update(msg tea.Msg) (UsageTab, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "r":
			u = u.startLoading()
			return u, loadUsageCmd(u.baseCfg)
		case "f":
			u.filter = u.filter.next()
			u = u.applyFilter()
			u = u.filterLedger()
			return u, nil
		case "F":
			u.filter = u.filter.prev()
			u = u.applyFilter()
			u = u.filterLedger()
			return u, nil
		case "l", "L":
			// Toggle between summary and ledger view
			if u.viewMode == usageViewSummary {
				u.viewMode = usageViewLedger
				if !u.ledgerLoaded && u.ledgerErr == nil {
					return u, loadLedgerCmd(u.baseCfg)
				}
			} else {
				u.viewMode = usageViewSummary
			}
			u.scrollPos = 0
			return u, nil
		case "up", "k":
			if u.viewMode == usageViewLedger {
				u.scrollPos = clamp(u.scrollPos-1, 0, u.ledgerMaxScroll())
			}
			return u, nil
		case "down", "j":
			if u.viewMode == usageViewLedger {
				u.scrollPos = clamp(u.scrollPos+1, 0, u.ledgerMaxScroll())
			}
			return u, nil
		case "home":
			if u.viewMode == usageViewLedger {
				u.scrollPos = 0
			}
			return u, nil
		case "end":
			if u.viewMode == usageViewLedger {
				u.scrollPos = u.ledgerMaxScroll()
			}
			return u, nil
		case "pgup":
			if u.viewMode == usageViewLedger {
				u.scrollPos = clamp(u.scrollPos-10, 0, u.ledgerMaxScroll())
			}
			return u, nil
		case "pgdn":
			if u.viewMode == usageViewLedger {
				u.scrollPos = clamp(u.scrollPos+10, 0, u.ledgerMaxScroll())
			}
			return u, nil
		}
	}
	if msg, ok := msg.(usageLedgerLoadedMsg); ok {
		u.ledger = msg.ledger
		u.ledgerErr = msg.err
		u.ledgerLoaded = true
		u = u.filterLedger()
		u.scrollPos = 0
	}
	return u, nil
}

func (u UsageTab) view(width, height int, cfg *config.Config) string {
	if !u.loaded && !u.loading {
		return dimStyle.Render("Loading usage data…")
	}
	if u.loading && !u.loaded {
		return dimStyle.Render("Loading usage data…")
	}
	if u.err != nil {
		return errorStyle.Render("Error loading usage data: " + u.err.Error())
	}
	if u.totals == nil || u.totals.TotalTokens == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			dimStyle.Render("No usage recorded yet."),
			dimStyle.Render("Run 'sfa ingest' to start tracking usage."),
			"",
			dimStyle.Render("Press r to refresh  •  f/F change time filter  •  l to view ledger."),
		)
	}

	if u.viewMode == usageViewLedger {
		return u.viewLedger(width, height, cfg)
	}
	return u.viewSummary(width, cfg)
}

func (u UsageTab) applyFilter() UsageTab {
	if u.baseCfg == nil {
		return u
	}
	totals, err := state.LoadUsageTotalsWithPricing(u.baseCfg, u.filter.usageFilter())
	u.totals = totals
	u.err = err
	return u
}

func (u UsageTab) filterLedger() UsageTab {
	if u.ledger == nil {
		u.filteredEvents = nil
		u.cachedLines = nil
		return u
	}
	// Filter events based on current filter
	filter := u.filter.usageFilter()
	var filtered []state.UsageEvent
	for _, event := range u.ledger.Events {
		if usageEventMatchesFilterLocal(event, filter) {
			filtered = append(filtered, event)
		}
	}
	// Sort by timestamp descending (newest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	u.filteredEvents = filtered
	
	// Pre-compute all formatted lines
	u.cachedLines = make([]string, 0)
	for i, event := range filtered {
		line := u.formatLedgerEvent(event, 200, u.baseCfg) // Use large width, will clip in view
		u.cachedLines = append(u.cachedLines, line)
		if i < len(filtered)-1 {
			u.cachedLines = append(u.cachedLines, "")
		}
	}
	u.scrollPos = 0
	return u
}

func (u UsageTab) ledgerEventMatchesFilter(event state.UsageEvent, filter state.UsageFilter) bool {
	if filter.CreatedAfter != nil && event.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}
	if filter.CreatedBefore != nil && event.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}
	return true
}

func (u UsageTab) ledgerMaxScroll() int {
	if len(u.cachedLines) == 0 {
		return 0
	}
	// Return max scroll position (last line index)
	return len(u.cachedLines) - 1
}

func (u UsageTab) viewSummary(width int, cfg *config.Config) string {
	var b strings.Builder
	filterLabel := u.filter.label()
	if u.filter != usageFilterAll {
		filterLabel = selectedStyle.Render(filterLabel)
	}

	// Summary section
	costStr := dimStyle.Render("(no pricing — run sfa pricing detect)")
	if u.totals.TotalCostUSD > 0 {
		costStr = successStyle.Render(fmt.Sprintf("$%.6f", u.totals.TotalCostUSD))
	}
	lastUpdated := dimStyle.Render("—")
	if !u.totals.UpdatedAt.IsZero() {
		lastUpdated = u.totals.UpdatedAt.Format("2006-01-02 15:04 UTC")
	}
	summaryBody := strings.Join([]string{
		fmt.Sprintf("  %s  %s", labelStyle.Render("Time filter:  "), filterLabel),
		fmt.Sprintf("  %s  %s", labelStyle.Render("Input tokens: "), formatTokenCount(u.totals.TotalInputTokens)),
		fmt.Sprintf("  %s  %s", labelStyle.Render("Output tokens:"), formatTokenCount(u.totals.TotalOutputTokens)),
		fmt.Sprintf("  %s  %s", labelStyle.Render("Total tokens: "), formatTokenCount(u.totals.TotalTokens)),
		fmt.Sprintf("  %s  %s", labelStyle.Render("Total cost:   "), costStr),
		fmt.Sprintf("  %s  %s", labelStyle.Render("Last updated: "), lastUpdated),
	}, "\n")
	b.WriteString(renderSection("Summary", summaryBody, width))

	// Per-model breakdown
	if len(u.totals.ByModel) > 0 {
		keys := make([]string, 0, len(u.totals.ByModel))
		for k := range u.totals.ByModel {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var rows strings.Builder
		for _, k := range keys {
			t := u.totals.ByModel[k]

			// Extract just the model name (strip "provider:" prefix) for pricing lookup.
			modelName := k
			if idx := strings.Index(k, ":"); idx >= 0 {
				modelName = k[idx+1:]
			}

			costPart := ""
			if t.CostUSD > 0 {
				costPart = fmt.Sprintf("  $%.6f", t.CostUSD)
			} else if cfg != nil {
				if _, _, found := config.LookupModelPrice(modelName, cfg); !found {
					costPart = "  " + dimStyle.Render("(no price)")
				}
			}

			row := fmt.Sprintf("%-28s  in:%-8s  out:%-8s  total:%-9s%s",
				truncateWidth(k, 28),
				formatTokenCount(t.InputTokens),
				formatTokenCount(t.OutputTokens),
				formatTokenCount(t.TotalTokens),
				costPart,
			)
			rows.WriteString(row + "\n")
		}
		b.WriteString(renderSection("Per-Model Breakdown", strings.TrimRight(rows.String(), "\n"), width))
	}

	// Pricing hint for unpriced models.
	if cfg != nil && len(u.totals.ByModel) > 0 {
		var unknown []string
		for k := range u.totals.ByModel {
			modelName := k
			if idx := strings.Index(k, ":"); idx >= 0 {
				modelName = k[idx+1:]
			}
			if _, _, found := config.LookupModelPrice(modelName, cfg); !found {
				unknown = append(unknown, modelName)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			hint := "Set prices with: sfa pricing set <model> <input-per-million> <output-per-million>\n" +
				"Run: sfa pricing detect\n\n" +
				"Unpriced: " + strings.Join(unknown, ", ")
			b.WriteString(renderSection("Pricing Needed", warnStyle.Render(hint), width))
		}
	}

	b.WriteString("\n" + dimStyle.Render("Press r to refresh  •  f/F change time filter  •  l for ledger view"))
	return b.String()
}

func (u UsageTab) viewLedger(width, height int, cfg *config.Config) string {
	if !u.ledgerLoaded {
		return dimStyle.Render("Loading ledger…")
	}
	if u.ledgerErr != nil {
		return errorStyle.Render("Error loading ledger: " + u.ledgerErr.Error())
	}
	if len(u.cachedLines) == 0 {
		return dimStyle.Render("No usage events in this time period.")
	}

	// Calculate available space for ledger
	ledgerHeight := height - 6 // Leave room for header, footer, filter info
	if ledgerHeight < 3 {
		ledgerHeight = 3
	}

	var b strings.Builder

	// Filter info
	filterLabel := u.filter.label()
	if u.filter != usageFilterAll {
		filterLabel = selectedStyle.Render(filterLabel)
	}
	b.WriteString(fmt.Sprintf("  %s: %s  •  %d events total\n\n", labelStyle.Render("Time filter"), filterLabel, len(u.filteredEvents)))

	// Validate scroll position
	if u.scrollPos > len(u.cachedLines)-1 {
		u.scrollPos = len(u.cachedLines) - 1
	}
	if u.scrollPos < 0 {
		u.scrollPos = 0
	}

	// Calculate visible range
	startLine := u.scrollPos
	endLine := startLine + ledgerHeight
	if endLine > len(u.cachedLines) {
		endLine = len(u.cachedLines)
		// Adjust start to fill the view
		startLine = endLine - ledgerHeight
		if startLine < 0 {
			startLine = 0
		}
	}

	// Get visible lines
	var visibleLines []string
	if len(u.cachedLines) > 0 && startLine < len(u.cachedLines) {
		visibleLines = u.cachedLines[startLine:endLine]
	}

	// Truncate lines to width and render
	var renderedLines []string
	for _, line := range visibleLines {
		truncated := truncateWidth(line, width-4)
		renderedLines = append(renderedLines, truncated)
	}

	content := strings.Join(renderedLines, "\n")
	if content == "" {
		content = dimStyle.Render("(no events)")
	}

	// Render with border
	borderStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	renderedContent := borderStyle.Width(width - 4).Render(content)
	b.WriteString(renderedContent)

	b.WriteString("\n" + dimStyle.Render("Press l to summary  •  f/F filter  •  ↑↓ scroll  •  PgUp/PgDn  •  Home/End"))

	// Add scroll indicator if needed
	if len(u.cachedLines) > ledgerHeight {
		scrollPercent := 0
		if len(u.cachedLines) > ledgerHeight {
			scrollPercent = int((float64(u.scrollPos) / float64(len(u.cachedLines)-ledgerHeight)) * 100)
		}
		scrollStr := fmt.Sprintf("  [%d%%]", scrollPercent)
		b.WriteString("\n" + dimStyle.Render(scrollStr))
	}

	return b.String()
}

func (u UsageTab) formatLedgerEvent(event state.UsageEvent, maxWidth int, cfg *config.Config) string {
	// Format: [2006-01-02 15:04] model | operation | in:1.2K out:3.4K | $0.012345
	timestamp := event.CreatedAt.Format("2006-01-02 15:04")

	modelName := event.Model
	if event.Provider != "" {
		modelName = event.Provider + ":" + event.Model
	}

	operation := event.Operation
	if operation == "" {
		operation = "unknown"
	}

	tokenInfo := fmt.Sprintf("in:%s out:%s", formatTokenCount(event.InputTokens), formatTokenCount(event.OutputTokens))

	costStr := ""
	if event.CostUSD > 0 {
		costStr = fmt.Sprintf(" │ $%.6f", event.CostUSD)
	} else if cfg != nil {
		modelForPrice := event.Model
		if _, _, found := config.LookupModelPrice(modelForPrice, cfg); !found {
			costStr = fmt.Sprintf(" │ %s", dimStyle.Render("(no price)"))
		}
	}

	// Build the line
	line := fmt.Sprintf("[%s] %s │ %s │ %s%s",
		timestamp,
		truncateWidth(modelName, 25),
		truncateWidth(operation, 15),
		tokenInfo,
		costStr,
	)

	// If line is too long, truncate it gracefully
	if lipgloss.Width(line) > maxWidth {
		line = truncateWidth(line, maxWidth)
	}

	return line
}

// usageEventMatchesFilterLocal checks if a usage event matches the given filter.
func usageEventMatchesFilterLocal(event state.UsageEvent, filter state.UsageFilter) bool {
	if filter.CreatedAfter != nil && event.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}
	if filter.CreatedBefore != nil && event.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}
	return true
}

func (f usageTimeFilter) next() usageTimeFilter {
	return (f + 1) % 4
}

func (f usageTimeFilter) prev() usageTimeFilter {
	return (f + 3) % 4
}

func (f usageTimeFilter) label() string {
	switch f {
	case usageFilterLast24Hours:
		return "Last 24 hours"
	case usageFilterLast7Days:
		return "Last 7 days"
	case usageFilterLast30Days:
		return "Last 30 days"
	default:
		return "All time"
	}
}

func (f usageTimeFilter) usageFilter() state.UsageFilter {
	if f == usageFilterAll {
		return state.UsageFilter{}
	}
	now := time.Now().UTC()
	var start time.Time
	switch f {
	case usageFilterLast24Hours:
		start = now.Add(-24 * time.Hour)
	case usageFilterLast7Days:
		start = now.Add(-7 * 24 * time.Hour)
	case usageFilterLast30Days:
		start = now.Add(-30 * 24 * time.Hour)
	default:
		return state.UsageFilter{}
	}
	return state.UsageFilter{CreatedAfter: &start, CreatedBefore: &now}
}

// formatTokenCount renders a token count as a compact human-readable string.
func formatTokenCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
