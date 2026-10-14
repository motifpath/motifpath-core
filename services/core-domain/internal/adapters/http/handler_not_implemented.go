package http

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// ErrNotImplemented is the error of an operation whose contract is published
// ahead of its implementation: it fails as a server error rather than answer
// as if it had worked.
var ErrNotImplemented = errors.New("not implemented yet")

// ListPracticeSubjects is not implemented yet.
func (h *Handler) ListPracticeSubjects(context.Context, generated.ListPracticeSubjectsRequestObject) (generated.ListPracticeSubjectsResponseObject, error) {
	return nil, ErrNotImplemented
}
