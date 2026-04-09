package record

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Filter defines the criteria for listing records.
// All fields are optional except Limit which must be between 1 and 200.
type Filter struct {
	EntityRef  *string
	ActorRef   *string
	RecordType *string
	Source     *string
	Tag        *string
	From       *time.Time
	To         *time.Time
	Limit      int // 1-200
	Cursor     *Cursor
}

// Cursor is the keyset pagination token based on the last seen record.
type Cursor struct {
	OccurredAt time.Time `json:"occurred_at"`
	RecordID   uuid.UUID `json:"record_id"`
}

// ListResult is the paginated output from RecordStore.List.
type ListResult struct {
	Items   []Record
	Next    *Cursor
	HasMore bool
}

// RecordStore is the port for persisting and querying records.
// Immutability is enforced at compile time: no Update, Delete, or Patch methods exist.
type RecordStore interface {
	Append(ctx context.Context, r Record) error
	GetByID(ctx context.Context, recordID uuid.UUID) (Record, error)
	List(ctx context.Context, filter Filter) (ListResult, error)
}
