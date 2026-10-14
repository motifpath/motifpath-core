package http

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// errPracticeSubjectsNotServed is returned until the composer can focus a
// session, which is what the practice subjects list is for.
var errPracticeSubjectsNotServed = errors.New("the practice subjects list is not served yet")

// ListPracticeSubjects is in the contract ahead of its implementation; it
// answers 500 rather than guess at a list the composer can't use yet.
func (h *Handler) ListPracticeSubjects(_ context.Context, _ generated.ListPracticeSubjectsRequestObject) (generated.ListPracticeSubjectsResponseObject, error) {
	return nil, errPracticeSubjectsNotServed
}
