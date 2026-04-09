package record

import (
	"time"

	"github.com/google/uuid"
)

// Record is the canonical, persisted representation of a traceability event.
// This is a pure domain model — storage mapping is the adapter's responsibility.
type Record struct {
	ID         uuid.UUID
	RecordID   uuid.UUID
	OccurredAt time.Time
	IngestedAt time.Time
	Source     string
	RecordType string
	EntityRef  *string
	ActorRef   *string
	Title      *string
	Payload    map[string]any
	ObjectRefs []string
	Checksum   string
	Tags       []string
	Metadata   map[string]any
}

// IngestRequest is the raw input received from the API before record_id and
// checksum are assigned. All pointer fields are optional.
type IngestRequest struct {
	OccurredAt *time.Time     `json:"occurred_at"`
	Source     string         `json:"source"`
	RecordType string         `json:"record_type"`
	EntityRef  *string        `json:"entity_ref"`
	ActorRef   *string        `json:"actor_ref"`
	Title      *string        `json:"title"`
	Payload    map[string]any `json:"payload"`
	ObjectRefs []string       `json:"object_refs"`
	Tags       []string       `json:"tags"`
	Metadata   map[string]any `json:"metadata"`
}
