package application

import (
	"context"

	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// EventLogService reads the event log for admins. Reading it changes
// nothing.
type EventLogService struct {
	log        ports.EventLogReader
	authorizer *AdminAuthorizer
}

func NewEventLogService(log ports.EventLogReader, authorizer *AdminAuthorizer) *EventLogService {
	return &EventLogService{log: log, authorizer: authorizer}
}

// List returns the page q asks for once the caller, by bearerToken, is
// established as an admin; otherwise domain.ErrForbidden or
// domain.ErrAuthorizationUnavailable, without reading the log.
func (s *EventLogService) List(ctx context.Context, bearerToken string, q domain.EventLogQuery) (domain.EventLogPage, error) {
	if err := s.authorizer.RequireAdmin(ctx, bearerToken); err != nil {
		return domain.EventLogPage{}, err
	}
	events, total, err := s.log.List(ctx, q)
	if err != nil {
		return domain.EventLogPage{}, err
	}
	return domain.EventLogPage{Items: events, Total: total, Limit: q.Limit, Offset: q.Offset}, nil
}
