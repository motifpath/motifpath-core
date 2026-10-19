package http

import "context"

type contextKey int

const (
	clerkUserIDContextKey contextKey = iota
	nameClaimContextKey
	acceptLanguageContextKey
)

// WithClerkUserID stores the authenticated caller's Clerk identity (the JWT
// sub claim) in ctx. Populated by ClerkAuthMiddleware; read by
// Handler.resolveCaller to look up the corresponding MotifPath User record.
func WithClerkUserID(ctx context.Context, clerkUserID string) context.Context {
	return context.WithValue(ctx, clerkUserIDContextKey, clerkUserID)
}

func ClerkUserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(clerkUserIDContextKey).(string)
	return v, ok
}

// WithNameClaim stores the "name" claim of the caller's session token — the
// user's full name as the identity provider holds it — in ctx. Populated by
// ClerkAuthMiddleware; read by registration and caller resolution to record
// and refresh the user's display name.
func WithNameClaim(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, nameClaimContextKey, name)
}

// NameClaimFromContext returns "" if the session token carried no name.
func NameClaimFromContext(ctx context.Context) string {
	v, _ := ctx.Value(nameClaimContextKey).(string)
	return v
}

// WithAcceptLanguageCandidate stores the request's normalized
// Accept-Language locale candidate (see domain.NormalizeAcceptLanguage) in
// ctx. Populated by AcceptLanguageMiddleware; read by Handler.RegisterUser
// to resolve a new user's initial locale.
func WithAcceptLanguageCandidate(ctx context.Context, candidate string) context.Context {
	return context.WithValue(ctx, acceptLanguageContextKey, candidate)
}

// AcceptLanguageCandidateFromContext returns "" if no candidate was stored.
func AcceptLanguageCandidateFromContext(ctx context.Context) string {
	v, _ := ctx.Value(acceptLanguageContextKey).(string)
	return v
}
