//go:build integration

package bdd

import (
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// Steps for the overview's minutes practised, day streak and skills up.
// Sessions are seeded raw, as the worker keeps them: a start, the latest
// event, and an end unless the session was abandoned.

func registerPracticeOverviewEffortSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^student "([^"]+)" has never practised$`, w.newStudentHasNeverPractised)
	sc.Step(`^"([^"]+)" practised from (\d+):(\d+) to (\d+):(\d+) on (\w+) and from (\d+):(\d+) to (\d+):(\d+) on (\w+)$`, w.practisedFromToOnTwoDays)
	sc.Step(`^"([^"]+)" practised (\d+) minutes (\d+) days ago and (\d+) minutes (\d+) days ago$`, w.practisedMinutesDaysAgo)
	sc.Step(`^"([^"]+)" started a session at (\d+):(\d+) yesterday, answered until (\d+):(\d+) and left early$`, w.startedAndLeftEarly)
	sc.Step(`^"([^"]+)" started a (\d+)-minute session at (\d+):(\d+) yesterday and sent nothing for it after an answer at (\d+):(\d+)$`, w.startedAndAbandoned)
	sc.Step(`^"([^"]+)" practised for (\d+) minutes and (\d+) seconds yesterday$`, w.practisedMinutesAndSeconds)

	sc.Step(`^"([^"]+)" finished a session on each of the last (\d+) days, today included$`, w.finishedOnLastDaysTodayIncluded)
	sc.Step(`^"([^"]+)" finished a session on each of the (\d+) days before today$`, w.finishedOnDaysBeforeToday)
	sc.Step(`^"([^"]+)" has not practised today$`, func(string) error { return nil })
	sc.Step(`^"([^"]+)" finished a session on (\d+) consecutive days, missed the next day, and has finished a session on each of the (\d+) days since, today included$`, w.finishedRunMissedAndSince)
	sc.Step(`^"([^"]+)" last finished a session (\d+) days ago$`, w.lastFinishedDaysAgo)
	sc.Step(`^"([^"]+)" finished a session (\d+) days ago and today, and their only session yesterday ended early$`, w.finishedAroundDayLeftEarly)
	sc.Step(`^"([^"]+)" finished a session with "([^"]+)" in hand and another with "([^"]+)" in hand today$`, w.finishedTwoToday)
	sc.Step(`^"([^"]+)" finished no session before today$`, func(string) error { return nil })
	sc.Step(`^"([^"]+)" finished a session at 23:30 yesterday in "([^"]+)", which is today in UTC$`, w.finishedLateYesterday)
	sc.Step(`^"([^"]+)" finished a session today$`, w.finishedToday)

	sc.Step(`^"([^"]+)"'s accuracy and fluency on "([^"]+)" on ([a-z-]+) both improved this week$`, w.accuracyAndFluencyImproved)
	sc.Step(`^"([^"]+)"'s best clean tempo on "([^"]+)" on ([a-z-]+) improved this week$`, w.bestCleanTempoImproved)
	sc.Step(`^"([^"]+)"'s accuracy on "([^"]+)" on ([a-z-]+) improved this week$`, w.accuracyImprovedOn)
	sc.Step(`^"([^"]+)"'s accuracy on "([^"]+)", which has items for every instrument, improved this week$`, w.accuracyImprovedOnEveryInstrument)
	sc.Step(`^no skill of "([^"]+)" improved this week$`, func(string) error { return nil })

	sc.Step(`^the overview shows (\d+) minutes practised in the last 7 days$`, w.overviewShowsMinutesLast7)
	sc.Step(`^the overview shows (\d+) minutes practised in the previous 7 days$`, w.overviewShowsMinutesPrevious7)
	sc.Step(`^the overview shows a day streak of (\d+)$`, w.overviewShowsDayStreak)
	sc.Step(`^the overview shows a best day streak of (\d+)$`, w.overviewShowsBestDayStreak)
	sc.Step(`^the overview shows (\d+) skills? up in the last 7 days$`, w.overviewShowsSkillsUp)
}

// ── Givens ───────────────────────────────────────────────────────────────

// newStudentHasNeverPractised signs name in as a student with no activity
// at all.
func (w *world) newStudentHasNeverPractised(name string) error {
	w.authenticateAs(name, domain.RoleStudent)
	return nil
}

// recordSession seeds a guitar session of name's that started at
// startedAt and last heard from at lastEventAt. A nil end means it was
// abandoned.
func (w *world) recordSession(name string, startedAt, lastEventAt time.Time, endedAt *time.Time, leftEarly bool) {
	id := instrumentID("guitar").String()
	w.practiceActivity.addSession(w.ensureRegistered(name, domain.RoleStudent).String(), fakePracticeSession{
		instrumentID: &id, startedAt: &startedAt, lastEventAt: lastEventAt, endedAt: endedAt, leftEarly: leftEarly,
	})
}

// practisedBetween seeds a session finished at to, started at from.
func (w *world) practisedBetween(name string, from, to time.Time) {
	w.recordSession(name, from, to, &to, false)
}

// weekday is the day called dayName in the scenarios' week, which starts
// on monday, in the student's time zone.
func (w *world) weekday(dayName string) (time.Time, error) {
	for i := range 7 {
		day := monday.AddDate(0, 0, i)
		if day.Weekday().String() == dayName {
			return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, w.location()), nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown weekday %q", dayName)
}

func clockOn(day time.Time, hour, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location())
}

func (w *world) practisedFromToOnTwoDays(name string, h1, m1, h2, m2 int, firstDay string, h3, m3, h4, m4 int, secondDay string) error {
	first, err := w.weekday(firstDay)
	if err != nil {
		return err
	}
	second, err := w.weekday(secondDay)
	if err != nil {
		return err
	}
	w.practisedBetween(name, clockOn(first, h1, m1), clockOn(first, h2, m2))
	w.practisedBetween(name, clockOn(second, h3, m3), clockOn(second, h4, m4))
	return nil
}

func (w *world) practisedMinutesDaysAgo(name string, firstMinutes, firstDaysAgo, secondMinutes, secondDaysAgo int) error {
	for _, s := range []struct{ minutes, daysAgo int }{{firstMinutes, firstDaysAgo}, {secondMinutes, secondDaysAgo}} {
		start := w.localTime(s.daysAgo, 18, 0)
		w.practisedBetween(name, start, start.Add(time.Duration(s.minutes)*time.Minute))
	}
	return nil
}

func (w *world) startedAndLeftEarly(name string, startHour, startMinute, lastHour, lastMinute int) error {
	last := w.localTime(1, lastHour, lastMinute)
	w.recordSession(name, w.localTime(1, startHour, startMinute), last, &last, true)
	return nil
}

func (w *world) startedAndAbandoned(name string, _, startHour, startMinute, lastHour, lastMinute int) error {
	w.recordSession(name, w.localTime(1, startHour, startMinute), w.localTime(1, lastHour, lastMinute), nil, false)
	return nil
}

func (w *world) practisedMinutesAndSeconds(name string, minutes, seconds int) error {
	start := w.localTime(1, 18, 0)
	w.practisedBetween(name, start, start.Add(time.Duration(minutes)*time.Minute+time.Duration(seconds)*time.Second))
	return nil
}

// finishSessionDaysAgo seeds a session with guitar in hand finished at
// 08:00 on the day daysAgo days before today, before the overview's now.
func (w *world) finishSessionDaysAgo(name string, daysAgo int) {
	w.finishSession(name, "guitar", w.localTime(daysAgo, 8, 0))
}

func (w *world) finishedOnLastDaysTodayIncluded(name string, days int) error {
	for i := range days {
		w.finishSessionDaysAgo(name, i)
	}
	return nil
}

func (w *world) finishedOnDaysBeforeToday(name string, days int) error {
	for i := 1; i <= days; i++ {
		w.finishSessionDaysAgo(name, i)
	}
	return nil
}

func (w *world) finishedRunMissedAndSince(name string, run, since int) error {
	for i := range since {
		w.finishSessionDaysAgo(name, i)
	}
	for i := range run {
		w.finishSessionDaysAgo(name, since+1+i)
	}
	return nil
}

func (w *world) lastFinishedDaysAgo(name string, daysAgo int) error {
	w.finishSessionDaysAgo(name, daysAgo)
	return nil
}

func (w *world) finishedAroundDayLeftEarly(name string, daysAgo int) error {
	w.finishSessionDaysAgo(name, daysAgo)
	w.finishSessionDaysAgo(name, 0)
	return w.onlySessionYesterdayEndedEarly(name)
}

func (w *world) finishedTwoToday(name, first, second string) error {
	w.finishSession(name, first, w.localTime(0, 7, 0))
	w.finishSession(name, second, w.localTime(0, 8, 0))
	return nil
}

func (w *world) finishedLateYesterday(name, timeZone string) error {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return err
	}
	y, m, d := w.summaryNow.In(loc).Date()
	at := time.Date(y, m, d-1, 23, 30, 0, 0, loc)
	if at.UTC().Day() != w.summaryNow.UTC().Day() {
		return fmt.Errorf("23:30 yesterday in %s is not today in UTC", timeZone)
	}
	w.finishSession(name, "guitar", at)
	return nil
}

func (w *world) finishedToday(name string) error {
	w.finishSessionDaysAgo(name, 0)
	return nil
}

// improvedOn gives name a counted state now and a snapshot from before the
// last 7 days on a new exercise of skill for instrument (empty: every
// instrument).
func (w *world) improvedOn(name, skill, instrument string, then domain.PracticeItemSnapshot, now domain.PracticeItemState) {
	var instruments []string
	if instrument != "" {
		instruments = []string{instrument}
	}
	key := w.putPracticeExercise(skill+"-"+instrument+"-improved", skill, instruments...)
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.AddDate(0, 0, 3)
	now.ItemKey, now.RulesVersion, now.Level, now.Counted, now.Box, now.DueAt, now.LastAt =
		key, domain.PracticeRulesVersion, domain.KnowledgeLevelLearning, 3, 1, &dueAt, &fixedNow
	then.ItemKey, then.Counted = key, 2
	w.practiceActivity.addSnapshot(studentID, w.snapshotDayBefore(), then)
	w.practiceStates.put(studentID, now)
}

func (w *world) accuracyAndFluencyImproved(name, skill, instrument string) error {
	w.improvedOn(name, skill, instrument,
		domain.PracticeItemSnapshot{Accuracy: 0.6, Fluency: 0.4},
		domain.PracticeItemState{Accuracy: 0.8, Fluency: 0.7})
	return nil
}

func (w *world) accuracyImprovedOn(name, skill, instrument string) error {
	w.improvedOn(name, skill, instrument,
		domain.PracticeItemSnapshot{Accuracy: 0.6},
		domain.PracticeItemState{Accuracy: 0.8})
	return nil
}

func (w *world) accuracyImprovedOnEveryInstrument(name, skill string) error {
	return w.accuracyImprovedOn(name, skill, "")
}

func (w *world) bestCleanTempoImproved(name, skill, instrument string) error {
	then, now := 80, 92
	w.improvedOn(name, skill, instrument,
		domain.PracticeItemSnapshot{Accuracy: 0.8, BestCleanBPM: &then},
		domain.PracticeItemState{Accuracy: 0.8, BestCleanBPM: &now})
	return nil
}

// ── Thens ────────────────────────────────────────────────────────────────

// overviewCount compares one count of the overview with want, naming it
// what.
func (w *world) overviewCount(what string, want int, count func(generated.PracticeOverview) int) error {
	o, err := w.overview()
	if err != nil {
		return err
	}
	if got := count(o); got != want {
		return fmt.Errorf("expected %d %s, got %d", want, what, got)
	}
	return nil
}

func (w *world) overviewShowsMinutesLast7(minutes int) error {
	return w.overviewCount("minutes practised in the last 7 days", minutes, func(o generated.PracticeOverview) int { return o.MinutesPractisedLast7 })
}

func (w *world) overviewShowsMinutesPrevious7(minutes int) error {
	return w.overviewCount("minutes practised in the previous 7 days", minutes, func(o generated.PracticeOverview) int { return o.MinutesPractisedPrevious7 })
}

func (w *world) overviewShowsDayStreak(days int) error {
	return w.overviewCount("days of current streak", days, func(o generated.PracticeOverview) int { return o.DayStreakCurrent })
}

func (w *world) overviewShowsBestDayStreak(days int) error {
	return w.overviewCount("days of best streak", days, func(o generated.PracticeOverview) int { return o.DayStreakBest })
}

func (w *world) overviewShowsSkillsUp(skills int) error {
	return w.overviewCount("skills up", skills, func(o generated.PracticeOverview) int { return o.SkillsUpLast7 })
}
