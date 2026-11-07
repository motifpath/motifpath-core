package http

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const onlyAdminsManageProgressions = "Only admins create, change and remove progressions."

// errProgressionDrillsUnavailable answers adding a chart-only progression to the
// drills, which isn't served yet.
var errProgressionDrillsUnavailable = errors.New("adding a progression to the drills is not available yet")

func (h *Handler) CreateProgression(ctx context.Context, request generated.CreateProgressionRequestObject) (generated.CreateProgressionResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreateProgression401JSONResponse(unauthorizedError()), nil
	}
	body := request.Body
	input := application.ProgressionInput{Mode: string(body.Mode), Bars: body.Bars}
	if body.Nicknames != nil {
		input.Nicknames = *body.Nicknames
	}
	if body.InDrills != nil {
		input.InDrills = *body.InDrills
	}
	input.SkillIDs = optionalUUIDStrings(body.SkillIds)
	input.ConceptIDs = optionalUUIDStrings(body.ConceptIds)
	created, err := h.progressionCatalog.Create(ctx, caller, input)
	if err == nil {
		out, mapErr := h.toGeneratedProgressionWithCreator(ctx, created)
		return generated.CreateProgression201JSONResponse(out), mapErr
	}
	var exists *domain.ProgressionExistsError
	if errors.As(err, &exists) {
		return h.progressionExistsResponse(ctx, exists)
	}
	var notAddable *domain.ProgressionNotAddableError
	if errors.As(err, &notAddable) {
		var out generated.CreateProgression409JSONResponse
		if err := out.FromProgressionNotAddableError(toGeneratedProgressionNotAddable(notAddable)); err != nil {
			return nil, err
		}
		return out, nil
	}
	kind, valErr := classify(err)
	switch kind {
	case errKindValidation:
		return generated.CreateProgression400JSONResponse(validationErrorResponse(valErr)), nil
	case errKindForbidden:
		return generated.CreateProgression403JSONResponse(forbiddenError(onlyAdminsManageProgressions)), nil
	case errKindNotFound, errKindOther:
	}
	return nil, err
}

func (h *Handler) progressionExistsResponse(ctx context.Context, exists *domain.ProgressionExistsError) (generated.CreateProgressionResponseObject, error) {
	existing, err := h.toGeneratedProgressionWithCreator(ctx, exists.Existing)
	if err != nil {
		return nil, err
	}
	var out generated.CreateProgression409JSONResponse
	if err := out.FromProgressionExistsError(generated.ProgressionExistsError{Message: exists.Error(), Progression: existing}); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) UpdateProgression(ctx context.Context, request generated.UpdateProgressionRequestObject) (generated.UpdateProgressionResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.UpdateProgression401JSONResponse(unauthorizedError()), nil
	}
	updated, err := h.progressionCatalog.UpdateNicknames(ctx, caller, request.ProgressionId.String(), request.Body.Nicknames)
	if err == nil {
		out, mapErr := h.toGeneratedProgressionWithCreator(ctx, updated)
		return generated.UpdateProgression200JSONResponse(out), mapErr
	}
	if errors.Is(err, domain.ErrProgressionIsCatalog) {
		return generated.UpdateProgression409JSONResponse{Message: "A catalog progression's names can't be changed."}, nil
	}
	kind, valErr := classify(err)
	switch kind {
	case errKindValidation:
		return generated.UpdateProgression400JSONResponse(validationErrorResponse(valErr)), nil
	case errKindNotFound:
		return generated.UpdateProgression404JSONResponse(notFoundError("progression not found")), nil
	case errKindForbidden:
		return generated.UpdateProgression403JSONResponse(forbiddenError(onlyAdminsManageProgressions)), nil
	case errKindOther:
	}
	return nil, err
}

func (h *Handler) DeleteProgression(ctx context.Context, request generated.DeleteProgressionRequestObject) (generated.DeleteProgressionResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.DeleteProgression401JSONResponse(unauthorizedError()), nil
	}
	err := h.progressionCatalog.Delete(ctx, caller, request.ProgressionId.String())
	if err == nil {
		return generated.DeleteProgression204Response{}, nil
	}
	var notRemovable *domain.ProgressionNotRemovableError
	if errors.As(err, &notRemovable) {
		return toGeneratedProgressionInUse(notRemovable), nil
	}
	kind, valErr := classify(err)
	switch kind {
	case errKindValidation:
		return generated.DeleteProgression400JSONResponse(validationErrorResponse(valErr)), nil
	case errKindNotFound:
		return generated.DeleteProgression404JSONResponse(notFoundError("progression not found")), nil
	case errKindForbidden:
		return generated.DeleteProgression403JSONResponse(forbiddenError(onlyAdminsManageProgressions)), nil
	case errKindOther:
	}
	return nil, err
}

// GetProgressionUses isn't served yet.
func (h *Handler) GetProgressionUses(context.Context, generated.GetProgressionUsesRequestObject) (generated.GetProgressionUsesResponseObject, error) {
	return nil, errProgressionDrillsUnavailable
}

// AddProgressionToDrills isn't served yet.
func (h *Handler) AddProgressionToDrills(context.Context, generated.AddProgressionToDrillsRequestObject) (generated.AddProgressionToDrillsResponseObject, error) {
	return nil, errProgressionDrillsUnavailable
}

func toGeneratedProgressionNotAddable(e *domain.ProgressionNotAddableError) generated.ProgressionNotAddableError {
	reasons := make([]generated.ProgressionNotAddableErrorReasons, len(e.Reasons))
	for i, reason := range e.Reasons {
		reasons[i] = generated.ProgressionNotAddableErrorReasons(reason)
	}
	return generated.ProgressionNotAddableError{Message: e.Error(), Reasons: reasons}
}

func toGeneratedProgressionInUse(e *domain.ProgressionNotRemovableError) generated.DeleteProgression409JSONResponse {
	uses := make([]generated.ProgressionUse, len(e.Uses))
	for i, u := range e.Uses {
		uses[i] = generated.ProgressionUse{
			SongChartId:  mustUUID(u.SongChartID),
			Title:        u.Title,
			Revision:     generated.ProgressionUseRevision(u.Revision),
			SectionIndex: u.SectionIndex,
			SectionLabel: u.SectionLabel,
		}
	}
	return generated.DeleteProgression409JSONResponse{
		Message: e.Error(),
		Reason:  generated.ProgressionInUseErrorReason(e.Reason),
		Uses:    uses,
	}
}

// toGeneratedProgressionWithCreator maps one progression, looking up its creator.
func (h *Handler) toGeneratedProgressionWithCreator(ctx context.Context, p domain.Progression) (generated.Progression, error) {
	names, err := h.identity.KnownDisplayNames(ctx, progressionUserIDs(p))
	if err != nil {
		return generated.Progression{}, err
	}
	return toGeneratedProgression(p, names), nil
}

func progressionUserIDs(progressions ...domain.Progression) []string {
	var ids []string
	for _, p := range progressions {
		if p.CreatedBy != nil {
			ids = append(ids, *p.CreatedBy)
		}
	}
	return ids
}
