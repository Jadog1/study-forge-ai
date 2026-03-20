package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise app data under ~/.study-forge-ai",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := config.EnsureInitialized()
		if err != nil {
			return err
		}
		printInitStatus(result, false)
		if result.Created {
			fmt.Println("Add your API key before running AI-backed commands.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
