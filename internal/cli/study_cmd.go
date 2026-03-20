package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/quiz"
)

var studyCmd = &cobra.Command{
	Use:   "study <quiz-path>",
	Short: "Print a quiz to the terminal for self-study",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := quiz.LoadQuiz(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("=== %s ===\n\n", q.Title)
		for i, s := range q.Sections {
			fmt.Printf("Q%d: %s\n", i+1, s.Question)
			fmt.Printf("    Hint: %s\n", s.Hint)
			fmt.Printf("    Tags: %v\n\n", s.Tags)
		}
		fmt.Println("Run 'sfa complete <quiz-path>' to record your answers.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(studyCmd)
}
