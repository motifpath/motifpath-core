package http

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// adminRole is the value the "role" custom Clerk claim must carry for a
// caller to use the admin endpoints. Sourced from the JWT directly rather
// than Core Domain Service's own User.role model, which does not exist yet
// -- see ADR-012 Part 3.
const adminRole = "admin"

func (h *Handler) ResolvePublishOutboxEntry(ctx context.Context, request generated.ResolvePublishOutboxEntryRequestObject) (generated.ResolvePublishOutboxEntryResponseObject, error) {
	if !authenticated(ctx) {
		return generated.ResolvePublishOutboxEntry401JSONResponse{Message: "missing or invalid Bearer token"}, nil
	}
	if !isAdmin(ctx) {
		return generated.ResolvePublishOutboxEntry403JSONResponse{Message: "the admin role is required for this operation"}, nil
	}

	var reason string
	if request.Body != nil && request.Body.Reason != nil {
		reason = *request.Body.Reason
	}

	entry, err := h.adminOutbox.ResolveEntry(ctx, request.EventId.String(), reason)
	if err != nil {
		if errors.Is(err, domain.ErrOutboxEntryNotFound) {
			return generated.ResolvePublishOutboxEntry404JSONResponse{
				Message: "no publish_outbox entry exists for this event_id",
			}, nil
		}
		return nil, err
	}

	resp, err := toGeneratedOutboxEntry(entry)
	if err != nil {
		return nil, err
	}
	return generated.ResolvePublishOutboxEntry200JSONResponse(resp), nil
}

func (h *Handler) RetryPublishOutboxEntry(ctx context.Context, request generated.RetryPublishOutboxEntryRequestObject) (generated.RetryPublishOutboxEntryResponseObject, error) {
	if !authenticated(ctx) {
		return generated.RetryPublishOutboxEntry401JSONResponse{Message: "missing or invalid Bearer token"}, nil
	}
	if !isAdmin(ctx) {
		return generated.RetryPublishOutboxEntry403JSONResponse{Message: "the admin role is required for this operation"}, nil
	}

	entry, err := h.adminOutbox.RetryEntry(ctx, request.EventId.String())
	if err != nil {
		if errors.Is(err, domain.ErrOutboxEntryNotFound) {
			return generated.RetryPublishOutboxEntry404JSONResponse{
				Message: "no publish_outbox entry exists for this event_id",
			}, nil
		}
		return nil, err
	}

	resp, err := toGeneratedOutboxEntry(entry)
	if err != nil {
		return nil, err
	}
	return generated.RetryPublishOutboxEntry200JSONResponse(resp), nil
}

// authenticated reuses StudentIDFromContext, which really just holds the
// JWT sub claim regardless of caller role -- an admin's identity lands
// there the same way a student's does. Its name reflects the endpoint it
// was first built for, not a constraint on who it applies to.
func authenticated(ctx context.Context) bool {
	_, ok := StudentIDFromContext(ctx)
	return ok
}

func isAdmin(ctx context.Context) bool {
	role, _ := RoleFromContext(ctx)
	return role == adminRole
}

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
