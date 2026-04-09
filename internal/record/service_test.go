package record_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ricaguroc/toi-motor/internal/record"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockRecordStore is an in-memory RecordStore used in unit tests.
type mockRecordStore struct {
	appended  []record.Record
	appendErr error

	getRecord record.Record
	getErr    error

	listResult record.ListResult
	listErr    error
}

func (m *mockRecordStore) Append(_ context.Context, r record.Record) error {
	if m.appendErr != nil {
		return m.appendErr
	}
	m.appended = append(m.appended, r)
	return nil
}

func (m *mockRecordStore) GetByID(_ context.Context, _ uuid.UUID) (record.Record, error) {
	return m.getRecord, m.getErr
}

func (m *mockRecordStore) List(_ context.Context, _ record.Filter) (record.ListResult, error) {
	return m.listResult, m.listErr
}

// mockPublisher records calls to Publish.
type mockPublisher struct {
	calls []publishCall
	err   error
}

type publishCall struct {
	subject string
	data    []byte
}

func (m *mockPublisher) Publish(_ context.Context, subject string, data []byte) error {
	m.calls = append(m.calls, publishCall{subject: subject, data: data})
	return m.err
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func validIngestRequest() record.IngestRequest {
	return record.IngestRequest{
		Source:     "test-service",
		RecordType: "user.created",
		Payload:    map[string]any{"key": "value"},
	}
}

// ---------------------------------------------------------------------------
// Ingest tests
// ---------------------------------------------------------------------------

func TestRecordService_Ingest_ValidRequest_ReturnsRecordWithChecksum(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{}
	svc := record.NewRecordService(store, nil)

	req := validIngestRequest()

	rec, err := svc.Ingest(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.RecordID == uuid.Nil {
		t.Error("expected non-nil RecordID")
	}
	if rec.Checksum == "" {
		t.Error("expected non-empty Checksum")
	}
	if rec.IngestedAt.IsZero() {
		t.Error("expected non-zero IngestedAt")
	}
	if rec.Source != req.Source {
		t.Errorf("source: got %q, want %q", rec.Source, req.Source)
	}
	if rec.RecordType != req.RecordType {
		t.Errorf("record_type: got %q, want %q", rec.RecordType, req.RecordType)
	}

	if len(store.appended) != 1 {
		t.Fatalf("expected 1 appended record, got %d", len(store.appended))
	}
}

func TestRecordService_Ingest_OccurredAtDefaultsToNow(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{}
	svc := record.NewRecordService(store, nil)

	before := time.Now().UTC().Add(-time.Second)
	rec, err := svc.Ingest(context.Background(), validIngestRequest())
	after := time.Now().UTC().Add(time.Second)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.OccurredAt.Before(before) || rec.OccurredAt.After(after) {
		t.Errorf("OccurredAt %v not within expected range [%v, %v]", rec.OccurredAt, before, after)
	}
}

func TestRecordService_Ingest_OccurredAtHonoured(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{}
	svc := record.NewRecordService(store, nil)

	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	req := validIngestRequest()
	req.OccurredAt = &ts

	rec, err := svc.Ingest(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !rec.OccurredAt.Equal(ts) {
		t.Errorf("OccurredAt: got %v, want %v", rec.OccurredAt, ts)
	}
}

func TestRecordService_Ingest_InvalidRequest_ReturnsValidationError(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{}
	svc := record.NewRecordService(store, nil)

	// Empty source triggers validation failure.
	req := record.IngestRequest{
		RecordType: "user.created",
	}

	_, err := svc.Ingest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, record.ErrValidation) {
		t.Errorf("expected ErrValidation, got: %v", err)
	}

	var ve record.ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationErrors, got: %T", err)
	}
	if len(ve.Errors) == 0 {
		t.Error("expected at least one validation error")
	}

	if len(store.appended) != 0 {
		t.Error("store should not have been called on validation failure")
	}
}

func TestRecordService_Ingest_NilPublisher_StillWorks(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{}
	// Explicitly pass nil publisher.
	svc := record.NewRecordService(store, nil)

	rec, err := svc.Ingest(context.Background(), validIngestRequest())
	if err != nil {
		t.Fatalf("unexpected error with nil publisher: %v", err)
	}

	if rec.Checksum == "" {
		t.Error("expected checksum even with nil publisher")
	}
}

func TestRecordService_Ingest_WithPublisher_PublishesEvent(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{}
	pub := &mockPublisher{}
	svc := record.NewRecordService(store, pub)

	rec, err := svc.Ingest(context.Background(), validIngestRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The publish is fired in a goroutine — give it a moment to complete.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && len(pub.calls) == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(pub.calls))
	}
	if pub.calls[0].subject != "record.ingested" {
		t.Errorf("subject: got %q, want %q", pub.calls[0].subject, "record.ingested")
	}
	// Data is a JSON payload containing the record_id
	if !strings.Contains(string(pub.calls[0].data), rec.RecordID.String()) {
		t.Errorf("data should contain record_id %q, got %q", rec.RecordID.String(), pub.calls[0].data)
	}
}

func TestRecordService_Ingest_ChecksumIsStable(t *testing.T) {
	t.Parallel()

	// Two identical ingests must produce different records (different UUID / ingestedAt)
	// but the checksum covers record_id, so they will differ too.
	// What we verify: checksum is always set and always 64 hex chars.
	store := &mockRecordStore{}
	svc := record.NewRecordService(store, nil)

	rec, err := svc.Ingest(context.Background(), validIngestRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rec.Checksum) != 64 {
		t.Errorf("checksum length: got %d, want 64", len(rec.Checksum))
	}
}

// ---------------------------------------------------------------------------
// GetByID tests
// ---------------------------------------------------------------------------

func TestRecordService_GetByID_Found(t *testing.T) {
	t.Parallel()

	expected := record.Record{
		RecordID: uuid.New(),
		Source:   "svc",
	}
	store := &mockRecordStore{getRecord: expected}
	svc := record.NewRecordService(store, nil)

	got, err := svc.GetByID(context.Background(), expected.RecordID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RecordID != expected.RecordID {
		t.Errorf("RecordID: got %v, want %v", got.RecordID, expected.RecordID)
	}
}

func TestRecordService_GetByID_NotFound(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{getErr: record.ErrNotFound}
	svc := record.NewRecordService(store, nil)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, record.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestRecordService_List_ReturnsPaginatedResult(t *testing.T) {
	t.Parallel()

	items := []record.Record{
		{RecordID: uuid.New(), Source: "a"},
		{RecordID: uuid.New(), Source: "b"},
	}
	store := &mockRecordStore{
		listResult: record.ListResult{Items: items, HasMore: false},
	}
	svc := record.NewRecordService(store, nil)

	result, err := svc.List(context.Background(), record.Filter{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("items count: got %d, want 2", len(result.Items))
	}
	if result.HasMore {
		t.Error("expected HasMore=false")
	}
}
