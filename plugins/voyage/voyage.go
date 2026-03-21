// Package voyage implements embeddings via the VoyageAI API.
package voyage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/studyforge/study-agent/plugins"
)

const (
	defaultModel = "voyage-3-large"
	apiURL       = "https://api.voyageai.com/v1/embeddings"
)

// Provider sends embedding requests to VoyageAI.
type Provider struct {
	APIKey string
	model  string
}

// New returns a Voyage embeddings provider.
func New(apiKey, model string) *Provider {
	if model == "" {
		model = defaultModel
	}
	return &Provider{APIKey: apiKey, model: model}
}

// Name satisfies plugins.EmbeddingProvider.
func (p *Provider) Name() string { return "voyage" }

// Model returns the configured model identifier.
func (p *Provider) Model() string { return p.model }

// Disabled returns true when the API key is not configured.
func (p *Provider) Disabled() bool {
	return p.APIKey == "" || p.APIKey == "YOUR_VOYAGE_API_KEY"
}

// Embed returns one embedding vector per input text.
func (p *Provider) Embed(input []string) ([][]float64, error) {
	result, err := p.EmbedWithMetadata(input)
	if err != nil {
		return nil, err
	}
	return result.Vectors, nil
}

// EmbedWithMetadata returns one embedding vector per input text with usage.
func (p *Provider) EmbedWithMetadata(input []string) (plugins.EmbedResult, error) {
	body, err := json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{
		Model: p.model,
		Input: input,
	})
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("voyage: marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("voyage: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("voyage: send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("voyage: read response: %w", err)
	}

	var parsed struct {
		Model string `json:"model,omitempty"`
		Data  []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			TotalTokens  int `json:"total_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage,omitempty"`
		Detail string `json:"detail,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("voyage: parse response: %w", err)
	}
	if parsed.Detail != "" {
		return plugins.EmbedResult{}, fmt.Errorf("voyage: API error: %s", parsed.Detail)
	}
	if parsed.Error != "" {
		return plugins.EmbedResult{}, fmt.Errorf("voyage: API error: %s", parsed.Error)
	}
	if len(parsed.Data) == 0 {
		return plugins.EmbedResult{}, fmt.Errorf("voyage: no embeddings in response")
	}

	vectors := make([][]float64, len(parsed.Data))
	for _, row := range parsed.Data {
		if row.Index < 0 || row.Index >= len(vectors) {
			return plugins.EmbedResult{}, fmt.Errorf("voyage: invalid embedding index %d", row.Index)
		}
		vectors[row.Index] = row.Embedding
	}
	for i, vec := range vectors {
		if len(vec) == 0 {
			return plugins.EmbedResult{}, fmt.Errorf("voyage: missing embedding for input index %d", i)
		}
	}

	inputTokens := parsed.Usage.InputTokens
	if inputTokens == 0 {
		for _, item := range input {
			inputTokens += len(strings.Fields(item))
		}
	}
	totalTokens := parsed.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = inputTokens
	}

	return plugins.EmbedResult{
		Vectors: vectors,
		Usage: plugins.TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: parsed.Usage.OutputTokens,
			TotalTokens:  totalTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider: p.Name(),
			Model:    firstNonEmpty(parsed.Model, p.model),
			At:       time.Now().UTC(),
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
