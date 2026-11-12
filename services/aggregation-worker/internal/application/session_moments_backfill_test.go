package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/aggregation-worker/internal/application"
	"github.com/motifpath/aggregation-worker/internal/domain"
)

// unmomentedSessions holds sessions stored before their moments were kept.
type unmomentedSessions struct {
	without []domain.PracticeSession
	put     []domain.PracticeSession
}

func (s *unmomentedSessions) ListWithoutMoments(context.Context) ([]domain.PracticeSession, error) {
	return s.without, nil
}

func (s *unmomentedSessions) Put(_ context.Context, session domain.PracticeSession) error {
	s.put = append(s.put, session)
	return nil
}

// answerTimes is when each answer with evidence was given.
type answerTimes map[string]time.Time

func (a answerTimes) AnsweredAt(_ context.Context, eventIDs []string) (map[string]time.Time, error) {
	found := map[string]time.Time{}
	for _, id := range eventIDs {
		if at, ok := a[id]; ok {
			found[id] = at
		}
	}
	return found, nil
}

func TestSessionMomentsBackfill_PlacesAnOlderSessionsAnswersAndEnd(t *testing.T) {
	start := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	stored := domain.PracticeSession{}.Started(domain.PracticeSessionStart{
		EventID: "start", StudentID: "alice", PracticeSessionID: "s1", OccurredAt: start, Minutes: 10,
	}).Answered("a1", start.Add(time.Minute), "").Answered("a2", start.Add(45*time.Minute), "").
		Answered("ungraded", start.Add(50*time.Minute), "")
	stored.Moments = nil
	end := domain.PracticeSessionEnd{EventID: "end", StudentID: "alice", PracticeSessionID: "s1", OccurredAt: start.Add(4 * time.Hour)}
	stored.End = &end
	stored.LastEventAt = end.OccurredAt
	sessions := &unmomentedSessions{without: []domain.PracticeSession{stored}}
	times := answerTimes{"a1": start.Add(time.Minute), "a2": start.Add(45 * time.Minute)}

	backfilled, err := application.NewSessionMomentsBackfill(sessions, times).Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, backfilled)
	require.Len(t, sessions.put, 1)
	// An answer that left no evidence has no time to place it at.
	assert.Equal(t, []domain.SessionMoment{
		{EventID: "a1", At: start.Add(time.Minute)},
		{EventID: "a2", At: start.Add(45 * time.Minute)},
		{EventID: "end", At: start.Add(4 * time.Hour)},
	}, sessions.put[0].Moments)
}

func TestSessionMomentsBackfill_LeavesNothingToDoTwice(t *testing.T) {
	sessions := &unmomentedSessions{}

	backfilled, err := application.NewSessionMomentsBackfill(sessions, answerTimes{}).Run(context.Background())

	require.NoError(t, err)
	assert.Zero(t, backfilled)
	assert.Empty(t, sessions.put)
}
