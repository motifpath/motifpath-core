package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/aggregation-worker/internal/application"
	"github.com/motifpath/aggregation-worker/internal/domain"
)

type fakeRepository struct {
	statuses    map[string]domain.CompletionStatus
	upsertCalls int
	getErr      error
	upsertErr   error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{statuses: map[string]domain.CompletionStatus{}}
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
			svc := application.NewProcessEventService(repo)

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
	svc := application.NewProcessEventService(repo)

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
	svc := application.NewProcessEventService(repo)

	err := svc.Handle(context.Background(), domain.TrackingEvent{
		EventType:     domain.EventTypeLessonStarted,
		StudentID:     "student-1",
		ContentNodeID: "node-1",
	})

	require.Error(t, err)
}
