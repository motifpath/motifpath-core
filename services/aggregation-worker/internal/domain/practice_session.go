package domain

import (
	"slices"
	"time"
)

const (
	EventTypePracticeSessionStarted EventType = "practice.session_started"
	EventTypePracticeSessionEnded   EventType = "practice.session_ended"
)

// abandonGrace is how long past its planned minutes a session may stay silent
// before it counts as abandoned.
const abandonGrace = 15 * time.Minute

// PlannedPracticeItem is one item of a session's plan, with why it was picked.
type PlannedPracticeItem struct {
	ItemKey string
	Reason  string
}

// PracticeSessionStart is a practice.session_started event: the plan as offered.
type PracticeSessionStart struct {
	EventID           string
	StudentID         string
	PracticeSessionID string
	OccurredAt        time.Time
	// InstrumentID is empty for a session practised in the head.
	InstrumentID string
	Minutes      int
	PlannedItems []PlannedPracticeItem
}

// PracticeSessionEnd is a practice.session_ended event.
type PracticeSessionEnd struct {
	EventID           string
	StudentID         string
	PracticeSessionID string
	OccurredAt        time.Time
	LeftEarly         bool
	AnsweredCount     int
	// FeltRatings are the student's answers to "How did it feel?", at most
	// two, about timed drills the plan asked about.
	FeltRatings []FeltRating
}

// Felt is the student's answer to "How did it feel?" about a timed drill.
type Felt string

const (
	FeltEasy       Felt = "easy"
	FeltAboutRight Felt = "about_right"
	FeltHard       Felt = "hard"
)

// FeltRating is how one timed drill felt to the student in a session. It
// only ever calibrates the drill's fluent time, never the student's mastery.
type FeltRating struct {
	DrillTemplateKey string
	Felt             Felt
}

// PracticeSessionStatus is how a session stands when it is read.
type PracticeSessionStatus string

const (
	PracticeSessionInProgress PracticeSessionStatus = "in_progress"
	PracticeSessionFinished   PracticeSessionStatus = "finished"
	PracticeSessionEndedEarly PracticeSessionStatus = "ended_early"
	PracticeSessionAbandoned  PracticeSessionStatus = "abandoned"
)

// PracticeSession is the raw record of one practice session: its start, the time
// of its latest event and its end. Whether it was abandoned is never stored; it
// depends on when the session is read, so a late event can reopen it.
//
// Events for a session can arrive in any order, so each method accepts its event
// whatever has been recorded before it.
type PracticeSession struct {
	ID        string
	StudentID string
	// StartedAt is nil until the session's start has arrived.
	StartedAt    *time.Time
	InstrumentID string
	Minutes      int
	PlannedItems []PlannedPracticeItem
	// PractisedTemplates are the timed drill templates of the session's
	// graded answers, each once, in order.
	PractisedTemplates []string
	// LastEventAt is the latest practice event recorded for the session.
	LastEventAt time.Time
	End         *PracticeSessionEnd
}

// Started records the session's start and its plan.
func (s PracticeSession) Started(e PracticeSessionStart) PracticeSession {
	s.ID, s.StudentID = e.PracticeSessionID, e.StudentID
	at := e.OccurredAt
	s.StartedAt = &at
	s.InstrumentID, s.Minutes, s.PlannedItems = e.InstrumentID, e.Minutes, e.PlannedItems
	return s.touched(at)
}

// Answered records an answer given in the session at the given time, and
// the timed drill template it practised; empty when it practised none.
func (s PracticeSession) Answered(at time.Time, template string) PracticeSession {
	if template != "" && !slices.Contains(s.PractisedTemplates, template) {
		s.PractisedTemplates = append(slices.Clone(s.PractisedTemplates), template)
		slices.Sort(s.PractisedTemplates)
	}
	return s.touched(at)
}

// FeltRatedTemplates are the templates of the session's felt ratings that
// it practised, in its ratings' order. A rating about a drill the session
// didn't practise says nothing about that drill, so it is left out. Answers
// may arrive after the end, so this is read from whatever has arrived.
func (s PracticeSession) FeltRatedTemplates() []string {
	if s.End == nil {
		return nil
	}
	var templates []string
	for _, r := range s.End.FeltRatings {
		if slices.Contains(s.PractisedTemplates, r.DrillTemplateKey) && !slices.Contains(templates, r.DrillTemplateKey) {
			templates = append(templates, r.DrillTemplateKey)
		}
	}
	return templates
}

// Ended records the session's end. A redelivered end changes nothing, and an end
// earlier than the one recorded doesn't replace it.
func (s PracticeSession) Ended(e PracticeSessionEnd) PracticeSession {
	s.ID, s.StudentID = e.PracticeSessionID, e.StudentID
	if s.End == nil || e.OccurredAt.After(s.End.OccurredAt) {
		end := e
		s.End = &end
	}
	return s.touched(e.OccurredAt)
}

func (s PracticeSession) touched(at time.Time) PracticeSession {
	if at.After(s.LastEventAt) {
		s.LastEventAt = at
	}
	return s
}

// StatusAt is how the session stands when read at now, and when it ended if it
// has. A session that never ended counts as abandoned, ended at its last event,
// once it has been silent for its planned minutes plus the grace; until its start
// has arrived its planned minutes are unknown, so it is still in progress.
func (s PracticeSession) StatusAt(now time.Time) (PracticeSessionStatus, *time.Time) {
	if s.End != nil {
		endedAt := s.End.OccurredAt
		if s.End.LeftEarly {
			return PracticeSessionEndedEarly, &endedAt
		}
		return PracticeSessionFinished, &endedAt
	}
	if s.StartedAt == nil {
		return PracticeSessionInProgress, nil
	}
	silence := time.Duration(s.Minutes)*time.Minute + abandonGrace
	if now.Sub(s.LastEventAt) >= silence {
		endedAt := s.LastEventAt
		return PracticeSessionAbandoned, &endedAt
	}
	return PracticeSessionInProgress, nil
}
