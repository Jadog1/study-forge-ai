// Package local implements the AIProvider interface against a locally running
// Ollama instance (or any OpenAI-compatible generate endpoint).
package local

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/studyforge/study-agent/plugins"
)

const (
	defaultEndpoint = "http://localhost:11434"
	defaultModel    = "llama3"
	ollamaPath      = "/api/generate"
	openAIPath      = "/v1/chat/completions"
)

// Provider sends prompts to a local Ollama or OpenAI-compatible endpoint.
type Provider struct {
	Endpoint string
	model    string
}

// New returns a local provider. Defaults to Ollama at localhost:11434 with llama3.
func New(endpoint, model string) *Provider {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if model == "" {
		model = defaultModel
	}
	return &Provider{Endpoint: endpoint, model: model}
}

// Name satisfies the AIProvider interface.
func (p *Provider) Name() string { return "local" }

// Model returns the configured model identifier.
func (p *Provider) Model() string { return p.model }

// Disabled returns true when no endpoint is configured.
func (p *Provider) Disabled() bool {
	return p.Endpoint == ""
}

// Generate sends prompt to the local model and returns the response text.
func (p *Provider) Generate(prompt string) (string, error) {
	result, err := p.GenerateWithMetadata(prompt)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// GenerateWithMetadata sends prompt to local model and returns token usage.
// It uses provider-reported counts when available and falls back to estimates.
func (p *Provider) GenerateWithMetadata(prompt string) (plugins.GenerateResult, error) {
	outcome, err := p.generateWithMetadata(prompt)
	if err != nil {
		return plugins.GenerateResult{}, err
	}

	usage := outcome.usage
	if usage == nil {
		usage = approximateUsage(prompt, outcome.text)
	}

	return plugins.GenerateResult{
		Text: outcome.text,
		Usage: plugins.TokenUsage{
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider:  p.Name(),
			Model:     coalesceString(outcome.model, p.model),
			RequestID: outcome.requestID,
			At:        time.Now().UTC(),
		},
	}, nil
}

func (p *Provider) generateWithMetadata(prompt string) (generationOutcome, error) {
	switch classifyEndpoint(p.Endpoint) {
	case endpointKindOpenAI:
		return p.generateChatCompletion(prompt, p.Endpoint)
	case endpointKindOllama:
		return p.generateOllama(prompt, p.Endpoint)
	}

	chatURL, err := buildEndpointURL(p.Endpoint, openAIPath)
	if err != nil {
		return generationOutcome{}, err
	}
	response, err := p.generateChatCompletion(prompt, chatURL)
	if err == nil {
		return response, nil
	}
	if !shouldFallback(err) {
		return generationOutcome{}, err
	}

	ollamaURL, ollamaErr := buildEndpointURL(p.Endpoint, ollamaPath)
	if ollamaErr != nil {
		return generationOutcome{}, ollamaErr
	}
	response, ollamaReqErr := p.generateOllama(prompt, ollamaURL)
	if ollamaReqErr == nil {
		return response, nil
	}

	return generationOutcome{}, fmt.Errorf("local: openai-compatible request failed: %v; ollama request failed: %w", err, ollamaReqErr)
}

// StreamGenerate sends prompt to the local model and streams text chunks.
func (p *Provider) StreamGenerate(prompt string, onChunk func(string) error) error {
	_, err := p.StreamGenerateWithMetadata(prompt, onChunk)
	return err
}

// StreamGenerateWithMetadata streams prompt completion and returns token usage.
func (p *Provider) StreamGenerateWithMetadata(prompt string, onChunk func(string) error) (plugins.GenerateResult, error) {
	outcome, err := p.streamWithMetadata(prompt, onChunk)
	if err != nil {
		return plugins.GenerateResult{}, err
	}

	usage := outcome.usage
	if usage == nil {
		usage = approximateUsage(prompt, outcome.text)
	}

	return plugins.GenerateResult{
		Text: outcome.text,
		Usage: plugins.TokenUsage{
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider:  p.Name(),
			Model:     coalesceString(outcome.model, p.model),
			RequestID: outcome.requestID,
			At:        time.Now().UTC(),
		},
	}, nil
}

func (p *Provider) streamWithMetadata(prompt string, onChunk func(string) error) (generationOutcome, error) {
	switch classifyEndpoint(p.Endpoint) {
	case endpointKindOpenAI:
		return p.streamChatCompletion(prompt, p.Endpoint, onChunk)
	case endpointKindOllama:
		return p.streamOllama(prompt, p.Endpoint, onChunk)
	}

	chatURL, err := buildEndpointURL(p.Endpoint, openAIPath)
	if err != nil {
		return generationOutcome{}, err
	}
	if outcome, err := p.streamChatCompletion(prompt, chatURL, onChunk); err == nil {
		return outcome, nil
	} else if !shouldFallback(err) {
		return generationOutcome{}, err
	}

	ollamaURL, ollamaErr := buildEndpointURL(p.Endpoint, ollamaPath)
	if ollamaErr != nil {
		return generationOutcome{}, ollamaErr
	}
	if outcome, err := p.streamOllama(prompt, ollamaURL, onChunk); err == nil {
		return outcome, nil
	} else if !shouldFallback(err) {
		return generationOutcome{}, err
	}

	result, err := p.GenerateWithMetadata(prompt)
	if err != nil {
		return generationOutcome{}, err
	}
	if err := onChunk(result.Text); err != nil {
		return generationOutcome{}, err
	}
	return generationOutcome{
		text:      result.Text,
		usage:     &result.Usage,
		model:     result.Metadata.Model,
		requestID: result.Metadata.RequestID,
	}, nil
}

func (p *Provider) generateOllama(prompt, endpoint string) (generationOutcome, error) {
	body, err := json.Marshal(generateRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return generationOutcome{}, newEndpointError("local: ollama request failed", resp.StatusCode, raw)
	}

	var gr generateResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return generationOutcome{}, &endpointError{message: fmt.Sprintf("local: parse ollama response: %v", err), fallback: true}
	}
	if gr.Error != "" {
		return generationOutcome{}, fmt.Errorf("local: model error: %s", gr.Error)
	}

	var usage *plugins.TokenUsage
	if gr.PromptEvalCount > 0 || gr.EvalCount > 0 {
		usage = &plugins.TokenUsage{
			InputTokens:  gr.PromptEvalCount,
			OutputTokens: gr.EvalCount,
			TotalTokens:  gr.PromptEvalCount + gr.EvalCount,
		}
	}

	return generationOutcome{
		text:  gr.Response,
		usage: usage,
		model: gr.Model,
	}, nil
}

func (p *Provider) streamOllama(prompt, endpoint string, onChunk func(string) error) (generationOutcome, error) {
	body, err := json.Marshal(generateRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: true,
	})
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: marshal stream request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: create stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: send stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return generationOutcome{}, fmt.Errorf("local: read stream response: %w", readErr)
		}
		return generationOutcome{}, newEndpointError("local: ollama stream request failed", resp.StatusCode, raw)
	}

	var text strings.Builder
	var requestID string
	var model string
	var usage *plugins.TokenUsage

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var chunk generateStreamResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return generationOutcome{}, &endpointError{message: fmt.Sprintf("local: parse ollama stream response: %v", err), fallback: true}
		}
		if chunk.Error != "" {
			return generationOutcome{}, fmt.Errorf("local: model error: %s", chunk.Error)
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.RequestID != "" {
			requestID = chunk.RequestID
		}
		if chunk.PromptEvalCount > 0 || chunk.EvalCount > 0 {
			usage = &plugins.TokenUsage{
				InputTokens:  chunk.PromptEvalCount,
				OutputTokens: chunk.EvalCount,
				TotalTokens:  chunk.PromptEvalCount + chunk.EvalCount,
			}
		}
		if chunk.Response != "" {
			text.WriteString(chunk.Response)
			if err := onChunk(chunk.Response); err != nil {
				return generationOutcome{}, err
			}
		}
		if chunk.Done {
			return generationOutcome{text: text.String(), usage: usage, model: model, requestID: requestID}, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return generationOutcome{}, fmt.Errorf("local: read ollama stream: %w", err)
	}
	return generationOutcome{text: text.String(), usage: usage, model: model, requestID: requestID}, nil
}

func (p *Provider) generateChatCompletion(prompt, endpoint string) (generationOutcome, error) {
	body, err := json.Marshal(chatRequest{
		Model:    p.model,
		Messages: []message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: marshal chat request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: send chat request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: read chat response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return generationOutcome{}, newEndpointError("local: openai-compatible request failed", resp.StatusCode, raw)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return generationOutcome{}, &endpointError{message: fmt.Sprintf("local: parse chat response: %v", err), fallback: true}
	}
	if cr.Error != nil {
		return generationOutcome{}, fmt.Errorf("local: API error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return generationOutcome{}, fmt.Errorf("local: no choices in response")
	}

	var usage *plugins.TokenUsage
	if cr.Usage != nil {
		usage = &plugins.TokenUsage{
			InputTokens:  cr.Usage.PromptTokens,
			OutputTokens: cr.Usage.CompletionTokens,
			TotalTokens:  cr.Usage.TotalTokens,
		}
	}

	return generationOutcome{
		text:      cr.Choices[0].Message.Content,
		usage:     usage,
		model:     cr.Model,
		requestID: cr.ID,
	}, nil
}

func (p *Provider) streamChatCompletion(prompt, endpoint string, onChunk func(string) error) (generationOutcome, error) {
	body, err := json.Marshal(chatRequest{
		Model:    p.model,
		Messages: []message{{Role: "user", Content: prompt}},
		Stream:   true,
	})
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: marshal chat stream request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: create chat stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return generationOutcome{}, fmt.Errorf("local: send chat stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return generationOutcome{}, fmt.Errorf("local: read chat stream response: %w", readErr)
		}
		return generationOutcome{}, newEndpointError("local: openai-compatible stream request failed", resp.StatusCode, raw)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return generationOutcome{}, &endpointError{message: "local: chat stream endpoint did not return text/event-stream", fallback: true}
	}

	var text strings.Builder
	var requestID string
	var model string
	var usage *plugins.TokenUsage

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
			return generationOutcome{text: text.String(), usage: usage, model: model, requestID: requestID}, nil
		}

		var chunk chatStreamResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return generationOutcome{}, &endpointError{message: fmt.Sprintf("local: parse chat stream response: %v", err), fallback: true}
		}
		if chunk.Error != nil {
			return generationOutcome{}, fmt.Errorf("local: API error: %s", chunk.Error.Message)
		}
		if chunk.ID != "" {
			requestID = chunk.ID
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Usage != nil {
			usage = &plugins.TokenUsage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
				TotalTokens:  chunk.Usage.TotalTokens,
			}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if content := chunk.Choices[0].Delta.Content; content != "" {
			text.WriteString(content)
			if err := onChunk(content); err != nil {
				return generationOutcome{}, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return generationOutcome{}, fmt.Errorf("local: read chat stream: %w", err)
	}
	return generationOutcome{text: text.String(), usage: usage, model: model, requestID: requestID}, nil
}

func buildEndpointURL(baseEndpoint, apiPath string) (string, error) {
	parsed, err := url.Parse(baseEndpoint)
	if err != nil {
		return "", fmt.Errorf("local: parse endpoint %q: %w", baseEndpoint, err)
	}

	trimmedPath := strings.TrimRight(parsed.Path, "/")
	if trimmedPath == openAIPath || trimmedPath == ollamaPath {
		return parsed.String(), nil
	}

	if trimmedPath == "" {
		parsed.Path = apiPath
	} else {
		parsed.Path = path.Join(trimmedPath, apiPath)
	}
	return parsed.String(), nil
}

type endpointKind int

const (
	endpointKindBase endpointKind = iota
	endpointKindOpenAI
	endpointKindOllama
)

func classifyEndpoint(endpoint string) endpointKind {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpointKindBase
	}

	switch strings.TrimRight(parsed.Path, "/") {
	case openAIPath:
		return endpointKindOpenAI
	case ollamaPath:
		return endpointKindOllama
	default:
		return endpointKindBase
	}
}

type endpointError struct {
	message  string
	fallback bool
}

func (e *endpointError) Error() string { return e.message }

func newEndpointError(prefix string, statusCode int, raw []byte) error {
	message := strings.TrimSpace(string(raw))
	if message == "" {
		message = http.StatusText(statusCode)
	}

	return &endpointError{
		message:  fmt.Sprintf("%s: status %d: %s", prefix, statusCode, message),
		fallback: statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed,
	}
}

func shouldFallback(err error) bool {
	var endpointErr *endpointError
	return errors.As(err, &endpointErr) && endpointErr.fallback
}

func approximateUsage(prompt, response string) *plugins.TokenUsage {
	inputTokens := len(strings.Fields(prompt))
	outputTokens := len(strings.Fields(response))
	return &plugins.TokenUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	}
}

func coalesceString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type generationOutcome struct {
	text      string
	usage     *plugins.TokenUsage
	model     string
	requestID string
}

// ── wire types ───────────────────────────────────────────────────────────────

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Model           string `json:"model,omitempty"`
	Response        string `json:"response"`
	PromptEvalCount int    `json:"prompt_eval_count,omitempty"`
	EvalCount       int    `json:"eval_count,omitempty"`
	Error           string `json:"error,omitempty"`
}

type generateStreamResponse struct {
	Model           string `json:"model,omitempty"`
	RequestID       string `json:"request_id,omitempty"`
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	PromptEvalCount int    `json:"prompt_eval_count,omitempty"`
	EvalCount       int    `json:"eval_count,omitempty"`
	Error           string `json:"error,omitempty"`
}

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
	ID      string `json:"id,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type chatStreamResponse struct {
	ID      string `json:"id,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
