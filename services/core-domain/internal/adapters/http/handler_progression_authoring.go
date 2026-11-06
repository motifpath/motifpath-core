package http

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

// errProgressionAuthoringUnavailable answers the progression authoring
// operations until they are served.
var errProgressionAuthoringUnavailable = errors.New("progression authoring is not available yet")

// CreateProgression isn't served yet.
func (h *Handler) CreateProgression(context.Context, generated.CreateProgressionRequestObject) (generated.CreateProgressionResponseObject, error) {
	return nil, errProgressionAuthoringUnavailable
}

// UpdateProgression isn't served yet.
func (h *Handler) UpdateProgression(context.Context, generated.UpdateProgressionRequestObject) (generated.UpdateProgressionResponseObject, error) {
	return nil, errProgressionAuthoringUnavailable
}

// DeleteProgression isn't served yet.
func (h *Handler) DeleteProgression(context.Context, generated.DeleteProgressionRequestObject) (generated.DeleteProgressionResponseObject, error) {
	return nil, errProgressionAuthoringUnavailable
}

// AddProgressionToDrills isn't served yet.
func (h *Handler) AddProgressionToDrills(context.Context, generated.AddProgressionToDrillsRequestObject) (generated.AddProgressionToDrillsResponseObject, error) {
	return nil, errProgressionAuthoringUnavailable
}
