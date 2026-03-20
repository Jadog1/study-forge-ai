package local

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/studyforge/study-agent/plugins"
)

const embeddingsPath = "/v1/embeddings"

// EmbeddingProvider sends OpenAI-compatible embeddings requests to a local endpoint.
type EmbeddingProvider struct {
	Endpoint string
	Model    string
}

// NewEmbeddingProvider returns a local embeddings provider.
func NewEmbeddingProvider(endpoint, model string) *EmbeddingProvider {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if model == "" {
		model = defaultModel
	}
	return &EmbeddingProvider{Endpoint: endpoint, Model: model}
}

// Name satisfies plugins.EmbeddingProvider.
func (p *EmbeddingProvider) Name() string { return "local" }

// Disabled returns true when no endpoint is configured.
func (p *EmbeddingProvider) Disabled() bool {
	return strings.TrimSpace(p.Endpoint) == ""
}

// Embed returns one embedding per input text from a local OpenAI-compatible endpoint.
func (p *EmbeddingProvider) Embed(input []string) ([][]float64, error) {
	result, err := p.EmbedWithMetadata(input)
	if err != nil {
		return nil, err
	}
	return result.Vectors, nil
}

// EmbedWithMetadata returns one embedding per input text with token metadata.
func (p *EmbeddingProvider) EmbedWithMetadata(input []string) (plugins.EmbedResult, error) {
	endpoint, err := buildEmbeddingsURL(p.Endpoint)
	if err != nil {
		return plugins.EmbedResult{}, err
	}

	body, err := json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{
		Model: p.Model,
		Input: input,
	})
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("local: marshal embeddings request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("local: create embeddings request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("local: send embeddings request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("local: read embeddings response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return plugins.EmbedResult{}, fmt.Errorf("local: embeddings request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var response struct {
		Model string `json:"model,omitempty"`
		Data  []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage,omitempty"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("local: parse embeddings response: %w", err)
	}
	if response.Error != nil {
		return plugins.EmbedResult{}, fmt.Errorf("local: API error: %s", response.Error.Message)
	}
	if len(response.Data) == 0 {
		return plugins.EmbedResult{}, fmt.Errorf("local: no embeddings in response")
	}

	vectors := make([][]float64, len(response.Data))
	for _, row := range response.Data {
		if row.Index < 0 || row.Index >= len(vectors) {
			return plugins.EmbedResult{}, fmt.Errorf("local: invalid embedding index %d", row.Index)
		}
		vectors[row.Index] = row.Embedding
	}
	for i, vec := range vectors {
		if len(vec) == 0 {
			return plugins.EmbedResult{}, fmt.Errorf("local: missing embedding for input index %d", i)
		}
	}

	promptTokens := response.Usage.PromptTokens
	if promptTokens == 0 {
		for _, item := range input {
			promptTokens += len(strings.Fields(item))
		}
	}
	totalTokens := response.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = promptTokens
	}

	return plugins.EmbedResult{
		Vectors: vectors,
		Usage: plugins.TokenUsage{
			InputTokens: promptTokens,
			TotalTokens: totalTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider: p.Name(),
			Model:    chooseModel(response.Model, p.Model),
			At:       time.Now().UTC(),
		},
	}, nil
}

func buildEmbeddingsURL(baseEndpoint string) (string, error) {
	parsed, err := url.Parse(baseEndpoint)
	if err != nil {
		return "", fmt.Errorf("local: parse embeddings endpoint %q: %w", baseEndpoint, err)
	}

	trimmedPath := strings.TrimRight(parsed.Path, "/")
	if trimmedPath == embeddingsPath {
		return parsed.String(), nil
	}
	if trimmedPath == "" {
		parsed.Path = embeddingsPath
	} else {
		parsed.Path = path.Join(trimmedPath, embeddingsPath)
	}
	return parsed.String(), nil
}

func chooseModel(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
