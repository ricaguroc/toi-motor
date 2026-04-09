package auth

import (
	"context"

	"github.com/google/uuid"
)

// APIKeyStore is the port for validating and tracking API key usage.
type APIKeyStore interface {
	// Validate checks rawKey against stored hashes. Returns the matching APIKey or
	// nil if no valid key is found.
	Validate(ctx context.Context, rawKey string) (*APIKey, error)

	// UpdateLastUsed records the current timestamp as the last usage time for the
	// given key ID.
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}
