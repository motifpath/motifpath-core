package repo

import (
	"fmt"
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// eventDocument mirrors the `events` collection schema from ADR-008: one flat
// document per event. Fields that don't apply to a given event_type are left zero
// and omitted from the stored document via omitempty.
type eventDocument struct {
	EventID    string    `bson:"event_id"`
	EventType  string    `bson:"event_type"`
	StudentID  string    `bson:"student_id"`
	SessionID  string    `bson:"session_id"`
	OccurredAt time.Time `bson:"occurred_at"`
	ReceivedAt time.Time `bson:"received_at"`

	ContentContext *contentContextDoc `bson:"content_context,omitempty"`
	ExerciseID     string             `bson:"exercise_id,omitempty"`
	TriggerContext *triggerContextDoc `bson:"trigger_context,omitempty"`
	Outcome        string             `bson:"outcome,omitempty"`
	FinalScore     *int               `bson:"final_score,omitempty"`

	// DurationSeconds is set on lesson.completed, ElapsedSeconds on exercise.progress.
	DurationSeconds *int `bson:"duration_seconds,omitempty"`
	ElapsedSeconds  *int `bson:"elapsed_seconds,omitempty"`

	// practice.* fields. The ones a practice event requires even at their
	// zero value -- answered_count, left_early, felt_ratings -- are pointers, so the
	// stored document always carries them for the event that has them.
	PracticeSessionID string               `bson:"practice_session_id,omitempty"`
	InstrumentID      string               `bson:"instrument_id,omitempty"`
	Minutes           *int                 `bson:"minutes,omitempty"`
	PlannedItems      []plannedItemDoc     `bson:"planned_items,omitempty"`
	ItemKey           string               `bson:"item_key,omitempty"`
	Response          *practiceResponseDoc `bson:"response,omitempty"`
	TapMs             *int                 `bson:"tap_ms,omitempty"`
	AnsweredCount     *int                 `bson:"answered_count,omitempty"`
	LeftEarly         *bool                `bson:"left_early,omitempty"`
	FeltRatings       *[]feltRatingDoc     `bson:"felt_ratings,omitempty"`
	MedianTapMs       *int                 `bson:"median_tap_ms,omitempty"`
	TapCount          *int                 `bson:"tap_count,omitempty"`

	// song_chart.* fields.
	SongChartContext  *songChartContextDoc `bson:"song_chart_context,omitempty"`
	AnchorID          string               `bson:"anchor_id,omitempty"`
	ChordDefinitionID string               `bson:"chord_definition_id,omitempty"`
	ChordVoicingID    string               `bson:"chord_voicing_id,omitempty"`
}

type songChartContextDoc struct {
	SongChartID    string `bson:"song_chart_id"`
	RevisionNumber int    `bson:"revision_number"`
}

type plannedItemDoc struct {
	ItemKey string `bson:"item_key"`
	Reason  string `bson:"reason"`
}

// practiceResponseDoc stores the raw response with only its shape's properties.
type practiceResponseDoc struct {
	ResponseType     string   `bson:"response_type"`
	NoteName         string   `bson:"note_name,omitempty"`
	Shape            string   `bson:"shape,omitempty"`
	Interval         string   `bson:"interval,omitempty"`
	String           *int     `bson:"string,omitempty"`
	Fret             *int     `bson:"fret,omitempty"`
	OptionIDs        []string `bson:"option_ids,omitempty"`
	LatencyMs        *int     `bson:"latency_ms,omitempty"`
	AudioMs          *int     `bson:"audio_ms,omitempty"`
	Rating           string   `bson:"rating,omitempty"`
	TempoBPM         *int     `bson:"tempo_bpm,omitempty"`
	ChangesPerMinute *int     `bson:"changes_per_minute,omitempty"`
}

type feltRatingDoc struct {
	DrillTemplateKey string `bson:"drill_template_key"`
	Felt             string `bson:"felt"`
}

type contentContextDoc struct {
	ContentNodeID string `bson:"content_node_id"`
	ContentType   string `bson:"content_type,omitempty"`
	TeacherID     string `bson:"teacher_id,omitempty"`
}

type triggerContextDoc struct {
	Source        string `bson:"source"`
	ContentNodeID string `bson:"content_node_id,omitempty"`
	ChallengeID   string `bson:"challenge_id,omitempty"`
}

func toDocument(event domain.TrackingEvent, receivedAt time.Time) eventDocument {
	base := event.Base()
	doc := eventDocument{
		EventID:    base.EventID,
		EventType:  string(base.EventType),
		StudentID:  base.StudentID,
		SessionID:  base.SessionID,
		OccurredAt: base.OccurredAt,
		ReceivedAt: receivedAt,
	}

	switch e := event.(type) {
	case domain.LessonStartedEvent:
		doc.ContentContext = toContentContextDoc(e.ContentContext)
	case domain.LessonResumedEvent:
		doc.ContentContext = toContentContextDoc(e.ContentContext)
	case domain.LessonCompletedEvent:
		doc.ContentContext = toContentContextDoc(e.ContentContext)
		doc.DurationSeconds = e.DurationSeconds
	case domain.ExerciseStartedEvent:
		doc.ExerciseID = e.ExerciseID
		doc.TriggerContext = toTriggerContextDoc(e.TriggerContext)
	case domain.ExerciseProgressEvent:
		doc.ExerciseID = e.ExerciseID
		doc.TriggerContext = toTriggerContextDoc(e.TriggerContext)
		doc.ElapsedSeconds = e.ElapsedSeconds
	case domain.ExerciseEndedEvent:
		doc.ExerciseID = e.ExerciseID
		doc.TriggerContext = toTriggerContextDoc(e.TriggerContext)
		doc.Outcome = string(e.Outcome)
		doc.FinalScore = e.FinalScore
	case domain.PracticeSessionStartedEvent:
		doc.PracticeSessionID = e.PracticeSessionID
		doc.InstrumentID = e.InstrumentID
		doc.Minutes = &e.Minutes
		doc.PlannedItems = make([]plannedItemDoc, 0, len(e.PlannedItems))
		for _, item := range e.PlannedItems {
			doc.PlannedItems = append(doc.PlannedItems, plannedItemDoc{ItemKey: item.ItemKey, Reason: string(item.Reason)})
		}
	case domain.PracticeItemAnsweredEvent:
		doc.PracticeSessionID = e.PracticeSessionID
		if e.TriggerContext != nil {
			doc.TriggerContext = toTriggerContextDoc(*e.TriggerContext)
		}
		doc.ItemKey = e.ItemKey
		doc.Response = toPracticeResponseDoc(e.Response)
		doc.TapMs = e.TapMs
	case domain.PracticeSessionEndedEvent:
		doc.PracticeSessionID = e.PracticeSessionID
		doc.AnsweredCount = &e.AnsweredCount
		doc.LeftEarly = &e.LeftEarly
		felt := make([]feltRatingDoc, 0, len(e.FeltRatings))
		for _, r := range e.FeltRatings {
			felt = append(felt, feltRatingDoc{DrillTemplateKey: r.DrillTemplateKey, Felt: string(r.Felt)})
		}
		doc.FeltRatings = &felt
	case domain.PracticeTapCheckCompletedEvent:
		doc.MedianTapMs = &e.MedianTapMs
		doc.TapCount = &e.TapCount
	case domain.SongChartOpenedEvent, domain.SongChartChordViewedEvent, domain.SongChartCompletedEvent:
		addSongChartDoc(&doc, event)
	}

	return doc
}

// fromDocument reconstructs the typed domain event a document represents, for
// the retry sweep and admin endpoints to republish a previously stored event.
// It is the inverse of toDocument.
func fromDocument(doc eventDocument) (domain.TrackingEvent, error) {
	base := domain.TrackingEventBase{
		EventID:    doc.EventID,
		EventType:  domain.EventType(doc.EventType),
		StudentID:  doc.StudentID,
		SessionID:  doc.SessionID,
		OccurredAt: doc.OccurredAt,
	}

	switch base.EventType {
	case domain.EventTypeLessonStarted:
		return domain.LessonStartedEvent{
			TrackingEventBase: base,
			ContentContext:    fromContentContextDoc(doc.ContentContext),
		}, nil
	case domain.EventTypeLessonResumed:
		return domain.LessonResumedEvent{
			TrackingEventBase: base,
			ContentContext:    fromContentContextDoc(doc.ContentContext),
		}, nil
	case domain.EventTypeLessonCompleted:
		return domain.LessonCompletedEvent{
			TrackingEventBase: base,
			ContentContext:    fromContentContextDoc(doc.ContentContext),
			DurationSeconds:   doc.DurationSeconds,
		}, nil
	case domain.EventTypeExerciseStarted:
		return domain.ExerciseStartedEvent{
			TrackingEventBase: base,
			ExerciseID:        doc.ExerciseID,
			TriggerContext:    fromTriggerContextDoc(doc.TriggerContext),
		}, nil
	case domain.EventTypeExerciseProgress:
		return domain.ExerciseProgressEvent{
			TrackingEventBase: base,
			ExerciseID:        doc.ExerciseID,
			TriggerContext:    fromTriggerContextDoc(doc.TriggerContext),
			ElapsedSeconds:    doc.ElapsedSeconds,
		}, nil
	case domain.EventTypeExerciseEnded:
		return domain.ExerciseEndedEvent{
			TrackingEventBase: base,
			ExerciseID:        doc.ExerciseID,
			TriggerContext:    fromTriggerContextDoc(doc.TriggerContext),
			Outcome:           domain.ExerciseOutcome(doc.Outcome),
			FinalScore:        doc.FinalScore,
		}, nil
	case domain.EventTypePracticeSessionStarted:
		return fromPracticeSessionStartedDoc(base, doc), nil
	case domain.EventTypePracticeItemAnswered:
		return fromPracticeItemAnsweredDoc(base, doc), nil
	case domain.EventTypePracticeSessionEnded:
		return fromPracticeSessionEndedDoc(base, doc), nil
	case domain.EventTypePracticeTapCheckCompleted:
		return fromPracticeTapCheckCompletedDoc(base, doc), nil
	case domain.EventTypeSongChartOpened:
		return domain.SongChartOpenedEvent{TrackingEventBase: base, SongChartContext: fromSongChartContextDoc(doc.SongChartContext)}, nil
	case domain.EventTypeSongChartChordViewed:
		return domain.SongChartChordViewedEvent{
			TrackingEventBase: base,
			SongChartContext:  fromSongChartContextDoc(doc.SongChartContext),
			AnchorID:          doc.AnchorID,
			ChordDefinitionID: doc.ChordDefinitionID,
			ChordVoicingID:    doc.ChordVoicingID,
		}, nil
	case domain.EventTypeSongChartCompleted:
		return domain.SongChartCompletedEvent{TrackingEventBase: base, SongChartContext: fromSongChartContextDoc(doc.SongChartContext)}, nil
	default:
		return nil, fmt.Errorf("%w: %q", domain.ErrInvalidEventType, doc.EventType)
	}
}

func fromContentContextDoc(d *contentContextDoc) domain.ContentContext {
	if d == nil {
		return domain.ContentContext{}
	}
	return domain.ContentContext{
		ContentNodeID: d.ContentNodeID,
		ContentType:   domain.ContentType(d.ContentType),
		TeacherID:     d.TeacherID,
	}
}

func fromTriggerContextDoc(d *triggerContextDoc) domain.TriggerContext {
	if d == nil {
		return domain.TriggerContext{}
	}
	return domain.TriggerContext{
		Source:        domain.TriggerSource(d.Source),
		ContentNodeID: d.ContentNodeID,
		ChallengeID:   d.ChallengeID,
	}
}

func toContentContextDoc(cc domain.ContentContext) *contentContextDoc {
	return &contentContextDoc{
		ContentNodeID: cc.ContentNodeID,
		ContentType:   string(cc.ContentType),
		TeacherID:     cc.TeacherID,
	}
}

func toTriggerContextDoc(tc domain.TriggerContext) *triggerContextDoc {
	return &triggerContextDoc{
		Source:        string(tc.Source),
		ContentNodeID: tc.ContentNodeID,
		ChallengeID:   tc.ChallengeID,
	}
}

func toPracticeResponseDoc(r domain.PracticeResponse) *practiceResponseDoc {
	return &practiceResponseDoc{
		ResponseType:     string(r.Type),
		NoteName:         r.NoteName,
		Shape:            r.Shape,
		Interval:         r.Interval,
		String:           r.String,
		Fret:             r.Fret,
		OptionIDs:        r.OptionIDs,
		LatencyMs:        r.LatencyMs,
		AudioMs:          r.AudioMs,
		Rating:           string(r.Rating),
		TempoBPM:         r.TempoBPM,
		ChangesPerMinute: r.ChangesPerMinute,
	}
}

func fromPracticeResponseDoc(d *practiceResponseDoc) domain.PracticeResponse {
	if d == nil {
		return domain.PracticeResponse{}
	}
	return domain.PracticeResponse{
		Type:             domain.PracticeResponseType(d.ResponseType),
		NoteName:         d.NoteName,
		Shape:            d.Shape,
		Interval:         d.Interval,
		String:           d.String,
		Fret:             d.Fret,
		OptionIDs:        d.OptionIDs,
		LatencyMs:        d.LatencyMs,
		AudioMs:          d.AudioMs,
		Rating:           domain.SelfRating(d.Rating),
		TempoBPM:         d.TempoBPM,
		ChangesPerMinute: d.ChangesPerMinute,
	}
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func fromPracticeSessionStartedDoc(base domain.TrackingEventBase, doc eventDocument) domain.PracticeSessionStartedEvent {
	planned := make([]domain.PlannedPracticeItem, 0, len(doc.PlannedItems))
	for _, item := range doc.PlannedItems {
		planned = append(planned, domain.PlannedPracticeItem{ItemKey: item.ItemKey, Reason: domain.PracticePickReason(item.Reason)})
	}
	return domain.PracticeSessionStartedEvent{
		TrackingEventBase: base,
		PracticeSessionID: doc.PracticeSessionID,
		InstrumentID:      doc.InstrumentID,
		Minutes:           derefInt(doc.Minutes),
		PlannedItems:      planned,
	}
}

func fromPracticeItemAnsweredDoc(base domain.TrackingEventBase, doc eventDocument) domain.PracticeItemAnsweredEvent {
	return domain.PracticeItemAnsweredEvent{
		TrackingEventBase: base,
		PracticeSessionID: doc.PracticeSessionID,
		TriggerContext:    fromAnswerTriggerContextDoc(doc.TriggerContext),
		ItemKey:           doc.ItemKey,
		Response:          fromPracticeResponseDoc(doc.Response),
		TapMs:             doc.TapMs,
	}
}

// fromAnswerTriggerContextDoc keeps an answer's trigger context absent when the
// answer was given in a practice session.
func fromAnswerTriggerContextDoc(d *triggerContextDoc) *domain.TriggerContext {
	if d == nil {
		return nil
	}
	tc := fromTriggerContextDoc(d)
	return &tc
}

func fromPracticeSessionEndedDoc(base domain.TrackingEventBase, doc eventDocument) domain.PracticeSessionEndedEvent {
	felt := []domain.FeltRating{}
	if doc.FeltRatings != nil {
		for _, r := range *doc.FeltRatings {
			felt = append(felt, domain.FeltRating{DrillTemplateKey: r.DrillTemplateKey, Felt: domain.Felt(r.Felt)})
		}
	}
	return domain.PracticeSessionEndedEvent{
		TrackingEventBase: base,
		PracticeSessionID: doc.PracticeSessionID,
		AnsweredCount:     derefInt(doc.AnsweredCount),
		LeftEarly:         doc.LeftEarly != nil && *doc.LeftEarly,
		FeltRatings:       felt,
	}
}

func fromPracticeTapCheckCompletedDoc(base domain.TrackingEventBase, doc eventDocument) domain.PracticeTapCheckCompletedEvent {
	return domain.PracticeTapCheckCompletedEvent{
		TrackingEventBase: base,
		MedianTapMs:       derefInt(doc.MedianTapMs),
		TapCount:          derefInt(doc.TapCount),
	}
}

// addSongChartDoc sets the fields of a song_chart.* event.
func addSongChartDoc(doc *eventDocument, event domain.TrackingEvent) {
	switch e := event.(type) {
	case domain.SongChartOpenedEvent:
		doc.SongChartContext = toSongChartContextDoc(e.SongChartContext)
	case domain.SongChartChordViewedEvent:
		doc.SongChartContext = toSongChartContextDoc(e.SongChartContext)
		doc.AnchorID = e.AnchorID
		doc.ChordDefinitionID = e.ChordDefinitionID
		doc.ChordVoicingID = e.ChordVoicingID
	case domain.SongChartCompletedEvent:
		doc.SongChartContext = toSongChartContextDoc(e.SongChartContext)
	}
}

func toSongChartContextDoc(c domain.SongChartContext) *songChartContextDoc {
	return &songChartContextDoc{SongChartID: c.SongChartID, RevisionNumber: c.RevisionNumber}
}

func fromSongChartContextDoc(d *songChartContextDoc) domain.SongChartContext {
	if d == nil {
		return domain.SongChartContext{}
	}
	return domain.SongChartContext{SongChartID: d.SongChartID, RevisionNumber: d.RevisionNumber}
}
