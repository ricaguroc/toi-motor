package query

import (
	"context"
	"fmt"

	"github.com/ricaguroc/toi-motor/internal/indexing"
)

// DefaultRetriever is a thin adapter that embeds a query and delegates to IndexStore.
type DefaultRetriever struct {
	embedder indexing.Embedder
	index    indexing.IndexStore
}

// NewRetriever constructs a DefaultRetriever wired to the given embedder and index.
func NewRetriever(embedder indexing.Embedder, index indexing.IndexStore) *DefaultRetriever {
	return &DefaultRetriever{embedder: embedder, index: index}
}

// Retrieve embeds queryText, applies filter, and maps search results to RetrievedChunks.
func (r *DefaultRetriever) Retrieve(ctx context.Context, queryText string, filter RetrievalFilter) ([]RetrievedChunk, error) {
	// 1. Embed the query text.
	embeddings, err := r.embedder.Embed(ctx, []string{queryText})
	if err != nil {
		return nil, fmt.Errorf("query/retriever: embed: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("query/retriever: embedder returned no vectors")
	}
	queryVec := embeddings[0]

	// 2. Build SearchFilter from RetrievalFilter.
	sf := indexing.SearchFilter{
		EntityRef: filter.EntityRef,
		From:      filter.From,
		To:        filter.To,
	}

	// 3. Call index.
	limit := filter.Limit
	if limit <= 0 {
		limit = 20 // sensible default
	}
	results, err := r.index.SearchSimilar(ctx, queryVec, sf, limit)
	if err != nil {
		return nil, fmt.Errorf("query/retriever: search: %w", err)
	}

	// 4. Map SearchResult → RetrievedChunk.
	chunks := make([]RetrievedChunk, len(results))
	for i, sr := range results {
		chunks[i] = RetrievedChunk{
			RecordID:   sr.RecordID,
			ChunkIndex: sr.ChunkIndex,
			ChunkText:  sr.ChunkText,
			Score:      sr.Score,
			OccurredAt: sr.OccurredAt,
			RecordType: sr.RecordType,
			EntityRef:  sr.EntityRef,
		}
	}

	return chunks, nil
}
