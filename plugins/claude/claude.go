// Package claude implements the AIProvider interface using the Anthropic
// Messages API. No third-party SDK is used — only the standard library.
package claude

import (
	"bufio"
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
	defaultModel     = "claude-3-5-sonnet-20241022"
	apiURL           = "https://api.anthropic.com/v1/messages"
	anthropicVersion = "2023-06-01"
	defaultMaxTokens = 4096
)

// Provider sends prompts to the Anthropic Messages endpoint.
type Provider struct {
	APIKey string
	Model  string
}

// New returns a Claude provider. If model is empty, claude-3-5-sonnet is used.
func New(apiKey, model string) *Provider {
	if model == "" {
		model = defaultModel
	}
	return &Provider{APIKey: apiKey, Model: model}
}

// Name satisfies the AIProvider interface.
func (p *Provider) Name() string { return "claude" }

// Disabled returns true when the API key is not configured.
func (p *Provider) Disabled() bool {
	return p.APIKey == "" || p.APIKey == "YOUR_CLAUDE_API_KEY"
}

// Generate sends prompt to Anthropic and returns the assistant reply.
func (p *Provider) Generate(prompt string) (string, error) {
	result, err := p.GenerateWithMetadata(prompt)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// GenerateWithMetadata sends prompt to Anthropic and returns text with usage.
func (p *Provider) GenerateWithMetadata(prompt string) (plugins.GenerateResult, error) {
	body, err := json.Marshal(messagesRequest{
		Model:     p.Model,
		MaxTokens: defaultMaxTokens,
		Messages:  []message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("claude: marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("claude: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("claude: send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("claude: read response: %w", err)
	}

	var mr messagesResponse
	if err := json.Unmarshal(raw, &mr); err != nil {
		return plugins.GenerateResult{}, fmt.Errorf("claude: parse response: %w", err)
	}
	if mr.Error != nil {
		return plugins.GenerateResult{}, fmt.Errorf("claude: API error: %s", mr.Error.Message)
	}
	if len(mr.Content) == 0 {
		return plugins.GenerateResult{}, fmt.Errorf("claude: no content in response")
	}
	return plugins.GenerateResult{
		Text: mr.Content[0].Text,
		Usage: plugins.TokenUsage{
			InputTokens:  mr.Usage.InputTokens,
			OutputTokens: mr.Usage.OutputTokens,
			TotalTokens:  mr.Usage.InputTokens + mr.Usage.OutputTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider:  p.Name(),
			Model:     fallbackModel(mr.Model, p.Model),
			RequestID: mr.ID,
			At:        time.Now().UTC(),
		},
	}, nil
}

// StreamGenerate sends prompt to Anthropic and emits buffered text chunks.
func (p *Provider) StreamGenerate(prompt string, onChunk func(string) error) error {
	body, err := json.Marshal(messagesRequest{
		Model:     p.Model,
		MaxTokens: defaultMaxTokens,
		Messages:  []message{{Role: "user", Content: prompt}},
		Stream:    true,
	})
	if err != nil {
		return fmt.Errorf("claude: marshal stream request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("claude: create stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("claude: send stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("claude: read stream response: %w", readErr)
		}
		return fmt.Errorf("claude: stream request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	buffer := claudeStreamBuffer{threshold: 48}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var event messageStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return fmt.Errorf("claude: parse stream response: %w", err)
		}
		if event.Error != nil {
			return fmt.Errorf("claude: API error: %s", event.Error.Message)
		}
		if event.Type == "content_block_delta" && event.Delta.Text != "" {
			if err := buffer.Add(event.Delta.Text, onChunk); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("claude: read stream: %w", err)
	}
	return buffer.Flush(onChunk)
}

// ── wire types ───────────────────────────────────────────────────────────────

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
	Stream    bool      `json:"stream,omitempty"`
}

type messagesResponse struct {
	ID      string `json:"id,omitempty"`
	Model   string `json:"model,omitempty"`
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func fallbackModel(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type messageStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type claudeStreamBuffer struct {
	pending   strings.Builder
	threshold int
}

func (b *claudeStreamBuffer) Add(text string, onChunk func(string) error) error {
	if text == "" {
		return nil
	}
	b.pending.WriteString(text)
	if b.pending.Len() >= b.threshold || strings.ContainsAny(text, "\n.!?;:") {
		return b.Flush(onChunk)
	}
	return nil
}

func (b *claudeStreamBuffer) Flush(onChunk func(string) error) error {
	if b.pending.Len() == 0 {
		return nil
	}
	chunk := b.pending.String()
	b.pending.Reset()
	return onChunk(chunk)
}
