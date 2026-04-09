package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthLive_AlwaysOK(t *testing.T) {
	hc := NewHealthChecker(nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()

	hc.Live(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body liveResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status ok, got %q", body.Status)
	}
}

func TestHealth_NilPool_Returns503Degraded(t *testing.T) {
	// nil pool → postgres check fails; all other deps also unconfigured.
	hc := NewHealthChecker(nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	hc.Health(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "degraded" {
		t.Errorf("expected status degraded, got %q", body.Status)
	}
	if body.Checks["postgres"] == "ok" {
		t.Errorf("expected postgres check to report a non-ok status, got %q", body.Checks["postgres"])
	}
}

func TestHealth_AllUnconfigured_Returns503Degraded(t *testing.T) {
	hc := NewHealthChecker(nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	hc.Health(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	for dep, status := range body.Checks {
		if status == "ok" {
			t.Errorf("dependency %q should not report ok when unconfigured, got %q", dep, status)
		}
	}
}

func TestHealthReady_DelegatesToHealth(t *testing.T) {
	hc := NewHealthChecker(nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	hc.Ready(w, req)

	// Ready should behave exactly as Health — 503 when nothing is configured.
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
