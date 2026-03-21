package ingestion

import (
	"strings"
	"testing"
)

func TestParseComposedKnowledge_QuotesColonPlainScalars(t *testing.T) {
	response := `sections:
  - title: Bias-Variance Tradeoff and Model Selection
    summary: Foundational concept about overfitting and generalization.
    tags:
      - model-evaluation
    concepts:
      - bias-variance-tradeoff
    components:
      - kind: formula
        content: Expected test MSE decomposition: E(y_0 - f(x_0))^2 = Var(f(x_0)) + [Bias(f(x_0))]^2 + Var(epsilon)
        tags:
          - error-decomposition
        concepts:
          - expected-test-error
`

	parsed, err := parseComposedKnowledge(response)
	if err != nil {
		t.Fatalf("expected fallback parser to succeed, got error: %v", err)
	}
	if len(parsed.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(parsed.Sections))
	}
	if len(parsed.Sections[0].Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(parsed.Sections[0].Components))
	}
	got := parsed.Sections[0].Components[0].Content
	if !strings.Contains(got, "Expected test MSE decomposition:") {
		t.Fatalf("expected content to be preserved, got: %q", got)
	}
}

func TestNormalizeComposeYAMLPlainScalars_LeavesQuotedValues(t *testing.T) {
	input := `sections:
  - title: "Already quoted"
    summary: "Already quoted summary"
    tags:
      - tag
    concepts:
      - concept
    components:
      - kind: "concept"
        content: "Already quoted content: x: y"
`

	out := normalizeComposeYAMLPlainScalars(input)
	if out != input {
		t.Fatalf("expected quoted values to remain unchanged")
	}
}
