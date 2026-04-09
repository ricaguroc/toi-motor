package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/ricaguroc/toi-motor/internal/indexing"
)

// Compile-time assertion: PostgresEmbeddingRepo must satisfy indexing.IndexStore.
var _ indexing.IndexStore = (*PostgresEmbeddingRepo)(nil)

// PostgresEmbeddingRepo implements indexing.IndexStore using PostgreSQL + pgvector.
type PostgresEmbeddingRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresEmbeddingRepo returns a PostgresEmbeddingRepo backed by the given pool.
func NewPostgresEmbeddingRepo(pool *pgxpool.Pool) *PostgresEmbeddingRepo {
	return &PostgresEmbeddingRepo{pool: pool}
}

// UpsertChunks persists chunks and their embeddings in a single transaction.
// chunks and embeddings must have the same length; each pair is upserted by
// (record_id, chunk_index) — existing rows are fully overwritten.
func (r *PostgresEmbeddingRepo) UpsertChunks(ctx context.Context, chunks []indexing.Chunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("postgres: upsert chunks: chunks and embeddings length mismatch (%d vs %d)", len(chunks), len(embeddings))
	}

	if len(chunks) == 0 {
		return nil
	}

	const q = `
		INSERT INTO record_embeddings
			(record_id, chunk_index, chunk_text, embedding, entity_ref, actor_ref, record_type, occurred_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (record_id, chunk_index)
		DO UPDATE SET
			chunk_text  = EXCLUDED.chunk_text,
			embedding   = EXCLUDED.embedding,
			entity_ref  = EXCLUDED.entity_ref,
			actor_ref   = EXCLUDED.actor_ref,
			record_type = EXCLUDED.record_type,
			occurred_at = EXCLUDED.occurred_at`

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: upsert chunks: begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback on non-committed tx is safe to ignore

	for i, chunk := range chunks {
		vec := pgvector.NewVector(embeddings[i])

		if _, err := tx.Exec(ctx, q,
			chunk.RecordID,
			chunk.ChunkIndex,
			chunk.Text,
			vec,
			chunk.EntityRef,
			chunk.ActorRef,
			chunk.RecordType,
			chunk.OccurredAt,
		); err != nil {
			return fmt.Errorf("postgres: upsert chunk (record_id=%s, chunk_index=%d): %w", chunk.RecordID, chunk.ChunkIndex, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: upsert chunks: commit: %w", err)
	}

	return nil
}

// SearchSimilar returns up to limit chunks ordered by cosine similarity to
// queryEmbedding (closest first). filter narrows the candidate set; nil fields
// mean "no constraint".
func (r *PostgresEmbeddingRepo) SearchSimilar(
	ctx context.Context,
	queryEmbedding []float32,
	filter indexing.SearchFilter,
	limit int,
) ([]indexing.SearchResult, error) {
	const q = `
		SELECT
			record_id, chunk_index, chunk_text,
			1 - (embedding <=> $1) AS score,
			occurred_at, record_type, entity_ref, actor_ref
		FROM record_embeddings
		WHERE
			($2::text IS NULL OR entity_ref = $2)
			AND ($3::text IS NULL OR actor_ref = $3)
			AND ($4::text IS NULL OR record_type = $4)
			AND ($5::timestamptz IS NULL OR occurred_at >= $5)
			AND ($6::timestamptz IS NULL OR occurred_at <= $6)
		ORDER BY embedding <=> $1
		LIMIT $7`

	vec := pgvector.NewVector(queryEmbedding)

	rows, err := r.pool.Query(ctx, q,
		vec,
		filter.EntityRef,
		filter.ActorRef,
		filter.RecordType,
		filter.From,
		filter.To,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: search similar: %w", err)
	}
	defer rows.Close()

	var results []indexing.SearchResult
	for rows.Next() {
		var res indexing.SearchResult
		if err := rows.Scan(
			&res.RecordID,
			&res.ChunkIndex,
			&res.ChunkText,
			&res.Score,
			&res.OccurredAt,
			&res.RecordType,
			&res.EntityRef,
			&res.ActorRef,
		); err != nil {
			return nil, fmt.Errorf("postgres: search similar: scan row: %w", err)
		}
		results = append(results, res)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: search similar: rows error: %w", err)
	}

	return results, nil
}
