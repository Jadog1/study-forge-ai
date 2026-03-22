package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/state"
)

var (
	exportClass             string
	exportIncludeEmbeddings bool
)

var exportCmd = &cobra.Command{
	Use:   "export [output-path]",
	Short: "Export the knowledge dataset to JSON for sharing",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputPath := ""
		if len(args) > 0 {
			outputPath = strings.TrimSpace(args[0])
		}
		if outputPath == "" {
			fileName := fmt.Sprintf("knowledge-export-%s.json", time.Now().UTC().Format("20060102-150405"))
			outputPath = filepath.Join(".", fileName)
		}

		result, err := state.ExportKnowledgeDataset(outputPath, state.KnowledgeExportOptions{
			Class:             exportClass,
			IncludeEmbeddings: exportIncludeEmbeddings,
		})
		if err != nil {
			return err
		}

		fmt.Printf("✓ Knowledge dataset exported: %s\n", result.OutputPath)
		if strings.TrimSpace(result.Class) != "" {
			fmt.Printf("  Class filter: %s\n", result.Class)
		}
		fmt.Printf("  Sections: %d\n", result.Sections)
		fmt.Printf("  Components: %d\n", result.Components)
		if result.IncludeEmbeddings {
			fmt.Println("  Embeddings: included")
		} else {
			fmt.Println("  Embeddings: excluded")
		}
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportClass, "class", "c", "", "Export only knowledge for a class")
	exportCmd.Flags().BoolVar(&exportIncludeEmbeddings, "include-embeddings", false, "Include section/component embeddings in the export")
	rootCmd.AddCommand(exportCmd)
}
