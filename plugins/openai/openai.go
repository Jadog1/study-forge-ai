// Package openai implements the AIProvider interface using the OpenAI Chat
// Completions API. No third-party SDK is used — only the standard library.
package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/studyforge/study-agent/plugins"
)

const (
	defaultModel = "gpt-4o"
	apiURL       = "https://api.openai.com/v1/chat/completions"
	embedAPIURL  = "https://api.openai.com/v1/embeddings"
	envGenTemp   = "SFA_GENERATION_TEMPERATURE"
)

// Provider sends prompts to the OpenAI Chat Completions endpoint.
type Provider struct {
	APIKey string
	model  string
}

// New returns an OpenAI provider. If model is empty, gpt-4o is used.
func New(apiKey, model string) *Provider {
	if model == "" {
		model = defaultModel
	}
	return &Provider{APIKey: apiKey, model: model}
}

// Name satisfies the AIProvider interface.
func (p *Provider) Name() string { return "openai" }

// Model returns the configured model identifier.
func (p *Provider) Model() string { return p.model }

// Disabled returns true when the API key is not configured.
func (p *Provider) Disabled() bool {
	return p.APIKey == "" || p.APIKey == "YOUR_OPENAI_API_KEY"
}

// Generate sends prompt to OpenAI and returns the assistant reply.
func (p *Provider) Generate(prompt string) (string, error) {
	result, err := p.GenerateWithMetadata(prompt)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// GenerateWithMetadata sends prompt to OpenAI and returns text with model usage.
func (p *Provider) GenerateWithMetadata(prompt string) (plugins.GenerateResult, error) {
	body, err := json.Marshal(chatRequest{
		Model:       p.model,
		Messages:    []message{{Role: "user", Content: prompt}},
		Temperature: generationTemperature(),
	})
	if err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("openai: send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("openai: read response: %w", err)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("openai: parse response: %w", err)
	}
	if cr.Error != nil {
		return plugins.GenerateResult{}, fmt.Errorf("openai: API error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return plugins.GenerateResult{}, fmt.Errorf("openai: no choices in response")
	}
	return plugins.GenerateResult{
		Text: cr.Choices[0].Message.Content,
		Usage: plugins.TokenUsage{
			InputTokens:  cr.Usage.PromptTokens,
			OutputTokens: cr.Usage.CompletionTokens,
			TotalTokens:  cr.Usage.TotalTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider:  p.Name(),
			Model:     coalesceString(cr.Model, p.model),
			RequestID: cr.ID,
			At:        time.Now().UTC(),
		},
	}, nil
}

// StreamGenerate sends prompt to OpenAI and emits buffered content chunks.
func (p *Provider) StreamGenerate(prompt string, onChunk func(string) error) error {
	body, err := json.Marshal(chatRequest{
		Model:       p.model,
		Messages:    []message{{Role: "user", Content: prompt}},
		Stream:      true,
		Temperature: generationTemperature(),
	})
	if err != nil {
		return fmt.Errorf("openai: marshal stream request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("openai: create stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("openai: send stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("openai: read stream response: %w", readErr)
		}
		return fmt.Errorf("openai: stream request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	buffer := streamBuffer{threshold: 48}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk chatStreamResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return fmt.Errorf("openai: parse stream response: %w", err)
		}
		if chunk.Error != nil {
			return fmt.Errorf("openai: API error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if err := buffer.Add(chunk.Choices[0].Delta.Content, onChunk); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("openai: read stream: %w", err)
	}
	return buffer.Flush(onChunk)
}

// Embed sends one or more texts to OpenAI and returns embedding vectors.
func (p *Provider) Embed(input []string) ([][]float64, error) {
	result, err := p.EmbedWithMetadata(input)
	if err != nil {
		return nil, err
	}
	return result.Vectors, nil
}

// EmbedWithMetadata sends one or more texts to OpenAI and returns vectors with usage.
func (p *Provider) EmbedWithMetadata(input []string) (plugins.EmbedResult, error) {
	body, err := json.Marshal(embeddingsRequest{
		Model: p.model,
		Input: input,
	})
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("openai: marshal embeddings request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, embedAPIURL, bytes.NewReader(body))
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("openai: create embeddings request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("openai: send embeddings request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("openai: read embeddings response: %w", err)
	}

	var er embeddingsResponse
	if err := json.Unmarshal(raw, &er); err != nil {
		return plugins.EmbedResult{}, fmt.Errorf("openai: parse embeddings response: %w", err)
	}
	if er.Error != nil {
		return plugins.EmbedResult{}, fmt.Errorf("openai: API error: %s", er.Error.Message)
	}
	if len(er.Data) == 0 {
		return plugins.EmbedResult{}, fmt.Errorf("openai: no embeddings in response")
	}

	vectors := make([][]float64, len(er.Data))
	for _, row := range er.Data {
		if row.Index < 0 || row.Index >= len(vectors) {
			return plugins.EmbedResult{}, fmt.Errorf("openai: invalid embedding index %d", row.Index)
		}
		vectors[row.Index] = row.Embedding
	}
	for i, vec := range vectors {
		if len(vec) == 0 {
			return plugins.EmbedResult{}, fmt.Errorf("openai: missing embedding for input index %d", i)
		}
	}
	return plugins.EmbedResult{
		Vectors: vectors,
		Usage: plugins.TokenUsage{
			InputTokens: er.Usage.PromptTokens,
			TotalTokens: er.Usage.TotalTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider: p.Name(),
			Model:    coalesceString(er.Model, p.model),
			At:       time.Now().UTC(),
		},
	}, nil
}

// ── wire types ───────────────────────────────────────────────────────────────

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
}

type chatResponse struct {
	ID      string `json:"id,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type chatStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
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

func coalesceString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type streamBuffer struct {
	pending   strings.Builder
	threshold int
}

func (b *streamBuffer) Add(text string, onChunk func(string) error) error {
	if text == "" {
		return nil
	}
	b.pending.WriteString(text)
	if b.pending.Len() >= b.threshold || strings.ContainsAny(text, "\n.!?;:") {
		return b.Flush(onChunk)
	}
	return nil
}

func (b *streamBuffer) Flush(onChunk func(string) error) error {
	if b.pending.Len() == 0 {
		return nil
	}
	chunk := b.pending.String()
	b.pending.Reset()
	return onChunk(chunk)
}

func generationTemperature() *float64 {
	raw := strings.TrimSpace(os.Getenv(envGenTemp))
	if raw == "" {
		defaultTemp := 0.7
		return &defaultTemp
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v > 2 {
		defaultTemp := 0.7
		return &defaultTemp
	}
	return &v
}
