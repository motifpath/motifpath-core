package http

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// errNotImplemented answers the path catalog and self-enrollment endpoints
// until their handlers land.
var errNotImplemented = errors.New("not implemented")

func (h *Handler) ListCatalogPaths(context.Context, generated.ListCatalogPathsRequestObject) (generated.ListCatalogPathsResponseObject, error) {
	return nil, errNotImplemented
}

func (h *Handler) GetCatalogPath(context.Context, generated.GetCatalogPathRequestObject) (generated.GetCatalogPathResponseObject, error) {
	return nil, errNotImplemented
}

func (h *Handler) ListCatalogPathCreators(context.Context, generated.ListCatalogPathCreatorsRequestObject) (generated.ListCatalogPathCreatorsResponseObject, error) {
	return nil, errNotImplemented
}

func (h *Handler) EnrollInLearningPath(context.Context, generated.EnrollInLearningPathRequestObject) (generated.EnrollInLearningPathResponseObject, error) {
	return nil, errNotImplemented
}
