package query

import "context"

// CompletionRequest holds the inputs for a single LLM completion call.
type CompletionRequest struct {
	SystemPrompt string
	UserMessage  string
}

// CompletionResponse holds the raw text returned by the LLM.
type CompletionResponse struct {
	RawContent string
}

// LanguageModel is the domain port for LLM completion. Implementations live in
// the platform layer and must satisfy this interface at compile time.
type LanguageModel interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
