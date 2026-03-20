package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/state"
)

var completeClass string

var completeCmd = &cobra.Command{
	Use:   "complete <quiz-path>",
	Short: "Record quiz results interactively and save performance data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		quizPath := args[0]
		q, err := quiz.LoadQuiz(quizPath)
		if err != nil {
			return err
		}

		// Fall back to the quiz's embedded class.
		class := completeClass
		if class == "" {
			class = q.Class
		}
		if class == "" {
			return fmt.Errorf("class is required — pass --class or embed 'class:' in the quiz YAML")
		}

		scanner := bufio.NewScanner(os.Stdin)
		results := state.QuizResults{
			QuizID:      quizPath,
			CompletedAt: time.Now(),
		}

		fmt.Printf("=== %s — Recording Results ===\n\n", q.Title)
		for _, s := range q.Sections {
			fmt.Printf("Q: %s\n", s.Question)
			fmt.Printf("   Answer: %s\n", s.Answer)
			fmt.Printf("   Reasoning: %s\n", s.Reasoning)
			fmt.Print("   Did you get it correct? (y/n): ")

			correct := false
			if scanner.Scan() {
				line := strings.TrimSpace(strings.ToLower(scanner.Text()))
				correct = line == "y" || line == "yes"
			}
			results.Results = append(results.Results, state.QuizResult{
				QuestionID: s.ID,
				Correct:    correct,
				TimeSpent:  0,
			})
			fmt.Println()
		}

		// Derive a stable quiz ID from the file path.
		quizID := strings.TrimSuffix(filepath.Base(quizPath), ".yaml")
		if err := state.SaveQuizResults(&results, class, quizID); err != nil {
			return fmt.Errorf("save results: %w", err)
		}

		correct := 0
		for _, r := range results.Results {
			if r.Correct {
				correct++
			}
		}
		total := len(results.Results)
		pct := 0.0
		if total > 0 {
			pct = float64(correct) / float64(total) * 100
		}
		fmt.Printf("Score: %d/%d (%.0f%%)\n", correct, total, pct)
		fmt.Println("✓ Results saved. Run 'sfa adapt <class>' to get adaptive follow-up questions.")
		return nil
	},
}

func init() {
	completeCmd.Flags().StringVarP(&completeClass, "class", "c", "", "Class to store results under (defaults to quiz's class field)")
	rootCmd.AddCommand(completeCmd)
}
