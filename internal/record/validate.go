package record

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	maxPayloadBytes = 1 * 1024 * 1024 // 1 MB
	maxFutureWindow = 24 * time.Hour
)

// ValidateIngestRequest checks every field of req against the domain rules and
// returns a slice of ValidationError, one per violation.  An empty slice means
// the request is valid.
func ValidateIngestRequest(req IngestRequest) []ValidationError {
	var errs []ValidationError

	add := func(field, msg string) {
		errs = append(errs, ValidationError{Field: field, Message: msg})
	}

	// source: required, 1-100 chars
	if req.Source == "" {
		add("source", "source is required")
	} else if len(req.Source) > 100 {
		add("source", "source must be at most 100 characters")
	}

	// record_type: required, 1-100 chars
	if req.RecordType == "" {
		add("record_type", "record_type is required")
	} else if len(req.RecordType) > 100 {
		add("record_type", "record_type must be at most 100 characters")
	}

	// occurred_at: optional — if present must be a valid time and not > now+24h
	if req.OccurredAt != nil {
		limit := time.Now().UTC().Add(maxFutureWindow)
		if req.OccurredAt.After(limit) {
			add("occurred_at", fmt.Sprintf("occurred_at cannot be more than %s in the future", maxFutureWindow))
		}
	}

	// entity_ref: optional, <= 500 chars
	if req.EntityRef != nil && len(*req.EntityRef) > 500 {
		add("entity_ref", "entity_ref must be at most 500 characters")
	}

	// actor_ref: optional, <= 500 chars
	if req.ActorRef != nil && len(*req.ActorRef) > 500 {
		add("actor_ref", "actor_ref must be at most 500 characters")
	}

	// title: optional, <= 1000 chars
	if req.Title != nil && len(*req.Title) > 1000 {
		add("title", "title must be at most 1000 characters")
	}

	// payload: optional — must be a JSON object (map), raw JSON <= 1MB
	if req.Payload != nil {
		raw, err := json.Marshal(req.Payload)
		if err != nil {
			add("payload", "payload could not be serialised to JSON")
		} else if len(raw) > maxPayloadBytes {
			add("payload", fmt.Sprintf("payload must be at most %d bytes when serialised", maxPayloadBytes))
		}
	}

	// tags: each 1-100 chars
	for i, tag := range req.Tags {
		if tag == "" {
			add("tags", fmt.Sprintf("tags[%d] must not be empty", i))
		} else if len(tag) > 100 {
			add("tags", fmt.Sprintf("tags[%d] must be at most 100 characters", i))
		}
	}

	// object_refs: each 1-1000 chars
	for i, ref := range req.ObjectRefs {
		if ref == "" {
			add("object_refs", fmt.Sprintf("object_refs[%d] must not be empty", i))
		} else if len(ref) > 1000 {
			add("object_refs", fmt.Sprintf("object_refs[%d] must be at most 1000 characters", i))
		}
	}

	return errs
}
