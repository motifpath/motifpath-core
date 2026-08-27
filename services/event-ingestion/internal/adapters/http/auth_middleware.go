package http

import (
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

// ClerkAuthMiddleware validates the Bearer token via Clerk — JWKS fetching, in-memory
// caching, and key-ID-miss recovery are all handled internally by the SDK, per
// ADR-007/ADR-009 — and, when the token is valid, attaches its sub claim to the
// request context via WithStudentID and the raw token via WithBearerToken.
//
// The Clerk SDK's own WithHeaderAuthorization deliberately does not reject a
// missing or invalid token itself; it just leaves the context unchanged. That
// lets a request with no token still reach the handler, which rejects it with 401
// via StudentIDFromContext / BearerTokenFromContext returning ok=false — matching
// the OpenAPI spec's documented 401, not Clerk's own default of 403.
//
// No role claim is read from the token. The outbox admin endpoints (ADR-013)
// establish the caller's role by forwarding the raw bearer token to the Core
// Domain Service, which is the single source of truth for role.
//
// Known gap: per ADR-007, the JWT sub claim is Clerk's internal user ID, not the
// MotifPath student_id — the Core Domain Service maps sub to student_id at
// registration time. The /events endpoint still uses sub directly as student_id;
// resolving it through the Core Domain Service is tracked separately (ADR-013,
// Neutral).
func ClerkAuthMiddleware(next http.Handler) http.Handler {
	return clerkhttp.WithHeaderAuthorization()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claims, ok := clerk.SessionClaimsFromContext(r.Context()); ok {
			ctx := WithStudentID(r.Context(), claims.Subject)
			if token, ok := bearerToken(r); ok {
				ctx = WithBearerToken(ctx, token)
			}
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	}))
}

// bearerToken extracts the raw token from an "Authorization: Bearer <token>"
// header. The scheme match is case-insensitive per RFC 7235.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(header[len(prefix):]), true
}
