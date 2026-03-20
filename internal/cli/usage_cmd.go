package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/state"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show AI token usage and cost breakdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		totals, err := state.LoadUsageTotalsWithPricing(cfg, state.UsageFilter{})
		if err != nil {
			return fmt.Errorf("load usage totals: %w", err)
		}
		if totals.TotalTokens == 0 {
			fmt.Println("No usage recorded yet. Run 'sfa ingest' to start tracking usage.")
			return nil
		}

		fmt.Println("AI Usage Summary")
		fmt.Println("════════════════")
		fmt.Printf("Input tokens:  %s\n", formatTokens(totals.TotalInputTokens))
		fmt.Printf("Output tokens: %s\n", formatTokens(totals.TotalOutputTokens))
		fmt.Printf("Total tokens:  %s\n", formatTokens(totals.TotalTokens))
		if totals.TotalCostUSD > 0 {
			fmt.Printf("Total cost:    $%.6f\n", totals.TotalCostUSD)
		} else {
			fmt.Println("Total cost:    (no pricing — run 'sfa pricing detect' to configure)")
		}
		if !totals.UpdatedAt.IsZero() {
			fmt.Printf("Last updated:  %s\n", totals.UpdatedAt.Format("2006-01-02 15:04 UTC"))
		}

		if len(totals.ByModel) > 0 {
			fmt.Println("\nBreakdown by Model")
			fmt.Println("──────────────────")
			keys := make([]string, 0, len(totals.ByModel))
			for k := range totals.ByModel {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				t := totals.ByModel[k]
				line := fmt.Sprintf("  %-42s  in:%-9s  out:%-9s  total:%-9s",
					k,
					formatTokens(t.InputTokens),
					formatTokens(t.OutputTokens),
					formatTokens(t.TotalTokens),
				)
				if t.CostUSD > 0 {
					line += fmt.Sprintf("  $%.6f", t.CostUSD)
				}
				fmt.Println(line)
			}
		}

		// Hint about unpriced models.
		ledger, ledgerErr := state.LoadUsageLedger()
		if ledgerErr == nil {
			models := make(map[string]bool)
			for _, e := range ledger.Events {
				if e.Model != "" && e.TotalTokens > 0 {
					if _, _, found := config.LookupModelPrice(e.Model, cfg); found {
						continue
					}
					key := strings.TrimSpace(e.Provider + ":" + e.Model)
					models[key] = true
				}
			}
			if len(models) > 0 {
				fmt.Println("\nSome models have no pricing configured. Run:")
				fmt.Println("  sfa pricing detect")
			}
		}
		return nil
	},
}

// formatTokens renders a token count as a compact human-readable string.
func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func init() {
	rootCmd.AddCommand(usageCmd)
}
