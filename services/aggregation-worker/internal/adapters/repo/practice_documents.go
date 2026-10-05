package repo

import (
	"time"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// practiceResponseDocument stores the raw response flat, with only its shape's
// fields, the way ingestion publishes it.
type practiceResponseDocument struct {
	ResponseType     string   `bson:"response_type"`
	NoteName         string   `bson:"note_name,omitempty"`
	String           *int     `bson:"string,omitempty"`
	Fret             *int     `bson:"fret,omitempty"`
	OptionIDs        []string `bson:"option_ids,omitempty"`
	LatencyMs        *int     `bson:"latency_ms,omitempty"`
	AudioMs          *int     `bson:"audio_ms,omitempty"`
	Rating           string   `bson:"rating,omitempty"`
	TempoBPM         *int     `bson:"tempo_bpm,omitempty"`
	ChangesPerMinute *int     `bson:"changes_per_minute,omitempty"`
}

// practiceEvidenceDocument is a `practice_evidence` document: one piece of
// evidence with its raw response, never modified once stored.
type practiceEvidenceDocument struct {
	EvidenceID        string                   `bson:"evidence_id"`
	StudentID         string                   `bson:"student_id"`
	ItemKey           string                   `bson:"item_key"`
	Source            string                   `bson:"source"`
	OccurredAt        time.Time                `bson:"occurred_at"`
	PracticeSessionID string                   `bson:"practice_session_id,omitempty"`
	TriggerContext    *triggerContextDocument  `bson:"trigger_context,omitempty"`
	GraderID          string                   `bson:"grader_id,omitempty"`
	Response          practiceResponseDocument `bson:"response"`
	Correct           *bool                    `bson:"correct,omitempty"`
	LatencyMs         *int                     `bson:"latency_ms,omitempty"`
	AudioMs           *int                     `bson:"audio_ms,omitempty"`
	TapMs             *int                     `bson:"tap_ms,omitempty"`
	Rating            string                   `bson:"rating,omitempty"`
	TempoBPM          *int                     `bson:"tempo_bpm,omitempty"`
	ChangesPerMinute  *int                     `bson:"changes_per_minute,omitempty"`
}

// triggerContextDocument is where an answer outside a practice session was given.
type triggerContextDocument struct {
	Source        string `bson:"source"`
	ContentNodeID string `bson:"content_node_id,omitempty"`
	ChallengeID   string `bson:"challenge_id,omitempty"`
}

func toTriggerContextDocument(tc *domain.TriggerContext) *triggerContextDocument {
	if tc == nil {
		return nil
	}
	return &triggerContextDocument{Source: tc.Source, ContentNodeID: tc.ContentNodeID, ChallengeID: tc.ChallengeID}
}

func (d *triggerContextDocument) toDomain() *domain.TriggerContext {
	if d == nil {
		return nil
	}
	return &domain.TriggerContext{Source: d.Source, ContentNodeID: d.ContentNodeID, ChallengeID: d.ChallengeID}
}

func toEvidenceDocument(e domain.PracticeEvidence) practiceEvidenceDocument {
	r := e.Response
	return practiceEvidenceDocument{
		EvidenceID:        e.EvidenceID,
		StudentID:         e.StudentID,
		ItemKey:           e.ItemKey,
		Source:            string(e.Source),
		OccurredAt:        e.OccurredAt,
		PracticeSessionID: e.PracticeSessionID,
		TriggerContext:    toTriggerContextDocument(e.TriggerContext),
		GraderID:          e.GraderID,
		Response: practiceResponseDocument{
			ResponseType:     string(r.Type),
			NoteName:         r.NoteName,
			String:           r.String,
			Fret:             r.Fret,
			OptionIDs:        r.OptionIDs,
			LatencyMs:        r.LatencyMs,
			AudioMs:          r.AudioMs,
			Rating:           string(r.Rating),
			TempoBPM:         r.TempoBPM,
			ChangesPerMinute: r.ChangesPerMinute,
		},
		Correct:          e.Correct,
		LatencyMs:        e.LatencyMs,
		AudioMs:          e.AudioMs,
		TapMs:            e.TapMs,
		Rating:           string(e.Rating),
		TempoBPM:         e.TempoBPM,
		ChangesPerMinute: e.ChangesPerMinute,
	}
}

func (d practiceEvidenceDocument) toDomain() domain.PracticeEvidence {
	r := d.Response
	return domain.PracticeEvidence{
		EvidenceID:        d.EvidenceID,
		StudentID:         d.StudentID,
		ItemKey:           d.ItemKey,
		Source:            domain.EvidenceSource(d.Source),
		OccurredAt:        d.OccurredAt.UTC(),
		PracticeSessionID: d.PracticeSessionID,
		TriggerContext:    d.TriggerContext.toDomain(),
		GraderID:          d.GraderID,
		Response: domain.PracticeResponse{
			Type:             domain.PracticeResponseType(r.ResponseType),
			NoteName:         r.NoteName,
			String:           r.String,
			Fret:             r.Fret,
			OptionIDs:        r.OptionIDs,
			LatencyMs:        r.LatencyMs,
			AudioMs:          r.AudioMs,
			Rating:           domain.SelfRating(r.Rating),
			TempoBPM:         r.TempoBPM,
			ChangesPerMinute: r.ChangesPerMinute,
		},
		Correct:          d.Correct,
		LatencyMs:        d.LatencyMs,
		AudioMs:          d.AudioMs,
		TapMs:            d.TapMs,
		Rating:           domain.SelfRating(d.Rating),
		TempoBPM:         d.TempoBPM,
		ChangesPerMinute: d.ChangesPerMinute,
	}
}

// practiceItemStateDocument is a `practice_item_state` document: one per
// (student_id, item_key), holding what the fold carries. core-domain reads it to
// compose sessions; level is the earned level, before the reader applies any lapse
// for an overdue review.
type practiceItemStateDocument struct {
	StudentID            string     `bson:"student_id"`
	ItemKey              string     `bson:"item_key"`
	RulesVersion         int        `bson:"rules_version"`
	Level                string     `bson:"level"`
	Attempts             int        `bson:"attempts"`
	Counted              int        `bson:"counted"`
	Accuracy             float64    `bson:"accuracy"`
	Fluency              float64    `bson:"fluency"`
	Box                  int        `bson:"box"`
	DueAt                *time.Time `bson:"due_at"`
	LastAt               *time.Time `bson:"last_at"`
	BestCleanBPM         *int       `bson:"best_clean_bpm"`
	BestChangesPerMinute *int       `bson:"best_changes_per_minute"`
	UpdatedAt            time.Time  `bson:"updated_at"`
}

func (d practiceItemStateDocument) toDomain() domain.ItemFold {
	return domain.ItemFold{
		Attempts:             d.Attempts,
		Counted:              d.Counted,
		Accuracy:             d.Accuracy,
		Fluency:              d.Fluency,
		Box:                  d.Box,
		DueAt:                utc(d.DueAt),
		LastAt:               utc(d.LastAt),
		BestCleanBPM:         d.BestCleanBPM,
		BestChangesPerMinute: d.BestChangesPerMinute,
	}
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
