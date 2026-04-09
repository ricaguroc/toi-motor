package query

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RetrievalFilter constrains a retrieval request.
// All fields are optional; nil means no constraint on that dimension.
type RetrievalFilter struct {
	EntityRef *string
	From      *time.Time
	To        *time.Time
	Limit     int
}

// RetrievedChunk is one chunk returned by the retriever with its provenance.
type RetrievedChunk struct {
	RecordID   uuid.UUID
	ChunkIndex int
	ChunkText  string
	Score      float32
	OccurredAt time.Time
	RecordType string
	EntityRef  *string
}

// Retriever is the port for semantic search over ingested records.
type Retriever interface {
	Retrieve(ctx context.Context, queryText string, filter RetrievalFilter) ([]RetrievedChunk, error)
}
