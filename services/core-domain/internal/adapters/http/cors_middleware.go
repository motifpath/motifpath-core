package http

import (
	"net/http"
	"slices"
	"strings"
)

const corsPreflightMaxAge = "300"

// ParseAllowedOrigins splits a comma-separated origin list (as supplied via the
// CORS_ALLOWED_ORIGINS environment variable), trimming whitespace and dropping
// empty entries.
func ParseAllowedOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// NewCORSMiddleware returns middleware that lets browser clients on the given
// origins read cross-origin responses from this service. Requests from any
// other origin pass through without CORS headers, so the browser blocks the
// response — which is the intended behaviour.
//
// Auth is carried in the Authorization header (Bearer), never cookies, so no
// Access-Control-Allow-Credentials is sent.
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := slices.Clone(allowedOrigins)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			originAllowed := origin != "" && slices.Contains(allowed, origin)

			if originAllowed {
				header := w.Header()
				header.Set("Access-Control-Allow-Origin", origin)
				header.Add("Vary", "Origin")
				header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				header.Set("Access-Control-Max-Age", corsPreflightMaxAge)
			}

			if r.Method == http.MethodOptions && originAllowed {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
