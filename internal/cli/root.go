package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
)

var rootCmd = &cobra.Command{
	Use:   "sfa",
	Short: "Study Forge AI web server CLI",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !shouldAutoInitialize(cmd) {
			return nil
		}

		result, err := config.EnsureInitialized()
		if err != nil {
			return err
		}
		if result.Created {
			printInitStatus(result, true)
		}
		return nil
	},
	Long: `Study Forge AI web server CLI.

Get started:
  sfa web
  sfa web --dev
`,
}

func shouldAutoInitialize(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}

	switch cmd.Name() {
	case "init", "help", "completion", "__complete", "__completeNoDesc":
		return false
	default:
		return true
	}
}

func printInitStatus(result *config.InitResult, automatic bool) {
	if result == nil {
		return
	}

	rootPath := config.DisplayPath(result.RootDir)
	configPath := config.DisplayPath(result.ConfigPath)
	if automatic {
		fmt.Printf("Initialized app data at %s\n", rootPath)
		fmt.Printf("Edit %s to add your API key.\n", configPath)
		return
	}

	if result.Created {
		fmt.Printf("Created %s\n", configPath)
	} else {
		fmt.Printf("%s already exists\n", configPath)
	}
	fmt.Printf("App data ready at %s\n", rootPath)
}
