package http

import (
	"context"
	"errors"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

func (h *Handler) ListLoggedEvents(ctx context.Context, request generated.ListLoggedEventsRequestObject) (generated.ListLoggedEventsResponseObject, error) {
	token, ok := BearerTokenFromContext(ctx)
	if !ok {
		return generated.ListLoggedEvents401JSONResponse{Message: "missing or invalid Bearer token"}, nil
	}
	params := request.Params
	var studentID, typeGroup *string
	if params.StudentId != nil {
		id := params.StudentId.String()
		studentID = &id
	}
	if params.TypeGroup != nil {
		group := string(*params.TypeGroup)
		typeGroup = &group
	}
	query, err := domain.NewEventLogQuery(studentID, typeGroup, params.Limit, params.Offset)
	var invalid *domain.InvalidQueryError
	if errors.As(err, &invalid) {
		resp := generated.ListLoggedEvents400JSONResponse{Message: "the request's parameters are invalid"}
		resp.Errors = append(resp.Errors, struct {
			Field  string `json:"field"`
			Reason string `json:"reason"`
		}{Field: invalid.Field, Reason: invalid.Reason})
		return resp, nil
	}
	if err != nil {
		return nil, err
	}

	page, err := h.eventLog.List(ctx, token, query)
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return generated.ListLoggedEvents403JSONResponse{Message: adminForbiddenMessage}, nil
	case errors.Is(err, domain.ErrAuthorizationUnavailable):
		return generated.ListLoggedEvents503JSONResponse{Message: authUnavailableMessage}, nil
	case err != nil:
		return nil, err
	}

	items := make([]generated.LoggedEvent, len(page.Items))
	for i, logged := range page.Items {
		event, err := toGeneratedTrackingEvent(logged.Event)
		if err != nil {
			return nil, err
		}
		items[i] = generated.LoggedEvent{ReceivedAt: logged.ReceivedAt, Event: event}
	}
	return generated.ListLoggedEvents200JSONResponse{Items: items, Total: page.Total, Limit: page.Limit, Offset: page.Offset}, nil
}
