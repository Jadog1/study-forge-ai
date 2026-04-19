package quiz

import (
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/repository"
	"github.com/studyforge/study-agent/internal/state"
	"github.com/studyforge/study-agent/plugins"
)

const (
	quizOperationOrchestrator = "quiz_orchestrator"
	quizOperationComponent    = "quiz_component"
)

func generateWithQuizUsage(provider plugins.AIProvider, prompt, operation, class string, cfg *config.Config, usageRepo repository.UsageRepository) (string, error) {
	if usageAware, ok := provider.(plugins.UsageAwareAIProvider); ok {
		result, err := usageAware.GenerateWithMetadata(prompt)
		if err != nil {
			appendQuizUsageFailure(provider, operation, class, usageRepo)
			return "", err
		}
		appendQuizUsageEvent(state.UsageEvent{
			Operation:    operation,
			Provider:     strings.TrimSpace(result.Metadata.Provider),
			Model:        strings.TrimSpace(result.Metadata.Model),
			RequestID:    result.Metadata.RequestID,
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
			TotalTokens:  result.Usage.TotalTokens,
			CostUSD:      quizCostForResult(result, cfg),
			Class:        strings.TrimSpace(class),
			CreatedAt:    time.Now().UTC(),
		}, usageRepo)
		return result.Text, nil
	}

	resp, err := provider.Generate(prompt)
	if err != nil {
		appendQuizUsageFailure(provider, operation, class, usageRepo)
		return "", err
	}

	inputTokens := estimateTokens(prompt)
	outputTokens := estimateTokens(resp)
	appendQuizUsageEvent(state.UsageEvent{
		Operation:    operation,
		Provider:     strings.TrimSpace(provider.Name()),
		Model:        strings.TrimSpace(provider.Model()),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		CostUSD:      config.CostForTokens(strings.TrimSpace(provider.Model()), inputTokens, outputTokens, cfg),
		Class:        strings.TrimSpace(class),
		CreatedAt:    time.Now().UTC(),
	}, usageRepo)

	return resp, nil
}

func appendQuizUsageFailure(provider plugins.AIProvider, operation, class string, usageRepo repository.UsageRepository) {
	appendQuizUsageEvent(state.UsageEvent{
		Operation: operation,
		Provider:  strings.TrimSpace(provider.Name()),
		Model:     strings.TrimSpace(provider.Model()),
		Class:     strings.TrimSpace(class),
		CreatedAt: time.Now().UTC(),
	}, usageRepo)
}

func appendQuizUsageEvent(event state.UsageEvent, usageRepo repository.UsageRepository) {
	if strings.TrimSpace(event.Provider) == "" {
		event.Provider = "unknown"
	}
	if strings.TrimSpace(event.Model) == "" {
		event.Model = "unknown"
	}
	if strings.TrimSpace(event.Operation) == "" {
		event.Operation = "quiz_generation"
	}
	if usageRepo == nil {
		usageRepo = resolveStore(nil).Usage()
	}
	_ = usageRepo.AppendUsageEvent(event)
}

func quizCostForResult(result plugins.GenerateResult, cfg *config.Config) float64 {
	if result.Usage.CostUSD > 0 {
		return result.Usage.CostUSD
	}
	return config.CostForTokens(strings.TrimSpace(result.Metadata.Model), result.Usage.InputTokens, result.Usage.OutputTokens, cfg)
}

func estimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	// Approximate token count from character length for providers without usage metadata.
	tokens := len([]rune(trimmed)) / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}

func resolveStore(store repository.Store) repository.Store {
	if store == nil {
		return repository.NewFilesystemStore()
	}
	return store
}
