package cli

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/state"
)

var pricingDetectPrompt bool

var pricingCmd = &cobra.Command{
	Use:   "pricing",
	Short: "Manage per-model pricing for cost tracking",
	Long: `Manage per-million-token pricing used to estimate AI call costs.

Built-in prices are provided for common models. Use 'pricing set' to add
pricing for custom or unreleased models, which are then saved to config.yaml.`,
}

var pricingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all model prices (built-in and configured overrides)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		all := config.BuiltInModelPrices()
		for k, v := range cfg.ModelPrices {
			all[k] = v
		}

		keys := make([]string, 0, len(all))
		for k := range all {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Printf("%-40s  %-16s  %-16s  %s\n", "Model", "Input/1M tokens", "Output/1M tokens", "Source")
		fmt.Println(strings.Repeat("─", 88))
		for _, k := range keys {
			p := all[k]
			source := "built-in"
			if _, ok := cfg.ModelPrices[k]; ok {
				source = "configured"
			}
			fmt.Printf("%-40s  $%-15.4f  $%-15.4f  %s\n", k, p.InputPerMillion, p.OutputPerMillion, source)
		}
		return nil
	},
}

var pricingSetCmd = &cobra.Command{
	Use:   "set <model> <input-per-million> <output-per-million>",
	Short: "Set per-million-token pricing for a model and save to config",
	Example: `  sfa pricing set my-custom-model 0.15 0.60
  sfa pricing set voyage-custom 0.20 0`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		model := args[0]
		inputPPM, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return fmt.Errorf("invalid input price %q: %w", args[1], err)
		}
		outputPPM, err := strconv.ParseFloat(args[2], 64)
		if err != nil {
			return fmt.Errorf("invalid output price %q: %w", args[2], err)
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := config.SetModelPrice(model, inputPPM, outputPPM, cfg); err != nil {
			return fmt.Errorf("save pricing: %w", err)
		}
		fmt.Printf("✓ Pricing for %q set: $%.4f/M input, $%.4f/M output\n", model, inputPPM, outputPPM)
		return nil
	},
}

var pricingUnsetCmd = &cobra.Command{
	Use:   "unset <model>",
	Short: "Remove custom pricing for a model (reverts to built-in if available)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		model := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.ModelPrices == nil {
			fmt.Printf("No custom pricing configured for %q\n", model)
			return nil
		}
		if _, ok := cfg.ModelPrices[model]; !ok {
			fmt.Printf("No custom pricing configured for %q\n", model)
			return nil
		}
		delete(cfg.ModelPrices, model)
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Printf("✓ Removed custom pricing for %q\n", model)
		return nil
	},
}

var pricingDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Scan usage history and report models without pricing",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		ledger, err := state.LoadUsageLedger()
		if err != nil {
			return fmt.Errorf("load usage ledger: %w", err)
		}

		// Collect unique model names from ledger events.
		models := make(map[string]bool)
		for _, e := range ledger.Events {
			if e.Model != "" && e.TotalTokens > 0 {
				models[e.Model] = true
			}
		}
		if len(models) == 0 {
			fmt.Println("No usage history found. Run 'sfa ingest' first.")
			return nil
		}

		var unknown []string
		for model := range models {
			if _, _, found := config.LookupModelPrice(model, cfg); !found {
				unknown = append(unknown, model)
			}
		}
		if len(unknown) == 0 {
			fmt.Println("All models in usage history have pricing configured.")
			return nil
		}

		sort.Strings(unknown)
		fmt.Printf("Models with unknown pricing (%d):\n\n", len(unknown))
		for _, m := range unknown {
			fmt.Printf("  %s\n", m)
			fmt.Printf("    sfa pricing set %q <input-per-million> <output-per-million>\n", m)
		}

		if !pricingDetectPrompt {
			fmt.Println()
			return nil
		}

		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("\nEnter pricing now? (y/n)")
		fmt.Print("> ")
		if !scanner.Scan() {
			return nil
		}
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if answer != "y" && answer != "yes" {
			return nil
		}

		updated := 0
		for _, model := range unknown {
			fmt.Printf("\nModel: %s\n", model)
			fmt.Print("Input $/1M tokens: ")
			if !scanner.Scan() {
				break
			}
			inputTxt := strings.TrimSpace(scanner.Text())
			in, err := strconv.ParseFloat(inputTxt, 64)
			if err != nil {
				fmt.Printf("  Skipped: invalid input price %q\n", inputTxt)
				continue
			}

			fmt.Print("Output $/1M tokens: ")
			if !scanner.Scan() {
				break
			}
			outputTxt := strings.TrimSpace(scanner.Text())
			out, err := strconv.ParseFloat(outputTxt, 64)
			if err != nil {
				fmt.Printf("  Skipped: invalid output price %q\n", outputTxt)
				continue
			}

			if err := config.SetModelPrice(model, in, out, cfg); err != nil {
				fmt.Printf("  Failed to save: %v\n", err)
				continue
			}
			updated++
			fmt.Printf("  Saved: $%.4f/M input, $%.4f/M output\n", in, out)
		}
		fmt.Printf("\nUpdated pricing for %d model(s).\n", updated)
		return nil
	},
}

func init() {
	pricingDetectCmd.Flags().BoolVar(&pricingDetectPrompt, "prompt", true, "Prompt interactively for unknown model prices and save them")
	pricingCmd.AddCommand(pricingListCmd)
	pricingCmd.AddCommand(pricingSetCmd)
	pricingCmd.AddCommand(pricingUnsetCmd)
	pricingCmd.AddCommand(pricingDetectCmd)
	rootCmd.AddCommand(pricingCmd)
}
