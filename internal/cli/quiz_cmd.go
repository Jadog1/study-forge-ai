package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/internal/tracking"
)

var (
	quizCount int
	quizType  string
	quizTags  []string
)

var quizCmd = &cobra.Command{
	Use:   "quiz <class>",
	Short: "Generate a unified adaptive quiz for a class",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		class := strings.TrimSpace(args[0])

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		orc, err := orchestrator.New(cfg)
		if err != nil {
			return err
		}

		opts := quiz.QuizOptions{
			Count:          quizCount,
			TypePreference: strings.TrimSpace(quizType),
			Tags:           quizTags,
		}

		fmt.Printf("Generating adaptive quiz for class %q...\n", class)
		q, path, err := quiz.NewQuizStream(class, opts, orc.Provider, cfg, func(e quiz.ProgressEvent) {
			if e.Label == "" {
				return
			}
			suffix := "..."
			if e.Done && e.Err == nil {
				suffix = " ✓"
			} else if e.Done && e.Err != nil {
				suffix = " ✗"
			}
			if e.Detail != "" {
				fmt.Printf("- %s: %s%s\n", e.Label, e.Detail, suffix)
				return
			}
			fmt.Printf("- %s%s\n", e.Label, suffix)
		})
		if err != nil {
			return err
		}

		quizID := strings.TrimSuffix(filepath.Base(path), ".yaml")
		sfqPath := strings.TrimSuffix(path, ".yaml") + ".sfq"
		_, cacheErr := state.RegisterTrackedQuiz(class, path, sfqPath)
		report, syncErr := tracking.SyncTrackedQuizSessions()
		sfqErr := sfq.Track(sfqPath)

		fmt.Printf("✓ Quiz saved: %s\n", path)
		fmt.Printf("  Quiz ID:    %s\n", quizID)
		fmt.Printf("  Title:      %s\n", q.Title)
		fmt.Printf("  Questions:  %d\n", len(q.Sections))

		if cacheErr != nil {
			fmt.Printf("  Tracked cache warning: %v\n", cacheErr)
		} else if sfqErr != nil {
			fmt.Printf("  Tracked session warning: %v\n", sfqErr)
		} else {
			fmt.Println("  Started tracked quiz session in browser...")
		}
		if syncErr != nil {
			fmt.Printf("  Session sync warning: %v\n", syncErr)
		} else {
			fmt.Printf("  Imported sessions: %d\n", report.ImportedSessions)
			fmt.Printf("  Pending tracked quizzes: %d\n", report.PendingQuizzes)
		}
		return nil
	},
}

func init() {
	quizCmd.Flags().IntVar(&quizCount, "count", 10, "Target question count")
	quizCmd.Flags().StringVar(&quizType, "type", "multiple-choice", "Preferred question type")
	quizCmd.Flags().StringSliceVar(&quizTags, "tags", nil, "Restrict source components to sections with matching tags")
	rootCmd.AddCommand(quizCmd)
}
