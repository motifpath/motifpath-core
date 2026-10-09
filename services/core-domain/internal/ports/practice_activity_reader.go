package ports

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
)

// PracticeActivityReader reads a student's raw practice and learning
// activity from the Aggregation Worker's MongoDB collections:
// `practice_sessions`, `learning_activity`, `practice_item_history` and
// `song_chart_completions`. It
// is read-only from this service's perspective — the worker is the only
// writer.
type PracticeActivityReader interface {
	// FinishedSessions returns the student's practice sessions that ended
	// at or after since without leaving early.
	FinishedSessions(ctx context.Context, studentID string, since time.Time) ([]domain.FinishedPracticeSession, error)

	// SessionSpans returns the start and latest event of each of the
	// student's practice sessions that started at or after since, however
	// they ended. A session whose start hasn't arrived is left out.
	SessionSpans(ctx context.Context, studentID string, since time.Time) ([]domain.PracticeSessionSpan, error)

	// CompletionTimes returns when the student completed a content node,
	// once per completion, at or after since.
	CompletionTimes(ctx context.Context, studentID string, since time.Time) ([]time.Time, error)

	// SnapshotsAt returns the student's state on each of itemKeys as it
	// stood at at, as near as daily snapshots tell: the snapshot of the UTC
	// day whose end is nearest to at, so up to 12 hours before or after
	// it. An item with no snapshot by then is absent from the result.
	SnapshotsAt(ctx context.Context, studentID string, itemKeys []string, at time.Time) (map[string]domain.PracticeItemSnapshot, error)

	// SongChartCompletions returns every time the student marked a song
	// chart as played, whatever has become of the chart since.
	SongChartCompletions(ctx context.Context, studentID string) ([]domain.SongChartCompletion, error)
}
