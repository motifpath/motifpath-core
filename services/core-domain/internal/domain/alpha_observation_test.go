package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func saoPauloTime(t *testing.T, month time.Month, day, hour, minute int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	return time.Date(2026, month, day, hour, minute, 0, 0, loc)
}

func date(month time.Month, day int) time.Time {
	return time.Date(2026, month, day, 0, 0, 0, 0, time.UTC)
}

func TestParseAlphaWeek(t *testing.T) {
	now := saoPauloTime(t, time.October, 14, 10, 0) // a Wednesday
	cases := []struct {
		name        string
		week        *time.Time
		wantMonday  time.Time
		wantRunning bool
		wantErr     bool
	}{
		{name: "no week is the running week", week: nil, wantMonday: date(time.October, 12), wantRunning: true},
		{name: "the running week's Monday", week: ptrTo(date(time.October, 12)), wantMonday: date(time.October, 12), wantRunning: true},
		{name: "a past Monday", week: ptrTo(date(time.October, 5)), wantMonday: date(time.October, 5)},
		{name: "a day that isn't a Monday", week: ptrTo(date(time.October, 14)), wantErr: true},
		{name: "a week after the running one", week: ptrTo(date(time.October, 19)), wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			week, err := domain.ParseAlphaWeek(c.week, now)

			if c.wantErr {
				var verr *domain.ValidationError
				require.ErrorAs(t, err, &verr)
				assert.Equal(t, "week", verr.Fields[0].Field)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantMonday, week.FirstDay())
			assert.Equal(t, c.wantMonday.AddDate(0, 0, 6), week.LastDay())
			assert.Equal(t, c.wantRunning, week.IsRunning(now))
		})
	}
}

func TestAlphaWeekOf_FollowsSaoPauloTime(t *testing.T) {
	sundayNight := saoPauloTime(t, time.October, 11, 23, 50) // Monday 02:50 in UTC

	assert.Equal(t, date(time.October, 5), domain.AlphaWeekOf(sundayNight).FirstDay())
	assert.Equal(t, date(time.October, 12), domain.AlphaWeekOf(sundayNight).Next().FirstDay())
	assert.Equal(t, date(time.September, 28), domain.AlphaWeekOf(sundayNight).Previous().FirstDay())
}

func TestPracticeSessionRecord_StateAt(t *testing.T) {
	start := saoPauloTime(t, time.October, 12, 18, 30)
	now := saoPauloTime(t, time.October, 14, 10, 0)
	ended := func(at time.Time, leftEarly bool, answered int) *domain.PracticeSessionRecordEnd {
		return &domain.PracticeSessionRecordEnd{At: at, LeftEarly: leftEarly, AnsweredCount: answered}
	}
	cases := []struct {
		name         string
		session      domain.PracticeSessionRecord
		wantState    domain.PracticeSessionState
		wantEndedAt  *time.Time
		wantAnswered int
	}{
		{
			name:      "ended without leaving early is finished, with the end's answer count",
			session:   domain.PracticeSessionRecord{StartedAt: start, PlannedMinutes: 10, LastEventAt: start.Add(11 * time.Minute), End: ended(start.Add(11*time.Minute), false, 9), AnswersArrived: 8},
			wantState: domain.PracticeSessionFinished, wantEndedAt: ptrTo(start.Add(11 * time.Minute)), wantAnswered: 9,
		},
		{
			name:      "left early ended early",
			session:   domain.PracticeSessionRecord{StartedAt: start, PlannedMinutes: 10, LastEventAt: start.Add(4 * time.Minute), End: ended(start.Add(4*time.Minute), true, 2), AnswersArrived: 2},
			wantState: domain.PracticeSessionEndedEarly, wantEndedAt: ptrTo(start.Add(4 * time.Minute)), wantAnswered: 2,
		},
		{
			name:      "silent for its planned minutes plus 15 is abandoned at its last event, with the answers that arrived",
			session:   domain.PracticeSessionRecord{StartedAt: start, PlannedMinutes: 10, LastEventAt: start.Add(3 * time.Minute), AnswersArrived: 3},
			wantState: domain.PracticeSessionAbandoned, wantEndedAt: ptrTo(start.Add(3 * time.Minute)), wantAnswered: 3,
		},
		{
			name:      "within its planned minutes plus 15 is in progress",
			session:   domain.PracticeSessionRecord{StartedAt: now.Add(-5 * time.Minute), PlannedMinutes: 10, LastEventAt: now.Add(-5 * time.Minute)},
			wantState: domain.PracticeSessionInProgress,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, endedAt := c.session.StateAt(now)

			assert.Equal(t, c.wantState, state)
			assert.Equal(t, c.wantEndedAt, endedAt)
			assert.Equal(t, c.wantAnswered, c.session.ItemsAnswered())
		})
	}
}

func TestFirstPathProgress(t *testing.T) {
	cases := []struct {
		name           string
		completed      int
		total          int
		wantPercent    int
		wantOverTarget bool
	}{
		{"11 of 18 is 61%", 11, 18, 61, false},
		{"exactly 70% is not over the target", 7, 10, 70, false},
		{"just over 70% is over the target and still shown rounded down", 701, 1000, 70, true},
		{"a whole path", 18, 18, 100, true},
		{"a path with no nodes", 0, 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := domain.FirstPathProgress{Completed: c.completed, Total: c.total}

			assert.Equal(t, c.wantPercent, p.Percent())
			assert.Equal(t, c.wantOverTarget, p.OverTarget())
		})
	}
}

// finishedOn is a finished session that started at the given São Paulo time.
func finishedOn(t *testing.T, month time.Month, day, hour int) domain.PracticeSessionRecord {
	t.Helper()
	start := saoPauloTime(t, month, day, hour, 0)
	return domain.PracticeSessionRecord{
		StartedAt: start, PlannedMinutes: 10, LastEventAt: start.Add(10 * time.Minute),
		End: &domain.PracticeSessionRecordEnd{At: start.Add(10 * time.Minute), AnsweredCount: 5},
	}
}

func TestMeasureAlphaStudent_Weeks(t *testing.T) {
	now := saoPauloTime(t, time.October, 14, 10, 0)
	week := domain.AlphaWeekOf(now)
	given := saoPauloTime(t, time.September, 21, 9, 0)
	endedEarly := finishedOn(t, time.October, 12, 19)
	endedEarly.End.LeftEarly = true
	abandoned := finishedOn(t, time.October, 13, 19)
	abandoned.End = nil

	cases := []struct {
		name           string
		given          time.Time
		sessions       []domain.PracticeSessionRecord
		wantInWeek     int
		wantInPrevious int
		wantWeeks      []domain.AlphaWeekCount
		wantWeeksCount int
		wantAtTarget   int
		wantEveryWeek  bool
	}{
		{
			name:  "only finished sessions count, in the week they started",
			given: given,
			sessions: []domain.PracticeSessionRecord{
				finishedOn(t, time.October, 12, 18), endedEarly, abandoned, finishedOn(t, time.October, 14, 7),
				finishedOn(t, time.October, 11, 23), // Sunday night belongs to the week before
			},
			wantInWeek: 2, wantInPrevious: 1,
			wantWeeks: []domain.AlphaWeekCount{
				{WeekStart: date(time.September, 21)}, {WeekStart: date(time.September, 28)},
				{WeekStart: date(time.October, 5), Finished: 1},
			},
			wantWeeksCount: 3,
		},
		{
			name:  "at the target every week so far",
			given: given,
			sessions: []domain.PracticeSessionRecord{
				finishedOn(t, time.September, 21, 9), finishedOn(t, time.September, 22, 9), finishedOn(t, time.September, 23, 9),
				finishedOn(t, time.September, 28, 9), finishedOn(t, time.September, 29, 9), finishedOn(t, time.September, 30, 9), finishedOn(t, time.October, 1, 9),
				finishedOn(t, time.October, 5, 9), finishedOn(t, time.October, 7, 9), finishedOn(t, time.October, 10, 9),
			},
			wantInPrevious: 3,
			wantWeeks: []domain.AlphaWeekCount{
				{WeekStart: date(time.September, 21), Finished: 3}, {WeekStart: date(time.September, 28), Finished: 4},
				{WeekStart: date(time.October, 5), Finished: 3},
			},
			wantWeeksCount: 3, wantAtTarget: 3, wantEveryWeek: true,
		},
		{
			name:  "a first path given in the running week has no weeks so far",
			given: saoPauloTime(t, time.October, 12, 9, 0),
			sessions: []domain.PracticeSessionRecord{
				finishedOn(t, time.October, 12, 18), finishedOn(t, time.October, 13, 18), finishedOn(t, time.October, 14, 7),
			},
			wantInWeek: 3, wantWeeks: []domain.AlphaWeekCount{},
		},
		{
			name:  "only the last 8 weeks so far are listed, but all are counted",
			given: saoPauloTime(t, time.August, 3, 9, 0),
			wantWeeks: []domain.AlphaWeekCount{
				{WeekStart: date(time.August, 17)}, {WeekStart: date(time.August, 24)}, {WeekStart: date(time.August, 31)},
				{WeekStart: date(time.September, 7)}, {WeekStart: date(time.September, 14)}, {WeekStart: date(time.September, 21)},
				{WeekStart: date(time.September, 28)}, {WeekStart: date(time.October, 5)},
			},
			wantWeeksCount: 10,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := domain.MeasureAlphaStudent(domain.AlphaStudentActivity{FirstPathGivenAt: c.given, Sessions: c.sessions}, week, now)

			assert.Equal(t, c.wantInWeek, m.FinishedInWeek)
			assert.Equal(t, c.wantInWeek >= 3, m.AtTargetInWeek())
			assert.Equal(t, c.wantInPrevious, m.FinishedInPreviousWeek)
			assert.Equal(t, c.wantWeeks, m.WeeksSoFar)
			assert.Equal(t, c.wantWeeksCount, m.WeeksSoFarCount)
			assert.Equal(t, c.wantAtTarget, m.WeeksAtTargetCount)
			assert.Equal(t, c.wantEveryWeek, m.AtTargetEveryWeek)
		})
	}
}

func TestMeasureAlphaStudent_Quiet(t *testing.T) {
	now := saoPauloTime(t, time.October, 14, 10, 0)
	cases := []struct {
		name       string
		given      time.Time
		lastActive *time.Time
		wantQuiet  bool
	}{
		{"last activity 7 days ago is quiet", saoPauloTime(t, time.September, 21, 9, 0), ptrTo(saoPauloTime(t, time.October, 7, 10, 0)), true},
		{"last activity within 7 days is not quiet", saoPauloTime(t, time.September, 21, 9, 0), ptrTo(saoPauloTime(t, time.October, 7, 10, 1)), false},
		{"no activity and a first path 7 days old is quiet", saoPauloTime(t, time.October, 5, 9, 0), nil, true},
		{"no activity and a newer first path is not quiet", saoPauloTime(t, time.October, 12, 9, 0), nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := domain.MeasureAlphaStudent(domain.AlphaStudentActivity{FirstPathGivenAt: c.given, LastActiveAt: c.lastActive}, domain.AlphaWeekOf(now), now)

			assert.Equal(t, c.wantQuiet, m.Quiet)
		})
	}
}

func TestSummarizeAlpha(t *testing.T) {
	student := func(name string, inWeek, inPrevious int, everyWeek bool, completed int) domain.AlphaStudent {
		return domain.AlphaStudent{
			StudentID: name, DisplayName: name,
			FirstPath: domain.FirstPathProgress{Completed: completed, Total: 100},
			Measure:   domain.AlphaStudentMeasure{FinishedInWeek: inWeek, FinishedInPreviousWeek: inPrevious, AtTargetEveryWeek: everyWeek},
		}
	}

	t.Run("counts students at each target and the median first path", func(t *testing.T) {
		got := domain.SummarizeAlpha([]domain.AlphaStudent{
			student("a", 3, 3, true, 17), student("b", 1, 3, true, 45),
			student("c", 0, 3, false, 72), student("d", 2, 1, false, 78),
		})

		assert.Equal(t, domain.AlphaCohortSummary{
			StudentCount: 4, AtTargetInWeek: 1, AtTargetInPreviousWeek: 3, AtTargetEveryWeek: 2,
			FirstPathOverTarget: 2, MedianFirstPathPercent: ptrTo(58),
		}, got)
	})

	t.Run("an odd count takes the middle first path", func(t *testing.T) {
		got := domain.SummarizeAlpha([]domain.AlphaStudent{student("a", 0, 0, false, 90), student("b", 0, 0, false, 10), student("c", 0, 0, false, 29)})

		assert.Equal(t, ptrTo(29), got.MedianFirstPathPercent)
	})

	t.Run("the median is taken on exact shares, then rounded down", func(t *testing.T) {
		a := student("a", 0, 0, false, 1)
		a.FirstPath = domain.FirstPathProgress{Completed: 1, Total: 3} // 33.3…%
		b := student("b", 0, 0, false, 1)
		b.FirstPath = domain.FirstPathProgress{Completed: 2, Total: 3} // 66.6…%

		got := domain.SummarizeAlpha([]domain.AlphaStudent{a, b})

		assert.Equal(t, ptrTo(50), got.MedianFirstPathPercent)
	})

	t.Run("no students has no median", func(t *testing.T) {
		assert.Nil(t, domain.SummarizeAlpha(nil).MedianFirstPathPercent)
	})
}

func TestSortAlphaStudents(t *testing.T) {
	student := func(id, name string, inWeek int) domain.AlphaStudent {
		return domain.AlphaStudent{StudentID: id, DisplayName: name, Measure: domain.AlphaStudentMeasure{FinishedInWeek: inWeek}}
	}
	students := []domain.AlphaStudent{
		student("3", "bia", 1), student("2", "alice", 1), student("4", "caio", 0), student("1", "alice", 1),
	}

	domain.SortAlphaStudents(students)

	var order []string
	for _, s := range students {
		order = append(order, s.StudentID)
	}
	assert.Equal(t, []string{"4", "1", "2", "3"}, order)
}

func TestSortAlphaStudents_NamesSortAlphabeticallyWhateverTheirAccentsOrCase(t *testing.T) {
	student := func(name string) domain.AlphaStudent {
		return domain.AlphaStudent{StudentID: name, DisplayName: name}
	}
	students := []domain.AlphaStudent{student("Zoe"), student("Érica"), student("bruno"), student("Álvaro"), student("Ana")}

	domain.SortAlphaStudents(students)

	var order []string
	for _, s := range students {
		order = append(order, s.DisplayName)
	}
	assert.Equal(t, []string{"Álvaro", "Ana", "bruno", "Érica", "Zoe"}, order)
}

func TestAlphaStudentFilter(t *testing.T) {
	student := func(name string, inWeek int, quiet bool) domain.AlphaStudent {
		return domain.AlphaStudent{
			StudentID: name, DisplayName: name,
			Measure: domain.AlphaStudentMeasure{FinishedInWeek: inWeek, Quiet: quiet},
		}
	}
	joao := student("João Araújo", 3, false)
	bia := student("Beatriz Rocha", 1, true)

	tests := []struct {
		name   string
		filter domain.AlphaStudentFilter
		want   []bool // joao, bia
	}{
		{"no filter keeps everyone", domain.AlphaStudentFilter{}, []bool{true, true}},
		{"below the target keeps fewer than 3 counted sessions", domain.AlphaStudentFilter{Keep: domain.AlphaKeepBelowTarget}, []bool{false, true}},
		{"quiet keeps quiet students", domain.AlphaStudentFilter{Keep: domain.AlphaKeepQuiet}, []bool{false, true}},
		{"a name matches ignoring case and accents", domain.AlphaStudentFilter{Name: "joao araujo"}, []bool{true, false}},
		{"a name and a keep must both match", domain.AlphaStudentFilter{Name: "rocha", Keep: domain.AlphaKeepBelowTarget}, []bool{false, true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, []bool{tt.filter.Matches(joao), tt.filter.Matches(bia)})
		})
	}
}

func TestAlphaStudentKeep_Valid(t *testing.T) {
	for _, keep := range []domain.AlphaStudentKeep{"", domain.AlphaKeepBelowTarget, domain.AlphaKeepQuiet} {
		assert.True(t, keep.Valid(), keep)
	}
	assert.False(t, domain.AlphaStudentKeep("at_target").Valid())
}

func TestCountAlphaFilters(t *testing.T) {
	students := []domain.AlphaStudent{
		{StudentID: "a", Measure: domain.AlphaStudentMeasure{FinishedInWeek: 0, Quiet: true}},
		{StudentID: "b", Measure: domain.AlphaStudentMeasure{FinishedInWeek: 2}},
		{StudentID: "c", Measure: domain.AlphaStudentMeasure{FinishedInWeek: 3}},
	}

	assert.Equal(t, domain.AlphaFilterCounts{All: 3, BelowTarget: 2, Quiet: 1}, domain.CountAlphaFilters(students))
}
