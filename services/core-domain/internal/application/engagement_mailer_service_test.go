package application_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

var saoPaulo = mustLocation("America/Sao_Paulo")

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// Monday 2 November 2026 is the Monday after the week of 26 October to 1
// November.
func monday(hour, minute int, loc *time.Location) time.Time {
	return time.Date(2026, 11, 2, hour, minute, 0, 0, loc)
}

// fakeWeeklyFigures returns a fixed overview per student and records the
// moment each one was asked for.
type fakeWeeklyFigures struct {
	mu       sync.Mutex
	byID     map[string]application.PracticeOverview
	askedAt  map[string]time.Time
	askedLoc map[string]*time.Location
}

func newFakeWeeklyFigures() *fakeWeeklyFigures {
	return &fakeWeeklyFigures{byID: map[string]application.PracticeOverview{}, askedAt: map[string]time.Time{}, askedLoc: map[string]*time.Location{}}
}

func (f *fakeWeeklyFigures) OverviewAt(_ context.Context, caller domain.User, loc *time.Location, at time.Time) (application.PracticeOverview, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.askedAt[caller.ID] = at
	f.askedLoc[caller.ID] = loc
	return f.byID[caller.ID], nil
}

// fakeEmailSendLog is an in-memory ports.EmailSendLog. Like a real store,
// it fails every call made on a context that is already cancelled.
type fakeEmailSendLog struct {
	mu     sync.Mutex
	claims map[domain.EmailSendClaim]bool
}

func newFakeEmailSendLog() *fakeEmailSendLog {
	return &fakeEmailSendLog{claims: map[domain.EmailSendClaim]bool{}}
}

func claimKey(c domain.EmailSendClaim) domain.EmailSendClaim {
	c.At = time.Time{}
	return c
}

func (f *fakeEmailSendLog) Claim(ctx context.Context, c domain.EmailSendClaim) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claims[claimKey(c)] {
		return false, nil
	}
	f.claims[claimKey(c)] = true
	return true, nil
}

func (f *fakeEmailSendLog) Release(ctx context.Context, c domain.EmailSendClaim) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.claims, claimKey(c))
	return nil
}

func (f *fakeEmailSendLog) Claimed(ctx context.Context, c domain.EmailSendClaim) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.claims[claimKey(c)], nil
}

// fakeEmailComposer turns each message into an Email whose subject names
// its kind, and keeps the message it was given.
type fakeEmailComposer struct {
	mu        sync.Mutex
	reports   []domain.ProgressReportMessage
	reminders []domain.ReminderMessage
	summaries []domain.ConciergeSummaryMessage
}

func (f *fakeEmailComposer) ProgressReport(m domain.ProgressReportMessage) (domain.Email, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, m)
	return domain.Email{To: m.To, Subject: "progress report", OneClickUnsubscribeURL: m.Links.OneClickUnsubscribe}, nil
}

func (f *fakeEmailComposer) Reminder(m domain.ReminderMessage) (domain.Email, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reminders = append(f.reminders, m)
	return domain.Email{To: m.To, Subject: "reminder", OneClickUnsubscribeURL: m.Links.OneClickUnsubscribe}, nil
}

func (f *fakeEmailComposer) ConciergeSummary(m domain.ConciergeSummaryMessage) (domain.Email, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.summaries = append(f.summaries, m)
	return domain.Email{To: m.To, Subject: "concierge summary"}, nil
}

// fakeEmailSender records what it sent. It fails for the addresses in
// failFor, has the server refuse for good the ones in rejectFor, and calls
// interruptFor's function for an address mid-send, as a shutdown would.
type fakeEmailSender struct {
	mu           sync.Mutex
	sent         []domain.Email
	failFor      map[string]bool
	rejectFor    map[string]bool
	interruptFor map[string]func()
}

func (f *fakeEmailSender) Send(ctx context.Context, email domain.Email) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if interrupt := f.interruptFor[email.To]; interrupt != nil {
		interrupt()
		return ctx.Err()
	}
	if f.failFor[email.To] {
		return errors.New("smtp: 451 try again later")
	}
	if f.rejectFor[email.To] {
		return fmt.Errorf("smtp: 554 access denied: %w", domain.ErrEmailRejected)
	}
	f.sent = append(f.sent, email)
	return nil
}

func (f *fakeEmailSender) sentTo(to, subject string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, e := range f.sent {
		if e.To == to && e.Subject == subject {
			count++
		}
	}
	return count
}

type mailerFixture struct {
	users       *fakeUserRepository
	enrollments *fakeCourseEnrollmentRepository
	paths       *fakeStudentPathRepository
	activity    *fakeAlphaActivityReader
	figures     *fakeWeeklyFigures
	sends       *fakeEmailSendLog
	composer    *fakeEmailComposer
	sender      *fakeEmailSender
	now         time.Time
	concierge   string
	only        string
	resend      bool
}

func newMailerFixture() *mailerFixture {
	return &mailerFixture{
		users:       newFakeUserRepository(),
		enrollments: newFakeCourseEnrollmentRepository(),
		paths:       newFakeStudentPathRepository(),
		activity:    newFakeAlphaActivityReader(),
		figures:     newFakeWeeklyFigures(),
		sends:       newFakeEmailSendLog(),
		composer:    &fakeEmailComposer{},
		sender:      &fakeEmailSender{failFor: map[string]bool{}, rejectFor: map[string]bool{}, interruptFor: map[string]func(){}},
		concierge:   "concierge@motifpath.com",
	}
}

func (f *mailerFixture) service() *application.EngagementMailerService {
	return application.NewEngagementMailerService(application.EngagementMailerDeps{
		Users:       f.users,
		Enrollments: f.enrollments,
		Paths:       f.paths,
		Activity:    f.activity,
		Figures:     f.figures,
		Sends:       f.sends,
		Composer:    f.composer,
		Sender:      f.sender,
		Tokens:      application.NewEmailPreferencesService(f.users, unsubscribeSecret),
	}, application.EngagementMailerConfig{
		AppBaseURL:       "https://app.motifpath.com",
		APIBaseURL:       "https://api.motifpath.com",
		ConciergeAddress: f.concierge,
		OnlyRecipient:    f.only,
		Resend:           f.resend,
	}, func() time.Time { return f.now })
}

func (f *mailerFixture) run(t *testing.T) {
	t.Helper()
	require.NoError(t, f.service().Run(context.Background()))
}

type studentOption func(*domain.User)

func withTimeZone(zone string) studentOption { return func(u *domain.User) { u.TimeZone = zone } }
func withoutEmail() studentOption            { return func(u *domain.User) { u.Email = "" } }
func withPrefs(report, reminders bool) studentOption {
	return func(u *domain.User) {
		u.EmailPreferences = domain.EmailPreferences{ProgressReport: report, Reminders: reminders}
	}
}
func withRole(role domain.Role) studentOption { return func(u *domain.User) { u.Role = role } }
func registeredAt(at time.Time) studentOption { return func(u *domain.User) { u.RegisteredAt = at } }

// addStudent registers a student who has started a course, with an address,
// both preferences on and no time zone, and returns their id.
func (f *mailerFixture) addStudent(t *testing.T, name string, opts ...studentOption) string {
	t.Helper()
	id := "id-" + name
	user := domain.User{
		ID: id, ClerkUserID: "clerk-" + name, Role: domain.RoleStudent, DisplayName: name,
		Locale: domain.Language{Code: "en"}, Email: name + "@example.com",
		EmailPreferences: domain.DefaultEmailPreferences(), RegisteredAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
	}
	for _, opt := range opts {
		opt(&user)
	}
	f.users.put(user)
	require.NoError(t, f.enrollments.Create(context.Background(), domain.CourseEnrollment{ID: "enrollment-" + name, StudentID: id, CourseID: "course-1", Status: domain.CourseEnrollmentStatusActive}))
	return id
}

func somethingToShow() application.PracticeOverview {
	return application.PracticeOverview{PracticeDaysLast7: 4, MinutesPractisedLast7: 75, MinutesPractisedPrevious7: 50, DayStreakCurrent: 2, DayStreakBest: 6, SessionsStartedLast7: 5}
}

func TestEngagementMailerService_ProgressReport(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		opts     []studentOption
		figures  application.PracticeOverview
		wantSent bool
	}{
		{name: "sent on Monday from 09:00 when the week has something to show", now: monday(9, 0, saoPaulo), figures: somethingToShow(), wantSent: true},
		{name: "not sent before 09:00 on Monday", now: monday(8, 59, saoPaulo), figures: somethingToShow()},
		{name: "not sent on Tuesday, even when Monday was missed", now: monday(9, 0, saoPaulo).AddDate(0, 0, 1), figures: somethingToShow()},
		{name: "not sent for a week with nothing to show", now: monday(9, 0, saoPaulo), figures: application.PracticeOverview{DayStreakBest: 6, MinutesPractisedPrevious7: 40}},
		{name: "sent for a week with only a completed lesson", now: monday(9, 0, saoPaulo), figures: application.PracticeOverview{LearningDaysLast7: 1}, wantSent: true},
		{name: "sent for a week with only a song played", now: monday(9, 0, saoPaulo), figures: application.PracticeOverview{SongsPlayedLast7: 1}, wantSent: true},
		{name: "sent for a week with only a session started", now: monday(9, 0, saoPaulo), figures: application.PracticeOverview{SessionsStartedLast7: 1}, wantSent: true},
		{name: "not sent when progress reports are off", now: monday(9, 0, saoPaulo), opts: []studentOption{withPrefs(false, true)}, figures: somethingToShow()},
		{name: "not sent to a student with no address", now: monday(9, 0, saoPaulo), opts: []studentOption{withoutEmail()}, figures: somethingToShow()},
		{name: "sent to a teacher who is learning", now: monday(9, 0, saoPaulo), opts: []studentOption{withRole(domain.RoleTeacher)}, figures: somethingToShow(), wantSent: true},
		{name: "sent to an admin who is learning", now: monday(9, 0, saoPaulo), opts: []studentOption{withRole(domain.RoleAdmin)}, figures: somethingToShow(), wantSent: true},
		{name: "waits for 09:00 in the student's own time zone", now: monday(8, 0, mustLocation("Europe/Lisbon")), opts: []studentOption{withTimeZone("Europe/Lisbon")}, figures: somethingToShow()},
		{name: "sent at 09:00 in the student's own time zone", now: monday(9, 0, mustLocation("Europe/Lisbon")), opts: []studentOption{withTimeZone("Europe/Lisbon")}, figures: somethingToShow(), wantSent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMailerFixture()
			f.concierge = ""
			id := f.addStudent(t, "alice", tt.opts...)
			f.figures.byID[id] = tt.figures
			f.activity.lastActive[id] = tt.now.Add(-time.Hour)
			f.now = tt.now

			f.run(t)

			want := 0
			if tt.wantSent {
				want = 1
			}
			assert.Equal(t, want, f.sender.sentTo("alice@example.com", "progress report"))
		})
	}
}

func TestEngagementMailerService_ProgressReport_Content(t *testing.T) {
	f := newMailerFixture()
	id := f.addStudent(t, "alice")
	f.figures.byID[id] = somethingToShow()
	f.activity.lastActive[id] = monday(8, 0, saoPaulo)
	f.now = monday(9, 30, saoPaulo)

	f.run(t)

	require.Len(t, f.composer.reports, 1)
	report := f.composer.reports[0]
	assert.Equal(t, "alice@example.com", report.To)
	assert.Equal(t, "en", report.Locale)
	assert.Equal(t, time.Date(2026, 10, 26, 0, 0, 0, 0, saoPaulo), report.WeekStart, "the week reported is Monday to Sunday")
	assert.Equal(t, 4, report.PracticeDays)
	assert.Equal(t, 75, report.Minutes)
	assert.Equal(t, 50, report.PreviousMinutes)
	assert.Equal(t, 2, report.DayStreakCurrent)
	assert.Equal(t, 6, report.DayStreakBest)
	assert.Equal(t, monday(0, 0, saoPaulo).Add(-time.Nanosecond), f.figures.askedAt[id], "figures are taken as of the end of Sunday")
	assert.Equal(t, saoPaulo.String(), f.figures.askedLoc[id].String(), "a student with no time zone is counted in São Paulo")
	assert.Contains(t, report.Links.OneClickUnsubscribe, "https://api.motifpath.com/email-unsubscribes?token=")
	assert.Contains(t, report.Links.UnsubscribePage, "https://app.motifpath.com/unsubscribe?token=")
	assert.Equal(t, "https://app.motifpath.com", report.Links.App)
}

func TestEngagementMailerService_ProgressReport_OncePerWeek(t *testing.T) {
	f := newMailerFixture()
	id := f.addStudent(t, "alice")
	f.figures.byID[id] = somethingToShow()

	for _, at := range []time.Time{monday(9, 0, saoPaulo), monday(9, 15, saoPaulo), monday(23, 45, saoPaulo)} {
		f.now = at
		f.run(t)
	}

	assert.Equal(t, 1, f.sender.sentTo("alice@example.com", "progress report"))
}

func TestEngagementMailerService_Reminder(t *testing.T) {
	friday := time.Date(2026, 10, 30, 20, 0, 0, 0, saoPaulo)
	tests := []struct {
		name       string
		lastActive *time.Time
		now        time.Time
		opts       []studentOption
		wantStage  domain.ReminderStage
	}{
		{name: "none on a day the student was already active", lastActive: ptrTime(monday(12, 0, saoPaulo)), now: monday(18, 0, saoPaulo)},
		{name: "a daily one from 18:00 on the 1st day without activity", lastActive: ptrTime(friday.AddDate(0, 0, 2)), now: monday(18, 0, saoPaulo), wantStage: domain.ReminderStageDaily},
		{name: "no daily one before 18:00", lastActive: ptrTime(friday.AddDate(0, 0, 2)), now: monday(17, 59, saoPaulo)},
		{name: "a daily one on the 2nd day", lastActive: ptrTime(friday.AddDate(0, 0, 1)), now: monday(18, 0, saoPaulo), wantStage: domain.ReminderStageDaily},
		{name: "an inactivity one from 18:00 on the 3rd day", lastActive: &friday, now: monday(18, 0, saoPaulo), wantStage: domain.ReminderStageInactivity},
		{name: "not sent before 18:00 on the 3rd day", lastActive: &friday, now: monday(17, 59, saoPaulo)},
		{name: "none on the 4th day, a missed 3rd day is skipped", lastActive: ptrTime(friday.AddDate(0, 0, -1)), now: monday(18, 0, saoPaulo)},
		{name: "none on the 5th day", lastActive: ptrTime(friday.AddDate(0, 0, -2)), now: monday(18, 0, saoPaulo)},
		{name: "a second inactivity one on the 10th day", lastActive: ptrTime(monday(10, 0, saoPaulo).AddDate(0, 0, -10)), now: monday(18, 0, saoPaulo), wantStage: domain.ReminderStageInactivityAgain},
		{name: "none on the 20th day", lastActive: ptrTime(monday(10, 0, saoPaulo).AddDate(0, 0, -20)), now: monday(18, 0, saoPaulo)},
		{name: "not sent when reminders are off", lastActive: &friday, now: monday(18, 0, saoPaulo), opts: []studentOption{withPrefs(true, false)}},
		{name: "no daily one when reminders are off", lastActive: ptrTime(friday.AddDate(0, 0, 2)), now: monday(18, 0, saoPaulo), opts: []studentOption{withPrefs(true, false)}},
		{name: "not sent to a student with no address", lastActive: &friday, now: monday(18, 0, saoPaulo), opts: []studentOption{withoutEmail()}},
		{
			name: "a student with no activity at all counts from registration", now: monday(18, 0, saoPaulo),
			opts: []studentOption{registeredAt(friday)}, wantStage: domain.ReminderStageInactivity,
		},
		{
			name: "counted in the student's own time zone", lastActive: ptrTime(time.Date(2026, 10, 30, 23, 30, 0, 0, saoPaulo)), now: monday(18, 0, mustLocation("Asia/Tokyo")),
			opts: []studentOption{withTimeZone("Asia/Tokyo")}, wantStage: domain.ReminderStageDaily,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMailerFixture()
			f.concierge = ""
			id := f.addStudent(t, "alice", tt.opts...)
			if tt.lastActive != nil {
				f.activity.lastActive[id] = *tt.lastActive
			}
			f.now = tt.now

			f.run(t)

			if tt.wantStage == "" {
				assert.Zero(t, f.sender.sentTo("alice@example.com", "reminder"))
				return
			}
			assert.Equal(t, 1, f.sender.sentTo("alice@example.com", "reminder"))
			require.Len(t, f.composer.reminders, 1)
			assert.Equal(t, tt.wantStage, f.composer.reminders[0].Stage)
			assert.Contains(t, f.composer.reminders[0].Links.OneClickUnsubscribe, "token=")
			assert.Equal(t, "https://app.motifpath.com/settings/emails", f.composer.reminders[0].Links.Settings)
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestEngagementMailerService_Reminder_OncePerDayAndSpell(t *testing.T) {
	friday := time.Date(2026, 10, 30, 20, 0, 0, 0, saoPaulo)
	f := newMailerFixture()
	f.concierge = ""
	id := f.addStudent(t, "alice")
	f.activity.lastActive[id] = friday

	f.now = monday(18, 0, saoPaulo)
	f.run(t)
	f.now = monday(19, 0, saoPaulo)
	f.run(t)
	assert.Equal(t, 1, f.sender.sentTo("alice@example.com", "reminder"))

	// The student comes back on Tuesday, then stops again: a new spell.
	f.activity.lastActive[id] = monday(10, 0, saoPaulo).AddDate(0, 0, 1)
	f.now = monday(18, 0, saoPaulo).AddDate(0, 0, 4)
	f.run(t)
	assert.Equal(t, 2, f.sender.sentTo("alice@example.com", "reminder"))
}

func TestEngagementMailerService_Reminder_AtMostOneADay(t *testing.T) {
	sunday := time.Date(2026, 11, 1, 20, 0, 0, 0, saoPaulo)
	f := newMailerFixture()
	f.concierge = ""
	id := f.addStudent(t, "alice")
	f.activity.lastActive[id] = sunday

	// Every evening of a spell, run three times each: days 1, 2, 3 and 10
	// get one reminder each, every other day none.
	for day := 1; day <= 12; day++ {
		for _, hour := range []int{18, 19, 23} {
			f.now = time.Date(2026, 11, 1+day, hour, 0, 0, 0, saoPaulo)
			f.run(t)
		}
	}

	stages := make([]domain.ReminderStage, len(f.composer.reminders))
	days := make([]int, len(f.composer.reminders))
	for i, r := range f.composer.reminders {
		stages[i], days[i] = r.Stage, r.DaysWithout
	}
	assert.Equal(t, []domain.ReminderStage{domain.ReminderStageDaily, domain.ReminderStageDaily, domain.ReminderStageInactivity, domain.ReminderStageInactivityAgain}, stages)
	assert.Equal(t, []int{1, 2, 3, 10}, days)
	assert.Equal(t, 4, f.sender.sentTo("alice@example.com", "reminder"))
}

func TestEngagementMailerService_NeverStartedStudentsGetNothing(t *testing.T) {
	f := newMailerFixture()
	f.users.put(domain.User{
		ID: "id-bruno", ClerkUserID: "clerk-bruno", Role: domain.RoleStudent, DisplayName: "Bruno", Email: "bruno@example.com",
		Locale: domain.Language{Code: "en"}, EmailPreferences: domain.DefaultEmailPreferences(), RegisteredAt: monday(9, 0, saoPaulo).AddDate(0, 0, -3),
	})
	f.figures.byID["id-bruno"] = somethingToShow()

	for _, at := range []time.Time{monday(8, 0, saoPaulo), monday(9, 0, saoPaulo), monday(18, 0, saoPaulo)} {
		f.now = at
		f.run(t)
	}

	for _, e := range f.sender.sent {
		assert.NotEqual(t, "bruno@example.com", e.To)
	}
	for _, s := range f.composer.summaries {
		for _, row := range s.Rows {
			assert.NotEqual(t, "Bruno", row.DisplayName, "a student who never started is not in the concierge summary")
		}
	}
}

func TestEngagementMailerService_OnlyRecipient(t *testing.T) {
	friday := time.Date(2026, 10, 30, 20, 0, 0, 0, saoPaulo)
	f := newMailerFixture()
	for _, name := range []string{"alice", "bruno"} {
		id := f.addStudent(t, name)
		f.activity.lastActive[id] = friday
		f.figures.byID[id] = somethingToShow()
	}
	f.only = "ALICE@example.com "

	for _, at := range []time.Time{monday(8, 0, saoPaulo), monday(9, 0, saoPaulo), monday(18, 0, saoPaulo)} {
		f.now = at
		f.run(t)
	}

	assert.Equal(t, 1, f.sender.sentTo("alice@example.com", "progress report"))
	assert.Equal(t, 1, f.sender.sentTo("alice@example.com", "reminder"))
	assert.Zero(t, f.sender.sentTo("bruno@example.com", "progress report"), "everyone else is left alone")
	assert.Zero(t, f.sender.sentTo("bruno@example.com", "reminder"))
	assert.Zero(t, f.sender.sentTo("concierge@motifpath.com", "concierge summary"), "the summary covers everyone, so it is left out")
}

func TestEngagementMailerService_OnlyTheConcierge(t *testing.T) {
	f := newMailerFixture()
	for _, name := range []string{"alice", "bruno"} {
		id := f.addStudent(t, name)
		f.activity.lastActive[id] = monday(7, 0, saoPaulo)
		f.figures.byID[id] = somethingToShow()
	}
	f.only = "concierge@motifpath.com"
	f.now = monday(9, 0, saoPaulo)

	f.run(t)

	assert.Equal(t, 1, f.sender.sentTo("concierge@motifpath.com", "concierge summary"))
	require.Len(t, f.composer.summaries, 1)
	assert.Len(t, f.composer.summaries[0].Rows, 2, "the summary still lists every learner")
	assert.Zero(t, f.sender.sentTo("alice@example.com", "progress report"), "no student email goes out")
}

func TestEngagementMailerService_Resend(t *testing.T) {
	f := newMailerFixture()
	id := f.addStudent(t, "alice")
	f.activity.lastActive[id] = monday(7, 0, saoPaulo)
	f.figures.byID[id] = somethingToShow()
	f.now = monday(9, 0, saoPaulo)
	f.run(t)
	require.Equal(t, 1, f.sender.sentTo("alice@example.com", "progress report"))
	require.Equal(t, 1, f.sender.sentTo("concierge@motifpath.com", "concierge summary"))
	recorded := len(f.sends.claims)

	f.resend = true
	f.only = "alice@example.com"
	f.run(t)
	f.run(t)
	assert.Equal(t, 3, f.sender.sentTo("alice@example.com", "progress report"), "each resend pass sends it again")

	f.only = "concierge@motifpath.com"
	f.run(t)
	assert.Equal(t, 2, f.sender.sentTo("concierge@motifpath.com", "concierge summary"))
	assert.Len(t, f.sends.claims, recorded, "a resend isn't recorded, so the schedule is left as it was")
}

func TestEngagementMailerService_ResendRecordsNothingForANewEmail(t *testing.T) {
	f := newMailerFixture()
	id := f.addStudent(t, "alice")
	f.activity.lastActive[id] = monday(7, 0, saoPaulo)
	f.figures.byID[id] = somethingToShow()
	f.now = monday(9, 0, saoPaulo)
	f.resend = true
	f.only = "alice@example.com"
	f.run(t)
	require.Equal(t, 1, f.sender.sentTo("alice@example.com", "progress report"))

	// The real pass later that morning still sends it: the resend left no record.
	f.resend, f.only = false, ""
	f.now = monday(10, 0, saoPaulo)
	f.run(t)
	assert.Equal(t, 2, f.sender.sentTo("alice@example.com", "progress report"))
}

func TestEngagementMailerService_EveryRoleThatLearns(t *testing.T) {
	friday := time.Date(2026, 10, 30, 20, 0, 0, 0, saoPaulo)
	f := newMailerFixture()
	learning := map[string]domain.Role{"tomas": domain.RoleTeacher, "marina": domain.RoleAdmin}
	for name, role := range learning {
		id := f.addStudent(t, name, withRole(role))
		f.activity.lastActive[id] = friday
	}
	// A teacher who only authors has started nothing.
	f.users.put(domain.User{
		ID: "id-helena", ClerkUserID: "clerk-helena", Role: domain.RoleTeacher, DisplayName: "helena", Email: "helena@example.com",
		Locale: domain.Language{Code: "en"}, EmailPreferences: domain.DefaultEmailPreferences(), RegisteredAt: friday.AddDate(0, 0, -30),
	})

	for _, at := range []time.Time{monday(8, 0, saoPaulo), monday(18, 0, saoPaulo)} {
		f.now = at
		f.run(t)
	}

	assert.Equal(t, 1, f.sender.sentTo("tomas@example.com", "reminder"))
	assert.Equal(t, 1, f.sender.sentTo("marina@example.com", "reminder"))
	for _, e := range f.sender.sent {
		assert.NotEqual(t, "helena@example.com", e.To, "a teacher who never started learning gets nothing")
	}
	require.Len(t, f.composer.summaries, 1)
	var listed []string
	for _, row := range f.composer.summaries[0].Rows {
		listed = append(listed, row.DisplayName)
	}
	assert.ElementsMatch(t, []string{"tomas", "marina"}, listed, "the concierge sees every learner, whatever their role")
	roles := map[string]domain.Role{}
	for _, row := range f.composer.summaries[0].Rows {
		roles[row.DisplayName] = row.Role
	}
	assert.Equal(t, map[string]domain.Role{"tomas": domain.RoleTeacher, "marina": domain.RoleAdmin}, roles, "each row says the learner's role")
}

func TestEngagementMailerService_StartedThroughAStandalonePathOrAFinishedCourse(t *testing.T) {
	friday := time.Date(2026, 10, 30, 20, 0, 0, 0, saoPaulo)
	f := newMailerFixture()
	f.concierge = ""
	for _, name := range []string{"carla", "dani"} {
		f.users.put(domain.User{
			ID: "id-" + name, ClerkUserID: "clerk-" + name, Role: domain.RoleStudent, DisplayName: name, Email: name + "@example.com",
			Locale: domain.Language{Code: "en"}, EmailPreferences: domain.DefaultEmailPreferences(),
		})
		f.activity.lastActive["id-"+name] = friday
	}
	archived := friday.AddDate(0, -1, 0)
	require.NoError(t, f.paths.Create(context.Background(), domain.StudentPath{ID: "path-carla", StudentID: "id-carla", ArchivedAt: &archived}))
	require.NoError(t, f.enrollments.Create(context.Background(), domain.CourseEnrollment{ID: "enrollment-dani", StudentID: "id-dani", CourseID: "course-1", Status: domain.CourseEnrollmentStatusCompleted}))
	f.now = monday(18, 0, saoPaulo)

	f.run(t)

	assert.Equal(t, 1, f.sender.sentTo("carla@example.com", "reminder"))
	assert.Equal(t, 1, f.sender.sentTo("dani@example.com", "reminder"))
}

func TestEngagementMailerService_ConciergeSummary(t *testing.T) {
	f := newMailerFixture()
	alice := f.addStudent(t, "alice", withPrefs(false, false))
	dani := f.addStudent(t, "dani", withoutEmail())
	f.figures.byID[alice] = somethingToShow()
	f.activity.lastActive[alice] = monday(10, 0, saoPaulo).AddDate(0, 0, -1)
	f.activity.lastActive[dani] = monday(10, 0, saoPaulo).AddDate(0, 0, -12)
	f.now = monday(8, 0, saoPaulo)

	f.run(t)

	require.Len(t, f.composer.summaries, 1)
	summary := f.composer.summaries[0]
	assert.Equal(t, "concierge@motifpath.com", summary.To)
	assert.Equal(t, "https://app.motifpath.com", summary.App, "the summary's mark is hosted on the app")
	assert.Equal(t, time.Date(2026, 10, 26, 0, 0, 0, 0, saoPaulo), summary.WeekStart)
	require.Len(t, summary.Rows, 2, "students who turned their own emails off, or have no address, are still listed")
	assert.Equal(t, "dani", summary.Rows[0].DisplayName, "the student inactive longest comes first")
	assert.Equal(t, "alice", summary.Rows[1].DisplayName)
	assert.Equal(t, 4, summary.Rows[1].PracticeDays)
	assert.Equal(t, 75, summary.Rows[1].Minutes)
	require.NotNil(t, summary.Rows[1].LastActive)
	assert.Equal(t, 1, summary.Rows[1].DaysInactive)
	assert.Equal(t, 1, f.sender.sentTo("concierge@motifpath.com", "concierge summary"))
}

func TestEngagementMailerService_ConciergeSummary_Schedule(t *testing.T) {
	tests := []struct {
		name      string
		concierge string
		runs      []time.Time
		want      int
	}{
		{name: "not before 08:00 on Monday in São Paulo", concierge: "concierge@motifpath.com", runs: []time.Time{monday(7, 59, saoPaulo)}, want: 0},
		{name: "once per week however often the mailer runs", concierge: "concierge@motifpath.com", runs: []time.Time{monday(8, 0, saoPaulo), monday(9, 0, saoPaulo), monday(12, 0, saoPaulo)}, want: 1},
		{name: "never without a concierge address", concierge: "", runs: []time.Time{monday(8, 0, saoPaulo)}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMailerFixture()
			f.concierge = tt.concierge
			f.addStudent(t, "alice")
			for _, at := range tt.runs {
				f.now = at
				f.run(t)
			}
			assert.Len(t, f.composer.summaries, tt.want)
		})
	}
}

func TestEngagementMailerService_SendFailure(t *testing.T) {
	f := newMailerFixture()
	f.concierge = ""
	alice := f.addStudent(t, "alice")
	bob := f.addStudent(t, "bob")
	f.figures.byID[alice] = somethingToShow()
	f.figures.byID[bob] = somethingToShow()
	f.sender.failFor["alice@example.com"] = true
	f.now = monday(9, 0, saoPaulo)

	err := f.service().Run(context.Background())

	require.Error(t, err, "a failed send is reported")
	assert.Equal(t, 1, f.sender.sentTo("bob@example.com", "progress report"), "one student's failure doesn't stop the others")

	// The failed send's claim was released, so the next run retries it.
	f.sender.failFor["alice@example.com"] = false
	f.now = monday(9, 15, saoPaulo)
	f.run(t)
	assert.Equal(t, 1, f.sender.sentTo("alice@example.com", "progress report"))
	assert.Equal(t, 1, f.sender.sentTo("bob@example.com", "progress report"))
}

func TestEngagementMailerService_RejectedEmail(t *testing.T) {
	f := newMailerFixture()
	f.concierge = ""
	alice := f.addStudent(t, "alice")
	f.figures.byID[alice] = somethingToShow()
	f.sender.rejectFor["alice@example.com"] = true
	f.now = monday(9, 0, saoPaulo)

	err := f.service().Run(context.Background())

	require.ErrorIs(t, err, domain.ErrEmailRejected, "the refusal is reported once")

	// The server refused that email for good, so later runs don't try it
	// again, nor report it again.
	f.sender.rejectFor["alice@example.com"] = false
	f.now = monday(9, 15, saoPaulo)
	f.run(t)
	assert.Zero(t, f.sender.sentTo("alice@example.com", "progress report"))
}

func TestEngagementMailerService_InterruptedSend(t *testing.T) {
	f := newMailerFixture()
	f.concierge = ""
	alice := f.addStudent(t, "alice")
	f.figures.byID[alice] = somethingToShow()
	ctx, cancel := context.WithCancel(context.Background())
	f.sender.interruptFor["alice@example.com"] = cancel
	f.now = monday(9, 0, saoPaulo)

	err := f.service().Run(ctx)

	require.ErrorIs(t, err, context.Canceled)

	// A pass stopped mid-send still gives the claim back, so the next pass
	// sends the email.
	delete(f.sender.interruptFor, "alice@example.com")
	f.now = monday(9, 15, saoPaulo)
	f.run(t)
	assert.Equal(t, 1, f.sender.sentTo("alice@example.com", "progress report"))
}
