package cli

import (
	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/tui"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch the interactive Bubble Tea workflow UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Launch()
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
