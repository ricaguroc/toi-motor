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

	"github.com/ricaguroc/toi-motor/internal/indexing"
)

// Ensure OllamaEmbedder satisfies the domain port at compile time.
var _ indexing.Embedder = (*OllamaEmbedder)(nil)

// OllamaEmbedder calls the Ollama /api/embed endpoint to produce dense
// vector embeddings. It is safe for concurrent use.
type OllamaEmbedder struct {
	client  *http.Client
	baseURL string
	model   string
}

// NewEmbedder returns a ready-to-use OllamaEmbedder.
// baseURL must point to the Ollama server (e.g. "http://localhost:11434").
// model is the embedding model to use (e.g. "nomic-embed-text").
func NewEmbedder(baseURL, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
	}
}

// embedRequest is the JSON body sent to POST /api/embed.
type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embedResponse is the JSON body returned by POST /api/embed.
// Ollama returns numbers as float64; we convert to float32 after decoding.
type embedResponse struct {
	Model         string      `json:"model"`
	Embeddings    [][]float64 `json:"embeddings"`
	TotalDuration int64       `json:"total_duration"`
}

// Embed sends texts to Ollama and returns one float32 vector per input text.
// The returned slice is in the same order as texts.
func (e *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// --- build request ---
	reqBody := embedRequest{
		Model: e.model,
		Input: texts,
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.baseURL+"/api/embed", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// --- execute ---
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: http request: %w", err)
	}
	defer resp.Body.Close()

	// --- check HTTP-level errors before decoding ---
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("ollama embedder: unexpected status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// --- decode response ---
	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embedder: decode response: %w", err)
	}

	// --- validate cardinality ---
	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embedder: expected %d embeddings, got %d",
			len(texts), len(result.Embeddings))
	}

	// --- convert float64 → float32 ---
	out := make([][]float32, len(result.Embeddings))
	for i, vec64 := range result.Embeddings {
		vec32 := make([]float32, len(vec64))
		for j, v := range vec64 {
			vec32[j] = float32(v)
		}
		out[i] = vec32
	}

	return out, nil
}

// Dimensions returns 768, the output size of nomic-embed-text (and compatible
// models). If the model changes, this value must be updated accordingly.
func (e *OllamaEmbedder) Dimensions() int { return 768 }
