// Package anthropic implements query.LanguageModel using the Anthropic Messages API.
package anthropic

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

// Compile-time interface check.
var _ query.LanguageModel = (*Client)(nil)

// Client calls the Anthropic /v1/messages endpoint.
type Client struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
}

// NewClient returns a ready-to-use Anthropic LLM client.
// baseURL is typically "https://api.anthropic.com".
func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
	}
}

// --- Request/Response types for Anthropic Messages API ---

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []message `json:"messages"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type messagesResponse struct {
	Content []contentBlock `json:"content"`
	Role    string         `json:"role"`
}

// Complete sends the system prompt and user message to Anthropic and returns
// the raw assistant response text.
func (c *Client) Complete(ctx context.Context, req query.CompletionRequest) (query.CompletionResponse, error) {
	body := messagesRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    req.SystemPrompt,
		Messages: []message{
			{Role: "user", Content: req.UserMessage},
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return query.CompletionResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return query.CompletionResponse{}, fmt.Errorf("anthropic: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return query.CompletionResponse{}, fmt.Errorf("anthropic: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return query.CompletionResponse{}, fmt.Errorf("anthropic: unexpected status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var result messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return query.CompletionResponse{}, fmt.Errorf("anthropic: decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return query.CompletionResponse{}, fmt.Errorf("anthropic: empty content in response")
	}

	// Concatenate all text blocks (usually just one).
	var sb strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}

	content := strings.TrimSpace(sb.String())
	if content == "" {
		return query.CompletionResponse{}, fmt.Errorf("anthropic: no text content in response")
	}

	return query.CompletionResponse{RawContent: content}, nil
}
