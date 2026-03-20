package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "sfa",
	Short: "AI-powered study assistant",
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
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Launch()
	},
	Long: `sfa ingests notes, generates quizzes, tracks performance,
and uses AI to create adaptive study materials.

Get started:
  sfa
  sfa class create linear-algebra
  sfa ingest ./my-notes --class linear-algebra
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

func displayClassFile(name, fileName string) string {
	return filepath.ToSlash(filepath.Join(config.DisplayRootDir(), "classes", name, fileName))
}
