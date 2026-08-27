package http

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

func (h *Handler) ResolvePublishOutboxEntry(ctx context.Context, request generated.ResolvePublishOutboxEntryRequestObject) (generated.ResolvePublishOutboxEntryResponseObject, error) {
	token, ok := BearerTokenFromContext(ctx)
	if !ok {
		return generated.ResolvePublishOutboxEntry401JSONResponse{Message: "missing or invalid Bearer token"}, nil
	}

	var reason string
	if request.Body != nil && request.Body.Reason != nil {
		reason = *request.Body.Reason
	}

	entry, err := h.adminOutbox.ResolveEntry(ctx, token, request.EventId.String(), reason)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.ResolvePublishOutboxEntry403JSONResponse{Message: adminForbiddenMessage}, nil
		case errors.Is(err, domain.ErrAuthorizationUnavailable):
			return generated.ResolvePublishOutboxEntry503JSONResponse{Message: authUnavailableMessage}, nil
		case errors.Is(err, domain.ErrOutboxEntryNotFound):
			return generated.ResolvePublishOutboxEntry404JSONResponse{Message: outboxNotFoundMessage}, nil
		default:
			return nil, err
		}
	}

	resp, err := toGeneratedOutboxEntry(entry)
	if err != nil {
		return nil, err
	}
	return generated.ResolvePublishOutboxEntry200JSONResponse(resp), nil
}

func (h *Handler) RetryPublishOutboxEntry(ctx context.Context, request generated.RetryPublishOutboxEntryRequestObject) (generated.RetryPublishOutboxEntryResponseObject, error) {
	token, ok := BearerTokenFromContext(ctx)
	if !ok {
		return generated.RetryPublishOutboxEntry401JSONResponse{Message: "missing or invalid Bearer token"}, nil
	}

	entry, err := h.adminOutbox.RetryEntry(ctx, token, request.EventId.String())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			return generated.RetryPublishOutboxEntry403JSONResponse{Message: adminForbiddenMessage}, nil
		case errors.Is(err, domain.ErrAuthorizationUnavailable):
			return generated.RetryPublishOutboxEntry503JSONResponse{Message: authUnavailableMessage}, nil
		case errors.Is(err, domain.ErrOutboxEntryNotFound):
			return generated.RetryPublishOutboxEntry404JSONResponse{Message: outboxNotFoundMessage}, nil
		default:
			return nil, err
		}
	}

	resp, err := toGeneratedOutboxEntry(entry)
	if err != nil {
		return nil, err
	}
	return generated.RetryPublishOutboxEntry200JSONResponse(resp), nil
}

const (
	adminForbiddenMessage  = "the admin role is required for this operation"
	authUnavailableMessage = "the caller's role could not be established: the Core Domain Service was unreachable"
	outboxNotFoundMessage  = "no publish_outbox entry exists for this event_id"
)

func toGeneratedOutboxEntry(entry ports.OutboxEntry) (generated.PublishOutboxEntry, error) {
	eventID, err := uuid.Parse(entry.EventID)
	if err != nil {
		return generated.PublishOutboxEntry{}, err
	}

	resp := generated.PublishOutboxEntry{
		EventId:   eventID,
		Status:    generated.PublishOutboxEntryStatus(entry.Status),
		Attempts:  entry.Attempts,
		UpdatedAt: entry.UpdatedAt,
	}
	if entry.LastError != "" {
		resp.LastError = &entry.LastError
	}
	return resp, nil
}
