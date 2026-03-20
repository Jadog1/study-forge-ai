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
	"strings"
)

const (
	defaultModel = "gpt-4o"
	apiURL       = "https://api.openai.com/v1/chat/completions"
)

// Provider sends prompts to the OpenAI Chat Completions endpoint.
type Provider struct {
	APIKey string
	Model  string
}

// New returns an OpenAI provider. If model is empty, gpt-4o is used.
func New(apiKey, model string) *Provider {
	if model == "" {
		model = defaultModel
	}
	return &Provider{APIKey: apiKey, Model: model}
}

// Name satisfies the AIProvider interface.
func (p *Provider) Name() string { return "openai" }

// Disabled returns true when the API key is not configured.
func (p *Provider) Disabled() bool {
	return p.APIKey == "" || p.APIKey == "YOUR_OPENAI_API_KEY"
}

// Generate sends prompt to OpenAI and returns the assistant reply.
func (p *Provider) Generate(prompt string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:    p.Model,
		Messages: []message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openai: read response: %w", err)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("openai: parse response: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("openai: API error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response")
	}
	return cr.Choices[0].Message.Content, nil
}

// StreamGenerate sends prompt to OpenAI and emits buffered content chunks.
func (p *Provider) StreamGenerate(prompt string, onChunk func(string) error) error {
	body, err := json.Marshal(chatRequest{
		Model:    p.Model,
		Messages: []message{{Role: "user", Content: prompt}},
		Stream:   true,
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

// ── wire types ───────────────────────────────────────────────────────────────

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
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
