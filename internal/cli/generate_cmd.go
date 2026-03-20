package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/sfq"
)

var generateTags []string

var generateCmd = &cobra.Command{
	Use:   "generate <class>",
	Short: "Generate a new quiz for a class using AI",
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

		fmt.Printf("Generating quiz for class %q ...\n", class)
		q, path, err := quiz.Generate(class, generateTags, orc.Provider, cfg)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Quiz saved: %s\n", path)
		fmt.Printf("  Title:     %s\n", q.Title)
		fmt.Printf("  Questions: %d\n", len(q.Sections))
		sfqPath := strings.TrimSuffix(path, ".yaml") + ".sfq"
		if sfqErr := sfq.Generate(sfqPath); sfqErr != nil {
			fmt.Printf("  (could not open quiz in browser: %s)\n", sfqErr)
		} else {
			fmt.Printf("  Opening quiz in browser...\n")
		}
		return nil
	},
}

func init() {
	generateCmd.Flags().StringSliceVar(&generateTags, "tags", nil, "Restrict source notes to these tags")
	rootCmd.AddCommand(generateCmd)
}
