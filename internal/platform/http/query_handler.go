package http

import (
	"errors"
	"net/http"

	"github.com/ricaguroc/toi-motor/internal/query"
)

// QueryHandler wires the QueryService to the HTTP query endpoint.
type QueryHandler struct {
	service *query.QueryService
}

// NewQueryHandler returns a QueryHandler backed by the given service.
// service may be nil when the query engine is not configured; in that case
// every request returns 503.
func NewQueryHandler(service *query.QueryService) *QueryHandler {
	return &QueryHandler{service: service}
}

// queryErrorResponse is the JSON error envelope for query-specific errors.
type queryErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Query handles POST /api/v1/query.
//
// Request body: query.QueryRequest (JSON)
// Responses:
//
//	200 — query.QueryResponse
//	422 — validation error (ErrValidation)
//	500 — LLM unavailable (ErrLLMUnavailable) or LLM parse error (ErrLLMParseError)
//	503 — query engine not configured (nil service)
func (h *QueryHandler) Query(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, queryErrorResponse{
			Error:   "query_engine_not_configured",
			Message: "query engine is not available; configure LLM_API_KEY, LLM_BASE_URL, LLM_MODEL, and OLLAMA_HOST (for embeddings)",
		})
		return
	}

	var req query.QueryRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, queryErrorResponse{
			Error:   "validation_error",
			Message: "malformed request body: " + err.Error(),
		})
		return
	}

	resp, err := h.service.Query(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, query.ErrValidation):
			writeJSON(w, http.StatusUnprocessableEntity, queryErrorResponse{
				Error:   "validation_error",
				Message: err.Error(),
			})
		case errors.Is(err, query.ErrLLMUnavailable):
			writeJSON(w, http.StatusInternalServerError, queryErrorResponse{
				Error:   "llm_unavailable",
				Message: err.Error(),
			})
		case errors.Is(err, query.ErrLLMParseError):
			writeJSON(w, http.StatusInternalServerError, queryErrorResponse{
				Error:   "llm_parse_error",
				Message: err.Error(),
			})
		default:
			writeJSON(w, http.StatusInternalServerError, queryErrorResponse{
				Error:   "internal_error",
				Message: "an unexpected error occurred",
			})
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
