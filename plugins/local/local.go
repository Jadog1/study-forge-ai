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
	Model    string
}

// New returns a local provider. Defaults to Ollama at localhost:11434 with llama3.
func New(endpoint, model string) *Provider {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if model == "" {
		model = defaultModel
	}
	return &Provider{Endpoint: endpoint, Model: model}
}

// Name satisfies the AIProvider interface.
func (p *Provider) Name() string { return "local" }

// Disabled returns true when no endpoint is configured.
func (p *Provider) Disabled() bool {
	return p.Endpoint == ""
}

// Generate sends prompt to the local model and returns the response text.
func (p *Provider) Generate(prompt string) (string, error) {
	switch classifyEndpoint(p.Endpoint) {
	case endpointKindOpenAI:
		return p.generateChatCompletion(prompt, p.Endpoint)
	case endpointKindOllama:
		return p.generateOllama(prompt, p.Endpoint)
	}

	chatURL, err := buildEndpointURL(p.Endpoint, openAIPath)
	if err != nil {
		return "", err
	}
	response, err := p.generateChatCompletion(prompt, chatURL)
	if err == nil {
		return response, nil
	}
	if !shouldFallback(err) {
		return "", err
	}

	ollamaURL, ollamaErr := buildEndpointURL(p.Endpoint, ollamaPath)
	if ollamaErr != nil {
		return "", ollamaErr
	}
	response, ollamaReqErr := p.generateOllama(prompt, ollamaURL)
	if ollamaReqErr == nil {
		return response, nil
	}

	return "", fmt.Errorf("local: openai-compatible request failed: %v; ollama request failed: %w", err, ollamaReqErr)
}

// StreamGenerate sends prompt to the local model and streams text chunks.
func (p *Provider) StreamGenerate(prompt string, onChunk func(string) error) error {
	switch classifyEndpoint(p.Endpoint) {
	case endpointKindOpenAI:
		return p.streamChatCompletion(prompt, p.Endpoint, onChunk)
	case endpointKindOllama:
		return p.streamOllama(prompt, p.Endpoint, onChunk)
	}

	chatURL, err := buildEndpointURL(p.Endpoint, openAIPath)
	if err != nil {
		return err
	}
	if err := p.streamChatCompletion(prompt, chatURL, onChunk); err == nil {
		return nil
	} else if !shouldFallback(err) {
		return err
	}

	ollamaURL, ollamaErr := buildEndpointURL(p.Endpoint, ollamaPath)
	if ollamaErr != nil {
		return ollamaErr
	}
	if err := p.streamOllama(prompt, ollamaURL, onChunk); err == nil {
		return nil
	} else if !shouldFallback(err) {
		return err
	}

	response, err := p.Generate(prompt)
	if err != nil {
		return err
	}
	return onChunk(response)
}

func (p *Provider) generateOllama(prompt, endpoint string) (string, error) {
	body, err := json.Marshal(generateRequest{
		Model:  p.Model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("local: marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("local: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("local: send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("local: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", newEndpointError("local: ollama request failed", resp.StatusCode, raw)
	}

	var gr generateResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return "", &endpointError{message: fmt.Sprintf("local: parse ollama response: %v", err), fallback: true}
	}
	if gr.Error != "" {
		return "", fmt.Errorf("local: model error: %s", gr.Error)
	}
	return gr.Response, nil
}

func (p *Provider) streamOllama(prompt, endpoint string, onChunk func(string) error) error {
	body, err := json.Marshal(generateRequest{
		Model:  p.Model,
		Prompt: prompt,
		Stream: true,
	})
	if err != nil {
		return fmt.Errorf("local: marshal stream request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("local: create stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("local: send stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("local: read stream response: %w", readErr)
		}
		return newEndpointError("local: ollama stream request failed", resp.StatusCode, raw)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var chunk generateStreamResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return &endpointError{message: fmt.Sprintf("local: parse ollama stream response: %v", err), fallback: true}
		}
		if chunk.Error != "" {
			return fmt.Errorf("local: model error: %s", chunk.Error)
		}
		if chunk.Response != "" {
			if err := onChunk(chunk.Response); err != nil {
				return err
			}
		}
		if chunk.Done {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("local: read ollama stream: %w", err)
	}
	return nil
}

func (p *Provider) generateChatCompletion(prompt, endpoint string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:    p.Model,
		Messages: []message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", fmt.Errorf("local: marshal chat request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("local: create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("local: send chat request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("local: read chat response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", newEndpointError("local: openai-compatible request failed", resp.StatusCode, raw)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", &endpointError{message: fmt.Sprintf("local: parse chat response: %v", err), fallback: true}
	}
	if cr.Error != nil {
		return "", fmt.Errorf("local: API error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("local: no choices in response")
	}
	return cr.Choices[0].Message.Content, nil
}

func (p *Provider) streamChatCompletion(prompt, endpoint string, onChunk func(string) error) error {
	body, err := json.Marshal(chatRequest{
		Model:    p.Model,
		Messages: []message{{Role: "user", Content: prompt}},
		Stream:   true,
	})
	if err != nil {
		return fmt.Errorf("local: marshal chat stream request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("local: create chat stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("local: send chat stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("local: read chat stream response: %w", readErr)
		}
		return newEndpointError("local: openai-compatible stream request failed", resp.StatusCode, raw)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return &endpointError{message: "local: chat stream endpoint did not return text/event-stream", fallback: true}
	}

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
			return nil
		}

		var chunk chatStreamResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return &endpointError{message: fmt.Sprintf("local: parse chat stream response: %v", err), fallback: true}
		}
		if chunk.Error != nil {
			return fmt.Errorf("local: API error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if content := chunk.Choices[0].Delta.Content; content != "" {
			if err := onChunk(content); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("local: read chat stream: %w", err)
	}
	return nil
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

// ── wire types ───────────────────────────────────────────────────────────────

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

type generateStreamResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
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
