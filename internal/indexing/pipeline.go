package indexing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ricaguroc/toi-motor/internal/record"
)

var (
	ErrPermanent = errors.New("permanent indexing error")
	ErrTransient = errors.New("transient indexing error")
)

// IngestedEvent is the NATS message payload from record.ingested.
type IngestedEvent struct {
	RecordID   uuid.UUID `json:"record_id"`
	OccurredAt string    `json:"occurred_at"`
	Source     string    `json:"source"`
	RecordType string    `json:"record_type"`
	EntityRef  *string   `json:"entity_ref"`
	ActorRef   *string   `json:"actor_ref"`
}

const batchSize = 32

// Pipeline orchestrates the end-to-end record indexing flow:
// unmarshal event → fetch record → generate text → chunk → embed → upsert.
type Pipeline struct {
	store    record.RecordStore
	chunker  Chunker
	embedder Embedder
	index    IndexStore
	maxRetry int
}

// NewPipeline constructs a Pipeline with the default retry limit of 5.
func NewPipeline(store record.RecordStore, chunker Chunker, embedder Embedder, index IndexStore) *Pipeline {
	return &Pipeline{
		store:    store,
		chunker:  chunker,
		embedder: embedder,
		index:    index,
		maxRetry: 5,
	}
}

// Process ingests a raw NATS message payload and indexes the referenced record.
// It returns ErrPermanent for unrecoverable failures (bad JSON, record not
// found, chunking failure) and ErrTransient when retries are exhausted.
func (p *Pipeline) Process(ctx context.Context, data []byte) error {
	// 1. Unmarshal event — permanent error if fails.
	var event IngestedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("%w: failed to unmarshal event: %v", ErrPermanent, err)
	}

	// 2. Fetch full record from store — permanent if not found.
	r, err := p.store.GetByID(ctx, event.RecordID)
	if err != nil {
		if errors.Is(err, record.ErrNotFound) {
			return fmt.Errorf("%w: record %s not found: %v", ErrPermanent, event.RecordID, err)
		}
		return fmt.Errorf("%w: failed to fetch record %s: %v", ErrPermanent, event.RecordID, err)
	}

	// 3. Generate text representation.
	text := GenerateText(r)

	// 4. Chunk the text — permanent if chunker fails.
	chunks, err := p.chunker.Chunk(r, text)
	if err != nil {
		return fmt.Errorf("%w: chunking failed: %v", ErrPermanent, err)
	}

	// 5. Extract chunk texts for embedding.
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Text
	}

	// 6. Embed with retry — split into batches of 32.
	var embeddings [][]float32
	var embedErr error
	embedFn := func() error {
		result := make([][]float32, 0, len(texts))
		for start := 0; start < len(texts); start += batchSize {
			end := start + batchSize
			if end > len(texts) {
				end = len(texts)
			}
			batch, err := p.embedder.Embed(ctx, texts[start:end])
			if err != nil {
				return err
			}
			result = append(result, batch...)
		}
		embeddings = result
		return nil
	}
	if embedErr = p.withRetry(ctx, embedFn); embedErr != nil {
		return fmt.Errorf("%w: embedding failed: %v", ErrTransient, embedErr)
	}

	// 7. Upsert chunks + embeddings to IndexStore with retry.
	var upsertErr error
	upsertFn := func() error {
		return p.index.UpsertChunks(ctx, chunks, embeddings)
	}
	if upsertErr = p.withRetry(ctx, upsertFn); upsertErr != nil {
		return fmt.Errorf("%w: upsert failed: %v", ErrTransient, upsertErr)
	}

	return nil
}

// withRetry runs fn up to p.maxRetry times with exponential backoff (1s, 2s,
// 4s, 8s, 16s). It returns nil on the first success, or a wrapped ErrTransient
// after all attempts are exhausted. Context cancellation aborts early.
func (p *Pipeline) withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < p.maxRetry; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<(attempt-1)) * time.Second // 1s, 2s, 4s, 8s, 16s
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return fmt.Errorf("%w: context cancelled during retry: %v", ErrTransient, ctx.Err())
			}
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		slog.Warn("retry attempt failed", "attempt", attempt+1, "max", p.maxRetry, "err", lastErr)
	}
	return fmt.Errorf("%w: exhausted %d retries: %v", ErrTransient, p.maxRetry, lastErr)
}
