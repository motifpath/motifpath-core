package application

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// WeeklyFigures reads a student's practice overview as it stood at a given
// moment.
type WeeklyFigures interface {
	OverviewAt(ctx context.Context, caller domain.User, loc *time.Location, at time.Time) (PracticeOverview, error)
}

// EngagementMailerDeps are what the engagement mailer reads and writes.
type EngagementMailerDeps struct {
	Users       ports.UserRepository
	Enrollments ports.CourseEnrollmentRepository
	Paths       ports.StudentPathRepository
	Activity    ports.LastActivityReader
	Figures     WeeklyFigures
	Sends       ports.EmailSendLog
	Composer    ports.EmailComposer
	Sender      ports.EmailSender
	Tokens      *EmailPreferencesService
}

// EngagementMailerConfig holds where the emails' links point and who the
// concierge is. An empty ConciergeAddress sends no concierge summary.
type EngagementMailerConfig struct {
	AppBaseURL       string
	APIBaseURL       string
	ConciergeAddress string
}

// EngagementMailerService decides, each time it runs, which engagement
// emails are due and sends them: a weekly progress report and reminders to
// students, in each student's own time zone, and a weekly
// summary of every student to the concierge. Every send is claimed in the
// send record first, so an email goes out at most once however often the
// service runs; a send that fails gives its claim back, so a later run on
// the same day tries again.
type EngagementMailerService struct {
	deps            EngagementMailerDeps
	cfg             EngagementMailerConfig
	defaultLocation *time.Location
	now             func() time.Time
}

func NewEngagementMailerService(deps EngagementMailerDeps, cfg EngagementMailerConfig, now func() time.Time) *EngagementMailerService {
	loc, err := time.LoadLocation(domain.EngagementTimeZone)
	if err != nil {
		panic(err) // embedded by time/tzdata, so it is always found
	}
	return &EngagementMailerService{deps: deps, cfg: cfg, defaultLocation: loc, now: now}
}

// Run sends every engagement email due now. One email's failure doesn't
// stop the others: Run carries on and returns every failure joined.
func (s *EngagementMailerService) Run(ctx context.Context) error {
	now := s.now()
	students, err := s.startedStudents(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, len(students))
	for i, student := range students {
		ids[i] = student.ID
	}
	lastActive, err := s.deps.Activity.LastActive(ctx, ids)
	if err != nil {
		return err
	}

	var errs []error
	for _, student := range students {
		loc := s.location(student)
		errs = append(errs, s.sendProgressReport(ctx, student, loc, now))
		errs = append(errs, s.sendReminder(ctx, student, loc, s.spellStart(student, lastActive), now))
	}
	errs = append(errs, s.sendConciergeSummary(ctx, students, lastActive, now))
	return errors.Join(errs...)
}

// learningRoles are the roles whose users may learn: teachers and admins
// learn too, and get the same emails as students once they do.
var learningRoles = []domain.Role{domain.RoleStudent, domain.RoleTeacher, domain.RoleAdmin}

// startedStudents lists the users of any role who have started a course or
// a path, active or not. A user who never started anything is still in the
// first run, or only authors, and gets no engagement email.
func (s *EngagementMailerService) startedStudents(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	for _, role := range learningRoles {
		withRole, err := s.deps.Users.ListByRole(ctx, role)
		if err != nil {
			return nil, err
		}
		users = append(users, withRole...)
	}
	started := make([]domain.User, 0, len(users))
	for _, user := range users {
		ok, err := s.hasStarted(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			started = append(started, user)
		}
	}
	slices.SortFunc(started, func(a, b domain.User) int { return cmp.Compare(a.ID, b.ID) })
	return started, nil
}

func (s *EngagementMailerService) hasStarted(ctx context.Context, studentID string) (bool, error) {
	enrollments, err := s.deps.Enrollments.ListByStudentID(ctx, studentID)
	if err != nil || len(enrollments) > 0 {
		return len(enrollments) > 0, err
	}
	paths, err := s.deps.Paths.ListStandaloneByStudentID(ctx, studentID)
	return len(paths) > 0, err
}

func (s *EngagementMailerService) sendProgressReport(ctx context.Context, student domain.User, loc *time.Location, now time.Time) error {
	if !student.EmailPreferences.ProgressReport || student.Email == "" {
		return nil
	}
	key, weekStart, due := domain.WeeklyEmailWeek(now, loc, domain.ProgressReportHour)
	if !due {
		return nil
	}
	claim := domain.EmailSendClaim{Recipient: student.ID, Kind: domain.EmailSendProgressReport, PeriodKey: key, At: now}
	if sent, err := s.deps.Sends.Claimed(ctx, claim); err != nil || sent {
		return err
	}
	figures, err := s.deps.Figures.OverviewAt(ctx, student, loc, endOfWeek(weekStart))
	if err != nil {
		return err
	}
	if !hasSomethingToShow(figures) {
		return nil
	}
	email, err := s.deps.Composer.ProgressReport(domain.ProgressReportMessage{
		To: student.Email, DisplayName: student.DisplayName, Locale: student.Locale.Code, WeekStart: weekStart,
		PracticeDays: figures.PracticeDaysLast7, Minutes: figures.MinutesPractisedLast7, PreviousMinutes: figures.MinutesPractisedPrevious7,
		DayStreakCurrent: figures.DayStreakCurrent, DayStreakBest: figures.DayStreakBest,
		SkillsUp: figures.SkillsUpLast7, SongsPlayed: figures.SongsPlayedLast7,
		Links: s.links(student.ID, domain.EmailKindProgressReport),
	})
	if err != nil {
		return err
	}
	return s.claimAndSend(ctx, claim, email)
}

// hasSomethingToShow reports whether the student started a practice
// session, completed a lesson or played a song in the week: a report of
// zeros would discourage, and reminders already cover a student who stopped.
func hasSomethingToShow(figures PracticeOverview) bool {
	return figures.SessionsStartedLast7 > 0 || figures.PracticeDaysLast7 > 0 || figures.MinutesPractisedLast7 > 0 ||
		figures.LearningDaysLast7 > 0 || figures.SongsPlayedLast7 > 0
}

func (s *EngagementMailerService) sendReminder(ctx context.Context, student domain.User, loc *time.Location, spellStart, now time.Time) error {
	if !student.EmailPreferences.Reminders || student.Email == "" {
		return nil
	}
	key, days, stage, due := domain.ReminderDue(spellStart, now, loc)
	if !due {
		return nil
	}
	claim := domain.EmailSendClaim{Recipient: student.ID, Kind: domain.EmailSendReminder, PeriodKey: key, At: now}
	if sent, err := s.deps.Sends.Claimed(ctx, claim); err != nil || sent {
		return err
	}
	email, err := s.deps.Composer.Reminder(domain.ReminderMessage{
		To: student.Email, DisplayName: student.DisplayName, Locale: student.Locale.Code,
		DaysWithout: days, Stage: stage, Links: s.links(student.ID, domain.EmailKindReminders),
	})
	if err != nil {
		return err
	}
	return s.claimAndSend(ctx, claim, email)
}

func (s *EngagementMailerService) sendConciergeSummary(ctx context.Context, students []domain.User, lastActive map[string]time.Time, now time.Time) error {
	if s.cfg.ConciergeAddress == "" {
		return nil
	}
	key, weekStart, due := domain.WeeklyEmailWeek(now, s.defaultLocation, domain.ConciergeSummaryHour)
	if !due {
		return nil
	}
	claim := domain.EmailSendClaim{Recipient: domain.ConciergeRecipient, Kind: domain.EmailSendConciergeSummary, PeriodKey: key, At: now}
	if sent, err := s.deps.Sends.Claimed(ctx, claim); err != nil || sent {
		return err
	}
	rows := make([]domain.ConciergeRow, 0, len(students))
	for _, student := range students {
		row, err := s.conciergeRow(ctx, student, weekStart, lastActive, now)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	slices.SortStableFunc(rows, func(a, b domain.ConciergeRow) int { return cmp.Compare(b.DaysInactive, a.DaysInactive) })
	email, err := s.deps.Composer.ConciergeSummary(domain.ConciergeSummaryMessage{To: s.cfg.ConciergeAddress, WeekStart: weekStart, Rows: rows})
	if err != nil {
		return err
	}
	return s.claimAndSend(ctx, claim, email)
}

// conciergeRow reports student's week as the student lived it: the same
// Monday-to-Sunday dates, in the student's own time zone.
func (s *EngagementMailerService) conciergeRow(ctx context.Context, student domain.User, weekStart time.Time, lastActive map[string]time.Time, now time.Time) (domain.ConciergeRow, error) {
	loc := s.location(student)
	y, m, d := weekStart.Date()
	figures, err := s.deps.Figures.OverviewAt(ctx, student, loc, endOfWeek(time.Date(y, m, d, 0, 0, 0, 0, loc)))
	if err != nil {
		return domain.ConciergeRow{}, err
	}
	row := domain.ConciergeRow{
		DisplayName: student.DisplayName, Email: student.Email,
		PracticeDays: figures.PracticeDaysLast7, Minutes: figures.MinutesPractisedLast7,
		DaysInactive: domain.DaysWithoutActivity(s.spellStart(student, lastActive), now, loc),
	}
	if last, ok := lastActive[student.ID]; ok {
		local := last.In(loc)
		row.LastActive = &local
	}
	return row, nil
}

// releaseTimeout bounds giving a claim back, which may run after the pass
// itself was cancelled.
const releaseTimeout = 5 * time.Second

// claimAndSend claims the email's place in the send record and sends it.
// A claim someone else already made sends nothing. A failed send gives the
// claim back, so a later pass tries again, unless the server refused the
// email for good: that claim is kept, so the refusal is reported once
// rather than on every pass.
func (s *EngagementMailerService) claimAndSend(ctx context.Context, claim domain.EmailSendClaim, email domain.Email) error {
	claimed, err := s.deps.Sends.Claim(ctx, claim)
	if err != nil || !claimed {
		return err
	}
	err = s.deps.Sender.Send(ctx, email)
	if err == nil {
		return nil
	}
	sendErr := fmt.Errorf("send %s %s to %s: %w", claim.Kind, claim.PeriodKey, claim.Recipient, err)
	if errors.Is(err, domain.ErrEmailRejected) {
		return sendErr
	}
	// The pass may have been cancelled mid-send, by a shutdown or its own
	// deadline; the claim must still be given back, or the email would
	// never go out.
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
	defer cancel()
	return errors.Join(sendErr, s.deps.Sends.Release(releaseCtx, claim))
}

// links builds a student email's links: the app, and both ways to stop
// receiving kind, which carry the same unsubscribe token.
func (s *EngagementMailerService) links(userID string, kind domain.EmailKind) domain.EmailLinks {
	token := url.QueryEscape(s.deps.Tokens.UnsubscribeToken(userID, kind))
	return domain.EmailLinks{
		App:                 s.cfg.AppBaseURL,
		UnsubscribePage:     s.cfg.AppBaseURL + "/unsubscribe?token=" + token,
		OneClickUnsubscribe: s.cfg.APIBaseURL + "/email-unsubscribes?token=" + token,
	}
}

// spellStart is when student's current inactive spell began: their last
// activity, or their registration when they have none.
func (s *EngagementMailerService) spellStart(student domain.User, lastActive map[string]time.Time) time.Time {
	if last, ok := lastActive[student.ID]; ok {
		return last
	}
	return student.RegisteredAt
}

// location is student's time zone, or the default one when they never
// reported one (or it no longer loads).
func (s *EngagementMailerService) location(student domain.User) *time.Location {
	if student.TimeZone == "" {
		return s.defaultLocation
	}
	loc, err := time.LoadLocation(student.TimeZone)
	if err != nil {
		return s.defaultLocation
	}
	return loc
}

// endOfWeek is the last instant of the week that began at weekStart, a
// Monday at 00:00: just before the next Monday's midnight.
func endOfWeek(weekStart time.Time) time.Time {
	return weekStart.AddDate(0, 0, 7).Add(-time.Nanosecond)
}
