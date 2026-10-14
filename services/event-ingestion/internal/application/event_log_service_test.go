package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// fakeEventLog is an in-memory ports.EventLogReader that records the query
// it was asked.
type fakeEventLog struct {
	asked  *domain.EventLogQuery
	events []domain.LoggedEvent
	total  int
}

func (f *fakeEventLog) List(_ context.Context, q domain.EventLogQuery) ([]domain.LoggedEvent, int, error) {
	f.asked = &q
	return f.events, f.total, nil
}

func TestEventLogService_List(t *testing.T) {
	event := domain.LoggedEvent{ReceivedAt: time.Date(2026, 10, 14, 7, 51, 12, 0, time.UTC)}
	query := domain.EventLogQuery{StudentID: "alice", Limit: 12, Offset: 12}

	t.Run("an admin reads a page with the total", func(t *testing.T) {
		log := &fakeEventLog{events: []domain.LoggedEvent{event}, total: 40}
		resolver := &fakeProfileResolver{profile: ports.Profile{Role: "admin"}}
		svc := application.NewEventLogService(log, application.NewAdminAuthorizer(resolver))

		page, err := svc.List(context.Background(), "token", query)

		require.NoError(t, err)
		assert.Equal(t, domain.EventLogPage{Items: []domain.LoggedEvent{event}, Total: 40, Limit: 12, Offset: 12}, page)
		assert.Equal(t, &query, log.asked)
		assert.Equal(t, "token", resolver.lastToken)
	})

	t.Run("anyone else is refused before the log is read", func(t *testing.T) {
		log := &fakeEventLog{}
		svc := application.NewEventLogService(log, application.NewAdminAuthorizer(&fakeProfileResolver{profile: ports.Profile{Role: "student"}}))

		_, err := svc.List(context.Background(), "token", query)

		assert.ErrorIs(t, err, domain.ErrForbidden)
		assert.Nil(t, log.asked)
	})

	t.Run("an unknown role is unavailable, not refused", func(t *testing.T) {
		log := &fakeEventLog{}
		svc := application.NewEventLogService(log, application.NewAdminAuthorizer(&fakeProfileResolver{err: ports.ErrProfileUnavailable}))

		_, err := svc.List(context.Background(), "token", query)

		assert.ErrorIs(t, err, domain.ErrAuthorizationUnavailable)
		assert.Nil(t, log.asked)
	})
}
