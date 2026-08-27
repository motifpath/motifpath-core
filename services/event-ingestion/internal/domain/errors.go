package domain

import "errors"

var (
	// ErrInvalidEventType is returned when a payload's event_type is not one of the
	// seven known values.
	ErrInvalidEventType = errors.New("invalid event type")

	// ErrMissingRequiredField is returned when a required field for the event's type
	// is absent from the payload.
	ErrMissingRequiredField = errors.New("missing required field")

	// ErrOutboxEntryNotFound is returned by the admin retry/resolve endpoints
	// when no publish_outbox entry exists for the given event_id.
	ErrOutboxEntryNotFound = errors.New("publish outbox entry not found")

	// ErrForbidden is returned when the authenticated caller is not permitted
	// to perform the requested operation -- for the admin endpoints, when the
	// caller's resolved role is not admin (a caller with no registered profile
	// is treated the same way).
	ErrForbidden = errors.New("caller is not authorized for this operation")

	// ErrAuthorizationUnavailable is returned when the caller's authorization
	// could not be established because the identity/authorization capability
	// was unreachable. Distinct from ErrForbidden: the answer is "unknown right
	// now", not "no". Per ADR-013 the admin endpoints fail closed on this.
	ErrAuthorizationUnavailable = errors.New("authorization could not be established")

	// ErrIdentityMismatch is returned by IngestEventService.Ingest when an
	// event's student_id is not the caller's own resolved MotifPath user id
	// (ADR-014). The caller is authenticated but is claiming authorship for
	// someone else.
	ErrIdentityMismatch = errors.New("event student_id is not the caller's identity")
)
