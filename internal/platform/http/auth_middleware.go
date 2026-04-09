package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ricaguroc/toi-motor/internal/auth"
)

type contextKey string

const ctxKeyAPIKey contextKey = "api_key"

// APIKeyMiddleware returns an HTTP middleware that requires a valid API key via
// the "Authorization: Bearer {key}" header. On success it stores the resolved
// *APIKey in the request context and fires UpdateLastUsed in a goroutine.
func APIKeyMiddleware(store auth.APIKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey, ok := extractBearerToken(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			key, err := store.Validate(r.Context(), rawKey)
			if err != nil || key == nil {
				writeUnauthorized(w)
				return
			}

			go func() {
				_ = store.UpdateLastUsed(context.Background(), key.ID)
			}()

			ctx := context.WithValue(r.Context(), ctxKeyAPIKey, key)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APIKeyFromContext retrieves the *APIKey stored by APIKeyMiddleware.
// Returns nil if no key is present.
func APIKeyFromContext(ctx context.Context) *auth.APIKey {
	v, _ := ctx.Value(ctxKeyAPIKey).(*auth.APIKey)
	return v
}

// extractBearerToken parses the Authorization header and returns the token and
// true when the format is "Bearer {token}", otherwise returns "", false.
func extractBearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}

	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}

	return parts[1], true
}

// writeUnauthorized writes a 401 JSON error response.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}
