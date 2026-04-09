package http

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ricaguroc/toi-motor/internal/record"
)

// RecordHandler wires the RecordService to HTTP endpoints.
type RecordHandler struct {
	service *record.RecordService
}

// NewRecordHandler returns a RecordHandler backed by the given service.
func NewRecordHandler(service *record.RecordService) *RecordHandler {
	return &RecordHandler{service: service}
}

// createResponse is the JSON body returned on a successful POST /records.
type createResponse struct {
	RecordID   uuid.UUID `json:"record_id"`
	IngestedAt time.Time `json:"ingested_at"`
	Checksum   string    `json:"checksum"`
}

// errorResponse is the generic JSON error envelope.
type errorResponse struct {
	Error  string                  `json:"error"`
	Fields []record.ValidationError `json:"fields,omitempty"`
}

// listResponse is the JSON body returned on GET /records.
type listResponse struct {
	Items   []record.Record `json:"items"`
	Cursor  *cursorResponse `json:"cursor,omitempty"`
	HasMore bool            `json:"has_more"`
	Count   int             `json:"count"`
}

type cursorResponse struct {
	OccurredAt time.Time `json:"occurred_at"`
	RecordID   uuid.UUID `json:"record_id"`
}

// Create handles POST /api/v1/records.
// Expects a JSON body matching record.IngestRequest.
// Returns 201 with {record_id, ingested_at, checksum} on success.
// Returns 400 on malformed JSON, 422 on validation errors.
func (h *RecordHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req record.IngestRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "malformed request body: " + err.Error()})
		return
	}

	rec, err := h.service.Ingest(r.Context(), req)
	if err != nil {
		var ve record.ValidationErrors
		if errors.As(err, &ve) {
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
				Error:  "validation failed",
				Fields: ve.Errors,
			})
			return
		}
		slog.Error("record: ingest failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, createResponse{
		RecordID:   rec.RecordID,
		IngestedAt: rec.IngestedAt,
		Checksum:   rec.Checksum,
	})
}

// GetByID handles GET /api/v1/records/{recordID}.
// Returns 200 with the full record JSON on success.
// Returns 400 on invalid UUID, 404 when not found.
func (h *RecordHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "recordID")
	recordID, err := uuid.Parse(rawID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid record_id: must be a UUID"})
		return
	}

	rec, err := h.service.GetByID(r.Context(), recordID)
	if err != nil {
		if errors.Is(err, record.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "record not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

// List handles GET /api/v1/records.
// Supported query parameters: entity_ref, actor_ref, record_type, source,
// tag, from (RFC3339), to (RFC3339), limit (1-200), cursor_time + cursor_id.
// Returns 200 with {items, cursor, has_more, count}.
func (h *RecordHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := record.Filter{
		EntityRef:  queryStringPtr(q.Get("entity_ref")),
		ActorRef:   queryStringPtr(q.Get("actor_ref")),
		RecordType: queryStringPtr(q.Get("record_type")),
		Source:     queryStringPtr(q.Get("source")),
		Tag:        queryStringPtr(q.Get("tag")),
	}

	if raw := q.Get("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid 'from' parameter: must be RFC3339"})
			return
		}
		filter.From = &t
	}

	if raw := q.Get("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid 'to' parameter: must be RFC3339"})
			return
		}
		filter.To = &t
	}

	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid 'limit': must be an integer between 1 and 200"})
			return
		}
		filter.Limit = n
	}

	// Keyset cursor: both cursor_time and cursor_id must be present together.
	cursorTime := q.Get("cursor_time")
	cursorID := q.Get("cursor_id")
	if cursorTime != "" && cursorID != "" {
		ct, err := time.Parse(time.RFC3339Nano, cursorTime)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid 'cursor_time': must be RFC3339Nano"})
			return
		}
		cid, err := uuid.Parse(cursorID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid 'cursor_id': must be a UUID"})
			return
		}
		filter.Cursor = &record.Cursor{OccurredAt: ct, RecordID: cid}
	}

	result, err := h.service.List(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	resp := listResponse{
		Items:   result.Items,
		HasMore: result.HasMore,
		Count:   len(result.Items),
	}

	if result.Next != nil {
		resp.Cursor = &cursorResponse{
			OccurredAt: result.Next.OccurredAt,
			RecordID:   result.Next.RecordID,
		}
	}

	// Return an empty array rather than null when there are no items.
	if resp.Items == nil {
		resp.Items = []record.Record{}
	}

	writeJSON(w, http.StatusOK, resp)
}

// queryStringPtr returns a pointer to s when s is non-empty, otherwise nil.
func queryStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
