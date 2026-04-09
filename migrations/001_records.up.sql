CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE records (
    -- Identity
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id    UUID        NOT NULL UNIQUE DEFAULT gen_random_uuid(),

    -- Temporal
    occurred_at  TIMESTAMPTZ NOT NULL,
    ingested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Classification
    source       TEXT        NOT NULL CHECK (char_length(source) BETWEEN 1 AND 100),
    record_type  TEXT        NOT NULL CHECK (char_length(record_type) BETWEEN 1 AND 100),

    -- References
    entity_ref   TEXT        CHECK (entity_ref IS NULL OR char_length(entity_ref) <= 500),
    actor_ref    TEXT        CHECK (actor_ref  IS NULL OR char_length(actor_ref)  <= 500),

    -- Content
    title        TEXT        CHECK (title IS NULL OR char_length(title) <= 1000),
    payload      JSONB       NOT NULL DEFAULT '{}',
    object_refs  TEXT[]      NOT NULL DEFAULT '{}',

    -- Integrity
    checksum     TEXT        NOT NULL CHECK (char_length(checksum) = 64),

    -- Soft metadata
    tags         TEXT[]      NOT NULL DEFAULT '{}',
    metadata     JSONB       NOT NULL DEFAULT '{}'
);

-- Access pattern indexes
CREATE INDEX records_entity_ref_idx       ON records (entity_ref)               WHERE entity_ref IS NOT NULL;
CREATE INDEX records_actor_ref_idx        ON records (actor_ref)                WHERE actor_ref  IS NOT NULL;
CREATE INDEX records_record_type_idx      ON records (record_type);
CREATE INDEX records_occurred_at_idx      ON records (occurred_at DESC);
CREATE INDEX records_source_idx           ON records (source);
CREATE INDEX records_tags_gin_idx         ON records USING GIN (tags);
CREATE INDEX records_payload_gin_idx      ON records USING GIN (payload);
CREATE INDEX records_entity_occurred_idx  ON records (entity_ref, occurred_at DESC) WHERE entity_ref IS NOT NULL;

-- Full-text search (Spanish tokenizer + entity/actor refs)
ALTER TABLE records
    ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        to_tsvector(
            'spanish',
            coalesce(title, '') || ' ' ||
            coalesce(entity_ref, '') || ' ' ||
            coalesce(actor_ref, '')
        )
    ) STORED;

CREATE INDEX records_fts_idx ON records USING GIN (search_vector);
