package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
)

var adaptCmd = &cobra.Command{
	Use:   "adapt <class>",
	Short: "Generate adaptive quiz questions based on past performance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		class := args[0]

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		orc, err := orchestrator.New(cfg)
		if err != nil {
			return err
		}

		fmt.Printf("Generating adaptive quiz for class %q based on performance history ...\n", class)
		q, path, err := quiz.Adapt(class, orc.Provider, cfg)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Adaptive quiz saved: %s\n", path)
		fmt.Printf("  Title:     %s\n", q.Title)
		fmt.Printf("  Questions: %d\n", len(q.Sections))
		fmt.Printf("\nRender with:  studyforge build %s\n", path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(adaptCmd)
}
