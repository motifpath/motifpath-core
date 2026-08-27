package application

import (
	"context"
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// AdminOutboxService implements the manual-remediation operations from
// ADR-012 Part 3, for the two admin-only endpoints. Per ADR-013 both
// operations authorize the caller through AdminAuthorizer before touching
// any outbox state.
type AdminOutboxService struct {
	outbox     ports.PublishOutboxRepository
	events     ports.EventRepository
	publisher  ports.EventPublisher
	authorizer *AdminAuthorizer
}

func NewAdminOutboxService(outbox ports.PublishOutboxRepository, events ports.EventRepository, publisher ports.EventPublisher, authorizer *AdminAuthorizer) *AdminOutboxService {
	return &AdminOutboxService{outbox: outbox, events: events, publisher: publisher, authorizer: authorizer}
}

// RetryEntry immediately attempts to publish eventID's event again,
// synchronously. A no-op if the entry is already published or
// resolved_manually. A failed attempt leaves the entry dead rather than
// pending -- it does not silently re-enable the automatic sweep; the
// operator must call this endpoint again once ready.
//
// bearerToken is the caller's validated JWT, forwarded to the
// identity/authorization capability to establish the caller's role. Returns
// domain.ErrForbidden or domain.ErrAuthorizationUnavailable without touching
// outbox state when authorization does not succeed.
func (s *AdminOutboxService) RetryEntry(ctx context.Context, bearerToken, eventID string) (ports.OutboxEntry, error) {
	if err := s.authorizer.RequireAdmin(ctx, bearerToken); err != nil {
		return ports.OutboxEntry{}, err
	}

	entry, found, err := s.outbox.Get(ctx, eventID)
	if err != nil {
		return ports.OutboxEntry{}, err
	}
	if !found {
		return ports.OutboxEntry{}, domain.ErrOutboxEntryNotFound
	}
	if entry.Status == domain.OutboxStatusPublished || entry.Status == domain.OutboxStatusResolvedManually {
		return entry, nil
	}

	event, err := s.events.FindByEventID(ctx, eventID)
	if err != nil {
		return ports.OutboxEntry{}, err
	}

	if publishErr := s.publisher.Publish(ctx, event); publishErr != nil {
		entry.Attempts++
		entry.LastError = publishErr.Error()
		entry.Status = domain.OutboxStatusDead
		entry.NextAttemptAt = time.Time{}
		if err := s.outbox.Update(ctx, entry); err != nil {
			return ports.OutboxEntry{}, err
		}
		return entry, nil
	}

	if err := s.outbox.MarkPublished(ctx, eventID); err != nil {
		return ports.OutboxEntry{}, err
	}
	entry.Status = domain.OutboxStatusPublished
	return entry, nil
}

// ResolveEntry marks eventID's entry resolved_manually without attempting
// to publish it again. A no-op if the entry is already published or
// resolved_manually.
//
// bearerToken is authorized exactly as in RetryEntry: a non-admin or
// unestablished caller is rejected before any outbox state is read.
func (s *AdminOutboxService) ResolveEntry(ctx context.Context, bearerToken, eventID string, reason string) (ports.OutboxEntry, error) {
	if err := s.authorizer.RequireAdmin(ctx, bearerToken); err != nil {
		return ports.OutboxEntry{}, err
	}

	entry, found, err := s.outbox.Get(ctx, eventID)
	if err != nil {
		return ports.OutboxEntry{}, err
	}
	if !found {
		return ports.OutboxEntry{}, domain.ErrOutboxEntryNotFound
	}
	if entry.Status == domain.OutboxStatusPublished || entry.Status == domain.OutboxStatusResolvedManually {
		return entry, nil
	}

	if err := s.outbox.MarkResolvedManually(ctx, eventID, reason); err != nil {
		return ports.OutboxEntry{}, err
	}
	entry.Status = domain.OutboxStatusResolvedManually
	return entry, nil
}
