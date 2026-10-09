package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// fakePracticeActivityReader is an in-memory ports.PracticeActivityReader
// for one student.
type fakePracticeActivityReader struct {
	sessions    []domain.FinishedPracticeSession
	spans       []domain.PracticeSessionSpan
	completions []time.Time
	snapshots   map[string]domain.PracticeItemSnapshot
	snapshotsAt time.Time
	songs       []domain.SongChartCompletion
	songsErr    error
	err         error
}

func (f *fakePracticeActivityReader) SongChartCompletions(_ context.Context, _ string) ([]domain.SongChartCompletion, error) {
	return f.songs, f.songsErr
}

func (f *fakePracticeActivityReader) FinishedSessions(_ context.Context, _ string, since time.Time) ([]domain.FinishedPracticeSession, error) {
	var out []domain.FinishedPracticeSession
	for _, s := range f.sessions {
		if !s.EndedAt.Before(since) {
			out = append(out, s)
		}
	}
	return out, f.err
}

func (f *fakePracticeActivityReader) SessionSpans(_ context.Context, _ string, since time.Time) ([]domain.PracticeSessionSpan, error) {
	var out []domain.PracticeSessionSpan
	for _, s := range f.spans {
		if !s.StartedAt.Before(since) {
			out = append(out, s)
		}
	}
	return out, f.err
}

func (f *fakePracticeActivityReader) CompletionTimes(_ context.Context, _ string, since time.Time) ([]time.Time, error) {
	var out []time.Time
	for _, t := range f.completions {
		if !t.Before(since) {
			out = append(out, t)
		}
	}
	return out, f.err
}

func (f *fakePracticeActivityReader) SnapshotsAt(_ context.Context, _ string, itemKeys []string, at time.Time) (map[string]domain.PracticeItemSnapshot, error) {
	f.snapshotsAt = at
	out := map[string]domain.PracticeItemSnapshot{}
	for _, k := range itemKeys {
		if s, ok := f.snapshots[k]; ok {
			out[k] = s
		}
	}
	return out, f.err
}

// summaryFixture is a practice fixture with a summary service over it.
type summaryFixture struct {
	*practiceFixture
	svc        *application.PracticeSummaryService
	activity   *fakePracticeActivityReader
	songCharts *fakeSongChartRepository
}

func newSummaryFixture(t *testing.T) *summaryFixture {
	f := &summaryFixture{
		practiceFixture: newPracticeFixture(t),
		activity:        &fakePracticeActivityReader{snapshots: map[string]domain.PracticeItemSnapshot{}},
		songCharts:      newFakeSongChartRepository(),
	}
	now := func() time.Time { return f.now }
	rollup := application.NewKnowledgeRollupService(f.knowledgeNodes, f.edges, practiceItemSource{f: f.practiceFixture}, f.states, now)
	f.svc = application.NewPracticeSummaryService(
		f.instruments, f.studentPaths, f.enrollments, f.learningPaths, f.courseVersions, f.contentNodes, rollup, f.activity, f.songCharts, now,
	)
	return f
}

func (f *summaryFixture) summary(t *testing.T, instrumentID *string, timeZone string) application.PracticeSummary {
	t.Helper()
	got, err := f.svc.Summary(context.Background(), studentCaller(), instrumentID, timeZone)
	require.NoError(t, err)
	return got
}

func TestPracticeSummaryService_Summary(t *testing.T) {
	guitar := practiceGuitar
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)

	t.Run("the student's instruments come from their active paths and courses, in the instruments' order", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.onPathFor([]string{practiceGuitar}, "skill-1")
		f.onPathFor(nil, "skill-2")
		require.NoError(t, f.enrollments.Create(context.Background(), domain.CourseEnrollment{
			ID: "enrollment-1", StudentID: studentCaller().ID, CourseID: "course-1", CourseVersionNumber: 2, Status: domain.CourseEnrollmentStatusActive,
		}))
		require.NoError(t, f.courseVersions.Create(context.Background(), domain.CourseVersion{ID: "v2", CourseID: "course-1", VersionNumber: 2, InstrumentIDsSnapshot: []string{practiceBass}}))

		got := f.summary(t, &guitar, "")

		assert.Equal(t, []string{practiceBass, practiceGuitar}, got.StudentInstrumentIDs, "a path for every instrument adds none")
	})

	t.Run("practice days count sessions finished with the instrument in hand, by the student's calendar", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC) // Monday noon in São Paulo
		bass := practiceBass
		f.activity.sessions = []domain.FinishedPracticeSession{
			{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)},
			{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 3, 2, 30, 0, 0, time.UTC)}, // Friday 23:30 local
			{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 2, 20, 0, 0, 0, time.UTC)}, // Friday too
			{InstrumentID: &bass, EndedAt: time.Date(2026, 10, 4, 13, 0, 0, 0, time.UTC)},
			{EndedAt: time.Date(2026, 10, 4, 13, 0, 0, 0, time.UTC)}, // in the head
		}

		got := f.summary(t, &guitar, "America/Sao_Paulo")

		assert.Equal(t, 2, got.PracticeDaysLast7)
	})

	t.Run("progress compares each skill now with its state when the last 7 days began", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
		f.onPathFor([]string{practiceGuitar}, "skill-1")
		keys := f.exerciseBatch("e", "skill-1", 2, 30)
		f.stateOf(keys[0], domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.9})
		f.stateOf(keys[1], domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.82})
		f.activity.snapshots[keys[0]] = domain.PracticeItemSnapshot{ItemKey: keys[0], Counted: 2, Accuracy: 0.72}
		f.activity.snapshots[keys[1]] = domain.PracticeItemSnapshot{ItemKey: keys[1], Counted: 2, Accuracy: 0.72}

		got := f.summary(t, &guitar, "America/Sao_Paulo")

		assert.Equal(t, time.Date(2026, 9, 29, 0, 0, 0, 0, saoPaulo), f.activity.snapshotsAt)
		require.Len(t, got.Progress, 1)
		assert.Equal(t, "skill-1", got.Progress[0].NodeID)
		assert.InDelta(t, 0.72, got.Progress[0].Before, 1e-9)
		assert.InDelta(t, 0.86, got.Progress[0].After, 1e-9)
	})

	t.Run("next steps are the top three, with how many there are in all", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.onPathFor([]string{practiceGuitar}, "skill-1", "skill-2", "skill-3", "skill-4", "skill-5")
		for _, id := range []string{"skill-1", "skill-2", "skill-3", "skill-4", "skill-5"} {
			f.exerciseBatch(id, id, 1, 30)
		}

		got := f.summary(t, &guitar, "")

		require.Len(t, got.NextSteps, 3)
		assert.Equal(t, 5, got.NextStepsTotal)
		assert.Equal(t, domain.PracticeNextStepReadyToStart, got.NextSteps[0].Kind)
		assert.Equal(t, "skill-1", got.NextSteps[0].Node.ID)
	})

	t.Run("a skill unconnected to what the student is learning is no next step", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.onPathFor([]string{practiceGuitar}, "skill-1")
		f.exerciseBatch("unlinked", "skill-unlinked", 1, 30)

		got := f.summary(t, &guitar, "")

		assert.Zero(t, got.NextStepsTotal)
	})

	t.Run("nodes are grouped, each with its standing and its children", func(t *testing.T) {
		f := newSummaryFixture(t)
		parent := "area"
		f.knowledgeNodes.put(domain.KnowledgeNode{ID: parent, Kind: domain.KnowledgeNodeKindSkill, Key: parent, InstrumentIDs: []string{practiceGuitar}})
		f.knowledgeNodes.put(domain.KnowledgeNode{ID: "leaf", Kind: domain.KnowledgeNodeKindSkill, Key: "leaf", ParentID: &parent, InstrumentIDs: []string{practiceGuitar}})
		f.exercise("e-leaf", "leaf", 30, practiceGuitar)

		got := f.summary(t, &guitar, "")

		require.Len(t, got.Groups, 1)
		require.NotNil(t, got.Groups[0].Area)
		assert.Equal(t, parent, got.Groups[0].Area.ID)
		assert.Equal(t, []string{"leaf"}, got.Children[parent])
		assert.Equal(t, 1, got.Standings["leaf"].Total)
	})

	t.Run("an unknown time zone is rejected on time_zone", func(t *testing.T) {
		f := newSummaryFixture(t)

		_, err := f.svc.Summary(context.Background(), studentCaller(), &guitar, "Mars/Olympus_Mons")

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "time_zone", valErr.Fields[0].Field)
	})

	t.Run("an instrument that doesn't exist is not found", func(t *testing.T) {
		f := newSummaryFixture(t)
		missing := "missing"

		_, err := f.svc.Summary(context.Background(), studentCaller(), &missing, "")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a failure reading activity fails the request", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.activity.err = errors.New("mongo down")

		_, err := f.svc.Summary(context.Background(), studentCaller(), &guitar, "")

		require.Error(t, err)
	})
}

func TestPracticeSummaryService_Overview(t *testing.T) {
	guitar, bass := practiceGuitar, practiceBass

	t.Run("days across instruments, learning days, and a card per instrument", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
		f.onPathFor([]string{practiceGuitar, practiceBass}, "skill-1")
		f.exercise("e-1", "skill-1", 30, practiceGuitar)
		f.activity.sessions = []domain.FinishedPracticeSession{
			{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)},
			{InstrumentID: &bass, EndedAt: time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)},
			{InstrumentID: &bass, EndedAt: time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)},
		}
		f.activity.completions = []time.Time{
			time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC),
			time.Date(2026, 10, 2, 14, 0, 0, 0, time.UTC),
			time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC),
		}

		got, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

		require.NoError(t, err)
		assert.Equal(t, 2, got.PracticeDaysLast7, "two instruments on one day count one day")
		assert.Equal(t, 2, got.LearningDaysLast7)
		require.Len(t, got.Instruments, 2)
		assert.Equal(t, practiceBass, got.Instruments[0].InstrumentID)
		assert.Equal(t, 2, got.Instruments[0].PracticeDaysLast7)
		assert.Nil(t, got.Instruments[0].TopNextStep, "skill-1 has nothing to practise on bass")
		assert.Equal(t, practiceGuitar, got.Instruments[1].InstrumentID)
		assert.Equal(t, 1, got.Instruments[1].PracticeDaysLast7)
		require.NotNil(t, got.Instruments[1].TopNextStep)
		assert.Equal(t, "skill-1", got.Instruments[1].TopNextStep.Node.ID)
	})

	t.Run("an unknown time zone is rejected on time_zone", func(t *testing.T) {
		f := newSummaryFixture(t)

		_, err := f.svc.Overview(context.Background(), studentCaller(), "Mars/Olympus_Mons")

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "time_zone", valErr.Fields[0].Field)
	})
}

func TestPracticeSummaryService_OverviewSongsPlayed(t *testing.T) {
	const asaBranca, amazingGrace, missing = "chart-asa-branca", "chart-amazing-grace", "chart-that-does-not-exist"
	// 15:00 UTC on Monday 5 October is noon in São Paulo; the last 7 days
	// there began at midnight on Tuesday 29 September.
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	daysAgo := func(days int) time.Time { return now.AddDate(0, 0, -days) }
	played := func(chartID string, at time.Time) domain.SongChartCompletion {
		return domain.SongChartCompletion{SongChartID: chartID, CompletedAt: at}
	}
	cases := []struct {
		name      string
		charts    []domain.SongChart
		played    []domain.SongChartCompletion
		wantTotal int
		wantLast7 int
	}{
		{
			name:      "each chart marked as played counts",
			charts:    []domain.SongChart{{ID: asaBranca}, {ID: amazingGrace}},
			played:    []domain.SongChartCompletion{played(asaBranca, daysAgo(0)), played(amazingGrace, daysAgo(0))},
			wantTotal: 2, wantLast7: 2,
		},
		{
			name:      "a song first played this week counts in this week's songs",
			charts:    []domain.SongChart{{ID: asaBranca}, {ID: amazingGrace}},
			played:    []domain.SongChartCompletion{played(asaBranca, daysAgo(10)), played(amazingGrace, daysAgo(1))},
			wantTotal: 2, wantLast7: 1,
		},
		{
			name:      "marking the same chart again counts it once, from when it was first played",
			charts:    []domain.SongChart{{ID: asaBranca}},
			played:    []domain.SongChartCompletion{played(asaBranca, daysAgo(0)), played(asaBranca, daysAgo(10))},
			wantTotal: 1, wantLast7: 0,
		},
		{
			name:      "a chart withdrawn after it was played still counts",
			charts:    []domain.SongChart{{ID: asaBranca, Status: domain.SongChartWithdrawn}},
			played:    []domain.SongChartCompletion{played(asaBranca, daysAgo(0))},
			wantTotal: 1, wantLast7: 1,
		},
		{
			name:      "a chart that doesn't exist is not counted",
			played:    []domain.SongChartCompletion{played(missing, daysAgo(0))},
			wantTotal: 0, wantLast7: 0,
		},
		{
			name:      "the first day of the last 7, in the student's time zone, is this week",
			charts:    []domain.SongChart{{ID: asaBranca}, {ID: amazingGrace}},
			played:    []domain.SongChartCompletion{played(asaBranca, time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)), played(amazingGrace, time.Date(2026, 9, 29, 2, 59, 0, 0, time.UTC))},
			wantTotal: 2, wantLast7: 1,
		},
		{
			name:      "a first mark dated after today, by a clock running ahead, is played but not this week",
			charts:    []domain.SongChart{{ID: asaBranca}},
			played:    []domain.SongChartCompletion{played(asaBranca, time.Date(2026, 10, 6, 3, 0, 0, 0, time.UTC))},
			wantTotal: 1, wantLast7: 0,
		},
		{
			name:      "a student who has played no song starts at zero",
			wantTotal: 0, wantLast7: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newSummaryFixture(t)
			f.now = now
			for _, chart := range c.charts {
				require.NoError(t, f.songCharts.Create(context.Background(), chart))
			}
			f.activity.songs = c.played

			got, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

			require.NoError(t, err)
			assert.Equal(t, c.wantTotal, got.SongsPlayedTotal)
			assert.Equal(t, c.wantLast7, got.SongsPlayedLast7)
		})
	}

	t.Run("a failure reading the completions is returned", func(t *testing.T) {
		f := newSummaryFixture(t)
		boom := errors.New("connection refused")
		f.activity.songsErr = boom

		_, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

		assert.ErrorIs(t, err, boom)
	})
}

func TestPracticeSummaryService_OverviewMinutesPractised(t *testing.T) {
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	// Noon on Monday 5 October in São Paulo: the last 7 days began at
	// midnight on Tuesday 29 September, the previous 7 on 22 September.
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, saoPaulo)
	at := func(day, hour, minute, second int) time.Time {
		return time.Date(2026, 10, day, hour, minute, second, 0, saoPaulo)
	}
	span := func(from, to time.Time) domain.PracticeSessionSpan {
		return domain.PracticeSessionSpan{StartedAt: from, LastEventAt: to}
	}
	cases := []struct {
		name         string
		spans        []domain.PracticeSessionSpan
		wantLast7    int
		wantPrevious int
	}{
		{
			name:      "every session this week adds up",
			spans:     []domain.PracticeSessionSpan{span(at(1, 18, 0, 0), at(1, 18, 12, 0)), span(at(3, 19, 0, 0), at(3, 19, 20, 0))},
			wantLast7: 32,
		},
		{
			name:         "sessions the week before count in the previous 7 days",
			spans:        []domain.PracticeSessionSpan{span(at(-4, 18, 0, 0), at(-4, 18, 20, 0)), span(at(-7, 18, 0, 0), at(-7, 18, 15, 0))},
			wantPrevious: 35,
		},
		{
			name:      "a session counts up to its last event, however it ended",
			spans:     []domain.PracticeSessionSpan{span(at(4, 18, 0, 0), at(4, 18, 4, 0))},
			wantLast7: 4,
		},
		{
			name:      "the total rounds down to whole minutes",
			spans:     []domain.PracticeSessionSpan{span(at(4, 18, 0, 0), at(4, 18, 7, 50))},
			wantLast7: 7,
		},
		{
			name:      "seconds add up across sessions before rounding",
			spans:     []domain.PracticeSessionSpan{span(at(3, 18, 0, 0), at(3, 18, 7, 30)), span(at(4, 18, 0, 0), at(4, 18, 7, 30))},
			wantLast7: 15,
		},
		{
			name:         "a session counts in the week it started, in the student's time zone",
			spans:        []domain.PracticeSessionSpan{span(time.Date(2026, 9, 28, 23, 50, 0, 0, saoPaulo), time.Date(2026, 9, 29, 0, 10, 0, 0, saoPaulo)), span(time.Date(2026, 9, 29, 0, 0, 0, 0, saoPaulo), time.Date(2026, 9, 29, 0, 5, 0, 0, saoPaulo))},
			wantLast7:    5,
			wantPrevious: 20,
		},
		{
			name:  "a last event before the start, by a clock out of step, counts nothing",
			spans: []domain.PracticeSessionSpan{span(at(4, 18, 10, 0), at(4, 18, 0, 0))},
		},
		{
			name:  "a session started after today, by a clock running ahead, counts nothing",
			spans: []domain.PracticeSessionSpan{span(at(6, 9, 0, 0), at(6, 9, 30, 0))},
		},
		{
			name: "a student who has never practised starts at zero",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newSummaryFixture(t)
			f.now = now
			f.activity.spans = c.spans

			got, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

			require.NoError(t, err)
			assert.Equal(t, c.wantLast7, got.MinutesPractisedLast7)
			assert.Equal(t, c.wantPrevious, got.MinutesPractisedPrevious7)
		})
	}
}

func TestPracticeSummaryService_OverviewDayStreak(t *testing.T) {
	guitar, bass := practiceGuitar, practiceBass
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	// Noon on Monday 5 October in São Paulo.
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, saoPaulo)
	finishedOn := func(days ...int) []domain.FinishedPracticeSession {
		var sessions []domain.FinishedPracticeSession
		for _, d := range days {
			sessions = append(sessions, domain.FinishedPracticeSession{InstrumentID: &guitar, EndedAt: now.AddDate(0, 0, d)})
		}
		return sessions
	}
	cases := []struct {
		name        string
		timeZone    string
		sessions    []domain.FinishedPracticeSession
		wantCurrent int
		wantBest    int
	}{
		{
			name:     "consecutive practice days up to today",
			sessions: finishedOn(-2, -1, 0), wantCurrent: 3, wantBest: 3,
		},
		{
			name:     "today without practice yet doesn't break the streak",
			sessions: finishedOn(-4, -3, -2, -1), wantCurrent: 4, wantBest: 4,
		},
		{
			name:     "a missed day ends the current streak and keeps the best one",
			sessions: finishedOn(-11, -10, -9, -8, -7, -6, -5, -4, -3, -1, 0), wantCurrent: 2, wantBest: 9,
		},
		{
			name:     "neither today nor yesterday practised means no current streak",
			sessions: finishedOn(-2), wantCurrent: 0, wantBest: 1,
		},
		{
			name: "two instruments on the same day are one streak day",
			sessions: []domain.FinishedPracticeSession{
				{InstrumentID: &guitar, EndedAt: now.Add(-2 * time.Hour)},
				{InstrumentID: &bass, EndedAt: now.Add(-time.Hour)},
				{EndedAt: now.Add(-30 * time.Minute)},
			},
			wantCurrent: 1, wantBest: 1,
		},
		{
			name:     "days follow the student's time zone",
			timeZone: "America/Sao_Paulo",
			sessions: []domain.FinishedPracticeSession{
				{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 4, 23, 30, 0, 0, saoPaulo)},
				{InstrumentID: &guitar, EndedAt: now},
			},
			wantCurrent: 2, wantBest: 2,
		},
		{
			name:     "the same sessions in UTC fall on one day",
			timeZone: "UTC",
			sessions: []domain.FinishedPracticeSession{
				{InstrumentID: &guitar, EndedAt: time.Date(2026, 10, 4, 23, 30, 0, 0, saoPaulo)},
				{InstrumentID: &guitar, EndedAt: now},
			},
			wantCurrent: 1, wantBest: 1,
		},
		{
			name:     "a day after today, by a clock running ahead, is no streak day",
			sessions: finishedOn(0, 1, 2), wantCurrent: 1, wantBest: 1,
		},
		{
			name: "a student who has never practised starts at zero",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newSummaryFixture(t)
			f.now = now
			f.activity.sessions = c.sessions
			timeZone := c.timeZone
			if timeZone == "" {
				timeZone = "America/Sao_Paulo"
			}

			got, err := f.svc.Overview(context.Background(), studentCaller(), timeZone)

			require.NoError(t, err)
			assert.Equal(t, c.wantCurrent, got.DayStreakCurrent)
			assert.Equal(t, c.wantBest, got.DayStreakBest)
		})
	}
}

func TestPracticeSummaryService_OverviewSkillsUp(t *testing.T) {
	bpm := func(n int) *int { return &n }
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)

	t.Run("each improved skill counts once per instrument, however many of its measures improved", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = now
		f.onPathFor([]string{practiceGuitar, practiceBass}, "notes-on-low-strings", "root-fifth-groove")
		f.exercise("e-notes", "notes-on-low-strings", 30, practiceGuitar)
		f.exercise("e-groove", "root-fifth-groove", 30, practiceBass)
		notes, groove := domain.ExerciseItemKey("e-notes"), domain.ExerciseItemKey("e-groove")
		f.stateOf(notes, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.9, Fluency: 0.8})
		f.activity.snapshots[notes] = domain.PracticeItemSnapshot{ItemKey: notes, Counted: 2, Accuracy: 0.7, Fluency: 0.5}
		f.stateOf(groove, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.8, BestCleanBPM: bpm(90)})
		f.activity.snapshots[groove] = domain.PracticeItemSnapshot{ItemKey: groove, Counted: 2, Accuracy: 0.8, BestCleanBPM: bpm(80)}

		got, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

		require.NoError(t, err)
		assert.Equal(t, 2, got.SkillsUpLast7)
	})

	t.Run("a skill improved on every instrument counts on each", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = now
		f.onPathFor([]string{practiceGuitar, practiceBass}, "reading-rhythm")
		f.exercise("e-rhythm", "reading-rhythm", 30)
		key := domain.ExerciseItemKey("e-rhythm")
		f.stateOf(key, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.9})
		f.activity.snapshots[key] = domain.PracticeItemSnapshot{ItemKey: key, Counted: 2, Accuracy: 0.6}

		got, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

		require.NoError(t, err)
		assert.Equal(t, 2, got.SkillsUpLast7)
	})

	t.Run("a concept that improved is not a skill up", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = now
		f.knowledgeNodes.put(domain.KnowledgeNode{ID: "intervals", Kind: domain.KnowledgeNodeKindConcept, Key: "intervals"})
		f.onPathFor([]string{practiceGuitar}, "intervals")
		f.exercise("e-intervals", "intervals", 30, practiceGuitar)
		key := domain.ExerciseItemKey("e-intervals")
		f.stateOf(key, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.9})
		f.activity.snapshots[key] = domain.PracticeItemSnapshot{ItemKey: key, Counted: 2, Accuracy: 0.6}

		got, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

		require.NoError(t, err)
		assert.Equal(t, 0, got.SkillsUpLast7)
	})

	t.Run("a skill that didn't improve is not a skill up", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.now = now
		f.onPathFor([]string{practiceGuitar}, "notes-on-low-strings")
		f.exercise("e-notes", "notes-on-low-strings", 30, practiceGuitar)
		key := domain.ExerciseItemKey("e-notes")
		f.stateOf(key, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.inDays(2), Accuracy: 0.7})
		f.activity.snapshots[key] = domain.PracticeItemSnapshot{ItemKey: key, Counted: 2, Accuracy: 0.7}

		got, err := f.svc.Overview(context.Background(), studentCaller(), "America/Sao_Paulo")

		require.NoError(t, err)
		assert.Equal(t, 0, got.SkillsUpLast7)
	})
}

func TestPracticeSummaryService_FretboardMap(t *testing.T) {
	cell := func(str, fret int) string { return domain.FretboardCellItemKey(practiceGuitar, str, fret) }

	t.Run("every cell that suits the instrument, with the student's level, and nothing else", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.skill("root-strings")
		f.cells[practiceGuitar] = []domain.ClassifiedItem{
			{ItemKey: cell(6, 1), NodeIDs: []string{"root-strings"}},
			{ItemKey: cell(6, 0), NodeIDs: []string{"root-strings"}},
		}
		f.exercise("name-the-third", "root-strings", 30)
		due := f.now.AddDate(0, 0, 2)
		f.states.put(studentCaller().ID, domain.PracticeItemState{ItemKey: cell(6, 1), RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: &due})

		got, err := f.svc.FretboardMap(context.Background(), studentCaller(), practiceGuitar)

		require.NoError(t, err)
		assert.Equal(t, practiceGuitar, got.InstrumentID)
		require.NotNil(t, got.LayoutInstrumentID)
		assert.Equal(t, practiceGuitar, *got.LayoutInstrumentID)
		require.Len(t, got.Cells, 2)
		assert.Equal(t, cell(6, 0), got.Cells[0].ItemKey)
		assert.Equal(t, domain.KnowledgeLevelNew, got.Cells[0].Level)
		assert.Equal(t, cell(6, 1), got.Cells[1].ItemKey)
		assert.Equal(t, domain.KnowledgeLevelAccurate, got.Cells[1].Level)
	})

	t.Run("an instrument without cells has an empty map", func(t *testing.T) {
		f := newSummaryFixture(t)

		got, err := f.svc.FretboardMap(context.Background(), studentCaller(), practiceBass)

		require.NoError(t, err)
		assert.Nil(t, got.LayoutInstrumentID)
		assert.Empty(t, got.Cells)
	})

	t.Run("an instrument that doesn't exist is not found", func(t *testing.T) {
		f := newSummaryFixture(t)

		_, err := f.svc.FretboardMap(context.Background(), studentCaller(), "missing")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a failure reading states fails the request", func(t *testing.T) {
		f := newSummaryFixture(t)
		f.cells[practiceGuitar] = []domain.ClassifiedItem{{ItemKey: cell(6, 0)}}
		f.states.err = errors.New("mongo down")

		_, err := f.svc.FretboardMap(context.Background(), studentCaller(), practiceGuitar)

		require.Error(t, err)
	})
}
