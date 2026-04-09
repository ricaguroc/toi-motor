package http

import (
	"net/http"
	"time"

	"github.com/ricaguroc/toi-motor/internal/query"
)

// SearchHandler exposes direct RAG search without LLM.
type SearchHandler struct {
	retriever query.Retriever
}

// NewSearchHandler creates a handler for semantic search.
// retriever may be nil if the embedding infrastructure is not configured.
func NewSearchHandler(retriever query.Retriever) *SearchHandler {
	return &SearchHandler{retriever: retriever}
}

type searchRequest struct {
	Q         string  `json:"q"`
	EntityRef *string `json:"entity_ref"`
	DateFrom  *string `json:"date_from"`
	DateTo    *string `json:"date_to"`
	Limit     int     `json:"limit"`
}

type searchResultItem struct {
	RecordID   string  `json:"record_id"`
	Score      float32 `json:"score"`
	RecordType string  `json:"record_type"`
	OccurredAt string  `json:"occurred_at"`
	EntityRef  *string `json:"entity_ref"`
	ChunkText  string  `json:"chunk_text"`
}

type searchResponse struct {
	Results []searchResultItem `json:"results"`
	Total   int                `json:"total"`
	QueryMs int64              `json:"query_ms"`
}

// Search handles POST /api/v1/search.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	if h.retriever == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "search_not_configured",
		})
		return
	}

	var req searchRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error":   "validation_error",
			"message": "invalid request body",
		})
		return
	}

	if req.Q == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error":   "validation_error",
			"message": "q is required",
		})
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	filter := query.RetrievalFilter{
		EntityRef: req.EntityRef,
		Limit:     limit,
	}

	if req.DateFrom != nil {
		t, err := time.Parse(time.RFC3339, *req.DateFrom)
		if err == nil {
			filter.From = &t
		}
	}
	if req.DateTo != nil {
		t, err := time.Parse(time.RFC3339, *req.DateTo)
		if err == nil {
			filter.To = &t
		}
	}

	start := time.Now()

	chunks, err := h.retriever.Retrieve(r.Context(), req.Q, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "search_failed",
			"message": err.Error(),
		})
		return
	}

	items := make([]searchResultItem, 0, len(chunks))
	for _, c := range chunks {
		items = append(items, searchResultItem{
			RecordID:   c.RecordID.String(),
			Score:      c.Score,
			RecordType: c.RecordType,
			OccurredAt: c.OccurredAt.Format(time.RFC3339),
			EntityRef:  c.EntityRef,
			ChunkText:  c.ChunkText,
		})
	}

	writeJSON(w, http.StatusOK, searchResponse{
		Results: items,
		Total:   len(items),
		QueryMs: time.Since(start).Milliseconds(),
	})
}

