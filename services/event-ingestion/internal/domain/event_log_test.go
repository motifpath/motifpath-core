package domain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/domain"
)

func TestNewEventLogQuery(t *testing.T) {
	ptr := func(v string) *string { return &v }
	num := func(v int) *int { return &v }
	cases := []struct {
		name      string
		studentID *string
		group     *string
		limit     *int
		offset    *int
		want      domain.EventLogQuery
		wantField string
	}{
		{name: "nothing chosen reads everyone's events, 20 at a time", want: domain.EventLogQuery{Limit: 20}},
		{name: "one student, one page further", studentID: ptr("s1"), limit: num(12), offset: num(12),
			want: domain.EventLogQuery{StudentID: "s1", Limit: 12, Offset: 12}},
		{name: "practice sessions", group: ptr("practice_sessions"), want: domain.EventLogQuery{Limit: 20,
			EventTypes: []domain.EventType{domain.EventTypePracticeSessionStarted, domain.EventTypePracticeSessionEnded}}},
		{name: "answers", group: ptr("answers"), want: domain.EventLogQuery{Limit: 20,
			EventTypes: []domain.EventType{domain.EventTypePracticeItemAnswered, domain.EventTypePracticeTapCheckCompleted}}},
		{name: "lessons", group: ptr("lessons"), want: domain.EventLogQuery{Limit: 20,
			EventTypes: []domain.EventType{domain.EventTypeLessonStarted, domain.EventTypeLessonResumed, domain.EventTypeLessonCompleted}}},
		{name: "exercises", group: ptr("exercises"), want: domain.EventLogQuery{Limit: 20,
			EventTypes: []domain.EventType{domain.EventTypeExerciseStarted, domain.EventTypeExerciseProgress, domain.EventTypeExerciseEnded}}},
		{name: "song charts", group: ptr("song_charts"), want: domain.EventLogQuery{Limit: 20,
			EventTypes: []domain.EventType{domain.EventTypeSongChartOpened, domain.EventTypeSongChartChordViewed, domain.EventTypeSongChartCompleted}}},
		{name: "an unknown group", group: ptr("videos"), wantField: "type_group"},
		{name: "a page of none", limit: num(0), wantField: "limit"},
		{name: "a page over 100", limit: num(101), wantField: "limit"},
		{name: "a negative offset", offset: num(-1), wantField: "offset"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := domain.NewEventLogQuery(c.studentID, c.group, c.limit, c.offset)

			if c.wantField != "" {
				var invalid *domain.InvalidQueryError
				require.True(t, errors.As(err, &invalid), "got %v", err)
				assert.Equal(t, c.wantField, invalid.Field)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}
