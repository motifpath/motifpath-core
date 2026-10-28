package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrEmailRejected means the mail server refused an email for good: sending
// the same email again would be refused the same way.
var ErrEmailRejected = errors.New("email rejected")

// EngagementTimeZone is the time zone of a student who never reported one,
// and the time zone the concierge works in.
const EngagementTimeZone = "America/Sao_Paulo"

// Local times from which each kind of engagement email may go out on its
// day. A run that comes later the same day still sends it; one that comes
// the next day doesn't.
const (
	ProgressReportHour   = 9
	ReminderHour         = 18
	ConciergeSummaryHour = 8
)

// ReminderStage says which reminder a student gets on a day of an inactive
// spell: a daily one on its first days, then the inactivity ones.
type ReminderStage string

const (
	ReminderStageDaily           ReminderStage = "daily"
	ReminderStageInactivity      ReminderStage = "inactivity"
	ReminderStageInactivityAgain ReminderStage = "inactivity_again"
)

// reminderStages are the days without activity on which a student is
// reminded, once each per inactive spell, and the reminder each day gets.
// The daily reminders stop once the inactivity ones start, and the days in
// between and after get nothing, so a student who stopped isn't emailed
// every day.
var reminderStages = map[int]ReminderStage{
	1:  ReminderStageDaily,
	2:  ReminderStageDaily,
	3:  ReminderStageInactivity,
	10: ReminderStageInactivityAgain,
}

// EmailSendKind names what an email was sent as, for the record that keeps
// each one from going out twice.
type EmailSendKind string

const (
	EmailSendProgressReport   EmailSendKind = "progress_report"
	EmailSendReminder         EmailSendKind = "inactivity_reminder"
	EmailSendConciergeSummary EmailSendKind = "concierge_summary"
)

// ConciergeRecipient stands for the concierge in the send record, which
// otherwise names a user id.
const ConciergeRecipient = "concierge"

// EmailSendClaim is one email's place in the send record: a recipient, a
// kind and the period it belongs to (a week, or a day threshold of an
// inactive spell). Only one claim per recipient, kind and period exists.
type EmailSendClaim struct {
	Recipient string
	Kind      EmailSendKind
	PeriodKey string
	At        time.Time
}

// Email is a message ready to send. OneClickUnsubscribeURL, when set, is
// what the mail client's own unsubscribe button posts to.
type Email struct {
	To                     string
	Subject                string
	Text                   string
	HTML                   string
	OneClickUnsubscribeURL string
}

// EmailLinks are the links a student email carries: the app, the two ways
// to stop that kind of email, and the email settings, where every kind can
// be turned back on.
type EmailLinks struct {
	App                 string
	UnsubscribePage     string
	OneClickUnsubscribe string
	Settings            string
}

// ProgressReportMessage is what a weekly progress report says. The week
// runs from WeekStart, a Monday at 00:00, to the following Sunday.
type ProgressReportMessage struct {
	To               string
	DisplayName      string
	Locale           string
	WeekStart        time.Time
	PracticeDays     int
	Minutes          int
	PreviousMinutes  int
	DayStreakCurrent int
	DayStreakBest    int
	SkillsUp         int
	SongsPlayed      int
	Links            EmailLinks
}

// ReminderMessage is what a reminder says: Stage names the reminder, on the
// spell's day DaysWithout.
type ReminderMessage struct {
	To          string
	DisplayName string
	Locale      string
	DaysWithout int
	Stage       ReminderStage
	Links       EmailLinks
}

// ConciergeSummaryMessage lists every student who has started learning, for
// the week from WeekStart, inactive longest first. App is the app's address,
// where the summary's mark is hosted.
type ConciergeSummaryMessage struct {
	To        string
	App       string
	WeekStart time.Time
	Rows      []ConciergeRow
}

// ConciergeRow is one student in the concierge summary. LastActive is nil
// for a student with no activity at all.
type ConciergeRow struct {
	DisplayName  string
	Role         Role
	Email        string
	PracticeDays int
	Minutes      int
	LastActive   *time.Time
	DaysInactive int
}

// WeeklyEmailWeek returns the week a Monday email reports on, when now (in
// loc) is a Monday at or after fromHour: the key of that week and its
// Monday at 00:00. ok is false at any other time.
func WeeklyEmailWeek(now time.Time, loc *time.Location, fromHour int) (key string, weekStart time.Time, ok bool) {
	local := now.In(loc)
	if local.Weekday() != time.Monday || local.Hour() < fromHour {
		return "", time.Time{}, false
	}
	y, m, d := local.Date()
	weekStart = time.Date(y, m, d-7, 0, 0, 0, 0, loc)
	year, week := weekStart.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week), weekStart, true
}

// DaysWithoutActivity counts the calendar days in loc from the day of
// lastActive to the day of now: 0 on the same day, 3 from Friday to Monday.
func DaysWithoutActivity(lastActive, now time.Time, loc *time.Location) int {
	// The local calendar dates, compared as UTC dates, so a daylight-saving
	// change in loc never shifts the count.
	return int(calendarDate(now.In(loc)).Sub(calendarDate(lastActive.In(loc))).Hours() / 24)
}

// ReminderDue reports whether a reminder is due at now for a student whose
// spell began at lastActive: only on a day that has a reminder stage, from
// ReminderHour. key names the spell and the day, so each reminder goes out
// once, and a student gets at most one a day.
func ReminderDue(lastActive, now time.Time, loc *time.Location) (key string, days int, stage ReminderStage, ok bool) {
	if now.In(loc).Hour() < ReminderHour {
		return "", 0, "", false
	}
	days = DaysWithoutActivity(lastActive, now, loc)
	stage, ok = reminderStages[days]
	if !ok {
		return "", days, "", false
	}
	spell := lastActive.In(loc).Format(time.DateOnly)
	return fmt.Sprintf("%s/day-%d", spell, days), days, stage, true
}
