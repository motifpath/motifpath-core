package http

import (
	"context"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

// roleClaims declares the shape of the "role" custom claim configured in
// Clerk's JWT template. Only the admin endpoints (ADR-012 Part 3) use it
// today; every other request simply carries an empty Role.
type roleClaims struct {
	Role string `json:"role"`
}

// ClerkAuthMiddleware validates the Bearer token via Clerk — JWKS fetching, in-memory
// caching, and key-ID-miss recovery are all handled internally by the SDK, per
// ADR-007/ADR-009 — and, when the token is valid, attaches its sub claim to the
// request context via WithStudentID, and its custom "role" claim via WithRole.
//
// The Clerk SDK's own WithHeaderAuthorization deliberately does not reject a
// missing or invalid token itself; it just leaves the context unchanged. That
// lets a request with no token still reach the handler, which rejects it with 401
// via StudentIDFromContext returning ok=false — matching the OpenAPI spec's
// documented 401, not Clerk's own default of 403. The admin endpoints follow the
// same pattern for their 403 (RoleFromContext returning a non-admin role).
//
// Known gap: per ADR-007, the JWT sub claim is Clerk's internal user ID, not the
// MotifPath student_id — the Core Domain Service is meant to map sub to student_id
// at registration time. Core Domain Service doesn't exist yet (Phase 4), so there
// is nothing to resolve that mapping against; sub is used directly as student_id
// here. This must be revisited once registration exists. The same applies to role:
// it is read from a Clerk JWT claim rather than Core Domain Service's own
// User.role model, per ADR-012 Part 3.
func ClerkAuthMiddleware(next http.Handler) http.Handler {
	return clerkhttp.WithHeaderAuthorization(
		clerkhttp.CustomClaimsConstructor(func(_ context.Context) any {
			return &roleClaims{}
		}),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claims, ok := clerk.SessionClaimsFromContext(r.Context()); ok {
			ctx := WithStudentID(r.Context(), claims.Subject)
			if custom, ok := claims.Custom.(*roleClaims); ok {
				ctx = WithRole(ctx, custom.Role)
			}
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	}))
}
