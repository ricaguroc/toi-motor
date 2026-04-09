package auth

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents an active API credential used to authenticate requests.
type APIKey struct {
	ID         uuid.UUID
	KeyPrefix  string
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
}
