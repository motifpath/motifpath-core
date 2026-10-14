package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/aggregation-worker/internal/application"
	"github.com/motifpath/aggregation-worker/internal/domain"
)

type fakeRepository struct {
	statuses    map[string]domain.CompletionStatus
	started     map[string]time.Time
	upsertCalls int
	getErr      error
	upsertErr   error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{statuses: map[string]domain.CompletionStatus{}, started: map[string]time.Time{}}
}

func repoKey(studentID, contentNodeID string) string {
	return studentID + "|" + contentNodeID
}

func (f *fakeRepository) GetStatus(_ context.Context, studentID, contentNodeID string) (domain.CompletionStatus, bool, error) {
	if f.getErr != nil {
		return "", false, f.getErr
	}
	status, found := f.statuses[repoKey(studentID, contentNodeID)]
	return status, found, nil
}

func (f *fakeRepository) Upsert(_ context.Context, studentID, contentNodeID string, status domain.CompletionStatus) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upsertCalls++
	f.statuses[repoKey(studentID, contentNodeID)] = status
	return nil
}

// MarkStarted keeps the repository's contract: a start later than the stored
// one leaves it as it is.
func (f *fakeRepository) MarkStarted(_ context.Context, studentID, contentNodeID string, at time.Time) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	key := repoKey(studentID, contentNodeID)
	if current, ok := f.started[key]; !ok || at.Before(current) {
		f.started[key] = at
	}
	return nil
}

func TestProcessEventService_KeepsWhenEachNodeWasFirstStarted(t *testing.T) {
	lesson := func(eventType domain.EventType, at time.Time) domain.TrackingEvent {
		return domain.TrackingEvent{EventType: eventType, StudentID: alice, ContentNodeID: introNode, OccurredAt: at}
	}
	cases := []struct {
		name   string
		events []domain.TrackingEvent
		want   map[string]time.Time
	}{
		{
			name:   "starting a node is its first start",
			events: []domain.TrackingEvent{lesson(domain.EventTypeLessonStarted, clock(19, 0))},
			want:   map[string]time.Time{repoKey(alice, introNode): clock(19, 0)},
		},
		{
			name: "resuming it later keeps the first start",
			events: []domain.TrackingEvent{lesson(domain.EventTypeLessonStarted, clock(19, 0)),
				lesson(domain.EventTypeLessonResumed, clock(20, 0))},
			want: map[string]time.Time{repoKey(alice, introNode): clock(19, 0)},
		},
		{
			name: "a start that arrives late moves the first start back",
			events: []domain.TrackingEvent{lesson(domain.EventTypeLessonCompleted, clock(19, 30)),
				lesson(domain.EventTypeLessonStarted, clock(19, 0))},
			want: map[string]time.Time{repoKey(alice, introNode): clock(19, 0)},
		},
		{
			name:   "a node completed with no start seen started when it was completed",
			events: []domain.TrackingEvent{lesson(domain.EventTypeLessonCompleted, clock(19, 30))},
			want:   map[string]time.Time{repoKey(alice, introNode): clock(19, 30)},
		},
		{
			name:   "other events start no node",
			events: []domain.TrackingEvent{lesson("exercise.started", clock(19, 0))},
			want:   map[string]time.Time{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newFakeRepository()
			svc := application.NewProcessEventService(repo, nil, newActivity())

			for _, e := range c.events {
				require.NoError(t, svc.Handle(context.Background(), e))
			}

			assert.Equal(t, c.want, repo.started)
		})
	}
}

func TestProcessEventService_Handle(t *testing.T) {
	tests := []struct {
		name        string
		events      []domain.EventType
		wantStatus  domain.CompletionStatus
		wantFound   bool
		wantUpserts int
	}{
		{
			name:        "lesson started moves an untouched node to in_progress",
			events:      []domain.EventType{domain.EventTypeLessonStarted},
			wantStatus:  domain.CompletionStatusInProgress,
			wantFound:   true,
			wantUpserts: 1,
		},
		{
			name:        "lesson resumed moves an untouched node to in_progress",
			events:      []domain.EventType{domain.EventTypeLessonResumed},
			wantStatus:  domain.CompletionStatusInProgress,
			wantFound:   true,
			wantUpserts: 1,
		},
		{
			name:        "lesson completed marks the node completed",
			events:      []domain.EventType{domain.EventTypeLessonStarted, domain.EventTypeLessonCompleted},
			wantStatus:  domain.CompletionStatusCompleted,
			wantFound:   true,
			wantUpserts: 2,
		},
		{
			name:        "a completed node is never downgraded by a later resumed event",
			events:      []domain.EventType{domain.EventTypeLessonCompleted, domain.EventTypeLessonResumed},
			wantStatus:  domain.CompletionStatusCompleted,
			wantFound:   true,
			wantUpserts: 1, // the second Handle call computes no change, so no Upsert
		},
		{
			name:        "duplicate delivery of the same event is a no-op",
			events:      []domain.EventType{domain.EventTypeLessonStarted, domain.EventTypeLessonStarted},
			wantStatus:  domain.CompletionStatusInProgress,
			wantFound:   true,
			wantUpserts: 1,
		},
		{
			name:        "non-lesson events are ignored entirely",
			events:      []domain.EventType{"exercise.answer_sent"},
			wantFound:   false,
			wantUpserts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			svc := application.NewProcessEventService(repo, nil, newActivity())

			for _, eventType := range tt.events {
				err := svc.Handle(context.Background(), domain.TrackingEvent{
					EventType:     eventType,
					StudentID:     "11111111-1111-1111-1111-111111111111",
					ContentNodeID: "22222222-2222-2222-2222-222222222222",
				})
				require.NoError(t, err)
			}

			status, found := repo.statuses[repoKey("11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")]
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.Equal(t, tt.wantStatus, status)
			}
			assert.Equal(t, tt.wantUpserts, repo.upsertCalls)
		})
	}
}

func TestProcessEventService_Handle_PropagatesGetStatusError(t *testing.T) {
	repo := newFakeRepository()
	repo.getErr = errors.New("connection refused")
	svc := application.NewProcessEventService(repo, nil, newActivity())

	err := svc.Handle(context.Background(), domain.TrackingEvent{
		EventType:     domain.EventTypeLessonStarted,
		StudentID:     "student-1",
		ContentNodeID: "node-1",
	})

	require.Error(t, err)
}

func TestProcessEventService_Handle_PropagatesUpsertError(t *testing.T) {
	repo := newFakeRepository()
	repo.upsertErr = errors.New("connection refused")
	svc := application.NewProcessEventService(repo, nil, newActivity())

	err := svc.Handle(context.Background(), domain.TrackingEvent{
		EventType:     domain.EventTypeLessonStarted,
		StudentID:     "student-1",
		ContentNodeID: "node-1",
	})

	require.Error(t, err)
}
