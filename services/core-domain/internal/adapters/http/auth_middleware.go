package http

import (
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

// ClerkAuthMiddleware validates the Bearer token via Clerk — JWKS fetching,
// in-memory caching, and key-ID-miss recovery are handled internally by the
// SDK, per ADR-007/ADR-009 — and, when the token is valid, attaches its sub
// claim to the request context as the Clerk identity via WithClerkUserID.
//
// The Clerk SDK's own WithHeaderAuthorization deliberately does not reject a
// missing or invalid token itself; it just leaves the context unchanged.
// That lets a request with no token still reach the handler, which rejects
// it with 401 via ClerkUserIDFromContext returning ok=false — matching the
// OpenAPI spec's documented 401, not Clerk's own default of 403.
//
// Unlike event-ingestion's ClerkAuthMiddleware, no custom "role" claim is
// read here. Role now comes from this service's own User record — the
// first real implementation of User.role in the codebase — resolved via
// Handler.resolveCaller, not trusted from the JWT. event-ingestion's admin
// endpoints still fake role via a JWT custom claim because they predate
// this service (ADR-012 Part 3 flags that as reconciliation debt to pay
// down once this service exists).
func ClerkAuthMiddleware(next http.Handler) http.Handler {
	return clerkhttp.WithHeaderAuthorization()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claims, ok := clerk.SessionClaimsFromContext(r.Context()); ok {
			r = r.WithContext(WithClerkUserID(r.Context(), claims.Subject))
		}
		next.ServeHTTP(w, r)
	}))
}
