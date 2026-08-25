package http

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
)

// Handler implements generated.StrictServerInterface.
type Handler struct {
	service *application.IngestEventService
}

var _ generated.StrictServerInterface = (*Handler)(nil)

func NewHandler(service *application.IngestEventService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) IngestTrackingEvent(ctx context.Context, request generated.IngestTrackingEventRequestObject) (generated.IngestTrackingEventResponseObject, error) {
	event, err := toDomainEvent(request.Body)
	if err != nil {
		return validationErrorResponse(err), nil
	}

	authenticatedStudentID, ok := StudentIDFromContext(ctx)
	if !ok || event.Base().StudentID != authenticatedStudentID {
		return generated.IngestTrackingEvent401JSONResponse{
			Message: "student_id does not match the authenticated session",
		}, nil
	}

	receivedAt, err := h.service.Ingest(ctx, event)
	if err != nil {
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

// ReadinessCheck is a placeholder until cmd/main.go wires in the Mongo and Kafka
// clients this handler needs to report real dependency health (Phase 3.8).
func (h *Handler) ReadinessCheck(_ context.Context, _ generated.ReadinessCheckRequestObject) (generated.ReadinessCheckResponseObject, error) {
	return generated.ReadinessCheck200JSONResponse{Status: "ok"}, nil
}
