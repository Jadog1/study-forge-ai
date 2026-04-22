package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/repository"
	"github.com/studyforge/study-agent/plugins"
)

// StreamEventKind classifies streaming events emitted during chat generation.
type StreamEventKind string

const (
	StreamEventChunk       StreamEventKind = "chunk"
	StreamEventActionStart StreamEventKind = "action-start"
	StreamEventActionDone  StreamEventKind = "action-done"
)

// StreamEvent carries a single streaming event from the chat agent loop.
type StreamEvent struct {
	Kind   StreamEventKind
	Text   string
	Label  string
	Detail string
	Err    error
}

// AskStream sends a prompt and emits the final reply in chunks.
// Tool-aware chat resolves any intermediate tool calls before chunking output.
func AskStream(provider plugins.AIProvider, cfg *config.Config, className, prompt string, onEvent func(StreamEvent) error) error {
	return NewService(nil).AskStream(provider, cfg, className, prompt, onEvent)
}

// AskStreamWithMode sends a prompt and emits response events using the supplied mode.
func AskStreamWithMode(provider plugins.AIProvider, cfg *config.Config, className, prompt string, mode Mode, onEvent func(StreamEvent) error) error {
	return NewService(nil).AskStreamWithMode(provider, cfg, className, prompt, mode, onEvent)
}

// AskStreamWithStore is like AskStream but uses the provided storage abstraction.
func AskStreamWithStore(provider plugins.AIProvider, cfg *config.Config, className, prompt string, store repository.Store, onEvent func(StreamEvent) error) error {
	return NewService(store).AskStream(provider, cfg, className, prompt, onEvent)
}

// AskStreamWithStoreAndMode is like AskStreamWithStore but uses a mode override.
func AskStreamWithStoreAndMode(provider plugins.AIProvider, cfg *config.Config, className, prompt string, mode Mode, store repository.Store, onEvent func(StreamEvent) error) error {
	return NewService(store).AskStreamWithMode(provider, cfg, className, prompt, mode, onEvent)
}

func askStreamWithStore(provider plugins.AIProvider, cfg *config.Config, className, prompt string, mode Mode, store repository.Store, history []ChatMessage, onEvent func(StreamEvent) error) error {
	request, err := buildConversationWithStore(cfg, className, prompt, mode, store, history)
	if err != nil {
		return err
	}
	_, err = runAgent(provider, cfg, className, request, store, onEvent)
	return err
}

func generateAgentResponse(provider plugins.AIProvider, request plugins.ChatCompletionRequest, onEvent func(StreamEvent) error) (string, bool, plugins.GenerateResult, error) {
	if onEvent == nil {
		text, result, err := chatGenerateWithMetadata(provider, request)
		return text, false, result, err
	}

	if usageStreamer, ok := provider.(plugins.StreamingChatUsageAwareAIProvider); ok {
		result, err := usageStreamer.StreamGenerateChatWithMetadata(request, func(part string) error {
			return onEvent(StreamEvent{Kind: StreamEventChunk, Text: part})
		})
		if err != nil {
			return "", true, plugins.GenerateResult{}, err
		}
		return result.Text, true, result, nil
	}

	if streamer, ok := provider.(plugins.StreamingChatAIProvider); ok {
		resp, err := streamProviderChatResponse(streamer, request, onEvent)
		if err != nil {
			return "", true, plugins.GenerateResult{}, err
		}
		requestText := renderConversationAsPrompt(request)
		inputTokens := len(strings.Fields(requestText))
		outputTokens := len(strings.Fields(resp))
		result := plugins.GenerateResult{
			Text: resp,
			Usage: plugins.TokenUsage{
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				TotalTokens:  inputTokens + outputTokens,
			},
			Metadata: plugins.CallMetadata{
				Provider: provider.Name(),
				Model:    provider.Model(),
				At:       time.Now().UTC(),
			},
		}
		return resp, true, result, nil
	}

	streamer, ok := provider.(plugins.StreamingAIProvider)
	if !ok {
		text, result, err := chatGenerateWithMetadata(provider, request)
		return text, false, result, err
	}
	if usageStreamer, ok := provider.(plugins.StreamingUsageAwareAIProvider); ok {
		result, err := streamProviderResponseWithMetadata(usageStreamer, request, onEvent)
		if err != nil {
			return "", true, plugins.GenerateResult{}, err
		}
		return result.Text, true, result, nil
	}

	resp, err := streamProviderResponse(streamer, request, onEvent)
	if err != nil {
		return "", true, plugins.GenerateResult{}, err
	}
	prompt := renderConversationAsPrompt(request)
	inputTokens := len(strings.Fields(prompt))
	outputTokens := len(strings.Fields(resp))
	result := plugins.GenerateResult{
		Text: resp,
		Usage: plugins.TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider: provider.Name(),
			Model:    provider.Model(),
			At:       time.Now().UTC(),
		},
	}
	return resp, true, result, nil
}


func chatGenerateWithMetadata(provider plugins.AIProvider, request plugins.ChatCompletionRequest) (string, plugins.GenerateResult, error) {
	if usageAware, ok := provider.(plugins.ChatUsageAwareAIProvider); ok {
		result, err := usageAware.GenerateChatWithMetadata(request)
		if err != nil {
			return "", plugins.GenerateResult{}, err
		}
		if result.Metadata.At.IsZero() {
			result.Metadata.At = time.Now().UTC()
		}
		if result.Metadata.Provider == "" {
			result.Metadata.Provider = provider.Name()
		}
		return result.Text, result, nil
	}
	if chatProvider, ok := provider.(plugins.ChatAIProvider); ok {
		text, err := chatProvider.GenerateChat(request)
		if err != nil {
			return "", plugins.GenerateResult{}, err
		}
		prompt := renderConversationAsPrompt(request)
		inputTokens := len(strings.Fields(prompt))
		outputTokens := len(strings.Fields(text))
		return text, plugins.GenerateResult{
			Text: text,
			Usage: plugins.TokenUsage{
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				TotalTokens:  inputTokens + outputTokens,
			},
			Metadata: plugins.CallMetadata{
				Provider: provider.Name(),
				Model:    provider.Model(),
				At:       time.Now().UTC(),
			},
		}, nil
	}
	prompt := renderConversationAsPrompt(request)
	if usageAware, ok := provider.(plugins.UsageAwareAIProvider); ok {
		result, err := usageAware.GenerateWithMetadata(prompt)
		if err != nil {
			return "", plugins.GenerateResult{}, err
		}
		if result.Metadata.At.IsZero() {
			result.Metadata.At = time.Now().UTC()
		}
		if result.Metadata.Provider == "" {
			result.Metadata.Provider = provider.Name()
		}
		return result.Text, result, nil
	}
	text, err := provider.Generate(prompt)
	if err != nil {
		return "", plugins.GenerateResult{}, err
	}
	inputTokens := len(strings.Fields(prompt))
	outputTokens := len(strings.Fields(text))
	return text, plugins.GenerateResult{
		Text: text,
		Usage: plugins.TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
		Metadata: plugins.CallMetadata{
			Provider: provider.Name(),
			Model:    provider.Model(),
			At:       time.Now().UTC(),
		},
	}, nil
}


func streamProviderChatResponse(provider plugins.StreamingChatAIProvider, request plugins.ChatCompletionRequest, onEvent func(StreamEvent) error) (string, error) {
	return streamProviderResponseWith(renderConversationAsPrompt(request), onEvent, func(onChunk func(string) error) error {
		return provider.StreamGenerateChat(request, onChunk)
	})
}

func streamProviderResponse(provider plugins.StreamingAIProvider, request plugins.ChatCompletionRequest, onEvent func(StreamEvent) error) (string, error) {
	prompt := renderConversationAsPrompt(request)
	return streamProviderResponseWith(prompt, onEvent, func(onChunk func(string) error) error {
		return provider.StreamGenerate(prompt, onChunk)
	})
}

func streamProviderResponseWithMetadata(provider plugins.StreamingUsageAwareAIProvider, request plugins.ChatCompletionRequest, onEvent func(StreamEvent) error) (plugins.GenerateResult, error) {
	prompt := renderConversationAsPrompt(request)
	var result plugins.GenerateResult
	resp, err := streamProviderResponseWith(prompt, onEvent, func(onChunk func(string) error) error {
		streamResult, streamErr := provider.StreamGenerateWithMetadata(prompt, onChunk)
		result = streamResult
		return streamErr
	})
	if err != nil {
		return plugins.GenerateResult{}, err
	}
	if strings.TrimSpace(result.Text) == "" {
		result.Text = resp
	}
	if result.Usage.TotalTokens == 0 && (result.Usage.InputTokens == 0 && result.Usage.OutputTokens == 0) {
		inputTokens := len(strings.Fields(prompt))
		outputTokens := len(strings.Fields(result.Text))
		result.Usage = plugins.TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		}
	}
	if result.Metadata.Provider == "" {
		result.Metadata.Provider = provider.Name()
	}
	if result.Metadata.Model == "" {
		result.Metadata.Model = provider.Model()
	}
	if result.Metadata.At.IsZero() {
		result.Metadata.At = time.Now().UTC()
	}
	return result, nil
}

func streamProviderResponseWith(prompt string, onEvent func(StreamEvent) error, streamFn func(onChunk func(string) error) error) (string, error) {
	var full strings.Builder
	var pending strings.Builder
	released := false
	const minBufferBeforeRelease = 100 // Require at least 100 chars before streaming

	err := streamFn(func(part string) error {
		if part == "" {
			return nil
		}

		full.WriteString(part)
		if released {
			return onEvent(StreamEvent{Kind: StreamEventChunk, Text: part})
		}

		pending.WriteString(part)
		buffered := pending.String()

		// Never release if tool call appears anywhere in buffer
		if strings.Contains(buffered, toolCallStartTag) {
			return nil
		}

		// Keep buffering if it looks like a tool call might be starting
		if looksLikeToolCallPrefix(buffered) {
			return nil
		}

		// Wait for minimum buffer size before releasing to reduce risk of
		// emitting preamble prose before a tool call that appears later
		if len(buffered) < minBufferBeforeRelease {
			return nil
		}

		released = true
		pending.Reset()
		return onEvent(StreamEvent{Kind: StreamEventChunk, Text: buffered})
	})
	if err != nil {
		return "", err
	}

	resp := full.String()
	// Only emit buffered text if the complete response doesn't contain a tool call
	if !released && !strings.Contains(resp, toolCallStartTag) {
		buffered := pending.String()
		if buffered != "" {
			if err := onEvent(StreamEvent{Kind: StreamEventChunk, Text: buffered}); err != nil {
				return "", fmt.Errorf("chat stream callback: %w", err)
			}
		}
	}
	return resp, nil
}

func looksLikeToolCallPrefix(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, toolCallStartTag) {
		return true
	}
	return strings.HasPrefix(toolCallStartTag, trimmed)
}

func isToolCallResponse(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), toolCallStartTag)
}

func emitChunked(text string, size int, onEvent func(StreamEvent) error) error {
	for _, chunk := range chunkText(text, size) {
		if err := onEvent(StreamEvent{Kind: StreamEventChunk, Text: chunk}); err != nil {
			return fmt.Errorf("chat stream callback: %w", err)
		}
	}
	if text == "" {
		if err := onEvent(StreamEvent{Kind: StreamEventChunk, Text: ""}); err != nil {
			return fmt.Errorf("chat stream callback: %w", err)
		}
	}
	return nil
}

func chunkText(s string, size int) []string {
	if size <= 0 || len(s) <= size {
		return []string{s}
	}
	chunks := make([]string, 0, (len(s)+size-1)/size)
	for len(s) > size {
		chunks = append(chunks, s[:size])
		s = s[size:]
	}
	if s != "" {
		chunks = append(chunks, s)
	}
	return chunks
}
