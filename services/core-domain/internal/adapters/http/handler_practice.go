package http

import (
	"context"
	"strings"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// anyInstrumentGroupNames names the group of practice nodes that suit any
// instrument, in each language MotifPath offers.
var anyInstrumentGroupNames = generated.LocalizedNames{"en": "Any instrument", "pt_BR": "Qualquer instrumento"}

func (h *Handler) GetPracticeSummary(ctx context.Context, request generated.GetPracticeSummaryRequestObject) (generated.GetPracticeSummaryResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetPracticeSummary401JSONResponse(unauthorizedError()), nil
	}

	summary, err := h.practiceSummary.Summary(ctx, caller, uuidPtrToStringPtr(request.Params.InstrumentId), deref(request.Params.TimeZone))
	switch kind, valErr := classify(err); {
	case err == nil:
		return generated.GetPracticeSummary200JSONResponse(toGeneratedPracticeSummary(summary)), nil
	case kind == errKindValidation:
		return generated.GetPracticeSummary400JSONResponse(validationErrorResponse(valErr)), nil
	case kind == errKindNotFound:
		return generated.GetPracticeSummary404JSONResponse(notFoundError(notFoundMessage(err))), nil
	default:
		return nil, err
	}
}

func (h *Handler) GetPracticeOverview(ctx context.Context, request generated.GetPracticeOverviewRequestObject) (generated.GetPracticeOverviewResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetPracticeOverview401JSONResponse(unauthorizedError()), nil
	}

	overview, err := h.practiceSummary.Overview(ctx, caller, deref(request.Params.TimeZone))
	switch kind, valErr := classify(err); {
	case err == nil:
		return generated.GetPracticeOverview200JSONResponse(toGeneratedPracticeOverview(overview)), nil
	case kind == errKindValidation:
		return generated.GetPracticeOverview400JSONResponse(validationErrorResponse(valErr)), nil
	default:
		return nil, err
	}
}

func toGeneratedPracticeSummary(summary application.PracticeSummary) generated.PracticeSummary {
	out := generated.PracticeSummary{
		InstrumentId:         uuidPtrFromStringPtr(summary.InstrumentID),
		StudentInstrumentIds: make([]openapi_types.UUID, len(summary.StudentInstrumentIDs)),
		PracticeDaysLast7:    summary.PracticeDaysLast7,
		ProgressThisWeek:     make([]generated.SkillProgress, len(summary.Progress)),
		NextSteps:            make([]generated.PracticeNextStep, len(summary.NextSteps)),
		NextStepsTotal:       summary.NextStepsTotal,
		Groups:               make([]generated.PracticeNodeGroup, len(summary.Groups)),
	}
	for i, id := range summary.StudentInstrumentIDs {
		out.StudentInstrumentIds[i] = mustUUID(id)
	}
	for i, line := range summary.Progress {
		out.ProgressThisWeek[i] = generated.SkillProgress{
			NodeId:  mustUUID(line.NodeID),
			Names:   generated.LocalizedNames(summary.Nodes[line.NodeID].Names),
			Measure: generated.SkillProgressMeasure(line.Measure),
			Before:  float32(line.Before),
			After:   float32(line.After),
		}
	}
	for i, step := range summary.NextSteps {
		out.NextSteps[i] = toGeneratedPracticeNextStep(step)
	}
	for i, group := range summary.Groups {
		g := generated.PracticeNodeGroup{AnyInstrument: group.AnyInstrument, Names: anyInstrumentGroupNames, Nodes: make([]generated.PracticeNodeProgress, len(group.Nodes))}
		if group.Area != nil {
			id := mustUUID(group.Area.ID)
			g.AreaNodeId, g.Names = &id, generated.LocalizedNames(group.Area.Names)
		}
		for j, n := range group.Nodes {
			g.Nodes[j] = toGeneratedPracticeNodeProgress(n, summary.Standings[n.ID], summary.Children[n.ID])
		}
		out.Groups[i] = g
	}
	return out
}

// toGeneratedPracticeNodeProgress maps one node's standing. A wide node,
// one with children to show it by, has no level of its own.
func toGeneratedPracticeNodeProgress(node domain.KnowledgeNode, standing domain.NodeStanding, childIDs []string) generated.PracticeNodeProgress {
	out := generated.PracticeNodeProgress{
		NodeId:       mustUUID(node.ID),
		Names:        generated.LocalizedNames(node.Names),
		Fading:       standing.Fading,
		ChildNodeIds: make([]openapi_types.UUID, len(childIDs)),
	}
	if len(childIDs) == 0 && standing.Level != nil {
		level := generated.KnowledgeLevel(*standing.Level)
		out.Level = &level
	}
	out.Coverage.MetCount, out.Coverage.ItemCount = standing.Covered, standing.Total
	out.Readiness.MetCount, out.Readiness.RequiredCount = standing.Readiness.Met, standing.Readiness.Total
	for i, id := range childIDs {
		out.ChildNodeIds[i] = mustUUID(id)
	}
	return out
}

func toGeneratedPracticeNextStep(step domain.PracticeNextStep) generated.PracticeNextStep {
	out := generated.PracticeNextStep{
		Kind:   generated.PracticeNextStepKind(step.Kind),
		NodeId: mustUUID(step.Node.ID),
		Names:  generated.LocalizedNames(step.Node.Names),
	}
	if step.Level != nil {
		level := generated.KnowledgeLevel(*step.Level)
		out.Level = &level
	}
	return out
}

func toGeneratedPracticeOverview(overview application.PracticeOverview) generated.PracticeOverview {
	out := generated.PracticeOverview{
		PracticeDaysLast7: overview.PracticeDaysLast7,
		LearningDaysLast7: overview.LearningDaysLast7,
		Instruments:       make([]generated.PracticeInstrumentCard, len(overview.Instruments)),
	}
	for i, card := range overview.Instruments {
		out.Instruments[i] = generated.PracticeInstrumentCard{InstrumentId: mustUUID(card.InstrumentID), PracticeDaysLast7: card.PracticeDaysLast7}
		if card.TopNextStep != nil {
			step := toGeneratedPracticeNextStep(*card.TopNextStep)
			out.Instruments[i].TopNextStep = &step
		}
	}
	return out
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
