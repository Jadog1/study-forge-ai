package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/class"
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

func init() {
	classCmd.AddCommand(classCreateCmd)
	classCmd.AddCommand(classListCmd)
	rootCmd.AddCommand(classCmd)
}
