package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/search"
)

var (
	searchTags  []string
	searchClass string
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search ingested notes by tags or class",
	RunE: func(cmd *cobra.Command, args []string) error {
		var results []search.Result
		var err error

		switch {
		case len(searchTags) > 0:
			results, err = search.ByTags(searchTags)
		case searchClass != "":
			results, err = search.ByClass(searchClass)
		default:
			return fmt.Errorf("provide at least one of --tags or --class")
		}
		if err != nil {
			return err
		}

		if len(results) == 0 {
			fmt.Println("No notes matched.")
			return nil
		}

		fmt.Printf("Found %d note(s):\n\n", len(results))
		for _, r := range results {
			fmt.Printf("  [%s] %s\n", r.Note.Class, r.Note.ID)
			fmt.Printf("    Source:  %s\n", r.Note.Source)
			fmt.Printf("    Summary: %s\n", r.Note.Summary)
			fmt.Printf("    Tags:    %v\n\n", r.Note.Tags)
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().StringSliceVar(&searchTags, "tags", nil, "Filter notes by one or more tags")
	searchCmd.Flags().StringVarP(&searchClass, "class", "c", "", "Filter notes by class name")
	rootCmd.AddCommand(searchCmd)
}
