package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ricaguroc/toi-motor/internal/auth"
	platformhttp "github.com/ricaguroc/toi-motor/internal/platform/http"
)

// mockAPIKeyStore is a simple in-memory APIKeyStore for testing.
type mockAPIKeyStore struct {
	key    *auth.APIKey
	err    error
	called bool
}

func (m *mockAPIKeyStore) Validate(_ context.Context, rawKey string) (*auth.APIKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.key != nil {
		return m.key, nil
	}
	return nil, nil
}

func (m *mockAPIKeyStore) UpdateLastUsed(_ context.Context, _ uuid.UUID) error {
	m.called = true
	return nil
}

// okHandler is a simple next handler that records calls.
func okHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestAPIKeyMiddleware_NoHeader(t *testing.T) {
	t.Parallel()

	store := &mockAPIKeyStore{}
	nextCalled := false
	mw := platformhttp.APIKeyMiddleware(store)(okHandler(&nextCalled))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if nextCalled {
		t.Error("next handler should not have been called")
	}
}

func TestAPIKeyMiddleware_WrongFormat(t *testing.T) {
	t.Parallel()

	store := &mockAPIKeyStore{}
	nextCalled := false
	mw := platformhttp.APIKeyMiddleware(store)(okHandler(&nextCalled))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if nextCalled {
		t.Error("next handler should not have been called")
	}
}

func TestAPIKeyMiddleware_InvalidKey(t *testing.T) {
	t.Parallel()

	store := &mockAPIKeyStore{} // Validate returns nil, nil → no match
	nextCalled := false
	mw := platformhttp.APIKeyMiddleware(store)(okHandler(&nextCalled))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-key")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if nextCalled {
		t.Error("next handler should not have been called")
	}
}

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	t.Parallel()

	now := time.Now()
	expected := &auth.APIKey{
		ID:        uuid.New(),
		KeyPrefix: "abcd1234",
		Name:      "test key",
		CreatedAt: now,
	}

	store := &mockAPIKeyStore{key: expected}
	nextCalled := false

	var ctxKey *auth.APIKey
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		ctxKey = platformhttp.APIKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := platformhttp.APIKeyMiddleware(store)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer super-secret-key")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !nextCalled {
		t.Error("next handler should have been called")
	}
	if ctxKey == nil {
		t.Fatal("expected APIKey in context, got nil")
	}
	if ctxKey.ID != expected.ID {
		t.Errorf("context key ID mismatch: got %v, want %v", ctxKey.ID, expected.ID)
	}
}
