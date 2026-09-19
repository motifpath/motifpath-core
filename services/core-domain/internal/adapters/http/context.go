package http

import "context"

type contextKey int

const (
	clerkUserIDContextKey contextKey = iota
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
