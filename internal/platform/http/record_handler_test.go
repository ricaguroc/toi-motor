package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ricaguroc/toi-motor/internal/auth"
	"github.com/ricaguroc/toi-motor/internal/record"
	platformhttp "github.com/ricaguroc/toi-motor/internal/platform/http"
)

// ---------------------------------------------------------------------------
// Shared mock infrastructure
// ---------------------------------------------------------------------------

// mockRecordStore is an in-memory RecordStore for handler tests.
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

// mockAuthStore always approves the key "valid-key" and rejects everything else.
type mockAuthStore struct {
	validKey string
	apiKey   *auth.APIKey
}

func newMockAuthStore() *mockAuthStore {
	return &mockAuthStore{
		validKey: "valid-key",
		apiKey: &auth.APIKey{
			ID:        uuid.New(),
			KeyPrefix: "valid",
			Name:      "test",
			CreatedAt: time.Now(),
		},
	}
}

func (m *mockAuthStore) Validate(_ context.Context, rawKey string) (*auth.APIKey, error) {
	if rawKey == m.validKey {
		return m.apiKey, nil
	}
	return nil, nil
}

func (m *mockAuthStore) UpdateLastUsed(_ context.Context, _ uuid.UUID) error {
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newRouter(store *mockRecordStore) http.Handler {
	svc := record.NewRecordService(store, nil)
	authStore := newMockAuthStore()
	return platformhttp.NewRouter(svc, nil, nil, authStore, nil)
}

func doRequest(t *testing.T, router http.Handler, method, path string, body any, authHeader string) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v (body: %q)", err, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /api/v1/records
// ---------------------------------------------------------------------------

func TestRecordHandler_Create_NoAuthHeader_Returns401(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})
	rr := doRequest(t, router, http.MethodPost, "/api/v1/records", nil, "")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

func TestRecordHandler_Create_InvalidAuthHeader_Returns401(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})
	rr := doRequest(t, router, http.MethodPost, "/api/v1/records", nil, "Bearer wrong-key")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

func TestRecordHandler_Create_MissingSource_Returns422(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})
	body := map[string]any{
		"record_type": "user.created",
		// source intentionally omitted
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/records", body, "Bearer valid-key")

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want 422", rr.Code)
	}

	var resp struct {
		Error  string                   `json:"error"`
		Fields []record.ValidationError `json:"fields"`
	}
	decodeJSON(t, rr, &resp)

	if len(resp.Fields) == 0 {
		t.Error("expected validation fields in response")
	}

	found := false
	for _, f := range resp.Fields {
		if f.Field == "source" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'source' field error, got: %+v", resp.Fields)
	}
}

func TestRecordHandler_Create_ValidRequest_Returns201WithRecordID(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})
	body := map[string]any{
		"source":      "test-service",
		"record_type": "user.created",
		"payload":     map[string]any{"user_id": 42},
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/records", body, "Bearer valid-key")

	if rr.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		RecordID   uuid.UUID `json:"record_id"`
		IngestedAt time.Time `json:"ingested_at"`
		Checksum   string    `json:"checksum"`
	}
	decodeJSON(t, rr, &resp)

	if resp.RecordID == uuid.Nil {
		t.Error("expected non-nil record_id in response")
	}
	if resp.Checksum == "" {
		t.Error("expected non-empty checksum in response")
	}
	if resp.IngestedAt.IsZero() {
		t.Error("expected non-zero ingested_at in response")
	}
}

func TestRecordHandler_Create_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/records", bytes.NewBufferString("{not-json}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-key")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// GET /api/v1/records/{recordID}
// ---------------------------------------------------------------------------

func TestRecordHandler_GetByID_Found_Returns200(t *testing.T) {
	t.Parallel()

	recordID := uuid.New()
	expected := record.Record{
		ID:         uuid.New(),
		RecordID:   recordID,
		Source:     "svc",
		RecordType: "order.created",
		IngestedAt: time.Now().UTC().Truncate(time.Second),
		OccurredAt: time.Now().UTC().Truncate(time.Second),
		Checksum:   "abc123",
		ObjectRefs: []string{},
		Tags:       []string{},
	}

	store := &mockRecordStore{getRecord: expected}
	router := newRouter(store)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/records/"+recordID.String(), nil, "Bearer valid-key")

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var got record.Record
	decodeJSON(t, rr, &got)

	if got.RecordID != recordID {
		t.Errorf("record_id: got %v, want %v", got.RecordID, recordID)
	}
}

func TestRecordHandler_GetByID_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{getErr: record.ErrNotFound}
	router := newRouter(store)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/records/"+uuid.New().String(), nil, "Bearer valid-key")

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rr.Code)
	}
}

func TestRecordHandler_GetByID_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})
	rr := doRequest(t, router, http.MethodGet, "/api/v1/records/not-a-uuid", nil, "Bearer valid-key")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestRecordHandler_GetByID_NoAuth_Returns401(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})
	rr := doRequest(t, router, http.MethodGet, "/api/v1/records/"+uuid.New().String(), nil, "")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// GET /api/v1/records
// ---------------------------------------------------------------------------

func TestRecordHandler_List_NoFilters_Returns200WithItems(t *testing.T) {
	t.Parallel()

	items := []record.Record{
		{RecordID: uuid.New(), Source: "svc-a", RecordType: "evt.one", ObjectRefs: []string{}, Tags: []string{}},
		{RecordID: uuid.New(), Source: "svc-b", RecordType: "evt.two", ObjectRefs: []string{}, Tags: []string{}},
	}
	store := &mockRecordStore{
		listResult: record.ListResult{Items: items, HasMore: false},
	}
	router := newRouter(store)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/records", nil, "Bearer valid-key")

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Items   []record.Record `json:"items"`
		HasMore bool            `json:"has_more"`
		Count   int             `json:"count"`
	}
	decodeJSON(t, rr, &resp)

	if resp.Count != 2 {
		t.Errorf("count: got %d, want 2", resp.Count)
	}
	if resp.HasMore {
		t.Error("expected has_more=false")
	}
}

func TestRecordHandler_List_EmptyResult_ReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	store := &mockRecordStore{
		listResult: record.ListResult{Items: nil, HasMore: false},
	}
	router := newRouter(store)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/records", nil, "Bearer valid-key")

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}

	var resp struct {
		Items []any `json:"items"`
	}
	decodeJSON(t, rr, &resp)

	// items must be [] not null
	if resp.Items == nil {
		t.Error("expected items to be an empty array, got null")
	}
}

func TestRecordHandler_List_WithFilters_Returns200(t *testing.T) {
	t.Parallel()

	items := []record.Record{
		{RecordID: uuid.New(), Source: "billing", RecordType: "payment.created", ObjectRefs: []string{}, Tags: []string{}},
	}
	store := &mockRecordStore{
		listResult: record.ListResult{Items: items, HasMore: false},
	}
	router := newRouter(store)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/records?source=billing&limit=10", nil, "Bearer valid-key")

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Count int `json:"count"`
	}
	decodeJSON(t, rr, &resp)

	if resp.Count != 1 {
		t.Errorf("count: got %d, want 1", resp.Count)
	}
}

func TestRecordHandler_List_InvalidLimit_Returns400(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})
	rr := doRequest(t, router, http.MethodGet, "/api/v1/records?limit=999", nil, "Bearer valid-key")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestRecordHandler_List_WithHasMore_ReturnsCursor(t *testing.T) {
	t.Parallel()

	last := record.Record{
		RecordID:   uuid.New(),
		OccurredAt: time.Now().UTC().Truncate(time.Second),
		Source:     "svc",
		RecordType: "evt",
		ObjectRefs: []string{},
		Tags:       []string{},
	}
	nextCursor := &record.Cursor{OccurredAt: last.OccurredAt, RecordID: last.RecordID}

	store := &mockRecordStore{
		listResult: record.ListResult{
			Items:   []record.Record{last},
			HasMore: true,
			Next:    nextCursor,
		},
	}
	router := newRouter(store)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/records?limit=1", nil, "Bearer valid-key")

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}

	var resp struct {
		HasMore bool `json:"has_more"`
		Cursor  *struct {
			OccurredAt time.Time `json:"occurred_at"`
			RecordID   uuid.UUID `json:"record_id"`
		} `json:"cursor"`
	}
	decodeJSON(t, rr, &resp)

	if !resp.HasMore {
		t.Error("expected has_more=true")
	}
	if resp.Cursor == nil {
		t.Fatal("expected cursor in response")
	}
	if resp.Cursor.RecordID != last.RecordID {
		t.Errorf("cursor.record_id: got %v, want %v", resp.Cursor.RecordID, last.RecordID)
	}
}

func TestRecordHandler_List_NoAuth_Returns401(t *testing.T) {
	t.Parallel()

	router := newRouter(&mockRecordStore{})
	rr := doRequest(t, router, http.MethodGet, "/api/v1/records", nil, "")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

// Ensure the test file compiles even without direct use of errors package.
var _ = errors.New
