package http

import "context"

type contextKey int

const (
	studentIDContextKey contextKey = iota
	bearerTokenContextKey
)

// WithStudentID stores the authenticated student's ID (the JWT sub claim) in ctx.
// Populated by the Clerk JWT middleware (wired in cmd/main.go); read by
// IngestTrackingEvent to detect a mismatch between the token subject and the event
// payload's student_id.
func WithStudentID(ctx context.Context, studentID string) context.Context {
	return context.WithValue(ctx, studentIDContextKey, studentID)
}

func StudentIDFromContext(ctx context.Context) (string, bool) {
	studentID, ok := ctx.Value(studentIDContextKey).(string)
	return studentID, ok
}

// WithBearerToken stores the raw, already-validated bearer token in ctx.
// Populated by the Clerk JWT middleware; read by the outbox admin endpoints,
// which forward it to the Core Domain Service to establish the caller's role
// (ADR-013). No role claim is decoded from the token itself.
func WithBearerToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, bearerTokenContextKey, token)
}

func BearerTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(bearerTokenContextKey).(string)
	return token, ok
}
