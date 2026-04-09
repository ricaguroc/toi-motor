CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE record_embeddings (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id    UUID        NOT NULL REFERENCES records(record_id) ON DELETE RESTRICT,
    chunk_index  INT         NOT NULL DEFAULT 0,
    chunk_text   TEXT        NOT NULL,
    embedding    vector(768) NOT NULL,

    -- Denormalized from records for filter-then-search without JOIN
    entity_ref   TEXT,
    actor_ref    TEXT,
    record_type  TEXT        NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT record_embeddings_unique_chunk UNIQUE (record_id, chunk_index)
);

-- HNSW index for approximate nearest neighbor (cosine similarity)
CREATE INDEX record_embeddings_hnsw_idx
    ON record_embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- Entity-scoped retrieval
CREATE INDEX record_embeddings_entity_idx    ON record_embeddings (entity_ref) WHERE entity_ref IS NOT NULL;
CREATE INDEX record_embeddings_occurred_idx  ON record_embeddings (occurred_at DESC);
CREATE INDEX record_embeddings_type_idx      ON record_embeddings (record_type);
