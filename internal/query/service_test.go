package query

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// stubRetriever returns a fixed slice of chunks or an error.
type stubRetriever struct {
	chunks      []RetrievedChunk
	err         error
	capturedReq *retrieverCall
}

type retrieverCall struct {
	queryText string
	filter    RetrievalFilter
}

func (r *stubRetriever) Retrieve(_ context.Context, queryText string, filter RetrievalFilter) ([]RetrievedChunk, error) {
	r.capturedReq = &retrieverCall{queryText: queryText, filter: filter}
	if r.err != nil {
		return nil, r.err
	}
	return r.chunks, nil
}

// stubLLM returns a fixed raw string or an error.
type stubLLM struct {
	raw     string
	err     error
	called  bool
	lastReq *CompletionRequest
}

func (l *stubLLM) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	l.called = true
	l.lastReq = &req
	if l.err != nil {
		return CompletionResponse{}, l.err
	}
	return CompletionResponse{RawContent: l.raw}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validChunk(recordID uuid.UUID) RetrievedChunk {
	return RetrievedChunk{
		RecordID:   recordID,
		ChunkIndex: 0,
		ChunkText:  "some operational text",
		Score:      0.9,
		OccurredAt: time.Now().UTC(),
		RecordType: "event",
	}
}

func goodLLMPayload(recordID uuid.UUID) string {
	resp := llmResponse{
		Answer:            "The lot was inspected on Monday.",
		Confidence:        "high",
		RecordsCited:      []string{recordID.String()},
		SuggestedFollowup: []string{"What happened next?"},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// ---------------------------------------------------------------------------
// Validation tests
// ---------------------------------------------------------------------------

func TestQueryService_EmptyQ_ReturnsErrValidation(t *testing.T) {
	t.Parallel()
	svc := NewQueryService(&stubRetriever{}, &stubLLM{})

	_, err := svc.Query(context.Background(), QueryRequest{Q: ""})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestQueryService_QTooLong_ReturnsErrValidation(t *testing.T) {
	t.Parallel()
	svc := NewQueryService(&stubRetriever{}, &stubLLM{})

	longQ := strings.Repeat("a", 2001)
	_, err := svc.Query(context.Background(), QueryRequest{Q: longQ})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Entity scope tests
// ---------------------------------------------------------------------------

func TestQueryService_EntityScopeSet_SkipsExtraction(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id)}}
	llm := &stubLLM{raw: goodLLMPayload(id)}

	scope := "lot:L-2024-999"
	svc := NewQueryService(ret, llm)
	_, err := svc.Query(context.Background(), QueryRequest{
		Q:           "What happened?",
		EntityScope: &scope,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ret.capturedReq == nil {
		t.Fatal("retriever was never called")
	}
	if ret.capturedReq.filter.EntityRef == nil || *ret.capturedReq.filter.EntityRef != scope {
		t.Errorf("expected entity ref %q, got %v", scope, ret.capturedReq.filter.EntityRef)
	}
}

func TestQueryService_NoEntityInQuery_RetrieverCalledWithoutEntityFilter(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id)}}
	llm := &stubLLM{raw: goodLLMPayload(id)}

	svc := NewQueryService(ret, llm)
	_, err := svc.Query(context.Background(), QueryRequest{Q: "What is the status?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ret.capturedReq == nil {
		t.Fatal("retriever was never called")
	}
	if ret.capturedReq.filter.EntityRef != nil {
		t.Errorf("expected nil entity ref, got %v", ret.capturedReq.filter.EntityRef)
	}
}

func TestQueryService_EntityInQuery_RetrieverCalledWithExtractedEntity(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id)}}
	llm := &stubLLM{raw: goodLLMPayload(id)}

	svc := NewQueryService(ret, llm)
	_, err := svc.Query(context.Background(), QueryRequest{Q: "Show me lote L-2024-001 history"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ret.capturedReq == nil {
		t.Fatal("retriever was never called")
	}
	if ret.capturedReq.filter.EntityRef == nil {
		t.Fatal("expected entity ref to be set from extraction, got nil")
	}
	if *ret.capturedReq.filter.EntityRef != "lot:L-2024-001" {
		t.Errorf("expected lot:L-2024-001, got %q", *ret.capturedReq.filter.EntityRef)
	}
}

// ---------------------------------------------------------------------------
// Zero-chunk path
// ---------------------------------------------------------------------------

func TestQueryService_NoChunksReturned_LowConfidenceNoLLMCall(t *testing.T) {
	t.Parallel()

	ret := &stubRetriever{chunks: []RetrievedChunk{}}
	llm := &stubLLM{}

	svc := NewQueryService(ret, llm)
	resp, err := svc.Query(context.Background(), QueryRequest{Q: "Any events?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if llm.called {
		t.Error("LLM should NOT be called when no chunks are retrieved")
	}
	if resp.Confidence != "low" {
		t.Errorf("expected confidence low, got %q", resp.Confidence)
	}
	if resp.RetrievedCount != 0 {
		t.Errorf("expected retrieved_count 0, got %d", resp.RetrievedCount)
	}
	if resp.Answer == "" {
		t.Error("expected a non-empty answer for the no-records case")
	}
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestQueryService_ValidQuery_LLMCalledResponseParsed(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id)}}
	llm := &stubLLM{raw: goodLLMPayload(id)}

	svc := NewQueryService(ret, llm)
	resp, err := svc.Query(context.Background(), QueryRequest{Q: "Tell me about " + id.String()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !llm.called {
		t.Error("expected LLM to be called")
	}
	if resp.Confidence != "high" {
		t.Errorf("expected confidence high, got %q", resp.Confidence)
	}
	if resp.RetrievedCount != 1 {
		t.Errorf("expected retrieved_count 1, got %d", resp.RetrievedCount)
	}
}

// ---------------------------------------------------------------------------
// LLM error paths
// ---------------------------------------------------------------------------

func TestQueryService_LLMReturnsInvalidJSON_ErrLLMParseError(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id)}}
	llm := &stubLLM{raw: "not json at all {{{"}

	svc := NewQueryService(ret, llm)
	_, err := svc.Query(context.Background(), QueryRequest{Q: "What happened?"})
	if !errors.Is(err, ErrLLMParseError) {
		t.Fatalf("expected ErrLLMParseError, got %v", err)
	}
}

func TestQueryService_LLMReturnsError_ErrLLMUnavailable(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id)}}
	llm := &stubLLM{err: errors.New("connection timeout")}

	svc := NewQueryService(ret, llm)
	_, err := svc.Query(context.Background(), QueryRequest{Q: "What happened?"})
	if !errors.Is(err, ErrLLMUnavailable) {
		t.Fatalf("expected ErrLLMUnavailable, got %v", err)
	}
}

func TestQueryService_LLMInvalidConfidence_ErrLLMParseError(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id)}}

	badPayload := llmResponse{
		Answer:       "some answer",
		Confidence:   "very-sure", // invalid value
		RecordsCited: []string{id.String()},
	}
	b, _ := json.Marshal(badPayload)
	llm := &stubLLM{raw: string(b)}

	svc := NewQueryService(ret, llm)
	_, err := svc.Query(context.Background(), QueryRequest{Q: "What happened?"})
	if !errors.Is(err, ErrLLMParseError) {
		t.Fatalf("expected ErrLLMParseError, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// records_cited propagation
// ---------------------------------------------------------------------------

func TestQueryService_RecordsCitedMatchChunks(t *testing.T) {
	t.Parallel()

	id1 := uuid.New()
	id2 := uuid.New()

	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id1), validChunk(id2)}}

	gaps := "none"
	payload := llmResponse{
		Answer:            "Two records found.",
		Confidence:        "medium",
		RecordsCited:      []string{id1.String(), id2.String()},
		Gaps:              &gaps,
		SuggestedFollowup: []string{},
	}
	b, _ := json.Marshal(payload)
	llm := &stubLLM{raw: string(b)}

	svc := NewQueryService(ret, llm)
	resp, err := svc.Query(context.Background(), QueryRequest{Q: "Show all records"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.RecordsCited) != 2 {
		t.Errorf("expected 2 records_cited, got %d", len(resp.RecordsCited))
	}

	citedSet := make(map[string]struct{}, len(resp.RecordsCited))
	for _, r := range resp.RecordsCited {
		citedSet[r] = struct{}{}
	}
	for _, id := range []uuid.UUID{id1, id2} {
		if _, ok := citedSet[id.String()]; !ok {
			t.Errorf("expected record %s in records_cited", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Timing
// ---------------------------------------------------------------------------

func TestQueryService_QueryMs_IsPositive(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	ret := &stubRetriever{chunks: []RetrievedChunk{validChunk(id)}}
	llm := &stubLLM{raw: goodLLMPayload(id)}

	svc := NewQueryService(ret, llm)
	resp, err := svc.Query(context.Background(), QueryRequest{Q: "How long does this take?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// query_ms must be >= 0 (0 is acceptable for very fast in-process calls)
	if resp.QueryMs < 0 {
		t.Errorf("expected query_ms >= 0, got %d", resp.QueryMs)
	}
}

// ---------------------------------------------------------------------------
// Default limit / cap
// ---------------------------------------------------------------------------

func TestQueryService_DefaultLimit_Is10(t *testing.T) {
	t.Parallel()

	ret := &stubRetriever{chunks: []RetrievedChunk{}}
	llm := &stubLLM{}

	svc := NewQueryService(ret, llm)
	_, _ = svc.Query(context.Background(), QueryRequest{Q: "anything"})

	if ret.capturedReq == nil {
		t.Fatal("retriever not called")
	}
	if ret.capturedReq.filter.Limit != 10 {
		t.Errorf("expected default limit 10, got %d", ret.capturedReq.filter.Limit)
	}
}

func TestQueryService_LimitCappedAt20(t *testing.T) {
	t.Parallel()

	ret := &stubRetriever{chunks: []RetrievedChunk{}}
	llm := &stubLLM{}

	svc := NewQueryService(ret, llm)
	_, _ = svc.Query(context.Background(), QueryRequest{Q: "anything", LimitRecords: 100})

	if ret.capturedReq == nil {
		t.Fatal("retriever not called")
	}
	if ret.capturedReq.filter.Limit != 20 {
		t.Errorf("expected limit capped at 20, got %d", ret.capturedReq.filter.Limit)
	}
}
