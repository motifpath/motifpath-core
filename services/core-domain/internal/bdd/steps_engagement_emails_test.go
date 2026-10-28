//go:build integration

package bdd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/email"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// The engagement scenarios happen around one fixed Monday, 2 November 2026,
// whose reported week runs from Monday 26 October to Sunday 1 November.
// Weekday names in a step are days of that week; "N days ago" counts back
// from 10:00 on that Monday, in São Paulo.
var (
	engagementSaoPaulo = mustLoadLocation("America/Sao_Paulo")
	reportedWeekStart  = time.Date(2026, 10, 26, 0, 0, 0, 0, engagementSaoPaulo)
	mailerMonday       = time.Date(2026, 11, 2, 0, 0, 0, 0, engagementSaoPaulo)
)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// mailWorld is the engagement mailer's state in a scenario.
type mailWorld struct {
	now       time.Time
	concierge string
	sends     *bddEmailSendLog
	sender    *bddEmailSender
	// lastRun holds what the last "When the engagement mailer runs" sent;
	// the mailer runs that set a scenario up don't count.
	lastRun []domain.Email
}

// bddEmailSendLog is an in-memory send record.
type bddEmailSendLog struct {
	mu     sync.Mutex
	claims map[string]bool
}

func sendKey(c domain.EmailSendClaim) string {
	return c.Recipient + "|" + string(c.Kind) + "|" + c.PeriodKey
}

func (l *bddEmailSendLog) Claim(_ context.Context, c domain.EmailSendClaim) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.claims[sendKey(c)] {
		return false, nil
	}
	l.claims[sendKey(c)] = true
	return true, nil
}

func (l *bddEmailSendLog) Release(_ context.Context, c domain.EmailSendClaim) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.claims, sendKey(c))
	return nil
}

func (l *bddEmailSendLog) Claimed(_ context.Context, c domain.EmailSendClaim) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.claims[sendKey(c)], nil
}

// bddEmailSender keeps what it is given instead of delivering it.
type bddEmailSender struct {
	mu   sync.Mutex
	sent []domain.Email
}

func (s *bddEmailSender) Send(_ context.Context, e domain.Email) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, e)
	return nil
}

func registerEngagementEmailSteps(sc *godog.ScenarioContext, w *world) {
	// The clock and the configuration.
	sc.Step(`^it is (Monday|Tuesday) (\d\d):(\d\d) in "([^"]+)"$`, w.itIsWeekdayAt)
	sc.Step(`^it is (\d\d):(\d\d) in "([^"]+)"$`, func(hour, minute int, zone string) error { return w.itIsWeekdayAt("Monday", hour, minute, zone) })
	sc.Step(`^the concierge's email address is "([^"]+)"$`, func(address string) error { w.mailer().concierge = address; return nil })
	sc.Step(`^no concierge email address is configured$`, func() error { w.mailer().concierge = ""; return nil })
	sc.Step(`^the engagement mailer didn't run on Monday$`, func() error { return nil })

	// Who the students are.
	sc.Step(`^"([^"]+)" has already registered as a teacher$`, func(name string) error { w.ensureRegistered(name, domain.RoleTeacher); return nil })
	sc.Step(`^"([^"]+)" has already registered as an admin$`, func(name string) error { w.ensureRegistered(name, domain.RoleAdmin); return nil })
	sc.Step(`^"([^"]+)" is enrolled in the course "([^"]+)"$`, w.isEnrolledInCourse)
	sc.Step(`^"([^"]+)"'s only course enrolment is completed$`, w.onlyEnrolmentIsCompleted)
	sc.Step(`^"([^"]+)" has never started a course or a path$`, func(name string) error { w.ensureRegistered(name, domain.RoleStudent); return nil })
	sc.Step(`^"([^"]+)" registered (\d+) days ago$`, w.registeredDaysAgo)
	sc.Step(`^"([^"]+)" registered as a student before email addresses were stored and hasn't signed in since$`, w.registeredBeforeEmails)
	sc.Step(`^"([^"]+)" is a student enrolled in a course, whose last activity was (\d+) days ago$`, w.enrolledStudentLastActiveDaysAgo)

	// What they did.
	sc.Step(`^"([^"]+)" practised on (\d+) days of the week that ended on Sunday$`, func(name string, days int) error { return w.practisedOnDaysOfWeek(name, days, 10*days) })
	sc.Step(`^"([^"]+)" practised on (\d+) days of the week that ended on Sunday, for (\d+) minutes in all$`, w.practisedOnDaysOfWeek)
	sc.Step(`^"([^"]+)" practised on 2 days of the week that ended on Sunday, but not on Saturday or Sunday$`, w.practisedEarlyInWeekOnly)
	sc.Step(`^"([^"]+)" practised for (\d+) minutes the week before$`, w.practisedTheWeekBefore)
	sc.Step(`^"([^"]+)"'s current day streak is 2 and their best is (\d+)$`, w.bestStreakEarlier)
	sc.Step(`^"([^"]+)"'s best day streak is (\d+)$`, w.bestStreakEarlier)
	sc.Step(`^"([^"]+)" started no practice session, completed no lesson and played no song in the week that ended on Sunday$`, w.nothingInReportedWeek)
	sc.Step(`^"([^"]+)" completed a lesson on Wednesday of the week that ended on Sunday and didn't practise$`, w.completedALessonOnWednesday)
	sc.Step(`^"([^"]+)" had a day streak of (\d+) that ended on (Friday|Sunday)$`, w.streakEndingOn)
	sc.Step(`^"([^"]+)"'s last activity was on (Friday|Saturday|Sunday)$`, w.lastActiveOnWeekday)
	sc.Step(`^"([^"]+)"'s last activity was (\d+) days? ago$`, w.lastActiveDaysAgo)
	sc.Step(`^"([^"]+)"'s last activity was on Monday at (\d\d):(\d\d)$`, w.lastActiveOnMondayAt)
	sc.Step(`^"([^"]+)"'s last practice session was on Friday$`, w.lastPracticeOnFriday)
	sc.Step(`^"([^"]+)" opened a song chart on Sunday$`, func(name string) error { return w.lastActiveOnWeekday(name, "Sunday") })
	sc.Step(`^"([^"]+)" practised (\d+) days ago and had no activity since$`, w.practisedDaysAgoNothingSince)

	// What was sent before the scenario's run.
	sc.Step(`^"([^"]+)" was sent a progress report at Monday (\d\d):(\d\d) in "([^"]+)"$`, w.mailerRanAtMonday)
	sc.Step(`^"([^"]+)" was sent an? (?:inactivity|daily) reminder at Monday (\d\d):(\d\d) in "([^"]+)"$`, w.mailerRanAtMonday)
	sc.Step(`^a student summary was sent at Monday (\d\d):(\d\d) in "([^"]+)"$`, func(hour, minute int, zone string) error { return w.mailerRanAtMonday("", hour, minute, zone) })
	sc.Step(`^"([^"]+)" was sent an inactivity reminder on the 3rd day$`, func(name string) error { return w.remindedOnDays(name, 3) })
	sc.Step(`^"([^"]+)" was sent inactivity reminders on the 3rd and the 10th day$`, func(name string) error { return w.remindedOnDays(name, 3, 10) })
	sc.Step(`^"([^"]+)" was sent inactivity reminders on the 3rd and the 10th day of an earlier spell$`, w.remindedInEarlierSpell)

	sc.Step(`^the engagement mailer runs$`, w.engagementMailerRuns)
	sc.Step(`^at Monday (\d\d):(\d\d) in "([^"]+)" the next run sends "([^"]+)" a progress report$`, w.nextRunSendsReport)

	// What the run sent.
	sc.Step(`^"([^"]+)" is sent a progress report(?: at "([^"]+)")?$`, w.isSentKind(domain.EmailKindProgressReport))
	sc.Step(`^"([^"]+)" is sent an? (inactivity|daily) reminder(?: at "([^"]+)")?$`, w.isSentReminder)
	sc.Step(`^"([^"]+)" is sent a second inactivity reminder$`, w.isSentSecondReminder)
	sc.Step(`^"([^"]+)" is not sent (?:another |a |an )?(progress report|inactivity reminder|daily reminder)$`, w.isNotSentKind)
	sc.Step(`^"([^"]+)" is not sent any email$`, w.isNotSentAnyEmail)
	sc.Step(`^"([^"]+)"'s progress report shows (\d+) practice days and (\d+) minutes, up from (\d+)$`, w.reportShowsFigures)
	sc.Step(`^"([^"]+)"'s progress report shows a current day streak of (\d+) and a best of (\d+)$`, w.reportShowsStreak)
	sc.Step(`^"([^"]+)"'s progress report invites them to start a day streak and shows their best of (\d+)$`, w.reportInvitesNewStreak)
	sc.Step(`^"([^"]+)"'s progress report doesn't say a streak was lost or broken$`, w.reportNeverSaysLost)
	sc.Step(`^"([^"]+)"'s progress report is written in Portuguese \(Brazil\)$`, w.reportInPortuguese)
	sc.Step(`^"([^"]+)"'s progress report carries an unsubscribe link for progress reports$`, w.carriesUnsubscribeFor(domain.EmailKindProgressReport))
	sc.Step(`^"([^"]+)"'s (inactivity|daily) reminder carries an unsubscribe link for reminders$`, w.reminderCarriesUnsubscribe)
	sc.Step(`^"([^"]+)"'s (inactivity|daily) reminder doesn't mention a day streak$`, w.reminderNeverMentionsStreak)
	sc.Step(`^"([^"]+)"'s daily reminder is written in Portuguese \(Brazil\)$`, w.dailyReminderInPortuguese)
	sc.Step(`^a student summary is sent to "([^"]+)"$`, w.summarySentTo)
	sc.Step(`^no (?:other )?student summary is sent$`, w.noSummarySent)
	sc.Step(`^the student summary has a row for "([^"]+)"$`, w.summaryHasRowFor)
	sc.Step(`^the student summary lists "([^"]+)" before "([^"]+)"$`, w.summaryListsBefore)
	sc.Step(`^the student summary's row for "([^"]+)" shows their name, (\d+) practice days, (\d+) minutes and their last activity on Sunday$`, w.summaryRowShows)
}

func (w *world) mailer() *mailWorld {
	if w.mail == nil {
		w.mail = &mailWorld{sends: &bddEmailSendLog{claims: map[string]bool{}}, sender: &bddEmailSender{}}
	}
	return w.mail
}

// runMailer runs the engagement mailer at at and returns what that run sent.
func (w *world) runMailer(at time.Time) ([]domain.Email, error) {
	m := w.mailer()
	composer, err := email.NewComposer()
	if err != nil {
		return nil, err
	}
	service := application.NewEngagementMailerService(application.EngagementMailerDeps{
		Users: w.users, Enrollments: w.courseEnrollments, Paths: w.studentPaths, Activity: w.alpha.activity,
		Figures: w.practiceSummary, Sends: m.sends, Composer: composer, Sender: m.sender, Tokens: w.emailPreferences,
	}, application.EngagementMailerConfig{
		AppBaseURL: "https://app.motifpath.com", APIBaseURL: "https://api.motifpath.com", ConciergeAddress: m.concierge,
	}, func() time.Time { return at })
	before := len(m.sender.sent)
	if err := service.Run(context.Background()); err != nil {
		return nil, err
	}
	return append([]domain.Email(nil), m.sender.sent[before:]...), nil
}

// ── The clock ───────────────────────────────────────────────────────────────

func (w *world) itIsWeekdayAt(day string, hour, minute int, zone string) error {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return err
	}
	date := mailerMonday
	if day == "Tuesday" {
		date = date.AddDate(0, 0, 1)
	}
	w.mailer().now = time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, loc)
	return nil
}

func (w *world) engagementMailerRuns() error {
	sent, err := w.runMailer(w.mailer().now)
	w.mailer().lastRun = sent
	return err
}

func (w *world) nextRunSendsReport(hour, minute int, zone, persona string) error {
	if err := w.itIsWeekdayAt("Monday", hour, minute, zone); err != nil {
		return err
	}
	if err := w.engagementMailerRuns(); err != nil {
		return err
	}
	return w.isSentKind(domain.EmailKindProgressReport)(persona, "")
}

// mailerRanAtMonday runs the mailer at a Monday time as setup: what it sent
// is part of the scenario's past, not of its run.
func (w *world) mailerRanAtMonday(_ string, hour, minute int, zone string) error {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return err
	}
	_, err = w.runMailer(time.Date(mailerMonday.Year(), mailerMonday.Month(), mailerMonday.Day(), hour, minute, 0, 0, loc))
	return err
}

// remindedOnDays runs the mailer at 18:00 on each given day of persona's
// current inactive spell.
func (w *world) remindedOnDays(persona string, days ...int) error {
	spellStart := w.alpha.activity.lastActive[w.studentID(persona)].In(engagementSaoPaulo)
	for _, day := range days {
		at := time.Date(spellStart.Year(), spellStart.Month(), spellStart.Day()+day, 18, 0, 0, 0, engagementSaoPaulo)
		sent, err := w.runMailer(at)
		if err != nil {
			return err
		}
		if !sentKindTo(sent, w.emailOf(persona), domain.EmailKindReminders) {
			return fmt.Errorf("setup: no reminder went to %q on day %d", persona, day)
		}
	}
	return nil
}

func (w *world) remindedInEarlierSpell(persona string) error {
	w.setLastActive(persona, mailerMonday.Add(10*time.Hour).AddDate(0, 0, -30))
	return w.remindedOnDays(persona, 3, 10)
}

// ── Who the students are ────────────────────────────────────────────────────

func (w *world) isEnrolledInCourse(persona, course string) error {
	id := w.studentID(persona)
	return w.courseEnrollments.Create(context.Background(), domain.CourseEnrollment{
		ID: deterministicUUID("enrollment", persona+"/"+course).String(), StudentID: id, CourseID: deterministicUUID("course", course).String(),
		CourseTitle: course, CourseVersionNumber: 1, Status: domain.CourseEnrollmentStatusActive,
	})
}

func (w *world) onlyEnrolmentIsCompleted(persona string) error {
	enrollments, err := w.courseEnrollments.ListByStudentID(context.Background(), w.studentID(persona))
	if err != nil {
		return err
	}
	if len(enrollments) != 1 {
		return fmt.Errorf("setup: expected %q to hold one enrolment, got %d", persona, len(enrollments))
	}
	return w.courseEnrollments.Complete(context.Background(), enrollments[0].ID)
}

func (w *world) registeredDaysAgo(persona string, days int) error {
	user, err := w.users.GetByID(context.Background(), w.studentID(persona))
	if err != nil {
		return err
	}
	user.RegisteredAt = mailerMonday.Add(10*time.Hour).AddDate(0, 0, -days)
	w.users.put(user)
	return nil
}

func (w *world) enrolledStudentLastActiveDaysAgo(persona string, days int) error {
	if err := w.isEnrolledInCourse(persona, "fingerstyle-journey"); err != nil {
		return err
	}
	return w.lastActiveDaysAgo(persona, days)
}

func (w *world) emailOf(persona string) string {
	user, err := w.users.GetByID(context.Background(), w.userMotifID[persona].String())
	if err != nil || user.Email == "" {
		return w.emailClaim(persona)
	}
	return user.Email
}

// ── What they did ───────────────────────────────────────────────────────────

// weekDay is 10:00 on a day of the reported week, Monday 0 to Sunday 6.
func weekDay(offset int) time.Time {
	return reportedWeekStart.AddDate(0, 0, offset).Add(10 * time.Hour)
}

var weekdayOffsets = map[string]int{"Monday": 0, "Tuesday": 1, "Wednesday": 2, "Thursday": 3, "Friday": 4, "Saturday": 5, "Sunday": 6}

// addPracticeSession records a finished session and moves persona's last
// activity to its end, as the worker would.
func (w *world) addPracticeSession(persona string, start time.Time, minutes int) {
	end := start.Add(time.Duration(minutes) * time.Minute)
	started, ended := start, end
	w.practiceActivity.addSession(w.studentID(persona), fakePracticeSession{startedAt: &started, lastEventAt: end, endedAt: &ended})
	if end.After(w.alpha.activity.lastActive[w.studentID(persona)]) {
		w.setLastActive(persona, end)
	}
}

func (w *world) setLastActive(persona string, at time.Time) {
	w.alpha.activity.lastActive[w.studentID(persona)] = at
}

// practisedOnDaysOfWeek spreads minutes over days of the reported week: the
// weekend first, so the week ends on a 2-day streak, then from Monday on.
func (w *world) practisedOnDaysOfWeek(persona string, days, minutes int) error {
	order := []int{5, 6, 0, 1, 2, 3, 4}
	if days < 1 || days > 7 {
		return fmt.Errorf("can't practise on %d days of a week", days)
	}
	for i := range days {
		share := minutes / days
		if i < minutes%days {
			share++
		}
		w.addPracticeSession(persona, weekDay(order[i]), share)
	}
	return nil
}

func (w *world) practisedEarlyInWeekOnly(persona string) error {
	w.addPracticeSession(persona, weekDay(0), 10)
	w.addPracticeSession(persona, weekDay(1), 10)
	return nil
}

func (w *world) practisedTheWeekBefore(persona string, minutes int) error {
	w.addPracticeSession(persona, weekDay(-7), minutes)
	return nil
}

// bestStreakEarlier adds a run of days long before the reported week, so it
// counts towards the best streak and nothing else. The reported week's
// sessions already end on the current streak.
func (w *world) bestStreakEarlier(persona string, days int) error {
	for i := range days {
		w.addPracticeSession(persona, weekDay(-21+i), 5)
	}
	return nil
}

func (w *world) nothingInReportedWeek(persona string) error {
	w.addPracticeSession(persona, weekDay(-14), 10)
	w.setLastActive(persona, mailerMonday.Add(8*time.Hour))
	return nil
}

func (w *world) completedALessonOnWednesday(persona string) error {
	w.practiceActivity.addCompletion(w.studentID(persona), weekDay(2))
	w.setLastActive(persona, weekDay(2))
	return nil
}

func (w *world) streakEndingOn(persona string, days int, lastDay string) error {
	for i := range days {
		w.addPracticeSession(persona, weekDay(weekdayOffsets[lastDay]-i), 10)
	}
	return nil
}

func (w *world) lastActiveOnWeekday(persona, day string) error {
	w.setLastActive(persona, weekDay(weekdayOffsets[day]))
	return nil
}

func (w *world) lastActiveDaysAgo(persona string, days int) error {
	w.setLastActive(persona, mailerMonday.Add(10*time.Hour).AddDate(0, 0, -days))
	return nil
}

func (w *world) lastActiveOnMondayAt(persona string, hour, minute int) error {
	w.setLastActive(persona, mailerMonday.Add(time.Duration(hour)*time.Hour+time.Duration(minute)*time.Minute))
	return nil
}

func (w *world) lastPracticeOnFriday(persona string) error {
	w.addPracticeSession(persona, weekDay(4), 10)
	return nil
}

func (w *world) practisedDaysAgoNothingSince(persona string, days int) error {
	start := mailerMonday.Add(10*time.Hour).AddDate(0, 0, -days)
	w.addPracticeSession(persona, start, 10)
	w.setLastActive(persona, start)
	return nil
}

// ── What the run sent ───────────────────────────────────────────────────────

// sentKindTo reports whether sent holds an email of kind to address. A
// student email's kind is the one its unsubscribe token names.
func sentKindTo(sent []domain.Email, address string, kind domain.EmailKind) bool {
	return findKind(sent, address, kind) != nil
}

func findKind(sent []domain.Email, address string, kind domain.EmailKind) *domain.Email {
	for i, e := range sent {
		if e.To == address && strings.Contains(e.OneClickUnsubscribeURL, "."+string(kind)+".") {
			return &sent[i]
		}
	}
	return nil
}

// dailyReminderSubjects are the daily reminder's subjects in every
// language, which tell it apart from an inactivity reminder: both carry the
// reminders unsubscribe token.
var dailyReminderSubjects = map[string]bool{"Time for a little practice?": true, "Hora de praticar um pouco?": true}

// findReminder returns the daily or the inactivity reminder, as which
// names it, that sent holds for address.
func findReminder(sent []domain.Email, address, which string) *domain.Email {
	for i, e := range sent {
		if e.To == address && strings.Contains(e.OneClickUnsubscribeURL, "."+string(domain.EmailKindReminders)+".") &&
			dailyReminderSubjects[e.Subject] == (which == "daily") {
			return &sent[i]
		}
	}
	return nil
}

func (w *world) sentReminder(persona, which string) (*domain.Email, error) {
	reminder := findReminder(w.mailer().lastRun, w.emailOf(persona), which)
	if reminder == nil {
		return nil, fmt.Errorf("expected a %s reminder to %q, the run sent %s", which, persona, describeSent(w.mailer().lastRun))
	}
	return reminder, nil
}

func (w *world) isSentReminder(persona, which, address string) error {
	if address == "" {
		address = w.emailOf(persona)
	}
	if findReminder(w.mailer().lastRun, address, which) == nil {
		return fmt.Errorf("expected a %s reminder to %s, the run sent %s", which, address, describeSent(w.mailer().lastRun))
	}
	return nil
}

func (w *world) isSentKind(kind domain.EmailKind) func(string, string) error {
	return func(persona, address string) error {
		if address == "" {
			address = w.emailOf(persona)
		}
		if !sentKindTo(w.mailer().lastRun, address, kind) {
			return fmt.Errorf("expected a %s to %s, the run sent %s", kind, address, describeSent(w.mailer().lastRun))
		}
		return nil
	}
}

func (w *world) isSentSecondReminder(persona string) error {
	reminder := findReminder(w.mailer().lastRun, w.emailOf(persona), "inactivity")
	if reminder == nil || reminder.Subject != "Pick up where you left off" {
		return fmt.Errorf("expected the spell's second reminder to %q, the run sent %s", persona, describeSent(w.mailer().lastRun))
	}
	return nil
}

func (w *world) isNotSentKind(persona, what string) error {
	sent, address := w.mailer().lastRun, w.emailOf(persona)
	var found bool
	switch what {
	case "progress report":
		found = sentKindTo(sent, address, domain.EmailKindProgressReport)
	case "daily reminder":
		found = findReminder(sent, address, "daily") != nil
	default:
		found = findReminder(sent, address, "inactivity") != nil
	}
	if found {
		return fmt.Errorf("expected no %s to %q, the run sent %s", what, persona, describeSent(sent))
	}
	return nil
}

func (w *world) isNotSentAnyEmail(persona string) error {
	for _, e := range w.mailer().lastRun {
		if e.To == w.emailOf(persona) {
			return fmt.Errorf("expected no email to %q, got %q", persona, e.Subject)
		}
	}
	return nil
}

// sentReport returns the progress report the run sent to persona.
func (w *world) sentReport(persona string) (*domain.Email, error) {
	report := findKind(w.mailer().lastRun, w.emailOf(persona), domain.EmailKindProgressReport)
	if report == nil {
		return nil, fmt.Errorf("expected a progress report to %q, the run sent %s", persona, describeSent(w.mailer().lastRun))
	}
	return report, nil
}

func (w *world) reportContains(persona string, wants ...string) error {
	report, err := w.sentReport(persona)
	if err != nil {
		return err
	}
	for _, want := range wants {
		if !strings.Contains(report.Text, want) {
			return fmt.Errorf("expected %q's progress report to say %q:\n%s", persona, want, report.Text)
		}
	}
	return nil
}

func (w *world) reportShowsFigures(persona string, days, minutes, previous int) error {
	return w.reportContains(persona, fmt.Sprintf("%d of 7", days), fmt.Sprintf("%d min (the week before: %d)", minutes, previous))
}

func (w *world) reportShowsStreak(persona string, current, best int) error {
	return w.reportContains(persona, fmt.Sprintf("Day streak: %d days (best: %d)", current, best))
}

func (w *world) reportInvitesNewStreak(persona string, best int) error {
	return w.reportContains(persona, fmt.Sprintf("Practise today to start a new day streak. Your best is %d days.", best))
}

func (w *world) reportNeverSaysLost(persona string) error {
	report, err := w.sentReport(persona)
	if err != nil {
		return err
	}
	for _, part := range []string{report.Text, report.HTML} {
		lower := strings.ToLower(part)
		for _, word := range []string{"lost", "broke", "broken"} {
			if strings.Contains(lower, word) {
				return fmt.Errorf("%q's progress report says %q", persona, word)
			}
		}
	}
	return nil
}

func (w *world) reportInPortuguese(persona string) error {
	report, err := w.sentReport(persona)
	if err != nil {
		return err
	}
	if report.Subject != "Sua semana no MotifPath" {
		return fmt.Errorf("expected the Portuguese subject, got %q", report.Subject)
	}
	return nil
}

func (w *world) carriesUnsubscribeFor(kind domain.EmailKind) func(string) error {
	return func(persona string) error {
		sent := findKind(w.mailer().lastRun, w.emailOf(persona), kind)
		if sent == nil {
			return fmt.Errorf("expected a %s to %q", kind, persona)
		}
		if !strings.Contains(sent.OneClickUnsubscribeURL, "https://api.motifpath.com/email-unsubscribes?token=") {
			return fmt.Errorf("expected a one-click unsubscribe, got %q", sent.OneClickUnsubscribeURL)
		}
		if !strings.Contains(sent.Text, "https://app.motifpath.com/unsubscribe?token=") {
			return fmt.Errorf("expected an unsubscribe link in the body:\n%s", sent.Text)
		}
		return nil
	}
}

func (w *world) reminderCarriesUnsubscribe(persona, which string) error {
	if _, err := w.sentReminder(persona, which); err != nil {
		return err
	}
	return w.carriesUnsubscribeFor(domain.EmailKindReminders)(persona)
}

func (w *world) dailyReminderInPortuguese(persona string) error {
	reminder, err := w.sentReminder(persona, "daily")
	if err != nil {
		return err
	}
	if reminder.Subject != "Hora de praticar um pouco?" {
		return fmt.Errorf("expected the Portuguese subject, got %q", reminder.Subject)
	}
	return nil
}

func (w *world) reminderNeverMentionsStreak(persona, which string) error {
	reminder, err := w.sentReminder(persona, which)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(reminder.Text+reminder.HTML), "streak") {
		return fmt.Errorf("%q's reminder mentions a streak", persona)
	}
	return nil
}

// studentSummary returns the concierge summary the run sent.
func (w *world) studentSummary() (*domain.Email, error) {
	for i, e := range w.mailer().lastRun {
		if strings.HasPrefix(e.Subject, "Students, week of") {
			return &w.mailer().lastRun[i], nil
		}
	}
	return nil, fmt.Errorf("expected a student summary, the run sent %s", describeSent(w.mailer().lastRun))
}

func (w *world) summarySentTo(address string) error {
	summary, err := w.studentSummary()
	if err != nil {
		return err
	}
	if summary.To != address {
		return fmt.Errorf("expected the summary to go to %s, it went to %s", address, summary.To)
	}
	return nil
}

func (w *world) noSummarySent() error {
	if summary, err := w.studentSummary(); err == nil {
		return fmt.Errorf("expected no student summary, one went to %s", summary.To)
	}
	return nil
}

func (w *world) summaryHasRowFor(persona string) error {
	summary, err := w.studentSummary()
	if err != nil {
		return err
	}
	if !strings.Contains(summary.Text, defaultNameClaim(persona)+" <") {
		return fmt.Errorf("expected a row for %q:\n%s", persona, summary.Text)
	}
	return nil
}

func (w *world) summaryListsBefore(first, second string) error {
	summary, err := w.studentSummary()
	if err != nil {
		return err
	}
	a, b := strings.Index(summary.Text, defaultNameClaim(first)+" <"), strings.Index(summary.Text, defaultNameClaim(second)+" <")
	if a < 0 || b < 0 || a > b {
		return fmt.Errorf("expected %q before %q:\n%s", first, second, summary.Text)
	}
	return nil
}

func (w *world) summaryRowShows(persona string, days, minutes int) error {
	summary, err := w.studentSummary()
	if err != nil {
		return err
	}
	want := fmt.Sprintf("%s <%s>\n  practice days: %d, minutes: %d, last active: Nov 1", defaultNameClaim(persona), w.emailOf(persona), days, minutes)
	if !strings.Contains(summary.Text, want) {
		return fmt.Errorf("expected the summary to hold %q:\n%s", want, summary.Text)
	}
	return nil
}

func describeSent(sent []domain.Email) string {
	if len(sent) == 0 {
		return "nothing"
	}
	parts := make([]string, len(sent))
	for i, e := range sent {
		parts[i] = fmt.Sprintf("%q to %s", e.Subject, e.To)
	}
	return strings.Join(parts, ", ")
}
