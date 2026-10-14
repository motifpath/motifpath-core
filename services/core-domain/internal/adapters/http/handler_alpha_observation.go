package http

import (
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const onlyAdminsObserve = "only admins may observe the alpha"

func (h *Handler) GetAlphaObservation(ctx context.Context, request generated.GetAlphaObservationRequestObject) (generated.GetAlphaObservationResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetAlphaObservation401JSONResponse(unauthorizedError()), nil
	}
	observation, err := h.alphaObservation.Observe(ctx, caller, weekParam(request.Params.Week))
	var verr *domain.ValidationError
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return generated.GetAlphaObservation403JSONResponse(forbiddenError(onlyAdminsObserve)), nil
	case errors.As(err, &verr):
		return generated.GetAlphaObservation400JSONResponse(validationErrorResponse(verr)), nil
	case err != nil:
		return nil, err
	}
	students := make([]generated.AlphaStudent, len(observation.Students))
	for i, s := range observation.Students {
		students[i] = toAlphaStudent(s)
	}
	s := observation.Summary
	return generated.GetAlphaObservation200JSONResponse{
		Week: toAlphaWeekRange(observation.Week, observation.Running),
		Summary: generated.AlphaCohortSummary{
			StudentCount: s.StudentCount, AtTargetInWeek: s.AtTargetInWeek, AtTargetInPreviousWeek: s.AtTargetInPreviousWeek,
			AtTargetEveryWeek: s.AtTargetEveryWeek, FirstPathOverTarget: s.FirstPathOverTarget, MedianFirstPathPercent: s.MedianFirstPathPercent,
		},
		Students: students,
	}, nil
}

func (h *Handler) GetAlphaStudentObservation(ctx context.Context, request generated.GetAlphaStudentObservationRequestObject) (generated.GetAlphaStudentObservationResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetAlphaStudentObservation401JSONResponse(unauthorizedError()), nil
	}
	observation, err := h.alphaObservation.ObserveStudent(ctx, caller, request.StudentId.String(), weekParam(request.Params.Week))
	var verr *domain.ValidationError
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return generated.GetAlphaStudentObservation403JSONResponse(forbiddenError(onlyAdminsObserve)), nil
	case errors.As(err, &verr):
		return generated.GetAlphaStudentObservation400JSONResponse(validationErrorResponse(verr)), nil
	case errors.Is(err, domain.ErrNotFound):
		return generated.GetAlphaStudentObservation404JSONResponse(notFoundError("no student with this user_id is in the alpha")), nil
	case err != nil:
		return nil, err
	}
	weeks := make([]generated.AlphaSessionWeek, len(observation.SessionWeeks))
	for i, w := range observation.SessionWeeks {
		weeks[i] = toAlphaSessionWeek(w)
	}
	nodes := make([]generated.AlphaPathNode, len(observation.Nodes))
	for i, n := range observation.Nodes {
		nodes[i] = generated.AlphaPathNode{
			Position: n.Position, ContentNodeId: mustUUID(n.ContentNodeID), Title: n.Title,
			Status: generated.AlphaPathNodeStatus(n.Status), CompletedAt: n.CompletedAt, StartedAt: n.StartedAt,
		}
	}
	return generated.GetAlphaStudentObservation200JSONResponse{
		Week:           toAlphaWeekRange(observation.Week, observation.Running),
		Student:        toAlphaStudent(observation.Student),
		SessionWeeks:   weeks,
		FirstPathNodes: nodes,
	}, nil
}

func weekParam(week *openapi_types.Date) *time.Time {
	if week == nil {
		return nil
	}
	return &week.Time
}

func toAlphaWeekRange(week domain.AlphaWeek, running bool) generated.AlphaWeekRange {
	return generated.AlphaWeekRange{
		WeekStart: openapi_types.Date{Time: week.FirstDay()},
		WeekEnd:   openapi_types.Date{Time: week.LastDay()},
		Running:   running,
	}
}

func toAlphaStudent(v application.AlphaStudentView) generated.AlphaStudent {
	m := v.Student.Measure
	weeks := make([]generated.AlphaWeekCount, len(m.WeeksSoFar))
	for i, w := range m.WeeksSoFar {
		weeks[i] = generated.AlphaWeekCount{WeekStart: openapi_types.Date{Time: w.WeekStart}, FinishedSessions: w.Finished}
	}
	p := v.Student.FirstPath
	return generated.AlphaStudent{
		StudentId:              mustUUID(v.Student.StudentID),
		DisplayName:            v.Student.DisplayName,
		RegisteredAt:           v.RegisteredAt,
		FinishedSessionsInWeek: m.FinishedInWeek,
		AtTargetInWeek:         m.AtTargetInWeek(),
		WeeksSoFar:             weeks,
		WeeksSoFarCount:        m.WeeksSoFarCount,
		WeeksAtTargetCount:     m.WeeksAtTargetCount,
		AtTargetEveryWeek:      m.AtTargetEveryWeek,
		LastActiveAt:           v.LastActiveAt,
		Quiet:                  m.Quiet,
		FirstPath: generated.AlphaFirstPath{
			StudentPathId: mustUUID(v.FirstPathID), Title: v.FirstPathTitle, GivenAt: v.FirstPathGivenAt,
			CompletedNodes: p.Completed, TotalNodes: p.Total, CompletedPercent: p.Percent(), OverTarget: p.OverTarget(),
		},
	}
}

func toAlphaSessionWeek(w application.AlphaSessionWeek) generated.AlphaSessionWeek {
	sessions := make([]generated.AlphaSession, len(w.Sessions))
	for i, s := range w.Sessions {
		sessions[i] = generated.AlphaSession{
			PracticeSessionId: mustUUID(s.Record.ID),
			StartedAt:         s.Record.StartedAt,
			PlannedMinutes:    s.Record.PlannedMinutes,
			EndedAt:           s.EndedAt,
			State:             generated.AlphaSessionState(s.State),
			ItemsAnswered:     s.Record.ItemsAnswered(),
		}
	}
	return generated.AlphaSessionWeek{Week: toAlphaWeekRange(w.Week, w.Running), FinishedSessions: w.Finished, Sessions: sessions}
}
