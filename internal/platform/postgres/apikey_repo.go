package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/ricaguroc/toi-motor/internal/auth"
)

// PostgresAPIKeyStore implements auth.APIKeyStore using PostgreSQL.
type PostgresAPIKeyStore struct {
	pool *pgxpool.Pool
}

// NewPostgresAPIKeyStore returns a PostgresAPIKeyStore backed by the given pool.
func NewPostgresAPIKeyStore(pool *pgxpool.Pool) *PostgresAPIKeyStore {
	return &PostgresAPIKeyStore{pool: pool}
}

// Validate iterates all active (non-revoked, non-expired) API keys and returns the
// first one whose stored bcrypt hash matches rawKey. Returns nil if no match is found.
func (s *PostgresAPIKeyStore) Validate(ctx context.Context, rawKey string) (*auth.APIKey, error) {
	const q = `
		SELECT id, key_hash, key_prefix, name, created_at, last_used_at, expires_at, revoked_at
		FROM   api_keys
		WHERE  revoked_at IS NULL
		  AND  (expires_at IS NULL OR expires_at > now())`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: validate api key: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			k       auth.APIKey
			keyHash string
		)

		err := rows.Scan(
			&k.ID,
			&keyHash,
			&k.KeyPrefix,
			&k.Name,
			&k.CreatedAt,
			&k.LastUsedAt,
			&k.ExpiresAt,
			&k.RevokedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("postgres: validate api key: scan: %w", err)
		}

		if err := bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(rawKey)); err == nil {
			return &k, nil
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: validate api key: rows: %w", err)
	}

	return nil, nil
}

// UpdateLastUsed sets last_used_at to the current time for the given key ID.
func (s *PostgresAPIKeyStore) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE api_keys SET last_used_at = $1 WHERE id = $2`

	_, err := s.pool.Exec(ctx, q, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("postgres: update last used: %w", err)
	}

	return nil
}
