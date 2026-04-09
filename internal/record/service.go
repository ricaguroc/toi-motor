package record

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// EventPublisher is the port for publishing domain events after a record is
// ingested. Implementations may be NATS, an in-memory bus, etc.
// The field is optional — if nil, event publishing is skipped silently.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// RecordService orchestrates the ingestion, retrieval and listing of records.
type RecordService struct {
	store     RecordStore
	publisher EventPublisher // optional — nil is valid
}

// NewRecordService returns a ready-to-use RecordService.
// publisher may be nil when NATS is not yet wired.
func NewRecordService(store RecordStore, publisher EventPublisher) *RecordService {
	return &RecordService{
		store:     store,
		publisher: publisher,
	}
}

// Ingest validates req, builds a new Record, computes its checksum, persists it
// and (if a publisher is configured) fires a domain event asynchronously.
// Returns the persisted Record or a wrapped ErrValidation on invalid input.
func (s *RecordService) Ingest(ctx context.Context, req IngestRequest) (Record, error) {
	if errs := ValidateIngestRequest(req); len(errs) > 0 {
		return Record{}, ValidationErrors{Errors: errs}
	}

	now := time.Now().UTC()

	occurredAt := now
	if req.OccurredAt != nil {
		occurredAt = req.OccurredAt.UTC()
	}

	// Ensure non-nullable fields have defaults when omitted by the caller.
	payload := req.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	objectRefs := req.ObjectRefs
	if objectRefs == nil {
		objectRefs = []string{}
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	r := Record{
		ID:         uuid.New(),
		RecordID:   uuid.New(),
		OccurredAt: occurredAt,
		IngestedAt: now,
		Source:     req.Source,
		RecordType: req.RecordType,
		EntityRef:  req.EntityRef,
		ActorRef:   req.ActorRef,
		Title:      req.Title,
		Payload:    payload,
		ObjectRefs: objectRefs,
		Tags:       tags,
		Metadata:   metadata,
	}

	checksum, err := ComputeChecksum(r)
	if err != nil {
		return Record{}, fmt.Errorf("record: compute checksum: %w", err)
	}
	r.Checksum = checksum

	if err := s.store.Append(ctx, r); err != nil {
		return Record{}, fmt.Errorf("record: append: %w", err)
	}

	if s.publisher != nil {
		go s.publishEvent(r)
	}

	return r, nil
}

// GetByID returns the record identified by the given record_id (business key).
// Returns a wrapped ErrNotFound when no matching record exists.
func (s *RecordService) GetByID(ctx context.Context, recordID uuid.UUID) (Record, error) {
	r, err := s.store.GetByID(ctx, recordID)
	if err != nil {
		return Record{}, err
	}
	return r, nil
}

// List returns a paginated result set matching the given filter.
func (s *RecordService) List(ctx context.Context, filter Filter) (ListResult, error) {
	result, err := s.store.List(ctx, filter)
	if err != nil {
		return ListResult{}, fmt.Errorf("record: list: %w", err)
	}
	return result, nil
}

// ingestedEvent is the JSON payload published after a record is persisted.
type ingestedEvent struct {
	RecordID   string  `json:"record_id"`
	OccurredAt string  `json:"occurred_at"`
	Source     string  `json:"source"`
	RecordType string  `json:"record_type"`
	EntityRef  *string `json:"entity_ref"`
	ActorRef   *string `json:"actor_ref"`
}

// publishEvent fires the event for a newly ingested record, ignoring errors so
// as not to block the ingestion path.
func (s *RecordService) publishEvent(r Record) {
	payload, err := json.Marshal(ingestedEvent{
		RecordID:   r.RecordID.String(),
		OccurredAt: r.OccurredAt.Format(time.RFC3339),
		Source:     r.Source,
		RecordType: r.RecordType,
		EntityRef:  r.EntityRef,
		ActorRef:   r.ActorRef,
	})
	if err != nil {
		log.Printf("record: marshal event for %s: %v", r.RecordID, err)
		return
	}

	if err := s.publisher.Publish(context.Background(), "record.ingested", payload); err != nil {
		log.Printf("record: publish event for %s: %v", r.RecordID, err)
	}
}
