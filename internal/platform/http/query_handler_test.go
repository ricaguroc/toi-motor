package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	platformhttp "github.com/ricaguroc/toi-motor/internal/platform/http"
	"github.com/ricaguroc/toi-motor/internal/query"
)

// ---------------------------------------------------------------------------
// Stub infrastructure for query tests
// ---------------------------------------------------------------------------

// stubQueryService implements the same Query method as query.QueryService so we
// can inject controlled responses via the handler. Because QueryHandler accepts
// *query.QueryService (concrete type), we route requests through a real
// QueryService that is backed by stub Retriever / LanguageModel implementations.
// ---------------------------------------------------------------------------

// stubRetriever satisfies query.Retriever and returns a fixed set of chunks.
type stubRetriever struct {
	chunks []query.RetrievedChunk
	err    error
}

func (s *stubRetriever) Retrieve(_ context.Context, _ string, _ query.RetrievalFilter) ([]query.RetrievedChunk, error) {
	return s.chunks, s.err
}

// stubLLM satisfies query.LanguageModel and returns a fixed completion.
type stubLLM struct {
	response query.CompletionResponse
	err      error
}

func (s *stubLLM) Complete(_ context.Context, _ query.CompletionRequest) (query.CompletionResponse, error) {
	return s.response, s.err
}

// newQueryRouter builds a chi router with a *query.QueryService backed by the
// given retriever and LLM stubs. Pass nil stubs to get a nil queryService
// (simulates unconfigured engine).
func newQueryRouter(retriever query.Retriever, llm query.LanguageModel) http.Handler {
	var svc *query.QueryService
	if retriever != nil && llm != nil {
		svc = query.NewQueryService(retriever, llm)
	}
	authStore := newMockAuthStore()
	return platformhttp.NewRouter(nil, svc, nil, authStore, nil)
}

// validLLMResponse returns a JSON completion that the service can parse.
func validLLMResponse() string {
	b, _ := json.Marshal(map[string]any{
		"answer":             "Hubo 3 incidentes en el turno.",
		"confidence":        "high",
		"records_cited":     []string{"rec-001", "rec-002"},
		"gaps":              nil,
		"suggested_followup": []string{},
	})
	return string(b)
}

// doQueryRequest fires an authenticated POST /api/v1/query.
func doQueryRequest(t *testing.T, router http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/query", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-key")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// POST /api/v1/query with no body → 422 (empty q fails validation inside service).
func TestQueryHandler_NoBody_Returns422(t *testing.T) {
	t.Parallel()

	// Retriever and LLM won't be reached because validation fires first.
	router := newQueryRouter(&stubRetriever{}, &stubLLM{})
	rr := doQueryRequest(t, router, nil)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want 422 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, rr, &resp)
	if resp["error"] == "" {
		t.Errorf("expected non-empty error field in response")
	}
}

// POST /api/v1/query with valid q → 200 with answer, confidence, records_cited.
func TestQueryHandler_ValidQuery_Returns200(t *testing.T) {
	t.Parallel()

	chunks := []query.RetrievedChunk{
		{RecordID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), ChunkText: "Texto del registro 1", Score: 0.95},
	}
	retriever := &stubRetriever{chunks: chunks}
	llm := &stubLLM{response: query.CompletionResponse{RawContent: validLLMResponse()}}

	router := newQueryRouter(retriever, llm)
	rr := doQueryRequest(t, router, map[string]any{"q": "¿Cuántos incidentes hubo en el turno?"})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp query.QueryResponse
	decodeJSON(t, rr, &resp)

	if resp.Answer == "" {
		t.Error("answer must not be empty")
	}
	if resp.Confidence == "" {
		t.Error("confidence must not be empty")
	}
	if resp.RecordsCited == nil {
		t.Error("records_cited must not be nil")
	}
}

// POST /api/v1/query when LLM returns an error → 500 with llm_unavailable.
func TestQueryHandler_LLMUnavailable_Returns500(t *testing.T) {
	t.Parallel()

	chunks := []query.RetrievedChunk{
		{RecordID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), ChunkText: "Texto del registro", Score: 0.90},
	}
	retriever := &stubRetriever{chunks: chunks}
	llm := &stubLLM{err: fmt.Errorf("connection refused")}

	router := newQueryRouter(retriever, llm)
	rr := doQueryRequest(t, router, map[string]any{"q": "¿Estado del sistema?"})

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	decodeJSON(t, rr, &resp)
	if resp["error"] != "llm_unavailable" {
		t.Errorf("error field: got %q, want %q", resp["error"], "llm_unavailable")
	}
}

// POST /api/v1/query when LLM returns unparseable JSON → 500 with llm_parse_error.
func TestQueryHandler_LLMParseError_Returns500(t *testing.T) {
	t.Parallel()

	chunks := []query.RetrievedChunk{
		{RecordID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), ChunkText: "Texto del registro", Score: 0.88},
	}
	retriever := &stubRetriever{chunks: chunks}
	// Return something that isn't valid JSON for the LLM response parser.
	llm := &stubLLM{response: query.CompletionResponse{RawContent: "not valid json at all"}}

	router := newQueryRouter(retriever, llm)
	rr := doQueryRequest(t, router, map[string]any{"q": "¿Estado del sistema?"})

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	decodeJSON(t, rr, &resp)
	if resp["error"] != "llm_parse_error" {
		t.Errorf("error field: got %q, want %q", resp["error"], "llm_parse_error")
	}
}

// POST /api/v1/query when query engine is not configured → 503.
func TestQueryHandler_EngineNotConfigured_Returns503(t *testing.T) {
	t.Parallel()

	// nil stubs → newQueryRouter passes nil queryService to the router.
	router := newQueryRouter(nil, nil)
	rr := doQueryRequest(t, router, map[string]any{"q": "¿Estado del sistema?"})

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	decodeJSON(t, rr, &resp)
	if resp["error"] != "query_engine_not_configured" {
		t.Errorf("error field: got %q, want %q", resp["error"], "query_engine_not_configured")
	}
}
