// Package plugins defines the AIProvider interface and is the integration
// point for all AI backend implementations.
package plugins

import "time"

// ChatMessage represents one role-tagged turn for providers with native chat APIs.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest carries a system instruction plus ordered chat turns.
type ChatCompletionRequest struct {
	System   string        `json:"system,omitempty"`
	Messages []ChatMessage `json:"messages"`
}

// AIProvider is the common interface every AI backend must satisfy.
// Implementations live in sub-packages: openai, claude, local.
type AIProvider interface {
	// Generate sends a plain-text prompt and returns the model's response.
	Generate(prompt string) (string, error)
	// Name returns a human-readable identifier for the provider.
	Name() string
	// Model returns the model identifier used by this provider.
	Model() string
	// Disabled returns true when the provider cannot accept requests,
	// e.g. the API key is not configured or the endpoint is unreachable.
	Disabled() bool
}

// TokenUsage captures token and cost details for a single provider call.
// Cost values are optional because not all providers return them.
type TokenUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
}

// CallMetadata captures provider/model metadata for auditing and billing.
type CallMetadata struct {
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	RequestID string    `json:"request_id,omitempty"`
	At        time.Time `json:"at"`
}

// GenerateResult includes model output plus optional metadata.
type GenerateResult struct {
	Text     string       `json:"text"`
	Usage    TokenUsage   `json:"usage"`
	Metadata CallMetadata `json:"metadata"`
}

// UsageAwareAIProvider is an optional extension for providers that can return
// model and token metadata for each text generation call.
type UsageAwareAIProvider interface {
	AIProvider
	GenerateWithMetadata(prompt string) (GenerateResult, error)
}

// ChatAIProvider is an optional extension for providers with native chat APIs.
type ChatAIProvider interface {
	AIProvider
	GenerateChat(request ChatCompletionRequest) (string, error)
}

// ChatUsageAwareAIProvider extends chat generation with metadata.
type ChatUsageAwareAIProvider interface {
	ChatAIProvider
	GenerateChatWithMetadata(request ChatCompletionRequest) (GenerateResult, error)
}

// StreamingAIProvider is an optional extension interface for providers that
// can emit text incrementally as it is generated.
type StreamingAIProvider interface {
	AIProvider
	// StreamGenerate sends a plain-text prompt and invokes onChunk for each
	// piece of generated text in order.
	StreamGenerate(prompt string, onChunk func(string) error) error
}

// StreamingChatAIProvider streams responses for providers with native chat APIs.
type StreamingChatAIProvider interface {
	ChatAIProvider
	StreamGenerateChat(request ChatCompletionRequest, onChunk func(string) error) error
}

// StreamingUsageAwareAIProvider extends streaming generation with usage
// metadata for providers that can report token counts on streamed responses.
type StreamingUsageAwareAIProvider interface {
	StreamingAIProvider
	// StreamGenerateWithMetadata streams chunks and returns a final generation
	// result including usage and metadata when available.
	StreamGenerateWithMetadata(prompt string, onChunk func(string) error) (GenerateResult, error)
}

// StreamingChatUsageAwareAIProvider extends streaming chat generation with metadata.
type StreamingChatUsageAwareAIProvider interface {
	StreamingChatAIProvider
	StreamGenerateChatWithMetadata(request ChatCompletionRequest, onChunk func(string) error) (GenerateResult, error)
}

// EmbeddingProvider is an optional extension interface for providers that can
// produce numeric embeddings for one or more text inputs.
type EmbeddingProvider interface {
	// Name returns a human-readable identifier for the provider.
	Name() string
	// Model returns the model identifier used by this provider.
	Model() string
	// Disabled returns true when the provider cannot accept requests.
	Disabled() bool
	// Embed returns one embedding vector per input text, preserving order.
	Embed(input []string) ([][]float64, error)
}

// EmbedResult includes vectors plus optional metadata.
type EmbedResult struct {
	Vectors  [][]float64  `json:"vectors"`
	Usage    TokenUsage   `json:"usage"`
	Metadata CallMetadata `json:"metadata"`
}

// UsageAwareEmbeddingProvider is an optional extension for embedding
// providers that can return token/model metadata for each request.
type UsageAwareEmbeddingProvider interface {
	EmbeddingProvider
	EmbedWithMetadata(input []string) (EmbedResult, error)
}
