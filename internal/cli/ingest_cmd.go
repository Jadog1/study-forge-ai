package cli

import (
	"fmt"

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

		fmt.Printf("Ingesting notes from %q", folderPath)
		if ingestClass != "" {
			fmt.Printf(" [class: %s]", ingestClass)
		}
		fmt.Println(" ...")

		notes, err := ingestion.IngestFolder(folderPath, ingestClass, orc.Provider, cfg)
		if err != nil {
			return err
		}

		idx, err := state.LoadNotesIndex()
		if err != nil {
			return err
		}
		for _, n := range notes {
			idx.AddOrUpdate(n)
		}
		if err := state.SaveNotesIndex(idx); err != nil {
			return fmt.Errorf("save notes index: %w", err)
		}

		fmt.Printf("✓ Ingested %d note(s). Index updated.\n", len(notes))
		return nil
	},
}

func init() {
	ingestCmd.Flags().StringVarP(&ingestClass, "class", "c", "", "Associate ingested notes with a class")
	rootCmd.AddCommand(ingestCmd)
}
