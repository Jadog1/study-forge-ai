package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	classpkg "github.com/studyforge/study-agent/internal/class"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/sfq"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/internal/tracking"
)

var (
	quizCount                    int
	quizAssessment               string
	quizType                     string
	quizTags                     []string
	quizSections                 []string
	quizCoveragePrimary          []string
	quizCoverageSecondary        []string
	quizCoverageSecondaryWeight  float64
	quizCoverageExcludeUnmatched bool
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
			AssessmentKind:  strings.TrimSpace(quizAssessment),
			Count:           quizCount,
			TypePreference:  strings.TrimSpace(quizType),
			Tags:            quizTags,
			FocusedSections: quizSections,
		}
		if len(quizCoveragePrimary) > 0 || len(quizCoverageSecondary) > 0 {
			groups := make([]classpkg.ScopeGroup, 0, 2)
			if len(quizCoveragePrimary) > 0 {
				groups = append(groups, classpkg.ScopeGroup{Labels: append([]string(nil), quizCoveragePrimary...), Weight: 1.0})
			}
			if len(quizCoverageSecondary) > 0 {
				groups = append(groups, classpkg.ScopeGroup{Labels: append([]string(nil), quizCoverageSecondary...), Weight: quizCoverageSecondaryWeight})
			}
			opts.CoverageScope = &classpkg.CoverageScope{
				Class:            class,
				Kind:             classpkg.NormalizeContextProfile(quizAssessment),
				ExcludeUnmatched: quizCoverageExcludeUnmatched,
				Groups:           groups,
			}
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
			if report.BackfilledSessions > 0 {
				fmt.Printf("  Backfilled sessions: %d\n", report.BackfilledSessions)
			}
			if report.UnmappedAnswers > 0 {
				fmt.Printf("  Unmapped answers: %d\n", report.UnmappedAnswers)
			}
			fmt.Printf("  Pending tracked quizzes: %d\n", report.PendingQuizzes)
		}
		return nil
	},
}

func init() {
	quizCmd.Flags().IntVar(&quizCount, "count", 10, "Target question count")
	quizCmd.Flags().StringVar(&quizAssessment, "assessment", "quiz", "Assessment profile kind (e.g. quiz, exam, focused)")
	quizCmd.Flags().StringVar(&quizType, "type", "context-default", "Preferred question type or context-default")
	quizCmd.Flags().StringSliceVar(&quizTags, "tags", nil, "Restrict source components to sections with matching tags")
	quizCmd.Flags().StringSliceVar(&quizSections, "sections", nil, "Section IDs or title substrings to focus on (used with --assessment focused)")
	quizCmd.Flags().StringSliceVar(&quizCoveragePrimary, "coverage-primary", nil, "Coverage primary roster labels for this run")
	quizCmd.Flags().StringSliceVar(&quizCoverageSecondary, "coverage-secondary", nil, "Coverage secondary roster labels for this run")
	quizCmd.Flags().Float64Var(&quizCoverageSecondaryWeight, "coverage-secondary-weight", 0.30, "Coverage weight multiplier for secondary labels")
	quizCmd.Flags().BoolVar(&quizCoverageExcludeUnmatched, "coverage-exclude-unmatched", false, "Exclude unmatched material when using coverage labels")
	rootCmd.AddCommand(quizCmd)
}
