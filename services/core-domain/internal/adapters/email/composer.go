// Package email writes the platform's engagement emails and sends them.
package email

import (
	"bytes"
	"embed"
	htmltemplate "html/template"
	"strconv"
	"strings"
	texttemplate "text/template"

	"github.com/motifpath/core-domain/internal/domain"
)

//go:embed templates
var templateFiles embed.FS

// Composer writes each kind of engagement email, as HTML and as plain text,
// in its recipient's language. html/template escapes everything a template
// prints, so a name holding markup shows as text.
type Composer struct {
	studentHTML   *htmltemplate.Template
	studentText   *texttemplate.Template
	conciergeHTML *htmltemplate.Template
	conciergeText *texttemplate.Template
}

func NewComposer() (*Composer, error) {
	studentHTML, err := htmltemplate.ParseFS(templateFiles, "templates/student.html.tmpl")
	if err != nil {
		return nil, err
	}
	studentText, err := texttemplate.ParseFS(templateFiles, "templates/student.txt.tmpl")
	if err != nil {
		return nil, err
	}
	conciergeHTML, err := htmltemplate.ParseFS(templateFiles, "templates/concierge.html.tmpl")
	if err != nil {
		return nil, err
	}
	conciergeText, err := texttemplate.ParseFS(templateFiles, "templates/concierge.txt.tmpl")
	if err != nil {
		return nil, err
	}
	return &Composer{studentHTML: studentHTML, studentText: studentText, conciergeHTML: conciergeHTML, conciergeText: conciergeText}, nil
}

// studentView is a student email laid out by the shared student template:
// paragraphs, optional figures, one action and the footer with its
// unsubscribe link. Every string is already in the student's language.
type studentView struct {
	Greeting         string
	Paragraphs       []string
	Stats            []stat
	ActionLabel      string
	ActionURL        string
	Footer           string
	UnsubscribeLabel string
	UnsubscribeURL   string
}

type stat struct {
	Label string
	Value string
}

func (c *Composer) ProgressReport(m domain.ProgressReportMessage) (domain.Email, error) {
	words := catalogFor(m.Locale)
	weekEnd := m.WeekStart.AddDate(0, 0, 6)
	streak := words.firstStreak
	switch {
	case m.DayStreakCurrent > 0:
		streak = words.streak(m.DayStreakCurrent, m.DayStreakBest)
	case m.DayStreakBest > 0:
		streak = words.newStreak(m.DayStreakBest)
	}
	view := studentView{
		Greeting:   words.greeting(firstName(m.DisplayName)),
		Paragraphs: []string{words.reportIntro(words.date(m.WeekStart), words.date(weekEnd))},
		Stats: []stat{
			{Label: words.practiceDaysLabel, Value: words.practiceDays(m.PracticeDays)},
			{Label: words.minutesLabel, Value: words.minutes(m.Minutes, m.PreviousMinutes)},
			{Label: words.skillsUpLabel, Value: strconv.Itoa(m.SkillsUp)},
			{Label: words.songsPlayedLabel, Value: strconv.Itoa(m.SongsPlayed)},
		},
		ActionLabel: words.reportAction, ActionURL: m.Links.App,
		Footer: words.reportFooter, UnsubscribeLabel: words.reportUnsubscribe, UnsubscribeURL: m.Links.UnsubscribePage,
	}
	view.Paragraphs = append(view.Paragraphs, streak)
	return c.studentEmail(m.To, words.reportSubject, view, m.Links.OneClickUnsubscribe)
}

func (c *Composer) Reminder(m domain.ReminderMessage) (domain.Email, error) {
	words := catalogFor(m.Locale)
	view := studentView{
		Greeting:    words.greeting(firstName(m.DisplayName)),
		Paragraphs:  []string{words.reminderBody(m.DaysWithout, m.Second)},
		ActionLabel: words.reminderAction, ActionURL: m.Links.App,
		Footer: words.reminderFooter, UnsubscribeLabel: words.reminderUnsubscribe, UnsubscribeURL: m.Links.UnsubscribePage,
	}
	return c.studentEmail(m.To, words.reminderSubject(m.Second), view, m.Links.OneClickUnsubscribe)
}

func (c *Composer) studentEmail(to, subject string, view studentView, oneClick string) (domain.Email, error) {
	var html, text bytes.Buffer
	if err := c.studentHTML.Execute(&html, view); err != nil {
		return domain.Email{}, err
	}
	if err := c.studentText.Execute(&text, view); err != nil {
		return domain.Email{}, err
	}
	return domain.Email{To: to, Subject: subject, HTML: html.String(), Text: text.String(), OneClickUnsubscribeURL: oneClick}, nil
}

// conciergeView is the team's weekly summary. It is written in English
// only: it goes to the team, not to a student.
type conciergeView struct {
	WeekStart string
	Rows      []conciergeRow
}

type conciergeRow struct {
	Name         string
	Email        string
	PracticeDays int
	Minutes      int
	LastActive   string
	DaysInactive int
}

func (c *Composer) ConciergeSummary(m domain.ConciergeSummaryMessage) (domain.Email, error) {
	view := conciergeView{WeekStart: english.date(m.WeekStart)}
	for _, row := range m.Rows {
		lastActive := "never"
		if row.LastActive != nil {
			lastActive = english.date(*row.LastActive)
		}
		email := row.Email
		if email == "" {
			email = "no address"
		}
		view.Rows = append(view.Rows, conciergeRow{
			Name: row.DisplayName, Email: email, PracticeDays: row.PracticeDays, Minutes: row.Minutes,
			LastActive: lastActive, DaysInactive: row.DaysInactive,
		})
	}
	var html, text bytes.Buffer
	if err := c.conciergeHTML.Execute(&html, view); err != nil {
		return domain.Email{}, err
	}
	if err := c.conciergeText.Execute(&text, view); err != nil {
		return domain.Email{}, err
	}
	return domain.Email{To: m.To, Subject: "Students, week of " + view.WeekStart, HTML: html.String(), Text: text.String()}, nil
}

// firstName greets a student by the first word of their full name.
func firstName(displayName string) string {
	fields := strings.Fields(displayName)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
