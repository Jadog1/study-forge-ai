// Package plugins defines the AIProvider interface and is the integration
// point for all AI backend implementations.
package plugins

// AIProvider is the common interface every AI backend must satisfy.
// Implementations live in sub-packages: openai, claude, local.
type AIProvider interface {
	// Generate sends a plain-text prompt and returns the model's response.
	Generate(prompt string) (string, error)
	// Name returns a human-readable identifier for the provider.
	Name() string
	// Disabled returns true when the provider cannot accept requests,
	// e.g. the API key is not configured or the endpoint is unreachable.
	Disabled() bool
}

// StreamingAIProvider is an optional extension interface for providers that
// can emit text incrementally as it is generated.
type StreamingAIProvider interface {
	AIProvider
	// StreamGenerate sends a plain-text prompt and invokes onChunk for each
	// piece of generated text in order.
	StreamGenerate(prompt string, onChunk func(string) error) error
}
