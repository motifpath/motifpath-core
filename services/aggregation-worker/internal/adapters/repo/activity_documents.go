package repo

import (
	"time"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// practiceSessionDocument is a `practice_sessions` document: one per (student_id,
// practice_session_id). The end fields are null until the session's end arrives,
// and the start fields until its start does.
type practiceSessionDocument struct {
	StudentID         string                      `bson:"student_id"`
	PracticeSessionID string                      `bson:"practice_session_id"`
	StartedAt         *time.Time                  `bson:"started_at"`
	InstrumentID      string                      `bson:"instrument_id,omitempty"`
	Minutes           int                         `bson:"minutes"`
	PlannedItems      []plannedItemDocument       `bson:"planned_items"`
	LastEventAt       time.Time                   `bson:"last_event_at"`
	End               *practiceSessionEndDocument `bson:"end"`
	UpdatedAt         time.Time                   `bson:"updated_at"`
}

type plannedItemDocument struct {
	ItemKey string `bson:"item_key"`
	Reason  string `bson:"reason"`
}

type practiceSessionEndDocument struct {
	EventID       string    `bson:"event_id"`
	EndedAt       time.Time `bson:"ended_at"`
	LeftEarly     bool      `bson:"left_early"`
	AnsweredCount int       `bson:"answered_count"`
}

func toSessionDocument(s domain.PracticeSession) practiceSessionDocument {
	doc := practiceSessionDocument{
		StudentID:         s.StudentID,
		PracticeSessionID: s.ID,
		StartedAt:         s.StartedAt,
		InstrumentID:      s.InstrumentID,
		Minutes:           s.Minutes,
		LastEventAt:       s.LastEventAt,
	}
	for _, p := range s.PlannedItems {
		doc.PlannedItems = append(doc.PlannedItems, plannedItemDocument{ItemKey: p.ItemKey, Reason: p.Reason})
	}
	if s.End != nil {
		doc.End = &practiceSessionEndDocument{
			EventID:       s.End.EventID,
			EndedAt:       s.End.OccurredAt,
			LeftEarly:     s.End.LeftEarly,
			AnsweredCount: s.End.AnsweredCount,
		}
	}
	return doc
}

func (d practiceSessionDocument) toDomain() domain.PracticeSession {
	s := domain.PracticeSession{
		ID:           d.PracticeSessionID,
		StudentID:    d.StudentID,
		StartedAt:    utc(d.StartedAt),
		InstrumentID: d.InstrumentID,
		Minutes:      d.Minutes,
		LastEventAt:  d.LastEventAt.UTC(),
	}
	for _, p := range d.PlannedItems {
		s.PlannedItems = append(s.PlannedItems, domain.PlannedPracticeItem{ItemKey: p.ItemKey, Reason: p.Reason})
	}
	if d.End != nil {
		s.End = &domain.PracticeSessionEnd{
			EventID:           d.End.EventID,
			StudentID:         d.StudentID,
			PracticeSessionID: d.PracticeSessionID,
			OccurredAt:        d.End.EndedAt.UTC(),
			LeftEarly:         d.End.LeftEarly,
			AnsweredCount:     d.End.AnsweredCount,
		}
	}
	return s
}

// learningActivityDocument is a `learning_activity` document: one completion of a
// content node, never modified once stored.
type learningActivityDocument struct {
	EventID       string    `bson:"event_id"`
	StudentID     string    `bson:"student_id"`
	ContentNodeID string    `bson:"content_node_id"`
	CompletedAt   time.Time `bson:"completed_at"`
}

// practiceItemSnapshotDocument is a `practice_item_history` document: an item's
// state at the end of a UTC day it was practised on, stored as its midnight. level
// is the earned level, before the reader applies any lapse.
type practiceItemSnapshotDocument struct {
	StudentID            string    `bson:"student_id"`
	ItemKey              string    `bson:"item_key"`
	Day                  time.Time `bson:"day"`
	RulesVersion         int       `bson:"rules_version"`
	Level                string    `bson:"level"`
	Counted              int       `bson:"counted"`
	Accuracy             float64   `bson:"accuracy"`
	Fluency              float64   `bson:"fluency"`
	Box                  int       `bson:"box"`
	BestCleanBPM         *int      `bson:"best_clean_bpm"`
	BestChangesPerMinute *int      `bson:"best_changes_per_minute"`
	UpdatedAt            time.Time `bson:"updated_at"`
}
