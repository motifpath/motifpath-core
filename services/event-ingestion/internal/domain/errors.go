package domain

import "errors"

var (
	// ErrInvalidEventType is returned when a payload's event_type is not one of the
	// seven known values.
	ErrInvalidEventType = errors.New("invalid event type")

	// ErrMissingRequiredField is returned when a required field for the event's type
	// is absent from the payload.
	ErrMissingRequiredField = errors.New("missing required field")
)
