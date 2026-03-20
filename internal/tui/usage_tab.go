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
	totals  *state.UsageTotals
	baseCfg *config.Config
	err     error
	loaded  bool
	loading bool
	width   int
	filter  usageTimeFilter
}

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
			return u, nil
		case "F":
			u.filter = u.filter.prev()
			u = u.applyFilter()
			return u, nil
		}
	}
	return u, nil
}

func (u UsageTab) view(width, _ int, cfg *config.Config) string {
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
			dimStyle.Render("Press r to refresh  •  f/F change time filter."),
		)
	}

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

	b.WriteString("\n" + dimStyle.Render("Press r to refresh  •  f/F change time filter  •  sfa pricing list to view/manage pricing"))
	return b.String()
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
