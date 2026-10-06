package http

import (
	"context"
	"errors"
	"strings"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// errNotImplemented answers the practice summary until its handler lands.
var errNotImplemented = errors.New("not implemented")

func (h *Handler) GetPracticeSummary(context.Context, generated.GetPracticeSummaryRequestObject) (generated.GetPracticeSummaryResponseObject, error) {
	return nil, errNotImplemented
}

func (h *Handler) GetPracticeOverview(context.Context, generated.GetPracticeOverviewRequestObject) (generated.GetPracticeOverviewResponseObject, error) {
	return nil, errNotImplemented
}

func (h *Handler) CreatePracticeSessionPlan(ctx context.Context, request generated.CreatePracticeSessionPlanRequestObject) (generated.CreatePracticeSessionPlanResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.CreatePracticeSessionPlan401JSONResponse(unauthorizedError()), nil
	}
	if request.Body == nil {
		return generated.CreatePracticeSessionPlan400JSONResponse(validationErrorResponse(domain.NewValidationError("minutes", "is required"))), nil
	}

	plan, err := h.practiceSession.ComposePlan(ctx, caller, uuidPtrToStringPtr(request.Body.InstrumentId), request.Body.Minutes)
	switch kind, valErr := classify(err); {
	case err == nil:
		var creatorIDs []string
		for _, item := range plan.Items {
			if item.Exercise != nil && item.Exercise.CreatedBy != "" {
				creatorIDs = append(creatorIDs, item.Exercise.CreatedBy)
			}
		}
		names, err := h.loadUserNames(ctx, creatorIDs)
		if err != nil {
			return nil, err
		}
		return generated.CreatePracticeSessionPlan200JSONResponse(toGeneratedPracticeSessionPlan(plan, names)), nil
	case kind == errKindValidation:
		return generated.CreatePracticeSessionPlan400JSONResponse(validationErrorResponse(valErr)), nil
	case kind == errKindNotFound:
		return generated.CreatePracticeSessionPlan404JSONResponse(notFoundError(notFoundMessage(err))), nil
	default:
		return nil, err
	}
}

// notFoundMessage is a domain.ErrNotFound's own explanation, without the
// sentinel's prefix.
func notFoundMessage(err error) string {
	return strings.TrimPrefix(err.Error(), domain.ErrNotFound.Error()+": ")
}

func toGeneratedPracticeSessionPlan(plan domain.PracticeSessionPlan, names userNames) generated.PracticeSessionPlan {
	items := make([]generated.PracticeSessionItem, len(plan.Items))
	for i, item := range plan.Items {
		items[i] = toGeneratedPracticeSessionItem(item, names)
	}
	return generated.PracticeSessionPlan{
		PracticeSessionId: mustUUID(plan.ID),
		InstrumentId:      uuidPtrFromStringPtr(plan.InstrumentID),
		Minutes:           plan.Minutes,
		Items:             items,
	}
}

func toGeneratedPracticeSessionItem(item domain.PracticeSessionItem, names userNames) generated.PracticeSessionItem {
	out := generated.PracticeSessionItem{
		ItemKey:          item.ItemKey,
		Kind:             generated.PracticeItemKind(item.Kind),
		Reason:           generated.PracticePickReason(item.Reason),
		NodeId:           uuidPtrFromStringPtr(item.NodeID),
		Level:            generated.KnowledgeLevel(item.Level),
		EstimatedSeconds: item.EstimatedSeconds,
	}
	if p := item.PlayAlong; p != nil {
		out.PlayAlong = &struct {
			BestCleanTempoBpm *int               `json:"best_clean_tempo_bpm"`
			DiagramId         openapi_types.UUID `json:"diagram_id"`
			StartTempoBpm     int                `json:"start_tempo_bpm"`
			TargetTempoBpm    int                `json:"target_tempo_bpm"`
		}{
			BestCleanTempoBpm: p.BestCleanTempoBPM,
			DiagramId:         mustUUID(p.DiagramID),
			StartTempoBpm:     p.StartTempoBPM,
			TargetTempoBpm:    p.TargetTempoBPM,
		}
	}
	if e := item.Exercise; e != nil {
		exercise := toExercise(*e, names)
		out.Exercise = &exercise
	}
	return out
}
