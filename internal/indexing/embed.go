package indexing

import "context"

// Embedder converts raw text into dense vector representations.
// Implementations are provided by platform adapters (e.g. Ollama, OpenAI).
// Callers are responsible for batching; the adapter sends whatever it receives.
type Embedder interface {
	// Embed returns one embedding vector per input text, in the same order.
	// Returns an error if the underlying model call fails or the response is malformed.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the fixed length of each embedding vector.
	// This value is used when creating the vector index schema.
	Dimensions() int
}
