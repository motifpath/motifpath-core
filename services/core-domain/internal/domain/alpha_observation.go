package domain

import (
	"cmp"
	"slices"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	// The alpha's weeks are in São Paulo time wherever the service runs,
	// including images without the system's zone database.
	_ "time/tzdata"
)

const (
	// AlphaTargetSessions is how many finished practice sessions in a week
	// put a student at the target for that week.
	AlphaTargetSessions = 3

	// alphaWeeksListed is how many of a student's most recent weeks so far
	// are listed; all of them are still counted.
	alphaWeeksListed = 8

	// alphaQuietAfter is how long without any activity makes a student
	// quiet.
	alphaQuietAfter = 7 * 24 * time.Hour

	// sessionAbandonGrace is how long past its planned minutes a session
	// may stay silent before it counts as abandoned. The worker that keeps
	// sessions applies the same rule; both must agree.
	sessionAbandonGrace = 15 * time.Minute
)

// alphaLocation is the one time zone every alpha week is measured in, so
// all students share the same weeks.
var alphaLocation = func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err) // embedded by time/tzdata, so it is always found
	}
	return loc
}()

// AlphaWeek is a Monday 00:00 to Sunday 24:00 week in São Paulo time.
type AlphaWeek struct {
	start time.Time
}

// AlphaWeekOf is the week t falls in.
func AlphaWeekOf(t time.Time) AlphaWeek {
	local := t.In(alphaLocation)
	daysSinceMonday := (int(local.Weekday()) + 6) % 7
	monday := time.Date(local.Year(), local.Month(), local.Day()-daysSinceMonday, 0, 0, 0, 0, alphaLocation)
	return AlphaWeek{start: monday}
}

// ParseAlphaWeek is the week starting on the Monday monday names, by its
// calendar date; nil is the running week. A date that isn't a Monday, or a
// week after the running one, is a validation error on week.
func ParseAlphaWeek(monday *time.Time, now time.Time) (AlphaWeek, error) {
	running := AlphaWeekOf(now)
	if monday == nil {
		return running, nil
	}
	if monday.Weekday() != time.Monday {
		return AlphaWeek{}, NewValidationError("week", "must be a Monday")
	}
	week := AlphaWeek{start: time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, alphaLocation)}
	if week.start.After(running.start) {
		return AlphaWeek{}, NewValidationError("week", "must not start after the running week")
	}
	return week, nil
}

// Start is the week's first instant; End the first instant after it.
func (w AlphaWeek) Start() time.Time { return w.start }
func (w AlphaWeek) End() time.Time   { return w.start.AddDate(0, 0, 7) }

// FirstDay and LastDay are the week's Monday and Sunday as calendar dates.
func (w AlphaWeek) FirstDay() time.Time { return calendarDate(w.start) }
func (w AlphaWeek) LastDay() time.Time  { return calendarDate(w.start.AddDate(0, 0, 6)) }

func (w AlphaWeek) Previous() AlphaWeek { return AlphaWeek{start: w.start.AddDate(0, 0, -7)} }
func (w AlphaWeek) Next() AlphaWeek     { return AlphaWeek{start: w.start.AddDate(0, 0, 7)} }

// Contains reports whether t falls in the week.
func (w AlphaWeek) Contains(t time.Time) bool {
	return !t.Before(w.start) && t.Before(w.End())
}

// IsRunning reports whether now falls in the week.
func (w AlphaWeek) IsRunning(now time.Time) bool { return w.Contains(now) }

func calendarDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// PracticeSessionState is how a practice session stands when it is read.
type PracticeSessionState string

const (
	PracticeSessionFinished   PracticeSessionState = "finished"
	PracticeSessionEndedEarly PracticeSessionState = "ended_early"
	PracticeSessionAbandoned  PracticeSessionState = "abandoned"
	PracticeSessionInProgress PracticeSessionState = "in_progress"
)

// PracticeSessionRecord is one practice session whose start has arrived, as
// the worker keeps it.
type PracticeSessionRecord struct {
	ID             string
	StudentID      string
	StartedAt      time.Time
	PlannedMinutes int
	// LastEventAt is the session's latest event.
	LastEventAt time.Time
	// End is nil until the session's end arrives.
	End *PracticeSessionRecordEnd
	// AnswersArrived counts the answers given in the session that have
	// arrived, each once.
	AnswersArrived int
}

// PracticeSessionRecordEnd is a session's end as the student sent it.
type PracticeSessionRecordEnd struct {
	At            time.Time
	LeftEarly     bool
	AnsweredCount int
}

// StateAt is how the session stands when read at now, and when it ended if
// it has. A session that never ended counts as abandoned, ended at its last
// event, once it has been silent for its planned minutes plus the grace.
func (s PracticeSessionRecord) StateAt(now time.Time) (PracticeSessionState, *time.Time) {
	if s.End != nil {
		endedAt := s.End.At
		if s.End.LeftEarly {
			return PracticeSessionEndedEarly, &endedAt
		}
		return PracticeSessionFinished, &endedAt
	}
	silence := time.Duration(s.PlannedMinutes)*time.Minute + sessionAbandonGrace
	if now.Sub(s.LastEventAt) >= silence {
		endedAt := s.LastEventAt
		return PracticeSessionAbandoned, &endedAt
	}
	return PracticeSessionInProgress, nil
}

// Finished reports whether the session ended without leaving early — the
// only sessions that count toward a week.
func (s PracticeSessionRecord) Finished() bool {
	return s.End != nil && !s.End.LeftEarly
}

// ItemsAnswered is the count the student's end reported, once it has
// arrived, and otherwise the answers that arrived.
func (s PracticeSessionRecord) ItemsAnswered() int {
	if s.End != nil {
		return s.End.AnsweredCount
	}
	return s.AnswersArrived
}

// FirstPathProgress is how many of a student's first path's nodes they
// have completed.
type FirstPathProgress struct {
	Completed int
	Total     int
}

// Percent is the completed share, rounded down to a whole percent.
func (p FirstPathProgress) Percent() int {
	if p.Total == 0 {
		return 0
	}
	return p.Completed * 100 / p.Total
}

// OverTarget reports whether strictly more than 70% of the path is
// completed, compared exactly rather than on the rounded percent.
func (p FirstPathProgress) OverTarget() bool {
	return p.Completed*10 > p.Total*7
}

// AlphaStudentActivity is what a student's weekly measures are taken from.
type AlphaStudentActivity struct {
	FirstPathGivenAt time.Time
	// Sessions are the student's sessions, of any state, at least from the
	// week their first path was given.
	Sessions []PracticeSessionRecord
	// LastActiveAt is nil when the student has no activity at all.
	LastActiveAt *time.Time
}

// AlphaWeekCount is one of a student's finished weeks with its counted
// sessions.
type AlphaWeekCount struct {
	// WeekStart is the week's Monday as a calendar date.
	WeekStart time.Time
	Finished  int
}

// AlphaStudentMeasure is one student measured for a week.
type AlphaStudentMeasure struct {
	FinishedInWeek         int
	FinishedInPreviousWeek int
	// WeeksSoFar lists the most recent weeks so far, oldest first;
	// WeeksSoFarCount counts all of them.
	WeeksSoFar         []AlphaWeekCount
	WeeksSoFarCount    int
	WeeksAtTargetCount int
	AtTargetEveryWeek  bool
	Quiet              bool
}

// AtTargetInWeek reports whether the student reached the target in the
// measured week.
func (m AlphaStudentMeasure) AtTargetInWeek() bool {
	return m.FinishedInWeek >= AlphaTargetSessions
}

// MeasureAlphaStudent measures a student for week, read at now. Weeks so far
// are the finished weeks from the one the first path was given in; the
// running week is never one of them.
func MeasureAlphaStudent(a AlphaStudentActivity, week AlphaWeek, now time.Time) AlphaStudentMeasure {
	m := AlphaStudentMeasure{
		FinishedInWeek:         finishedIn(a.Sessions, week),
		FinishedInPreviousWeek: finishedIn(a.Sessions, week.Previous()),
		WeeksSoFar:             []AlphaWeekCount{},
		Quiet:                  quiet(a, now),
	}
	running := AlphaWeekOf(now)
	for w := AlphaWeekOf(a.FirstPathGivenAt); w.Start().Before(running.Start()); w = w.Next() {
		count := AlphaWeekCount{WeekStart: w.FirstDay(), Finished: finishedIn(a.Sessions, w)}
		m.WeeksSoFar = append(m.WeeksSoFar, count)
		m.WeeksSoFarCount++
		if count.Finished >= AlphaTargetSessions {
			m.WeeksAtTargetCount++
		}
	}
	m.WeeksSoFar = m.WeeksSoFar[max(0, len(m.WeeksSoFar)-alphaWeeksListed):]
	m.AtTargetEveryWeek = m.WeeksSoFarCount > 0 && m.WeeksAtTargetCount == m.WeeksSoFarCount
	return m
}

func finishedIn(sessions []PracticeSessionRecord, week AlphaWeek) int {
	n := 0
	for _, s := range sessions {
		if s.Finished() && week.Contains(s.StartedAt) {
			n++
		}
	}
	return n
}

// quiet reports a student with no activity for alphaQuietAfter; with no
// activity at all, counted from when their first path was given.
func quiet(a AlphaStudentActivity, now time.Time) bool {
	since := a.FirstPathGivenAt
	if a.LastActiveAt != nil {
		since = *a.LastActiveAt
	}
	return now.Sub(since) >= alphaQuietAfter
}

// AlphaStudent is one student of the alpha with their measures.
type AlphaStudent struct {
	StudentID   string
	DisplayName string
	FirstPath   FirstPathProgress
	Measure     AlphaStudentMeasure
}

// AlphaCohortSummary is the alpha as a whole for a week.
type AlphaCohortSummary struct {
	StudentCount           int
	AtTargetInWeek         int
	AtTargetInPreviousWeek int
	AtTargetEveryWeek      int
	FirstPathOverTarget    int
	// MedianFirstPathPercent is nil when the alpha has no students.
	MedianFirstPathPercent *int
}

// SummarizeAlpha counts the students at each target and takes the median
// of their first paths' completion.
func SummarizeAlpha(students []AlphaStudent) AlphaCohortSummary {
	summary := AlphaCohortSummary{StudentCount: len(students)}
	paths := make([]FirstPathProgress, 0, len(students))
	for _, s := range students {
		if s.Measure.AtTargetInWeek() {
			summary.AtTargetInWeek++
		}
		if s.Measure.FinishedInPreviousWeek >= AlphaTargetSessions {
			summary.AtTargetInPreviousWeek++
		}
		if s.Measure.AtTargetEveryWeek {
			summary.AtTargetEveryWeek++
		}
		if s.FirstPath.OverTarget() {
			summary.FirstPathOverTarget++
		}
		paths = append(paths, s.FirstPath)
	}
	summary.MedianFirstPathPercent = medianPercent(paths)
	return summary
}

// medianPercent is the median of the paths' exact completed shares,
// rounded down to a whole percent; with an even count, the mean of the two
// middle shares. It stays in integers so no share is rounded before the
// end.
func medianPercent(paths []FirstPathProgress) *int {
	if len(paths) == 0 {
		return nil
	}
	shares := make([]FirstPathProgress, len(paths))
	for i, p := range paths {
		if p.Total == 0 {
			p = FirstPathProgress{Completed: 0, Total: 1}
		}
		shares[i] = p
	}
	slices.SortFunc(shares, func(a, b FirstPathProgress) int {
		return cmp.Compare(a.Completed*b.Total, b.Completed*a.Total)
	})
	mid := len(shares) / 2
	if len(shares)%2 == 1 {
		percent := shares[mid].Percent()
		return &percent
	}
	a, b := shares[mid-1], shares[mid]
	percent := 100 * (a.Completed*b.Total + b.Completed*a.Total) / (2 * a.Total * b.Total)
	return &percent
}

// SortAlphaStudents orders students as NewAlphaStudentOrder does.
func SortAlphaStudents(students []AlphaStudent) {
	slices.SortFunc(students, NewAlphaStudentOrder())
}

// NewAlphaStudentOrder compares students by fewest counted sessions in the
// week, then by display name as a Brazilian Portuguese reader alphabetizes
// it, so accents and case don't move a name out of place, then by student
// id. Each order holds its own collator, which can't be shared between
// goroutines.
func NewAlphaStudentOrder() func(a, b AlphaStudent) int {
	names := collate.New(language.BrazilianPortuguese)
	return func(a, b AlphaStudent) int {
		return cmp.Or(
			cmp.Compare(a.Measure.FinishedInWeek, b.Measure.FinishedInWeek),
			names.CompareString(a.DisplayName, b.DisplayName),
			cmp.Compare(a.StudentID, b.StudentID),
		)
	}
}

// AlphaStudentKeep is which students of the alpha a list keeps; the zero
// value keeps every student.
type AlphaStudentKeep string

const (
	// AlphaKeepBelowTarget keeps students with fewer counted sessions in
	// the week than the target.
	AlphaKeepBelowTarget AlphaStudentKeep = "below_target"
	// AlphaKeepQuiet keeps quiet students.
	AlphaKeepQuiet AlphaStudentKeep = "quiet"
)

// Valid reports whether k is a keep this service knows; the zero value
// keeps every student.
func (k AlphaStudentKeep) Valid() bool {
	return k == "" || k == AlphaKeepBelowTarget || k == AlphaKeepQuiet
}

// AlphaStudentFilter narrows a list of the alpha's students. Name, when
// set, keeps students whose display name contains it, ignoring case and
// accents; both fields must match.
type AlphaStudentFilter struct {
	Name string
	Keep AlphaStudentKeep
}

// Matches reports whether s passes every set field of f.
func (f AlphaStudentFilter) Matches(s AlphaStudent) bool {
	if f.Name != "" && !ContainsLoosely(s.DisplayName, f.Name) {
		return false
	}
	switch f.Keep {
	case AlphaKeepBelowTarget:
		return !s.Measure.AtTargetInWeek()
	case AlphaKeepQuiet:
		return s.Measure.Quiet
	}
	return true
}

// AlphaFilterCounts is how many students each keep holds, counted over the
// whole alpha so a count never moves when another filter is picked.
type AlphaFilterCounts struct {
	All         int
	BelowTarget int
	Quiet       int
}

// CountAlphaFilters counts students for every keep.
func CountAlphaFilters(students []AlphaStudent) AlphaFilterCounts {
	counts := AlphaFilterCounts{All: len(students)}
	for _, s := range students {
		if (AlphaStudentFilter{Keep: AlphaKeepBelowTarget}).Matches(s) {
			counts.BelowTarget++
		}
		if (AlphaStudentFilter{Keep: AlphaKeepQuiet}).Matches(s) {
			counts.Quiet++
		}
	}
	return counts
}
