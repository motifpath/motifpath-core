package application

import (
	"context"
	"fmt"

	"github.com/motifpath/aggregation-worker/internal/ports"
)

// SessionMomentsBackfill gives every session stored before sessions kept their
// moments the moments it can still find: each answer that left evidence, at the
// time it was given, and its end, none of them with a visible time, so its
// minutes practised can be counted as for any app that doesn't measure visible
// time, instead of from its whole span.
type SessionMomentsBackfill struct {
	sessions ports.UnmomentedSessionRepository
	answers  ports.AnswerTimeReader
}

func NewSessionMomentsBackfill(sessions ports.UnmomentedSessionRepository, answers ports.AnswerTimeReader) *SessionMomentsBackfill {
	return &SessionMomentsBackfill{sessions: sessions, answers: answers}
}

// Run backfills every session still without moments and reports how many it
// backfilled. Each one is stored with its moments, even none, so a second run
// finds nothing left to do.
func (b *SessionMomentsBackfill) Run(ctx context.Context) (int, error) {
	sessions, err := b.sessions.ListWithoutMoments(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing sessions without moments: %w", err)
	}
	for _, s := range sessions {
		answeredAt, err := b.answers.AnsweredAt(ctx, s.AnsweredEventIDs)
		if err != nil {
			return 0, fmt.Errorf("reading the answers of session %s: %w", s.ID, err)
		}
		for _, id := range s.AnsweredEventIDs {
			if at, ok := answeredAt[id]; ok {
				s = s.Seen(id, at, nil)
			}
		}
		if s.End != nil {
			s = s.Seen(s.End.EventID, s.End.OccurredAt, nil)
		}
		if err := b.sessions.Put(ctx, s); err != nil {
			return 0, fmt.Errorf("storing session %s: %w", s.ID, err)
		}
	}
	return len(sessions), nil
}
