package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ricaguroc/toi-motor/internal/query"
)

// Ensure OllamaLLMClient satisfies the domain port at compile time.
var _ query.LanguageModel = (*OllamaLLMClient)(nil)

// OllamaLLMClient calls the Ollama /api/chat endpoint to generate completions.
// It is safe for concurrent use.
type OllamaLLMClient struct {
	client  *http.Client
	baseURL string
	model   string
}

// NewLLMClient returns a ready-to-use OllamaLLMClient.
// baseURL must point to the Ollama server (e.g. "http://localhost:11434").
// model is the chat model to use (e.g. "llama3.1:8b").
func NewLLMClient(baseURL, model string) *OllamaLLMClient {
	return &OllamaLLMClient{
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
	}
}

// chatMessage represents a single message in the Ollama chat messages array.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the JSON body sent to POST /api/chat.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format"`
}

// chatResponseMessage is the nested message object in the Ollama chat response.
type chatResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the JSON body returned by POST /api/chat.
type chatResponse struct {
	Model   string              `json:"model"`
	Message chatResponseMessage `json:"message"`
	Done    bool                `json:"done"`
}

// Complete sends a system prompt and user message to Ollama and returns the
// raw content string from the assistant's response. stream is always false and
// format is always "json" so the model returns structured output.
func (c *OllamaLLMClient) Complete(ctx context.Context, req query.CompletionRequest) (query.CompletionResponse, error) {
	body := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserMessage},
		},
		Stream: false,
		Format: "json",
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return query.CompletionResponse{}, fmt.Errorf("ollama llm: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return query.CompletionResponse{}, fmt.Errorf("ollama llm: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return query.CompletionResponse{}, fmt.Errorf("ollama llm: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return query.CompletionResponse{}, fmt.Errorf("ollama llm: unexpected status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return query.CompletionResponse{}, fmt.Errorf("ollama llm: decode response: %w", err)
	}

	content := strings.TrimSpace(result.Message.Content)
	if content == "" {
		return query.CompletionResponse{}, fmt.Errorf("ollama llm: empty content in response")
	}

	return query.CompletionResponse{RawContent: content}, nil
}
