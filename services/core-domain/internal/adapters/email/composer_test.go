package email

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

var saoPaulo = func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err)
	}
	return loc
}()

func links(kind string) domain.EmailLinks {
	return domain.EmailLinks{
		App:                 "https://app.motifpath.com",
		UnsubscribePage:     "https://app.motifpath.com/unsubscribe?token=" + kind,
		OneClickUnsubscribe: "https://api.motifpath.com/email-unsubscribes?token=" + kind,
	}
}

func report(locale string) domain.ProgressReportMessage {
	return domain.ProgressReportMessage{
		To: "alice@example.com", DisplayName: "Alice Martins", Locale: locale,
		WeekStart:    time.Date(2026, 10, 26, 0, 0, 0, 0, saoPaulo),
		PracticeDays: 4, Minutes: 75, PreviousMinutes: 50, DayStreakCurrent: 2, DayStreakBest: 6, SkillsUp: 3, SongsPlayed: 1,
		Links: links("report"),
	}
}

func newTestComposer(t *testing.T) *Composer {
	t.Helper()
	composer, err := NewComposer()
	require.NoError(t, err)
	return composer
}

// bothParts asserts something of the plain-text and the HTML part alike.
func bothParts(t *testing.T, e domain.Email, check func(t *testing.T, part string)) {
	t.Helper()
	require.NotEmpty(t, e.Text)
	require.NotEmpty(t, e.HTML)
	check(t, e.Text)
	check(t, e.HTML)
}

func TestComposer_ProgressReport(t *testing.T) {
	tests := []struct {
		name        string
		message     domain.ProgressReportMessage
		wantSubject string
		wantIn      []string
	}{
		{
			name: "English shows the week's figures", message: report("en"), wantSubject: "Your week on MotifPath",
			wantIn: []string{"Hi Alice,", "Oct 26", "Nov 1", "4 of 7", "75 min", "50", "3", "Day streak: 2 days", "best: 6"},
		},
		{
			name: "Portuguese (Brazil) shows the same figures in Portuguese", message: report("pt_BR"), wantSubject: "Sua semana no MotifPath",
			wantIn: []string{"Oi, Alice,", "26/10", "1/11", "4 de 7", "75 min", "Sequência de dias: 2", "recorde: 6"},
		},
		{
			name: "any other locale is written in English", message: report("es"), wantSubject: "Your week on MotifPath",
			wantIn: []string{"Hi Alice,"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := newTestComposer(t).ProgressReport(tt.message)

			require.NoError(t, err)
			assert.Equal(t, "alice@example.com", e.To)
			assert.Equal(t, tt.wantSubject, e.Subject)
			bothParts(t, e, func(t *testing.T, part string) {
				for _, want := range tt.wantIn {
					assert.Contains(t, part, want)
				}
			})
		})
	}
}

func TestComposer_ProgressReport_Streak(t *testing.T) {
	tests := []struct {
		name    string
		current int
		best    int
		wantIn  string
	}{
		{name: "a current streak is shown with the best one", current: 2, best: 6, wantIn: "Day streak: 2 days (best: 6)"},
		{name: "a streak of one day reads in the singular", current: 1, best: 6, wantIn: "Day streak: 1 day (best: 6)"},
		{name: "no current streak invites a new one beside the best", current: 0, best: 6, wantIn: "Practise today to start a new day streak. Your best is 6 days."},
		{name: "no streak ever invites a first one", current: 0, best: 0, wantIn: "Practise today to start a day streak."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := report("en")
			m.DayStreakCurrent, m.DayStreakBest = tt.current, tt.best

			e, err := newTestComposer(t).ProgressReport(m)

			require.NoError(t, err)
			bothParts(t, e, func(t *testing.T, part string) {
				assert.Contains(t, part, tt.wantIn)
				for _, word := range []string{"lost", "broke", "broken", "missed"} {
					assert.NotContains(t, strings.ToLower(part), word, "an email never says a streak was lost")
				}
			})
		})
	}
}

func TestComposer_StudentEmailsOfferUnsubscribe(t *testing.T) {
	composer := newTestComposer(t)
	reportEmail, err := composer.ProgressReport(report("en"))
	require.NoError(t, err)
	reminderEmail, err := composer.Reminder(domain.ReminderMessage{To: "alice@example.com", DisplayName: "Alice", Locale: "en", DaysWithout: 3, Links: links("reminders")})
	require.NoError(t, err)

	tests := []struct {
		name     string
		email    domain.Email
		kind     string
		linkText string
	}{
		{name: "a progress report", email: reportEmail, kind: "report", linkText: "Stop progress reports"},
		{name: "a reminder", email: reminderEmail, kind: "reminders", linkText: "Stop reminders"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "https://api.motifpath.com/email-unsubscribes?token="+tt.kind, tt.email.OneClickUnsubscribeURL)
			bothParts(t, tt.email, func(t *testing.T, part string) {
				assert.Contains(t, part, "https://app.motifpath.com/unsubscribe?token="+tt.kind)
				assert.Contains(t, part, tt.linkText)
			})
		})
	}
}

func TestComposer_Reminder(t *testing.T) {
	tests := []struct {
		name        string
		message     domain.ReminderMessage
		wantSubject string
		wantIn      []string
	}{
		{
			name:        "the first reminder of a spell",
			message:     domain.ReminderMessage{To: "alice@example.com", DisplayName: "Alice Martins", Locale: "en", DaysWithout: 3, Links: links("reminders")},
			wantSubject: "A few minutes of practice today?",
			wantIn:      []string{"Hi Alice,", "3 days", "Practise now", "https://app.motifpath.com"},
		},
		{
			name:        "the second reminder of a spell",
			message:     domain.ReminderMessage{To: "alice@example.com", DisplayName: "Alice Martins", Locale: "en", DaysWithout: 10, Second: true, Links: links("reminders")},
			wantSubject: "Pick up where you left off",
			wantIn:      []string{"10 days"},
		},
		{
			name:        "in Portuguese (Brazil)",
			message:     domain.ReminderMessage{To: "alice@example.com", DisplayName: "Alice Martins", Locale: "pt_BR", DaysWithout: 3, Links: links("reminders")},
			wantSubject: "Que tal alguns minutos de prática hoje?",
			wantIn:      []string{"Oi, Alice,", "3 dias", "Praticar agora", "Parar lembretes"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := newTestComposer(t).Reminder(tt.message)

			require.NoError(t, err)
			assert.Equal(t, tt.wantSubject, e.Subject)
			bothParts(t, e, func(t *testing.T, part string) {
				for _, want := range tt.wantIn {
					assert.Contains(t, part, want)
				}
				assert.NotContains(t, strings.ToLower(part), "streak", "a reminder never mentions a streak")
				assert.NotContains(t, strings.ToLower(part), "sequência")
			})
		})
	}
}

func TestComposer_EscapesNamesInHTML(t *testing.T) {
	m := report("en")
	m.DisplayName = "<script>alert(1)</script> Bob"

	e, err := newTestComposer(t).ProgressReport(m)

	require.NoError(t, err)
	assert.NotContains(t, e.HTML, "<script>")
	assert.Contains(t, e.HTML, "&lt;script&gt;")
}

func TestComposer_ConciergeSummary(t *testing.T) {
	last := time.Date(2026, 11, 1, 18, 0, 0, 0, saoPaulo)
	message := domain.ConciergeSummaryMessage{
		To: "concierge@motifpath.com", WeekStart: time.Date(2026, 10, 26, 0, 0, 0, 0, saoPaulo),
		Rows: []domain.ConciergeRow{
			{DisplayName: "Dani Rocha", Email: "", PracticeDays: 0, Minutes: 0, DaysInactive: 12},
			{DisplayName: "Alice Martins", Email: "alice@example.com", PracticeDays: 4, Minutes: 75, LastActive: &last, DaysInactive: 1},
		},
	}

	e, err := newTestComposer(t).ConciergeSummary(message)

	require.NoError(t, err)
	assert.Equal(t, "concierge@motifpath.com", e.To)
	assert.Equal(t, "Students, week of Oct 26", e.Subject)
	assert.Empty(t, e.OneClickUnsubscribeURL, "the team's summary has no unsubscribe")
	bothParts(t, e, func(t *testing.T, part string) {
		assert.Less(t, strings.Index(part, "Dani Rocha"), strings.Index(part, "Alice Martins"), "rows keep the order they were given")
		assert.Contains(t, part, "never")
		assert.Contains(t, part, "Nov 1")
		assert.Contains(t, part, "75")
	})
}
