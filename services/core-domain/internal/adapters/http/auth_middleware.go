package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"

	"github.com/motifpath/core-domain/internal/domain"
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
	return clerkhttp.WithHeaderAuthorization(
		// The SDK fixes this constructor's return type as any; it decodes
		// the verified token payload into whatever struct is returned.
		clerkhttp.CustomClaimsConstructor(func(context.Context) any { return &sessionCustomClaims{} }),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claims, ok := clerk.SessionClaimsFromContext(r.Context()); ok {
			ctx := WithClerkUserID(r.Context(), claims.Subject)
			if custom, ok := claims.Custom.(*sessionCustomClaims); ok {
				ctx = WithIdentityClaims(ctx, domain.IdentityClaims{Name: custom.Name, Email: custom.Email})
			}
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	}))
}

// sessionCustomClaims holds the custom claims this service reads from the
// Clerk session token, which the Clerk instance's session-token template
// must add: "name" is the user's full name and "email" their primary email
// address. A token without one decodes to an empty value, which
// registration rejects and caller resolution ignores.
type sessionCustomClaims struct {
	Name  string
	Email string
}

// UnmarshalJSON keeps "name" and "email" only when each is a JSON string.
// The SDK decodes custom claims while verifying the token, so a decode error
// here would reject the whole token: a misconfigured template that emits
// some other type must cost the user that claim, never their sign-in.
func (c *sessionCustomClaims) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name  json.RawMessage `json:"name"`
		Email json.RawMessage `json:"email"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Name = stringClaim(raw.Name)
	c.Email = stringClaim(raw.Email)
	return nil
}

// stringClaim returns raw's value when it is a JSON string, and "" for any
// other type or an absent claim.
func stringClaim(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}
