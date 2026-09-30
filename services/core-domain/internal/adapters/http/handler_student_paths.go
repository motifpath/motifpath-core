package http

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func (h *Handler) EnrollInLearningPath(ctx context.Context, request generated.EnrollInLearningPathRequestObject) (generated.EnrollInLearningPathResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.EnrollInLearningPath401JSONResponse(unauthorizedError()), nil
	}

	if request.Body == nil || request.Body.LearningPathId == uuid.Nil {
		return generated.EnrollInLearningPath400JSONResponse(validationErrorResponse(domain.NewValidationError("learning_path_id", "is required"))), nil
	}

	sp, created, err := h.studentPath.EnrollInLearningPath(ctx, caller, request.Body.LearningPathId.String())
	if errors.Is(err, domain.ErrNotFound) {
		return generated.EnrollInLearningPath404JSONResponse(notFoundError("no published learning path exists with the given id, or one of its content nodes has never been published")), nil
	}
	if err != nil {
		return nil, err
	}

	cards, err := h.studentPathCards(ctx, caller.ID, sp)
	if err != nil {
		return nil, err
	}
	if !created {
		return generated.EnrollInLearningPath200JSONResponse(cards[0]), nil
	}
	return generated.EnrollInLearningPath201JSONResponse(cards[0]), nil
}

// studentPathCards renders paths, all owned by studentID, with their
// completed lesson counts and every referenced user's name, reading each
// in one lookup.
func (h *Handler) studentPathCards(ctx context.Context, studentID string, paths ...domain.StudentPath) ([]generated.StudentPath, error) {
	names, err := h.loadUserNames(ctx, studentPathUserIDs(paths...))
	if err != nil {
		return nil, err
	}
	completed, err := h.studentPath.CompletedCounts(ctx, studentID, paths)
	if err != nil {
		return nil, err
	}
	cards := make([]generated.StudentPath, len(paths))
	for i, sp := range paths {
		cards[i] = toStudentPath(sp, names, completed[sp.ID])
	}
	return cards, nil
}
