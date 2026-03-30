package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/search"
)

var (
	searchTags   []string
	searchClass  string
	searchSource string
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search ingested notes by tags, class, or source file",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Source-path search returns sections and components directly.
		if searchSource != "" {
			results, err := search.BySourcePath(searchSource)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Printf("No knowledge found for source %q.\n", searchSource)
				return nil
			}
			fmt.Printf("Found %d knowledge item(s) from %q:\n\n", len(results), searchSource)
			for _, r := range results {
				switch r.Kind {
				case "section":
					fmt.Printf("  [section] %s\n", r.Section.Title)
					fmt.Printf("    Class:   %s\n", r.Section.Class)
					fmt.Printf("    Summary: %s\n", r.Section.Summary)
					if len(r.Section.Tags) > 0 {
						fmt.Printf("    Tags:    %s\n", strings.Join(r.Section.Tags, ", "))
					}
					fmt.Println()
				case "component":
					fmt.Printf("  [component/%s] %s\n", r.Component.Kind, r.Component.SectionID)
					fmt.Printf("    Class:   %s\n", r.Component.Class)
					if len(r.Component.Content) > 120 {
						fmt.Printf("    Content: %s...\n\n", r.Component.Content[:120])
					} else {
						fmt.Printf("    Content: %s\n\n", r.Component.Content)
					}
				}
			}
			return nil
		}

		var results []search.Result
		var err error

		switch {
		case len(searchTags) > 0:
			results, err = search.ByTags(searchTags)
		case searchClass != "":
			results, err = search.ByClass(searchClass)
		default:
			return fmt.Errorf("provide at least one of --tags, --class, or --source")
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
	searchCmd.Flags().StringVarP(&searchSource, "source", "s", "", "Show all knowledge derived from a specific source file")
	rootCmd.AddCommand(searchCmd)
}
