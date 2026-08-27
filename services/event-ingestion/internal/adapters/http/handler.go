package http

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// Handler implements generated.StrictServerInterface.
type Handler struct {
	service     *application.IngestEventService
	adminOutbox *application.AdminOutboxService
	identity    ports.IdentityResolver
	mongoPinger ports.Pinger
	kafkaPinger ports.Pinger
}

var _ generated.StrictServerInterface = (*Handler)(nil)

func NewHandler(service *application.IngestEventService, adminOutbox *application.AdminOutboxService, identity ports.IdentityResolver, mongoPinger, kafkaPinger ports.Pinger) *Handler {
	return &Handler{service: service, adminOutbox: adminOutbox, identity: identity, mongoPinger: mongoPinger, kafkaPinger: kafkaPinger}
}

func (h *Handler) IngestTrackingEvent(ctx context.Context, request generated.IngestTrackingEventRequestObject) (generated.IngestTrackingEventResponseObject, error) {
	event, err := toDomainEvent(request.Body)
	if err != nil {
		return validationErrorResponse(err), nil
	}

	sub, hasSub := StudentIDFromContext(ctx)
	token, hasToken := BearerTokenFromContext(ctx)
	if !hasSub || !hasToken {
		return generated.IngestTrackingEvent401JSONResponse{
			Message: "missing or invalid Bearer token",
		}, nil
	}

	callerUserID, err := h.identity.ResolveUserID(ctx, sub, token)
	if err != nil {
		switch {
		case errors.Is(err, ports.ErrIdentityNotRegistered):
			return generated.IngestTrackingEvent401JSONResponse{
				Message: "the authenticated identity is not a registered MotifPath user",
			}, nil
		case errors.Is(err, ports.ErrProfileUnavailable):
			return generated.IngestTrackingEvent503JSONResponse{
				Message: "the caller's identity could not be resolved: the Core Domain Service was unreachable",
			}, nil
		default:
			return nil, err
		}
	}

	receivedAt, err := h.service.Ingest(ctx, callerUserID, event)
	if err != nil {
		if errors.Is(err, domain.ErrIdentityMismatch) {
			return generated.IngestTrackingEvent401JSONResponse{
				Message: "student_id does not match the authenticated identity",
			}, nil
		}
		return nil, err
	}

	eventID, err := uuid.Parse(event.Base().EventID)
	if err != nil {
		return nil, err
	}

	return generated.IngestTrackingEvent202JSONResponse{
		EventId:    eventID,
		ReceivedAt: receivedAt,
	}, nil
}

func validationErrorResponse(err error) generated.IngestTrackingEvent400JSONResponse {
	reason := "unrecognised event_type"
	if errors.Is(err, domain.ErrMissingRequiredField) {
		reason = "missing required field"
	}

	return generated.IngestTrackingEvent400JSONResponse{
		Message: "request body failed validation",
		Errors: []struct {
			Field  string `json:"field"`
			Reason string `json:"reason"`
		}{{Field: "body", Reason: reason + ": " + err.Error()}},
	}
}

// LivenessCheck always reports ok: it confirms the process is running, nothing more.
func (h *Handler) LivenessCheck(_ context.Context, _ generated.LivenessCheckRequestObject) (generated.LivenessCheckResponseObject, error) {
	return generated.LivenessCheck200JSONResponse{Status: "ok"}, nil
}

// ReadinessCheck pings MongoDB and the Kafka broker and reports per-dependency
// status. The service is ready only when both succeed.
func (h *Handler) ReadinessCheck(ctx context.Context, _ generated.ReadinessCheckRequestObject) (generated.ReadinessCheckResponseObject, error) {
	checks := map[string]generated.HealthStatusChecks{}
	ready := true

	if err := h.mongoPinger.Ping(ctx); err != nil {
		checks["mongodb"] = "fail"
		ready = false
	} else {
		checks["mongodb"] = "ok"
	}

	if err := h.kafkaPinger.Ping(ctx); err != nil {
		checks["kafka_producer"] = "fail"
		ready = false
	} else {
		checks["kafka_producer"] = "ok"
	}

	if !ready {
		return generated.ReadinessCheck503JSONResponse{Status: "degraded", Checks: &checks}, nil
	}
	return generated.ReadinessCheck200JSONResponse{Status: "ok", Checks: &checks}, nil
}
