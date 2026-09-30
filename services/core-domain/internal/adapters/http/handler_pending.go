package http

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// errNotImplemented answers the self-enrollment endpoint until its handler
// lands.
var errNotImplemented = errors.New("not implemented")

func (h *Handler) EnrollInLearningPath(context.Context, generated.EnrollInLearningPathRequestObject) (generated.EnrollInLearningPathResponseObject, error) {
	return nil, errNotImplemented
}
