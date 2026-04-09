package http

import (
	"encoding/json"
	"net/http"
)

// writeJSON serialises v as JSON and writes it with the given HTTP status code.
// Content-Type is always set to application/json.
// Any marshalling error is silently swallowed — this should never happen for
// well-formed Go types, and at that point the response headers are already sent.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// readJSON decodes the JSON body of r into v.
// Returns a non-nil error when the body is missing, not valid JSON, or the
// decoded type does not match v.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
