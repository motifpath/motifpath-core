//go:build integration

package bdd

import (
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// dayList matches "today", "Friday", "Monday and today" or
// "Thursday, Saturday and Wednesday".
const dayList = `((?:\w+, )*\w+(?: and \w+)?)`

func registerPracticeDaysSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^today is (\w+) for "([^"]+)"$`, w.todayIs)
	sc.Step(`^"([^"]+)" finished sessions with "([^"]+)" in hand on `+dayList+`$`, w.finishedSessionsOn)
	sc.Step(`^"([^"]+)" finished a session with "([^"]+)" in hand on (\w+) and with "([^"]+)" in hand on (\w+)$`, w.finishedOnTwoDays)
	sc.Step(`^"([^"]+)" completed a content node on `+dayList+`$`, w.completedOn)

	sc.Step(`^the (?:summary|overview) lists 7 days from (\w+) to (\w+)$`, w.listsDaysFromTo)
	sc.Step(`^`+dayList+` are practised and the other (\d+) days are not$`, w.onlyTheseDays(practisedMark))
	sc.Step(`^`+dayList+` are learning days and the other (\d+) days are not$`, w.onlyTheseDays(learnedMark))
	sc.Step(`^yesterday is listed as not practised in the summary's last 7 days$`, w.yesterdayNotPractised)
	sc.Step(`^(\w+) is practised in the overview's last 7 days$`, w.dayMarked(practisedMark, true))
	sc.Step(`^(\w+) is a learning day in the overview's last 7 days$`, w.dayMarked(learnedMark, true))
	sc.Step(`^(\w+) is not a learning day in the overview's last 7 days$`, w.dayMarked(learnedMark, false))
	sc.Step(`^(\d+) of the overview's last 7 days (?:is|are) practised$`, w.practisedDayCount)
}

// listedDay is one entry of a summary's or an overview's last 7 days. A
// summary has no learning days, so learned stays false there.
type listedDay struct {
	date      string
	practised bool
	learned   bool
}

type dayMark func(listedDay) bool

func practisedMark(d listedDay) bool { return d.practised }
func learnedMark(d listedDay) bool   { return d.learned }

// ── Givens ───────────────────────────────────────────────────────────────

var weekdays = map[string]time.Weekday{
	"Sunday": time.Sunday, "Monday": time.Monday, "Tuesday": time.Tuesday, "Wednesday": time.Wednesday,
	"Thursday": time.Thursday, "Friday": time.Friday, "Saturday": time.Saturday,
}

// daysAgo is how many days before the summary's today the named day is:
// "today" or the latest such weekday up to today.
func (w *world) daysAgo(name string) (int, error) {
	if name == "today" {
		return 0, nil
	}
	weekday, ok := weekdays[name]
	if !ok {
		return 0, fmt.Errorf("unknown day %q", name)
	}
	today := w.summaryNow.In(w.location()).Weekday()
	return (int(today) - int(weekday) + 7) % 7, nil
}

// splitDays turns "Thursday, Saturday and today" into its day names.
func splitDays(list string) []string {
	return strings.FieldsFunc(strings.ReplaceAll(list, " and ", ", "), func(r rune) bool { return r == ',' || r == ' ' })
}

// todayIs moves the summary's clock to noon on the latest such weekday up
// to the scenarios' fixed today, in the student's time zone.
func (w *world) todayIs(day, _ string) error {
	ago, err := w.daysAgo(day)
	if err != nil {
		return err
	}
	w.summaryNow = w.localTime(ago, 12, 0)
	return nil
}

func (w *world) finishedSessionsOn(name, instrument, days string) error {
	for _, day := range splitDays(days) {
		ago, err := w.daysAgo(day)
		if err != nil {
			return err
		}
		w.finishSession(name, instrument, w.localTime(ago, 9, 0))
	}
	return nil
}

// finishedOnTwoDays finishes one session with each instrument, on its day.
func (w *world) finishedOnTwoDays(name, first, firstDay, second, secondDay string) error {
	if err := w.finishedSessionsOn(name, first, firstDay); err != nil {
		return err
	}
	return w.finishedSessionsOn(name, second, secondDay)
}

func (w *world) completedOn(name, days string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	for _, day := range splitDays(days) {
		ago, err := w.daysAgo(day)
		if err != nil {
			return err
		}
		w.practiceActivity.addCompletion(studentID, w.localTime(ago, 10, 0))
	}
	return nil
}

// ── Thens ────────────────────────────────────────────────────────────────

// lastDays reads the last 7 days of whichever was read last, the summary
// or the overview.
func (w *world) lastDays() ([]listedDay, error) {
	switch resp := w.lastResp.(type) {
	case generated.GetPracticeSummary200JSONResponse:
		days := make([]listedDay, len(resp.Last7Days))
		for i, d := range resp.Last7Days {
			days[i] = listedDay{date: d.Date.String(), practised: d.Practised}
		}
		return days, nil
	case generated.GetPracticeOverview200JSONResponse:
		days := make([]listedDay, len(resp.Last7Days))
		for i, d := range resp.Last7Days {
			days[i] = listedDay{date: d.Date.String(), practised: d.Practised, learned: d.Learned}
		}
		return days, nil
	default:
		return nil, fmt.Errorf("expected a practice summary or overview, got %T (err %v)", w.lastResp, w.lastErr)
	}
}

// dateOf is the calendar date of the named day, as the API writes it.
func (w *world) dateOf(name string) (string, error) {
	ago, err := w.daysAgo(name)
	if err != nil {
		return "", err
	}
	return w.localTime(ago, 0, 0).Format(time.DateOnly), nil
}

func (w *world) listsDaysFromTo(first, last string) error {
	days, err := w.lastDays()
	if err != nil {
		return err
	}
	if len(days) != 7 {
		return fmt.Errorf("expected 7 days, got %d", len(days))
	}
	want := make([]string, 7)
	for i := range want {
		want[i] = w.localTime(6-i, 0, 0).Format(time.DateOnly)
	}
	for i, day := range days {
		if day.date != want[i] {
			return fmt.Errorf("expected the days %v, oldest first, got %+v", want, days)
		}
	}
	firstDate, err := w.dateOf(first)
	if err != nil {
		return err
	}
	lastDate, err := w.dateOf(last)
	if err != nil {
		return err
	}
	if days[0].date != firstDate || days[6].date != lastDate {
		return fmt.Errorf("expected the days to run from %s (%s) to %s (%s), got %s to %s", first, firstDate, last, lastDate, days[0].date, days[6].date)
	}
	return nil
}

func (w *world) onlyTheseDays(mark dayMark) func(string, int) error {
	return func(list string, others int) error {
		days, err := w.lastDays()
		if err != nil {
			return err
		}
		want := map[string]bool{}
		for _, name := range splitDays(list) {
			date, err := w.dateOf(name)
			if err != nil {
				return err
			}
			want[date] = true
		}
		unmarked := 0
		for _, day := range days {
			if mark(day) != want[day.date] {
				return fmt.Errorf("expected only %s marked, got %+v", list, days)
			}
			if !mark(day) {
				unmarked++
			}
		}
		if unmarked != others {
			return fmt.Errorf("expected %d unmarked days, got %d", others, unmarked)
		}
		return nil
	}
}

func (w *world) dayMarked(mark dayMark, want bool) func(string) error {
	return func(name string) error {
		days, err := w.lastDays()
		if err != nil {
			return err
		}
		date, err := w.dateOf(name)
		if err != nil {
			return err
		}
		for _, day := range days {
			if day.date == date {
				if mark(day) != want {
					return fmt.Errorf("expected %s (%s) marked %t, got %+v", name, date, want, day)
				}
				return nil
			}
		}
		return fmt.Errorf("%s (%s) is not among the last 7 days %+v", name, date, days)
	}
}

func (w *world) yesterdayNotPractised() error {
	days, err := w.lastDays()
	if err != nil {
		return err
	}
	yesterday := w.localTime(1, 0, 0).Format(time.DateOnly)
	for _, day := range days {
		if day.date == yesterday {
			if day.practised {
				return fmt.Errorf("expected yesterday (%s) not practised, got %+v", yesterday, days)
			}
			return nil
		}
	}
	return fmt.Errorf("yesterday (%s) is not among the last 7 days %+v", yesterday, days)
}

func (w *world) practisedDayCount(want int) error {
	days, err := w.lastDays()
	if err != nil {
		return err
	}
	got := 0
	for _, day := range days {
		if day.practised {
			got++
		}
	}
	if got != want {
		return fmt.Errorf("expected %d practised days, got %d: %+v", want, got, days)
	}
	return nil
}
