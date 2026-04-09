CREATE TABLE api_keys (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash      TEXT        NOT NULL UNIQUE,
    key_prefix    TEXT        NOT NULL,
    name          TEXT        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 255),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ
);

CREATE INDEX api_keys_key_hash_idx ON api_keys (key_hash);
