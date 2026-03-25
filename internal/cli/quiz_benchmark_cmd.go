package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/quiz"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/plugins"
	claudeprovider "github.com/studyforge/study-agent/plugins/claude"
)

var (
	quizBenchmarkModels []string
	quizBenchmarkRuns   int
	quizBenchmarkCount  int
	quizBenchmarkType   string
	quizBenchmarkTags   []string
	quizBenchmarkKeep   bool
	quizBenchmarkOut    string
)

var quizBenchmarkCmd = &cobra.Command{
	Use:   "quiz-benchmark <class>",
	Short: "Benchmark quiz generation across Claude models",
	Long: "Run repeated quiz generations across multiple Claude models and compare success rate,\nlatency, retries, question-count compliance, and estimated token cost.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		class := strings.TrimSpace(args[0])
		if class == "" {
			return fmt.Errorf("class is required")
		}
		if quizBenchmarkRuns <= 0 {
			return fmt.Errorf("--runs must be greater than 0")
		}
		if quizBenchmarkCount <= 0 {
			return fmt.Errorf("--count must be greater than 0")
		}
		models := normalizeModels(quizBenchmarkModels)
		if len(models) == 0 {
			return fmt.Errorf("at least one model is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if strings.TrimSpace(cfg.Claude.APIKey) == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY_SFA is not set")
		}

		report := benchmarkReport{
			Class:            class,
			Provider:         "claude",
			StartedAt:        time.Now().UTC(),
			RunsPerModel:     quizBenchmarkRuns,
			RequestedCount:   quizBenchmarkCount,
			TypePreference:   strings.TrimSpace(quizBenchmarkType),
			Tags:             append([]string(nil), quizBenchmarkTags...),
			Models:           models,
			KeepGenerated:    quizBenchmarkKeep,
			GenerationTemp:   strings.TrimSpace(os.Getenv("SFA_GENERATION_TEMPERATURE")),
			TotalRunRequests: len(models) * quizBenchmarkRuns,
		}

		if report.TypePreference == "" {
			report.TypePreference = "multiple-choice"
		}

		for _, model := range models {
			fmt.Printf("\nBenchmarking model %q\n", model)
			for runNum := 1; runNum <= quizBenchmarkRuns; runNum++ {
				provider := claudeprovider.New(cfg.Claude.APIKey, model)
				if provider.Disabled() {
					return fmt.Errorf("provider %q is disabled", model)
				}
				telemetry := newTelemetryProvider(provider, cfg)
				opts := quiz.QuizOptions{
					Count:          quizBenchmarkCount,
					TypePreference: report.TypePreference,
					Tags:           append([]string(nil), quizBenchmarkTags...),
				}

				progressErrors := 0
				started := time.Now()
				q, quizPath, runErr := quiz.NewQuizStream(class, opts, telemetry, cfg, func(e quiz.ProgressEvent) {
					if e.Err != nil {
						progressErrors++
					}
				})
				duration := time.Since(started)

				runResult := benchmarkRunResult{
					Model:              model,
					RunNumber:          runNum,
					DurationMS:         duration.Milliseconds(),
					Success:            runErr == nil,
					Error:              errString(runErr),
					ProgressErrors:     progressErrors,
					ProviderCalls:      telemetry.totalCalls,
					OrchestratorCalls:  telemetry.orchestratorCalls,
					ComponentCalls:     telemetry.componentCalls,
					OrchestratorRetries: maxInt(0, telemetry.orchestratorCalls-1),
					ComponentRetries:   telemetry.componentRetries(),
					InputTokens:        telemetry.inputTokens,
					OutputTokens:       telemetry.outputTokens,
					TotalTokens:        telemetry.inputTokens + telemetry.outputTokens,
					EstimatedCostUSD:   telemetry.costUSD,
					QuizPath:           quizPath,
					RecordedAt:         time.Now().UTC(),
				}

				if q != nil {
					runResult.QuestionCount = len(q.Sections)
					runResult.ExactCount = len(q.Sections) == quizBenchmarkCount
					runResult.UniqueTypes = uniqueQuestionTypes(q.Sections)
				}

				report.Runs = append(report.Runs, runResult)

				status := "ok"
				if runErr != nil {
					status = "error"
				}
				fmt.Printf("  Run %d/%d: %s | duration=%s | questions=%d | calls=%d | retries(o=%d,c=%d) | tokens=%d\n",
					runNum,
					quizBenchmarkRuns,
					status,
					duration.Round(time.Millisecond),
					runResult.QuestionCount,
					runResult.ProviderCalls,
					runResult.OrchestratorRetries,
					runResult.ComponentRetries,
					runResult.TotalTokens,
				)
				if runErr != nil {
					fmt.Printf("    error: %v\n", runErr)
				}

				if !quizBenchmarkKeep && strings.TrimSpace(quizPath) != "" {
					if rmErr := os.Remove(quizPath); rmErr != nil && !os.IsNotExist(rmErr) {
						fmt.Printf("    warning: could not remove generated quiz %s: %v\n", quizPath, rmErr)
					}
				}
			}
		}

		report.FinishedAt = time.Now().UTC()
		report.DurationMS = report.FinishedAt.Sub(report.StartedAt).Milliseconds()
		report.Summaries = summarizeRuns(report.Models, report.Runs)

		printBenchmarkSummary(report)

		if strings.TrimSpace(quizBenchmarkOut) != "" {
			if err := writeBenchmarkReport(quizBenchmarkOut, report); err != nil {
				return err
			}
			fmt.Printf("\nSaved benchmark report: %s\n", quizBenchmarkOut)
		}
		return nil
	},
}

func init() {
	quizBenchmarkCmd.Flags().StringSliceVar(&quizBenchmarkModels, "models", []string{"claude-4-5-haiku", "claude-4-5-sonnet"}, "Claude model IDs to compare")
	quizBenchmarkCmd.Flags().IntVar(&quizBenchmarkRuns, "runs", 5, "Number of runs per model")
	quizBenchmarkCmd.Flags().IntVar(&quizBenchmarkCount, "count", 10, "Target question count per run")
	quizBenchmarkCmd.Flags().StringVar(&quizBenchmarkType, "type", "multiple-choice", "Preferred question type")
	quizBenchmarkCmd.Flags().StringSliceVar(&quizBenchmarkTags, "tags", nil, "Restrict source components to sections with matching tags")
	quizBenchmarkCmd.Flags().BoolVar(&quizBenchmarkKeep, "keep", false, "Keep generated quiz files instead of deleting them")
	quizBenchmarkCmd.Flags().StringVar(&quizBenchmarkOut, "out", "", "Write full benchmark report JSON to this path")
	rootCmd.AddCommand(quizBenchmarkCmd)
}

type benchmarkReport struct {
	Class            string                 `json:"class"`
	Provider         string                 `json:"provider"`
	Models           []string               `json:"models"`
	RunsPerModel     int                    `json:"runs_per_model"`
	RequestedCount   int                    `json:"requested_count"`
	TypePreference   string                 `json:"type_preference"`
	Tags             []string               `json:"tags,omitempty"`
	KeepGenerated    bool                   `json:"keep_generated"`
	GenerationTemp   string                 `json:"generation_temperature,omitempty"`
	TotalRunRequests int                    `json:"total_run_requests"`
	StartedAt        time.Time              `json:"started_at"`
	FinishedAt       time.Time              `json:"finished_at"`
	DurationMS       int64                  `json:"duration_ms"`
	Runs             []benchmarkRunResult   `json:"runs"`
	Summaries        []benchmarkModelResult `json:"summaries"`
}

type benchmarkRunResult struct {
	Model               string    `json:"model"`
	RunNumber           int       `json:"run_number"`
	DurationMS          int64     `json:"duration_ms"`
	Success             bool      `json:"success"`
	Error               string    `json:"error,omitempty"`
	QuestionCount       int       `json:"question_count"`
	ExactCount          bool      `json:"exact_count"`
	UniqueTypes         int       `json:"unique_types"`
	ProgressErrors      int       `json:"progress_errors"`
	ProviderCalls       int       `json:"provider_calls"`
	OrchestratorCalls   int       `json:"orchestrator_calls"`
	ComponentCalls      int       `json:"component_calls"`
	OrchestratorRetries int       `json:"orchestrator_retries"`
	ComponentRetries    int       `json:"component_retries"`
	InputTokens         int       `json:"input_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	TotalTokens         int       `json:"total_tokens"`
	EstimatedCostUSD    float64   `json:"estimated_cost_usd"`
	QuizPath            string    `json:"quiz_path,omitempty"`
	RecordedAt          time.Time `json:"recorded_at"`
}

type benchmarkModelResult struct {
	Model                    string  `json:"model"`
	Runs                     int     `json:"runs"`
	SuccessRate              float64 `json:"success_rate"`
	ExactCountRate           float64 `json:"exact_count_rate"`
	AvgDurationMS            int64   `json:"avg_duration_ms"`
	AvgQuestionCount         float64 `json:"avg_question_count"`
	AvgUniqueTypes           float64 `json:"avg_unique_types"`
	AvgProviderCalls         float64 `json:"avg_provider_calls"`
	AvgOrchestratorRetries   float64 `json:"avg_orchestrator_retries"`
	AvgComponentRetries      float64 `json:"avg_component_retries"`
	AvgProgressErrors        float64 `json:"avg_progress_errors"`
	AvgInputTokens           float64 `json:"avg_input_tokens"`
	AvgOutputTokens          float64 `json:"avg_output_tokens"`
	AvgTotalTokens           float64 `json:"avg_total_tokens"`
	AvgEstimatedCostUSD      float64 `json:"avg_estimated_cost_usd"`
	TotalEstimatedCostUSD    float64 `json:"total_estimated_cost_usd"`
	TotalInputTokens         int     `json:"total_input_tokens"`
	TotalOutputTokens        int     `json:"total_output_tokens"`
	TotalProviderCalls       int     `json:"total_provider_calls"`
	TotalProgressErrors      int     `json:"total_progress_errors"`
	TotalOrchestratorRetries int     `json:"total_orchestrator_retries"`
	TotalComponentRetries    int     `json:"total_component_retries"`
}

type telemetryProvider struct {
	base              plugins.AIProvider
	cfg               *config.Config
	totalCalls        int
	orchestratorCalls int
	componentCalls    int
	inputTokens       int
	outputTokens      int
	costUSD           float64
	componentCallByID map[string]int
}

var componentIDPattern = regexp.MustCompile(`\(id:\s*([^,\)\s]+),\s*kind:`)

func newTelemetryProvider(base plugins.AIProvider, cfg *config.Config) *telemetryProvider {
	return &telemetryProvider{
		base:              base,
		cfg:               cfg,
		componentCallByID: make(map[string]int),
	}
}

func (t *telemetryProvider) Name() string { return t.base.Name() }

func (t *telemetryProvider) Model() string { return t.base.Model() }

func (t *telemetryProvider) Disabled() bool { return t.base.Disabled() }

func (t *telemetryProvider) Generate(prompt string) (string, error) {
	t.totalCalls++
	stage, componentID := classifyPrompt(prompt)
	switch stage {
	case "orchestrator":
		t.orchestratorCalls++
	case "component":
		t.componentCalls++
		if componentID == "" {
			componentID = "unknown"
		}
		t.componentCallByID[componentID]++
	}

	if usageAware, ok := t.base.(plugins.UsageAwareAIProvider); ok {
		result, err := usageAware.GenerateWithMetadata(prompt)
		if err != nil {
			return "", err
		}
		t.inputTokens += result.Usage.InputTokens
		t.outputTokens += result.Usage.OutputTokens
		callCost := result.Usage.CostUSD
		if callCost == 0 {
			model := strings.TrimSpace(result.Metadata.Model)
			if model == "" {
				model = t.base.Model()
			}
			callCost = config.CostForTokens(model, result.Usage.InputTokens, result.Usage.OutputTokens, t.cfg)
		}
		t.costUSD += callCost
		return result.Text, nil
	}

	return t.base.Generate(prompt)
}

func (t *telemetryProvider) componentRetries() int {
	total := 0
	for _, calls := range t.componentCallByID {
		if calls > 1 {
			total += calls - 1
		}
	}
	return total
}

func classifyPrompt(prompt string) (stage, componentID string) {
	trimmed := strings.TrimSpace(prompt)
	if strings.HasPrefix(trimmed, "You are the quiz orchestrator") {
		return "orchestrator", ""
	}
	if strings.HasPrefix(trimmed, "You are a question writer") {
		match := componentIDPattern.FindStringSubmatch(trimmed)
		if len(match) > 1 {
			return "component", strings.TrimSpace(match[1])
		}
		return "component", ""
	}
	return "other", ""
}

func uniqueQuestionTypes(sections []state.QuizSection) int {
	seen := make(map[string]bool)
	for _, s := range sections {
		t := strings.TrimSpace(s.Type)
		if t == "" {
			continue
		}
		seen[t] = true
	}
	return len(seen)
}

func normalizeModels(models []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(models))
	for _, raw := range models {
		model := strings.TrimSpace(raw)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		out = append(out, model)
	}
	return out
}

func summarizeRuns(models []string, runs []benchmarkRunResult) []benchmarkModelResult {
	grouped := make(map[string][]benchmarkRunResult)
	for _, run := range runs {
		grouped[run.Model] = append(grouped[run.Model], run)
	}

	order := append([]string(nil), models...)
	if len(order) == 0 {
		for model := range grouped {
			order = append(order, model)
		}
	}
	sort.Strings(order)

	summaries := make([]benchmarkModelResult, 0, len(order))
	for _, model := range order {
		entries := grouped[model]
		if len(entries) == 0 {
			continue
		}
		var success int
		var exactCount int
		var durationTotal int64
		var qCountTotal int
		var uniqueTypesTotal int
		var providerCallsTotal int
		var orchestratorRetriesTotal int
		var componentRetriesTotal int
		var progressErrorsTotal int
		var inputTokensTotal int
		var outputTokensTotal int
		var costTotal float64

		for _, run := range entries {
			if run.Success {
				success++
			}
			if run.ExactCount {
				exactCount++
			}
			durationTotal += run.DurationMS
			qCountTotal += run.QuestionCount
			uniqueTypesTotal += run.UniqueTypes
			providerCallsTotal += run.ProviderCalls
			orchestratorRetriesTotal += run.OrchestratorRetries
			componentRetriesTotal += run.ComponentRetries
			progressErrorsTotal += run.ProgressErrors
			inputTokensTotal += run.InputTokens
			outputTokensTotal += run.OutputTokens
			costTotal += run.EstimatedCostUSD
		}

		runsN := float64(len(entries))
		summaries = append(summaries, benchmarkModelResult{
			Model:                    model,
			Runs:                     len(entries),
			SuccessRate:              float64(success) / runsN,
			ExactCountRate:           float64(exactCount) / runsN,
			AvgDurationMS:            durationTotal / int64(len(entries)),
			AvgQuestionCount:         float64(qCountTotal) / runsN,
			AvgUniqueTypes:           float64(uniqueTypesTotal) / runsN,
			AvgProviderCalls:         float64(providerCallsTotal) / runsN,
			AvgOrchestratorRetries:   float64(orchestratorRetriesTotal) / runsN,
			AvgComponentRetries:      float64(componentRetriesTotal) / runsN,
			AvgProgressErrors:        float64(progressErrorsTotal) / runsN,
			AvgInputTokens:           float64(inputTokensTotal) / runsN,
			AvgOutputTokens:          float64(outputTokensTotal) / runsN,
			AvgTotalTokens:           float64(inputTokensTotal+outputTokensTotal) / runsN,
			AvgEstimatedCostUSD:      costTotal / runsN,
			TotalEstimatedCostUSD:    costTotal,
			TotalInputTokens:         inputTokensTotal,
			TotalOutputTokens:        outputTokensTotal,
			TotalProviderCalls:       providerCallsTotal,
			TotalProgressErrors:      progressErrorsTotal,
			TotalOrchestratorRetries: orchestratorRetriesTotal,
			TotalComponentRetries:    componentRetriesTotal,
		})
	}
	return summaries
}

func printBenchmarkSummary(report benchmarkReport) {
	fmt.Printf("\n=== Quiz Benchmark Summary ===\n")
	fmt.Printf("class: %s\n", report.Class)
	fmt.Printf("provider: %s\n", report.Provider)
	fmt.Printf("models: %s\n", strings.Join(report.Models, ", "))
	fmt.Printf("runs/model: %d\n", report.RunsPerModel)
	fmt.Printf("requested questions/run: %d\n", report.RequestedCount)
	if len(report.Tags) > 0 {
		fmt.Printf("tags: %s\n", strings.Join(report.Tags, ", "))
	}

	fmt.Println()
	fmt.Printf("%-28s %-6s %-8s %-8s %-10s %-8s %-8s %-8s %-8s\n",
		"MODEL", "RUNS", "SUCCESS", "EXACT", "AVG_SEC", "AVG_TOK", "AVG_$", "O_RETRY", "C_RETRY")
	for _, s := range report.Summaries {
		fmt.Printf("%-28s %-6d %-8.1f %-8.1f %-10.2f %-8.0f %-8.4f %-8.2f %-8.2f\n",
			s.Model,
			s.Runs,
			s.SuccessRate*100,
			s.ExactCountRate*100,
			float64(s.AvgDurationMS)/1000,
			s.AvgTotalTokens,
			s.AvgEstimatedCostUSD,
			s.AvgOrchestratorRetries,
			s.AvgComponentRetries,
		)
	}
}

func writeBenchmarkReport(path string, report benchmarkReport) error {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(clean), 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal benchmark report: %w", err)
	}
	if err := os.WriteFile(clean, data, 0644); err != nil {
		return fmt.Errorf("write benchmark report: %w", err)
	}
	return nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
