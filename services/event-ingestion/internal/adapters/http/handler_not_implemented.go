package http

import (
	"context"
	"errors"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
)

// ErrNotImplemented is the error of an operation whose contract is published
// ahead of its implementation: it fails as a server error rather than answer
// as if it had worked.
var ErrNotImplemented = errors.New("not implemented yet")

// ListLoggedEvents is not implemented yet.
func (h *Handler) ListLoggedEvents(context.Context, generated.ListLoggedEventsRequestObject) (generated.ListLoggedEventsResponseObject, error) {
	return nil, ErrNotImplemented
}
