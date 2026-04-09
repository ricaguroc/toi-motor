package http

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ricaguroc/toi-motor/internal/auth"
	"github.com/ricaguroc/toi-motor/internal/query"
	"github.com/ricaguroc/toi-motor/internal/record"
)

// Note: auth.APIKeyStore is the domain port; APIKeyMiddleware is the HTTP
// adapter that bridges the port to the HTTP layer (defined in auth_middleware.go).

// NewRouter builds and returns the chi router for the API server.
// It mounts all /api/v1 routes behind APIKeyMiddleware.
// queryService may be nil when the LLM is not configured; /query returns 503.
// retriever may be nil when embeddings are not configured; /search returns 503.
// health may be nil; in that case the health endpoints are omitted.
func NewRouter(recordService *record.RecordService, queryService *query.QueryService, retriever query.Retriever, authStore auth.APIKeyStore, health *HealthChecker) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger)

	// Health endpoints are intentionally outside the auth middleware group so
	// load balancers and orchestrators can probe them without an API key.
	if health != nil {
		r.Get("/health", health.Health)
		r.Get("/health/live", health.Live)
		r.Get("/health/ready", health.Ready)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(APIKeyMiddleware(authStore))

		recordHandler := NewRecordHandler(recordService)
		r.Post("/records", recordHandler.Create)
		r.Get("/records/{recordID}", recordHandler.GetByID)
		r.Get("/records", recordHandler.List)

		r.Post("/query", NewQueryHandler(queryService).Query)
		r.Post("/search", NewSearchHandler(retriever).Search)
	})

	return r
}
