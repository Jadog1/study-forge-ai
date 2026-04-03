package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/state"
)

var classCmd = &cobra.Command{
	Use:   "class",
	Short: "Manage classes (syllabus, rules, listing)",
}

var classCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Scaffold a new class with default syllabus and rules",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := class.Create(name); err != nil {
			return err
		}
		fmt.Printf("✓ Class %q created.\n", name)
		fmt.Printf("  Edit %s to add topics.\n", displayClassFile(name, "syllabus.yaml"))
		fmt.Printf("  Edit %s to set exam expectations.\n", displayClassFile(name, "rules.yaml"))
		return nil
	},
}

var classListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all classes",
	RunE: func(cmd *cobra.Command, args []string) error {
		classes, err := class.List()
		if err != nil {
			return err
		}
		if len(classes) == 0 {
			fmt.Println("No classes found. Run 'sfa class create <name>' to create one.")
			return nil
		}
		for _, c := range classes {
			fmt.Printf("  - %s\n", c)
		}
		return nil
	},
}

var (
	notesAddLabel   string
	notesAddPattern string
	notesAddWeek    int
	notesAddOrder   int
	notesAddTags    []string

	coverageKind            string
	coveragePrimaryLabels   []string
	coverageSecondaryLabels []string
	coverageSecondaryWeight float64
	coverageExclude         bool

	contextKind string
)

var classNotesCmd = &cobra.Command{
	Use:   "notes",
	Short: "Manage ordered class note roster",
}

var classNotesListCmd = &cobra.Command{
	Use:   "list <class>",
	Short: "List ordered note roster entries for a class",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		roster, err := class.LoadNoteRoster(className)
		if err != nil {
			return err
		}
		if len(roster.Entries) == 0 {
			fmt.Printf("No roster entries configured for class %q.\n", className)
			suggestions, suggestionErr := suggestSourcePathsForClass(className)
			if suggestionErr == nil && len(suggestions) > 0 {
				fmt.Println("Found source path suggestions from ingested knowledge:")
				for _, path := range suggestions {
					fmt.Printf("  - %s\n", path)
				}
			}
			fmt.Printf("Use 'sfa class notes add %s --label ... --pattern ...' to create entries.\n", className)
			return nil
		}
		fmt.Printf("Note roster for class %q:\n", className)
		for _, entry := range roster.Entries {
			extra := ""
			if entry.Week > 0 {
				extra = fmt.Sprintf(" (week %d)", entry.Week)
			}
			fmt.Printf("  %d. %s%s\n", entry.Order, entry.Label, extra)
			if entry.SourcePattern != "" {
				fmt.Printf("      pattern: %s\n", entry.SourcePattern)
			}
			if len(entry.Tags) > 0 {
				fmt.Printf("      tags: %s\n", strings.Join(entry.Tags, ", "))
			}
		}
		return nil
	},
}

var classNotesAddCmd = &cobra.Command{
	Use:   "add <class>",
	Short: "Add or update one note roster entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		if strings.TrimSpace(notesAddLabel) == "" {
			return fmt.Errorf("--label is required")
		}
		if strings.TrimSpace(notesAddPattern) == "" && len(notesAddTags) == 0 {
			return fmt.Errorf("provide at least one of --pattern or --tags")
		}
		entry := class.NoteRosterEntry{
			Label:         strings.TrimSpace(notesAddLabel),
			SourcePattern: strings.TrimSpace(notesAddPattern),
			Tags:          append([]string(nil), notesAddTags...),
			Week:          notesAddWeek,
			Order:         notesAddOrder,
		}
		roster, err := class.UpsertNoteRosterEntry(className, entry)
		if err != nil {
			return err
		}
		fmt.Printf("Saved note roster entry %q for class %q.\n", entry.Label, className)
		fmt.Printf("Roster now has %d entrie(s).\n", len(roster.Entries))
		return nil
	},
}

var classNotesRemoveCmd = &cobra.Command{
	Use:   "remove <class> <label>",
	Short: "Remove one note roster entry by label",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		label := strings.TrimSpace(args[1])
		roster, err := class.RemoveNoteRosterEntry(className, label)
		if err != nil {
			return err
		}
		fmt.Printf("Removed roster entry %q for class %q.\n", label, className)
		fmt.Printf("Roster now has %d entrie(s).\n", len(roster.Entries))
		return nil
	},
}

var classNotesReorderCmd = &cobra.Command{
	Use:   "reorder <class> <labels>",
	Short: "Reorder note roster entries with a comma-separated label sequence",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		labels := splitCSV(args[1])
		if len(labels) == 0 {
			return fmt.Errorf("labels must not be empty")
		}
		roster, err := class.ReorderNoteRosterEntries(className, labels)
		if err != nil {
			return err
		}
		fmt.Printf("Reordered %d roster entrie(s) for class %q.\n", len(roster.Entries), className)
		return nil
	},
}

var classCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Manage weighted assessment coverage scope",
}

var classCoverageShowCmd = &cobra.Command{
	Use:   "show <class>",
	Short: "Show class coverage scope for the selected assessment kind",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		kind := class.NormalizeContextProfile(coverageKind)
		scope, err := class.LoadCoverageScope(className, kind)
		if err != nil {
			return err
		}
		if scope == nil {
			fmt.Printf("No coverage scope configured for class %q (%s).\n", className, kind)
			return nil
		}
		roster, _ := class.LoadNoteRoster(className)
		fmt.Printf("Coverage scope for class %q (%s):\n", className, kind)
		fmt.Printf("  exclude_unmatched: %t\n", scope.ExcludeUnmatched)
		for i, group := range scope.Groups {
			fmt.Printf("  - group %d: weight %.2f\n", i+1, group.Weight)
			if len(group.Labels) > 0 {
				fmt.Printf("      labels: %s\n", strings.Join(group.Labels, ", "))
			}
			resolved := class.ResolveGroupPatterns(group, roster)
			if len(resolved) > 0 {
				fmt.Printf("      patterns: %s\n", strings.Join(resolved, ", "))
			}
			if len(group.Tags) > 0 {
				fmt.Printf("      tags: %s\n", strings.Join(group.Tags, ", "))
			}
		}
		return nil
	},
}

var classCoverageSetCmd = &cobra.Command{
	Use:   "set <class>",
	Short: "Set weighted coverage scope for an assessment kind",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		kind := class.NormalizeContextProfile(coverageKind)
		primary := append([]string(nil), coveragePrimaryLabels...)
		secondary := append([]string(nil), coverageSecondaryLabels...)
		if len(primary) == 0 && len(secondary) == 0 {
			return fmt.Errorf("provide at least one of --primary or --secondary")
		}
		groups := make([]class.ScopeGroup, 0, 2)
		if len(primary) > 0 {
			groups = append(groups, class.ScopeGroup{Labels: primary, Weight: 1.0})
		}
		if len(secondary) > 0 {
			if coverageSecondaryWeight < 0 {
				return fmt.Errorf("--secondary-weight must be >= 0")
			}
			groups = append(groups, class.ScopeGroup{Labels: secondary, Weight: coverageSecondaryWeight})
		}
		scope := &class.CoverageScope{
			Class:            className,
			Kind:             kind,
			ExcludeUnmatched: coverageExclude,
			Groups:           groups,
		}
		if err := class.SaveCoverageScope(className, kind, scope); err != nil {
			return err
		}
		fmt.Printf("Saved coverage scope for class %q (%s).\n", className, kind)
		fmt.Printf("  groups: %d\n", len(groups))
		fmt.Printf("  exclude_unmatched: %t\n", coverageExclude)
		return nil
	},
}

func suggestSourcePathsForClass(className string) ([]string, error) {
	idx, err := state.LoadSectionIndex()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, section := range idx.Sections {
		if !strings.EqualFold(strings.TrimSpace(section.Class), className) {
			continue
		}
		for _, sourcePath := range section.SourcePaths {
			normalized := strings.TrimSpace(sourcePath)
			if normalized == "" {
				continue
			}
			key := strings.ToLower(normalized)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return out, nil
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// resolveTextArg joins positional args as text, or reads from stdin when none given.
func resolveTextArg(args []string) (string, error) {
	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return strings.Join(args, " "), nil
}

var classContextCmd = &cobra.Command{
	Use:   "context",
	Short: "View or edit class context text without opening a file",
}

var classContextShowCmd = &cobra.Command{
	Use:   "show <class>",
	Short: "Print current context text for the selected profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		kind := class.NormalizeContextProfile(contextKind)
		text, err := class.LoadProfileContextText(className, kind)
		if err != nil {
			return err
		}
		if strings.TrimSpace(text) == "" {
			fmt.Printf("No context text configured for class %q (%s).\n", className, kind)
			return nil
		}
		fmt.Println(text)
		return nil
	},
}

var classContextSetCmd = &cobra.Command{
	Use:   "set <class> [text...]",
	Short: "Set context text (omit text to read from stdin)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		kind := class.NormalizeContextProfile(contextKind)
		text, err := resolveTextArg(args[1:])
		if err != nil {
			return err
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("context text must not be empty — use 'clear' to reset to default")
		}
		if err := class.SaveProfileContextText(className, kind, text); err != nil {
			return err
		}
		fmt.Printf("✓ Context updated for class %q (%s).\n", className, kind)
		return nil
	},
}

var classContextAppendCmd = &cobra.Command{
	Use:   "append <class> [text...]",
	Short: "Append text to context for the selected profile (omit text to read from stdin)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		kind := class.NormalizeContextProfile(contextKind)
		existing, err := class.LoadProfileContextText(className, kind)
		if err != nil {
			return err
		}
		addendum, err := resolveTextArg(args[1:])
		if err != nil {
			return err
		}
		if strings.TrimSpace(addendum) == "" {
			return fmt.Errorf("nothing to append")
		}
		combined := strings.TrimSpace(existing) + "\n\n" + strings.TrimSpace(addendum)
		if err := class.SaveProfileContextText(className, kind, combined); err != nil {
			return err
		}
		fmt.Printf("✓ Context appended for class %q (%s).\n", className, kind)
		return nil
	},
}

var classContextClearCmd = &cobra.Command{
	Use:   "clear <class>",
	Short: "Reset context text to default template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		className := strings.TrimSpace(args[0])
		kind := class.NormalizeContextProfile(contextKind)
		if err := class.SaveProfileContextText(className, kind, ""); err != nil {
			return err
		}
		fmt.Printf("✓ Context reset to default for class %q (%s).\n", className, kind)
		return nil
	},
}

func init() {
	classNotesAddCmd.Flags().StringVar(&notesAddLabel, "label", "", "Roster label (required)")
	classNotesAddCmd.Flags().StringVar(&notesAddPattern, "pattern", "", "Source path substring pattern")
	classNotesAddCmd.Flags().StringSliceVar(&notesAddTags, "tags", nil, "Optional tags for this roster entry")
	classNotesAddCmd.Flags().IntVar(&notesAddWeek, "week", 0, "Optional curriculum week number")
	classNotesAddCmd.Flags().IntVar(&notesAddOrder, "order", 0, "Optional explicit roster order")

	classNotesCmd.AddCommand(classNotesListCmd)
	classNotesCmd.AddCommand(classNotesAddCmd)
	classNotesCmd.AddCommand(classNotesRemoveCmd)
	classNotesCmd.AddCommand(classNotesReorderCmd)

	classCoverageShowCmd.Flags().StringVar(&coverageKind, "kind", "exam", "Assessment kind (quiz or exam)")
	classCoverageSetCmd.Flags().StringVar(&coverageKind, "kind", "exam", "Assessment kind (quiz or exam)")
	classCoverageSetCmd.Flags().StringSliceVar(&coveragePrimaryLabels, "primary", nil, "Primary roster labels")
	classCoverageSetCmd.Flags().StringSliceVar(&coverageSecondaryLabels, "secondary", nil, "Secondary roster labels")
	classCoverageSetCmd.Flags().Float64Var(&coverageSecondaryWeight, "secondary-weight", 0.30, "Weight multiplier for secondary labels")
	classCoverageSetCmd.Flags().BoolVar(&coverageExclude, "exclude-unmatched", false, "Exclude unmatched material from candidates")

	classCoverageCmd.AddCommand(classCoverageShowCmd)
	classCoverageCmd.AddCommand(classCoverageSetCmd)

	classContextShowCmdFlags := classContextShowCmd.Flags()
	classContextShowCmdFlags.StringVar(&contextKind, "kind", "exam", "Assessment kind (quiz or exam)")
	classContextSetCmd.Flags().StringVar(&contextKind, "kind", "exam", "Assessment kind (quiz or exam)")
	classContextAppendCmd.Flags().StringVar(&contextKind, "kind", "exam", "Assessment kind (quiz or exam)")
	classContextClearCmd.Flags().StringVar(&contextKind, "kind", "exam", "Assessment kind (quiz or exam)")
	classContextCmd.AddCommand(classContextShowCmd)
	classContextCmd.AddCommand(classContextSetCmd)
	classContextCmd.AddCommand(classContextAppendCmd)
	classContextCmd.AddCommand(classContextClearCmd)

	classCmd.AddCommand(classCreateCmd)
	classCmd.AddCommand(classListCmd)
	classCmd.AddCommand(classNotesCmd)
	classCmd.AddCommand(classCoverageCmd)
	classCmd.AddCommand(classContextCmd)
	rootCmd.AddCommand(classCmd)
}
