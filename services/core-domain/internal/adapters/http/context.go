package http

import "context"

type contextKey int

const clerkUserIDContextKey contextKey = iota

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
