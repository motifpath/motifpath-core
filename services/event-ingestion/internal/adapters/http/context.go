package http

import "context"

type contextKey int

const (
	studentIDContextKey contextKey = iota
	roleContextKey
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

// WithRole stores the authenticated caller's role, decoded from the JWT's
// custom "role" claim, in ctx. Populated by the Clerk JWT middleware; read
// by the admin endpoints (ADR-012 Part 3) to authorize access.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleContextKey, role)
}

func RoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(roleContextKey).(string)
	return role, ok
}
