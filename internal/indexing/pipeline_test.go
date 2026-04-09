package indexing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ricaguroc/toi-motor/internal/record"
)

// --- mock implementations ---

type mockRecordStore struct {
	rec    record.Record
	getErr error
}

func (m *mockRecordStore) Append(_ context.Context, _ record.Record) error { return nil }
func (m *mockRecordStore) GetByID(_ context.Context, _ uuid.UUID) (record.Record, error) {
	return m.rec, m.getErr
}
func (m *mockRecordStore) List(_ context.Context, _ record.Filter) (record.ListResult, error) {
	return record.ListResult{}, nil
}

type mockChunker struct {
	chunks []Chunk
	err    error
}

func (m *mockChunker) Chunk(_ record.Record, _ string) ([]Chunk, error) {
	return m.chunks, m.err
}

type mockEmbedder struct {
	// calls tracks how many times Embed has been called.
	calls      int
	// failUntil makes Embed return errEmbed for calls < failUntil.
	failUntil  int
	errEmbed   error
	embeddings [][]float32
}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	m.calls++
	if m.calls <= m.failUntil {
		return nil, m.errEmbed
	}
	if m.embeddings != nil {
		return m.embeddings[:len(texts)], nil
	}
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}

func (m *mockEmbedder) Dimensions() int { return 3 }

type mockIndexStore struct {
	upsertErr    error
	upsertCalled int
	lastChunks   []Chunk
	lastEmbeds   [][]float32
}

func (m *mockIndexStore) UpsertChunks(_ context.Context, chunks []Chunk, embeddings [][]float32) error {
	m.upsertCalled++
	m.lastChunks = chunks
	m.lastEmbeds = embeddings
	return m.upsertErr
}

func (m *mockIndexStore) SearchSimilar(_ context.Context, _ []float32, _ SearchFilter, _ int) ([]SearchResult, error) {
	return nil, nil
}

// --- helpers ---

func validRecord() record.Record {
	entity := "entity-abc"
	actor := "actor-xyz"
	return record.Record{
		ID:         uuid.New(),
		RecordID:   uuid.New(),
		OccurredAt: time.Now().UTC(),
		IngestedAt: time.Now().UTC(),
		Source:     "test-source",
		RecordType: "scan",
		EntityRef:  &entity,
		ActorRef:   &actor,
	}
}

func validEvent(recordID uuid.UUID) []byte {
	evt := IngestedEvent{
		RecordID:   recordID,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		Source:     "test-source",
		RecordType: "scan",
	}
	data, _ := json.Marshal(evt)
	return data
}

func makeChunks(r record.Record, n int) []Chunk {
	chunks := make([]Chunk, n)
	for i := range chunks {
		chunks[i] = Chunk{
			RecordID:   r.RecordID,
			ChunkIndex: i,
			Text:       "chunk text",
			RecordType: r.RecordType,
			OccurredAt: r.OccurredAt,
		}
	}
	return chunks
}

// newFastPipeline returns a pipeline with maxRetry=1 so tests don't sleep.
func newFastPipeline(store record.RecordStore, chunker Chunker, embedder Embedder, index IndexStore) *Pipeline {
	p := NewPipeline(store, chunker, embedder, index)
	p.maxRetry = 1
	return p
}

// --- tests ---

func TestPipeline_ValidRecord_UpsertCalled(t *testing.T) {
	r := validRecord()
	chunks := makeChunks(r, 2)

	store := &mockRecordStore{rec: r}
	chunker := &mockChunker{chunks: chunks}
	embedder := &mockEmbedder{}
	idx := &mockIndexStore{}

	p := newFastPipeline(store, chunker, embedder, idx)
	err := p.Process(context.Background(), validEvent(r.RecordID))
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if idx.upsertCalled != 1 {
		t.Errorf("expected UpsertChunks called once, got %d", idx.upsertCalled)
	}
	if len(idx.lastChunks) != 2 {
		t.Errorf("expected 2 chunks upserted, got %d", len(idx.lastChunks))
	}
	if len(idx.lastEmbeds) != 2 {
		t.Errorf("expected 2 embeddings upserted, got %d", len(idx.lastEmbeds))
	}
}

func TestPipeline_RecordNotFound_ErrPermanent(t *testing.T) {
	store := &mockRecordStore{getErr: record.ErrNotFound}
	chunker := &mockChunker{}
	embedder := &mockEmbedder{}
	idx := &mockIndexStore{}

	r := validRecord()
	p := newFastPipeline(store, chunker, embedder, idx)
	err := p.Process(context.Background(), validEvent(r.RecordID))

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPermanent) {
		t.Errorf("expected ErrPermanent, got: %v", err)
	}
}

func TestPipeline_MalformedJSON_ErrPermanent(t *testing.T) {
	store := &mockRecordStore{}
	chunker := &mockChunker{}
	embedder := &mockEmbedder{}
	idx := &mockIndexStore{}

	p := newFastPipeline(store, chunker, embedder, idx)
	err := p.Process(context.Background(), []byte(`{not valid json`))

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPermanent) {
		t.Errorf("expected ErrPermanent, got: %v", err)
	}
}

func TestPipeline_EmbedderFailsAllRetries_ErrTransient(t *testing.T) {
	r := validRecord()
	chunks := makeChunks(r, 1)
	embedErr := errors.New("model unavailable")

	store := &mockRecordStore{rec: r}
	chunker := &mockChunker{chunks: chunks}
	// failUntil=999 ensures every call fails within our maxRetry.
	embedder := &mockEmbedder{failUntil: 999, errEmbed: embedErr}
	idx := &mockIndexStore{}

	p := newFastPipeline(store, chunker, embedder, idx)
	err := p.Process(context.Background(), validEvent(r.RecordID))

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrTransient) {
		t.Errorf("expected ErrTransient, got: %v", err)
	}
}

func TestPipeline_EmbedderFailsThenSucceeds_Nil(t *testing.T) {
	r := validRecord()
	chunks := makeChunks(r, 1)
	embedErr := errors.New("temporary model error")

	store := &mockRecordStore{rec: r}
	chunker := &mockChunker{chunks: chunks}
	// fails on call 1, succeeds on call 2 → requires maxRetry >= 2
	embedder := &mockEmbedder{failUntil: 1, errEmbed: embedErr}
	idx := &mockIndexStore{}

	p := NewPipeline(store, chunker, embedder, idx)
	p.maxRetry = 2 // allow one retry with minimal backoff (1s delay on retry)

	// Use a context with a generous deadline so the 1s backoff passes.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := p.Process(ctx, validEvent(r.RecordID))
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if idx.upsertCalled != 1 {
		t.Errorf("expected UpsertChunks called once, got %d", idx.upsertCalled)
	}
}

func TestPipeline_IndexStoreFails_ErrTransient(t *testing.T) {
	r := validRecord()
	chunks := makeChunks(r, 1)
	upsertErr := errors.New("db connection lost")

	store := &mockRecordStore{rec: r}
	chunker := &mockChunker{chunks: chunks}
	embedder := &mockEmbedder{}
	idx := &mockIndexStore{upsertErr: upsertErr}

	p := newFastPipeline(store, chunker, embedder, idx)
	err := p.Process(context.Background(), validEvent(r.RecordID))

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrTransient) {
		t.Errorf("expected ErrTransient, got: %v", err)
	}
}

func TestPipeline_BatchEmbedding_MoreThan32Chunks(t *testing.T) {
	r := validRecord()
	// 35 chunks → two Embed calls: batch of 32 + batch of 3.
	chunks := makeChunks(r, 35)

	store := &mockRecordStore{rec: r}
	chunker := &mockChunker{chunks: chunks}
	embedder := &mockEmbedder{}
	idx := &mockIndexStore{}

	p := newFastPipeline(store, chunker, embedder, idx)
	err := p.Process(context.Background(), validEvent(r.RecordID))
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	// Embed called twice (two batches).
	if embedder.calls != 2 {
		t.Errorf("expected 2 Embed calls for 35 chunks, got %d", embedder.calls)
	}
	if len(idx.lastChunks) != 35 {
		t.Errorf("expected 35 chunks upserted, got %d", len(idx.lastChunks))
	}
}
