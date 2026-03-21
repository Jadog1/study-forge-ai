package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/ingestion"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/state"
)

var ingestClass string

var ingestCmd = &cobra.Command{
	Use:   "ingest <path>",
	Short: "Ingest raw notes from a folder and extract AI metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		folderPath := args[0]

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		orc, err := orchestrator.New(cfg)
		if err != nil {
			return err
		}

		// Check if embeddings are disabled and warn user
		if orc.EmbeddingProvider.Disabled() {
			fmt.Println("⚠ WARNING: Embeddings are not configured or disabled.")
			fmt.Println("  This means deduplication and semantic knowledge consolidation will NOT happen.")
			fmt.Printf("  Provider: %s (config: %s@%s)\n", orc.EmbeddingProvider.Name(), cfg.Embeddings.Provider, cfg.Embeddings.Model)
			fmt.Println("\nTo enable embeddings, configure it in your settings or ~/.study-forge-ai/config.yaml")
			fmt.Print("Continue without embeddings? (y/N) ")
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() || !strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
				return fmt.Errorf("ingestion cancelled")
			}
			fmt.Println()
		}

		fmt.Printf("Ingesting notes from %q", folderPath)
		if ingestClass != "" {
			fmt.Printf(" [class: %s]", ingestClass)
		}
		fmt.Println(" ...")

		knowledge, err := ingestion.IngestKnowledgeFolderStream(folderPath, ingestClass, orc.Provider, orc.EmbeddingProvider, cfg, func(e ingestion.ProgressEvent) {
			if e.Label == "" {
				return
			}
			status := "..."
			if e.Done {
				if e.Err != nil {
					status = "✗"
				} else {
					status = "✓"
				}
			}
			if e.Detail != "" {
				fmt.Printf("  %s %s: %s\n", status, e.Label, e.Detail)
				return
			}
			fmt.Printf("  %s %s\n", status, e.Label)
		})
		if err != nil {
			return err
		}

		idx, err := state.LoadNotesIndex()
		if err != nil {
			return err
		}
		for _, n := range knowledge.Notes {
			idx.AddOrUpdate(n)
		}
		if err := state.SaveNotesIndex(idx); err != nil {
			return fmt.Errorf("save notes index: %w", err)
		}

		fmt.Printf("✓ Ingested %d note(s). Sections: %d. Components: %d. Usage events: %d.\n", len(knowledge.Notes), knowledge.SectionsAdded, knowledge.ComponentsAdded, knowledge.UsageEvents)
		return nil
	},
}

func init() {
	ingestCmd.Flags().StringVarP(&ingestClass, "class", "c", "", "Associate ingested notes with a class")
	rootCmd.AddCommand(ingestCmd)
}
