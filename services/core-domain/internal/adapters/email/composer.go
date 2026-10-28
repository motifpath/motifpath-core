// Package email writes the platform's engagement emails and sends them.
package email

import (
	"bytes"
	"embed"
	"fmt"
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

// brandName is the name beside the mark, as text, so the header reads even
// when the client blocks images.
const brandName = "MotifPath"

// markPath is where the app serves the mark the emails show, at twice the
// size it is drawn.
const markPath = "/email/mark.png"

// longInactiveDays is from when the concierge summary marks a learner out.
const longInactiveDays = 10

// studentView is a student email laid out by the shared student template:
// a title, the greeting and paragraphs, optional figures and a highlighted
// line, one action, and the footer with its links. Every string is already
// in the student's language.
type studentView struct {
	MarkURL          string
	BrandName        string
	Eyebrow          string
	Title            string
	Greeting         string
	Paragraphs       []string
	Stats            []stat
	Highlight        string
	ActionLabel      string
	ActionURL        string
	Footer           string
	UnsubscribeLabel string
	UnsubscribeURL   string
	SettingsLabel    string
	SettingsURL      string
}

// StatRows lays the figures out two by two.
func (v studentView) StatRows() [][]stat {
	var rows [][]stat
	for i := 0; i < len(v.Stats); i += 2 {
		rows = append(rows, v.Stats[i:min(i+2, len(v.Stats))])
	}
	return rows
}

// stat is one figure: its value, what it counts, and an optional note.
type stat struct {
	Label string
	Value string
	Note  string
}

func (c *Composer) ProgressReport(m domain.ProgressReportMessage) (domain.Email, error) {
	words := catalogFor(m.Locale)
	from, to := words.date(m.WeekStart), words.date(m.WeekStart.AddDate(0, 0, 6))
	streak := words.firstStreak
	switch {
	case m.DayStreakCurrent > 0:
		streak = words.streak(m.DayStreakCurrent, m.DayStreakBest)
	case m.DayStreakBest > 0:
		streak = words.newStreak(m.DayStreakBest)
	}
	view := c.studentShell(words, m.Links, m.DisplayName)
	view.Eyebrow = strings.ToUpper(words.reportWeek(from, to))
	view.Title = words.reportSubject
	view.Paragraphs = []string{words.reportIntro(from, to)}
	view.Stats = []stat{
		{Label: words.practiceDaysLabel, Value: words.practiceDays(m.PracticeDays)},
		{Label: words.minutesLabel, Value: words.minutes(m.Minutes), Note: words.previousMinutes(m.PreviousMinutes)},
		{Label: words.skillsUpLabel, Value: strconv.Itoa(m.SkillsUp)},
		{Label: words.songsPlayedLabel, Value: strconv.Itoa(m.SongsPlayed)},
	}
	view.Highlight = streak
	view.ActionLabel = words.reportAction
	view.Footer, view.UnsubscribeLabel = words.reportFooter, words.reportUnsubscribe
	return c.studentEmail(m.To, words.reportSubject, view, m.Links.OneClickUnsubscribe)
}

func (c *Composer) Reminder(m domain.ReminderMessage) (domain.Email, error) {
	words := catalogFor(m.Locale)
	body, ok := words.reminderBody[m.Stage]
	if !ok {
		return domain.Email{}, fmt.Errorf("no reminder wording for stage %q", m.Stage)
	}
	view := c.studentShell(words, m.Links, m.DisplayName)
	view.Title = words.reminderSubject[m.Stage]
	view.Paragraphs = []string{body(m.DaysWithout)}
	view.ActionLabel = words.reminderAction
	view.Footer, view.UnsubscribeLabel = words.reminderFooter[m.Stage], words.reminderUnsubscribe
	return c.studentEmail(m.To, view.Title, view, m.Links.OneClickUnsubscribe)
}

// studentShell fills what every student email shares: the header, the
// greeting, where the action leads and the footer's links.
func (c *Composer) studentShell(words catalog, links domain.EmailLinks, displayName string) studentView {
	return studentView{
		MarkURL: links.App + markPath, BrandName: brandName,
		Greeting:       words.greeting(firstName(displayName)),
		ActionURL:      links.App,
		UnsubscribeURL: links.UnsubscribePage, SettingsLabel: words.settings, SettingsURL: links.Settings,
	}
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
	MarkURL   string
	BrandName string
	WeekStart string
	Count     int
	Rows      []conciergeRow
}

type conciergeRow struct {
	Name         string
	Email        string
	Role         string
	PracticeDays int
	Minutes      int
	LastActive   string
	Inactive     string
	LongInactive bool
}

// roleTags name the learners who aren't students, whose emails come from
// learning alongside their own work.
var roleTags = map[domain.Role]string{domain.RoleTeacher: "Teacher", domain.RoleAdmin: "Admin"}

func (c *Composer) ConciergeSummary(m domain.ConciergeSummaryMessage) (domain.Email, error) {
	view := conciergeView{MarkURL: m.App + markPath, BrandName: brandName, WeekStart: english.date(m.WeekStart), Count: len(m.Rows)}
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
			Name: row.DisplayName, Email: email, Role: roleTags[row.Role], PracticeDays: row.PracticeDays, Minutes: row.Minutes,
			LastActive: lastActive, Inactive: fmt.Sprintf("%d %s", row.DaysInactive, plural(row.DaysInactive, "day", "days")),
			LongInactive: row.DaysInactive >= longInactiveDays,
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
