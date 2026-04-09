package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	natsgo "github.com/nats-io/nats.go"
)

// HealthChecker holds the dependencies needed to assess service health.
type HealthChecker struct {
	pool          *pgxpool.Pool
	nc            *natsgo.Conn
	embeddingsURL string // Ollama URL for embedding model health check
}

// NewHealthChecker creates a HealthChecker.
// nc may be nil when NATS is not configured.
// embeddingsURL is the Ollama base URL (http://host:port).
func NewHealthChecker(pool *pgxpool.Pool, nc *natsgo.Conn, embeddingsURL string) *HealthChecker {
	return &HealthChecker{
		pool:          pool,
		nc:            nc,
		embeddingsURL: embeddingsURL,
	}
}

// healthResponse is the JSON body for /health and /health/ready.
type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// liveResponse is the minimal JSON body for /health/live.
type liveResponse struct {
	Status string `json:"status"`
}

// Live handles GET /health/live.
// Always returns 200 — this endpoint only confirms the process is running.
func (h *HealthChecker) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, liveResponse{Status: "ok"})
}

// Health handles GET /health (deep check).
func (h *HealthChecker) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := h.runChecks(ctx)
	status := "ok"
	for _, v := range checks {
		if v != "ok" {
			status = "degraded"
			break
		}
	}

	code := http.StatusOK
	if status == "degraded" {
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, healthResponse{Status: status, Checks: checks})
}

// Ready handles GET /health/ready.
// Identical to /health — returns 200 only when all dependencies are reachable.
func (h *HealthChecker) Ready(w http.ResponseWriter, r *http.Request) {
	h.Health(w, r)
}

// runChecks executes all dependency probes and returns a map of check → result.
func (h *HealthChecker) runChecks(ctx context.Context) map[string]string {
	checks := map[string]string{
		"postgres":   h.checkPostgres(ctx),
		"nats":       h.checkNATS(),
		"embeddings": h.checkEmbeddings(ctx),
	}
	return checks
}

func (h *HealthChecker) checkPostgres(ctx context.Context) string {
	if h.pool == nil {
		return "not configured"
	}
	if err := h.pool.Ping(ctx); err != nil {
		return err.Error()
	}
	return "ok"
}

func (h *HealthChecker) checkNATS() string {
	if h.nc == nil {
		return "not configured"
	}
	if h.nc.IsConnected() {
		return "ok"
	}
	return "disconnected"
}

func (h *HealthChecker) checkEmbeddings(ctx context.Context) string {
	if h.embeddingsURL == "" {
		return "not configured"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.embeddingsURL+"/api/version", nil)
	if err != nil {
		return fmt.Sprintf("build request failed: %s", err.Error())
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("request failed: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("unexpected status: %d", resp.StatusCode)
	}
	return "ok"
}
