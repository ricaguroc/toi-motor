package indexing

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SearchFilter constrains a similarity search to a subset of records.
// All fields are optional; a nil field means "no constraint on this dimension".
type SearchFilter struct {
	EntityRef  *string
	ActorRef   *string
	RecordType *string
	From       *time.Time
	To         *time.Time
}

// SearchResult is a single item returned by a similarity search.
type SearchResult struct {
	RecordID   uuid.UUID
	ChunkIndex int
	ChunkText  string
	Score      float32
	EntityRef  *string
	ActorRef   *string
	RecordType string
	OccurredAt time.Time
}

// IndexStore is the port for storing and querying record chunk embeddings.
// Implementations live in the platform layer (e.g. postgres/embedding_repo.go).
type IndexStore interface {
	// UpsertChunks persists chunks together with their precomputed embeddings.
	// chunks and embeddings must have the same length; each embedding corresponds
	// to the chunk at the same index.
	UpsertChunks(ctx context.Context, chunks []Chunk, embeddings [][]float32) error

	// SearchSimilar returns up to limit results whose embeddings are closest to
	// queryEmbedding, applying filter to narrow the candidate set.
	// Results are ordered by descending cosine similarity (highest score first).
	SearchSimilar(ctx context.Context, queryEmbedding []float32, filter SearchFilter, limit int) ([]SearchResult, error)
}
