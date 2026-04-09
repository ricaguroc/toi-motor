package record_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ricaguroc/toi-motor/internal/record"
)

// ptr returns a pointer to the given value — convenience helper for optional fields.
func ptr[T any](v T) *T { return &v }

// validRequest returns a minimal, fully-valid IngestRequest.
func validRequest() record.IngestRequest {
	return record.IngestRequest{
		Source:     "test-service",
		RecordType: "user.created",
	}
}

// fieldInErrors checks whether any ValidationError in errs targets the given field.
func fieldInErrors(errs []record.ValidationError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}

func TestValidateIngestRequest_ValidMinimal(t *testing.T) {
	errs := record.ValidateIngestRequest(validRequest())
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid minimal request, got: %+v", errs)
	}
}

func TestValidateIngestRequest_ValidFull(t *testing.T) {
	now := time.Now().UTC()
	req := record.IngestRequest{
		Source:     "my-service",
		RecordType: "order.placed",
		OccurredAt: &now,
		EntityRef:  ptr("entity-123"),
		ActorRef:   ptr("user-456"),
		Title:      ptr("Order placed"),
		Payload:    map[string]any{"amount": 99.9},
		Tags:       []string{"sale", "vip"},
		ObjectRefs: []string{"sku-001", "sku-002"},
	}
	errs := record.ValidateIngestRequest(req)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid full request, got: %+v", errs)
	}
}

// --- source ---

func TestValidateIngestRequest_Source_Missing(t *testing.T) {
	req := validRequest()
	req.Source = ""
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "source") {
		t.Errorf("expected validation error for missing source, got: %+v", errs)
	}
}

func TestValidateIngestRequest_Source_TooLong(t *testing.T) {
	req := validRequest()
	req.Source = strings.Repeat("x", 101)
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "source") {
		t.Errorf("expected validation error for source > 100 chars, got: %+v", errs)
	}
}

func TestValidateIngestRequest_Source_MaxLength_IsValid(t *testing.T) {
	req := validRequest()
	req.Source = strings.Repeat("x", 100)
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "source") {
		t.Errorf("source of exactly 100 chars must be valid, got: %+v", errs)
	}
}

// --- record_type ---

func TestValidateIngestRequest_RecordType_Missing(t *testing.T) {
	req := validRequest()
	req.RecordType = ""
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "record_type") {
		t.Errorf("expected validation error for missing record_type, got: %+v", errs)
	}
}

func TestValidateIngestRequest_RecordType_TooLong(t *testing.T) {
	req := validRequest()
	req.RecordType = strings.Repeat("t", 101)
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "record_type") {
		t.Errorf("expected validation error for record_type > 100 chars, got: %+v", errs)
	}
}

// --- occurred_at ---

func TestValidateIngestRequest_OccurredAt_Absent_IsValid(t *testing.T) {
	req := validRequest()
	req.OccurredAt = nil
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "occurred_at") {
		t.Errorf("nil occurred_at must be valid, got: %+v", errs)
	}
}

func TestValidateIngestRequest_OccurredAt_PastTime_IsValid(t *testing.T) {
	req := validRequest()
	past := time.Now().UTC().Add(-48 * time.Hour)
	req.OccurredAt = &past
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "occurred_at") {
		t.Errorf("past occurred_at must be valid, got: %+v", errs)
	}
}

func TestValidateIngestRequest_OccurredAt_TooFarInFuture(t *testing.T) {
	req := validRequest()
	future := time.Now().UTC().Add(25 * time.Hour)
	req.OccurredAt = &future
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "occurred_at") {
		t.Errorf("occurred_at > now+24h must fail, got: %+v", errs)
	}
}

func TestValidateIngestRequest_OccurredAt_ExactlyAtLimit_IsValid(t *testing.T) {
	req := validRequest()
	// Just inside the 24h window.
	limit := time.Now().UTC().Add(23 * time.Hour)
	req.OccurredAt = &limit
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "occurred_at") {
		t.Errorf("occurred_at within 24h window must be valid, got: %+v", errs)
	}
}

// --- entity_ref ---

func TestValidateIngestRequest_EntityRef_Absent_IsValid(t *testing.T) {
	req := validRequest()
	req.EntityRef = nil
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "entity_ref") {
		t.Errorf("nil entity_ref must be valid")
	}
}

func TestValidateIngestRequest_EntityRef_TooLong(t *testing.T) {
	req := validRequest()
	req.EntityRef = ptr(strings.Repeat("e", 501))
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "entity_ref") {
		t.Errorf("entity_ref > 500 chars must fail, got: %+v", errs)
	}
}

func TestValidateIngestRequest_EntityRef_MaxLength_IsValid(t *testing.T) {
	req := validRequest()
	req.EntityRef = ptr(strings.Repeat("e", 500))
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "entity_ref") {
		t.Errorf("entity_ref of exactly 500 chars must be valid")
	}
}

// --- actor_ref ---

func TestValidateIngestRequest_ActorRef_TooLong(t *testing.T) {
	req := validRequest()
	req.ActorRef = ptr(strings.Repeat("a", 501))
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "actor_ref") {
		t.Errorf("actor_ref > 500 chars must fail, got: %+v", errs)
	}
}

func TestValidateIngestRequest_ActorRef_MaxLength_IsValid(t *testing.T) {
	req := validRequest()
	req.ActorRef = ptr(strings.Repeat("a", 500))
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "actor_ref") {
		t.Errorf("actor_ref of exactly 500 chars must be valid")
	}
}

// --- title ---

func TestValidateIngestRequest_Title_TooLong(t *testing.T) {
	req := validRequest()
	req.Title = ptr(strings.Repeat("t", 1001))
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "title") {
		t.Errorf("title > 1000 chars must fail, got: %+v", errs)
	}
}

func TestValidateIngestRequest_Title_MaxLength_IsValid(t *testing.T) {
	req := validRequest()
	req.Title = ptr(strings.Repeat("t", 1000))
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "title") {
		t.Errorf("title of exactly 1000 chars must be valid")
	}
}

// --- payload ---

func TestValidateIngestRequest_Payload_Absent_IsValid(t *testing.T) {
	req := validRequest()
	req.Payload = nil
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "payload") {
		t.Errorf("nil payload must be valid")
	}
}

func TestValidateIngestRequest_Payload_Valid_Map(t *testing.T) {
	req := validRequest()
	req.Payload = map[string]any{"key": "value", "count": 42}
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "payload") {
		t.Errorf("valid payload map must not produce errors, got: %+v", errs)
	}
}

func TestValidateIngestRequest_Payload_TooLarge(t *testing.T) {
	req := validRequest()
	// Build a payload whose JSON exceeds 1MB.
	bigValue := strings.Repeat("x", 1*1024*1024)
	req.Payload = map[string]any{"data": bigValue}
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "payload") {
		t.Errorf("payload > 1MB must fail, got: %+v", errs)
	}
}

// --- tags ---

func TestValidateIngestRequest_Tags_Empty_IsValid(t *testing.T) {
	req := validRequest()
	req.Tags = []string{}
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "tags") {
		t.Errorf("empty tags slice must be valid")
	}
}

func TestValidateIngestRequest_Tags_ValidTags(t *testing.T) {
	req := validRequest()
	req.Tags = []string{"alpha", "beta", strings.Repeat("t", 100)}
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "tags") {
		t.Errorf("valid tags must not produce errors, got: %+v", errs)
	}
}

func TestValidateIngestRequest_Tags_EmptyTag(t *testing.T) {
	req := validRequest()
	req.Tags = []string{"valid", ""}
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "tags") {
		t.Errorf("empty string in tags must fail, got: %+v", errs)
	}
}

func TestValidateIngestRequest_Tags_TooLong(t *testing.T) {
	req := validRequest()
	req.Tags = []string{strings.Repeat("t", 101)}
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "tags") {
		t.Errorf("tag > 100 chars must fail, got: %+v", errs)
	}
}

// --- object_refs ---

func TestValidateIngestRequest_ObjectRefs_Empty_IsValid(t *testing.T) {
	req := validRequest()
	req.ObjectRefs = []string{}
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "object_refs") {
		t.Errorf("empty object_refs slice must be valid")
	}
}

func TestValidateIngestRequest_ObjectRefs_EmptyRef(t *testing.T) {
	req := validRequest()
	req.ObjectRefs = []string{"valid-ref", ""}
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "object_refs") {
		t.Errorf("empty string in object_refs must fail, got: %+v", errs)
	}
}

func TestValidateIngestRequest_ObjectRefs_TooLong(t *testing.T) {
	req := validRequest()
	req.ObjectRefs = []string{strings.Repeat("r", 1001)}
	errs := record.ValidateIngestRequest(req)
	if !fieldInErrors(errs, "object_refs") {
		t.Errorf("object_ref > 1000 chars must fail, got: %+v", errs)
	}
}

func TestValidateIngestRequest_ObjectRefs_MaxLength_IsValid(t *testing.T) {
	req := validRequest()
	req.ObjectRefs = []string{strings.Repeat("r", 1000)}
	errs := record.ValidateIngestRequest(req)
	if fieldInErrors(errs, "object_refs") {
		t.Errorf("object_ref of exactly 1000 chars must be valid")
	}
}

// --- multiple errors at once ---

func TestValidateIngestRequest_MultipleErrors(t *testing.T) {
	req := record.IngestRequest{
		Source:     "",                         // missing
		RecordType: strings.Repeat("x", 101),  // too long
	}
	errs := record.ValidateIngestRequest(req)

	if !fieldInErrors(errs, "source") {
		t.Errorf("expected source error in multi-error result")
	}
	if !fieldInErrors(errs, "record_type") {
		t.Errorf("expected record_type error in multi-error result")
	}
}
