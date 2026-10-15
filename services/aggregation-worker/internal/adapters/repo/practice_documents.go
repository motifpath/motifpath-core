package repo

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// practiceResponseDocument stores the raw response flat, with only its shape's
// fields, the way ingestion publishes it.
type practiceResponseDocument struct {
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
	NoteValue        *string  `bson:"note_value,omitempty"`
	ChangesPerMinute *int     `bson:"changes_per_minute,omitempty"`
	Choices          []string `bson:"choices,omitempty"`
	Reason           string   `bson:"reason,omitempty"`
	Way              string   `bson:"way,omitempty"`
	MusicKey         string   `bson:"music_key,omitempty"`
	Options          []string `bson:"options,omitempty"`
	Choice           string   `bson:"choice,omitempty"`
	Tone             string   `bson:"tone,omitempty"`
	// Taps and Cells are pointers so an empty list is kept: the response is stored
	// exactly as sent, and a take with no taps still sends taps.
	Taps       *[]timedTapDocument   `bson:"taps,omitempty"`
	FretWindow *fretWindowDocument   `bson:"fret_window,omitempty"`
	Cells      *[]answerCellDocument `bson:"cells,omitempty"`
}

// timedTapDocument is one tap of a take, after the first bar's downbeat.
type timedTapDocument struct {
	AtMs   int `bson:"at_ms"`
	String int `bson:"string"`
	Fret   int `bson:"fret"`
}

// fretWindowDocument is a fret window, by its lowest fret.
type fretWindowDocument struct {
	FromFret int `bson:"from_fret"`
}

func toFretWindowDocument(from *int) *fretWindowDocument {
	if from == nil {
		return nil
	}
	return &fretWindowDocument{FromFret: *from}
}

func (d *fretWindowDocument) toDomain() *int {
	if d == nil {
		return nil
	}
	from := d.FromFret
	return &from
}

func toCellDocuments(cells []domain.AnswerCell) []answerCellDocument {
	var docs []answerCellDocument
	for _, c := range cells {
		docs = append(docs, answerCellDocument(c))
	}
	return docs
}

func toDomainCells(docs []answerCellDocument) []domain.AnswerCell {
	var cells []domain.AnswerCell
	for _, c := range docs {
		cells = append(cells, domain.AnswerCell(c))
	}
	return cells
}

// toTapDocuments keeps nil as absent and an empty list as empty.
func toTapDocuments(taps []domain.TimedTap) *[]timedTapDocument {
	if taps == nil {
		return nil
	}
	docs := make([]timedTapDocument, len(taps))
	for i, t := range taps {
		docs[i] = timedTapDocument(t)
	}
	return &docs
}

func toDomainTaps(docs *[]timedTapDocument) []domain.TimedTap {
	if docs == nil {
		return nil
	}
	taps := make([]domain.TimedTap, len(*docs))
	for i, t := range *docs {
		taps[i] = domain.TimedTap(t)
	}
	return taps
}

// toKeptCellDocuments keeps nil as absent and an empty list as empty, as a
// response's cells must be.
func toKeptCellDocuments(cells []domain.AnswerCell) *[]answerCellDocument {
	if cells == nil {
		return nil
	}
	docs := make([]answerCellDocument, len(cells))
	for i, c := range cells {
		docs[i] = answerCellDocument(c)
	}
	return &docs
}

func toKeptDomainCells(docs *[]answerCellDocument) []domain.AnswerCell {
	if docs == nil {
		return nil
	}
	cells := make([]domain.AnswerCell, len(*docs))
	for i, c := range *docs {
		cells[i] = domain.AnswerCell(c)
	}
	return cells
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
	NoteValue         *string                  `bson:"note_value,omitempty"`
	ChangesPerMinute  *int                     `bson:"changes_per_minute,omitempty"`
	AnswerKey         *answerKeyDocument       `bson:"answer_key,omitempty"`
}

// answerKeyDocument is what a right answer was: the asked cell and its note (with
// the choices it was named among), every exercise option as the student was shown
// it (stored as core kept it), a shape's family and member, the asked degree and
// where it is in the shape, a generated question's answer and options, a
// guide-tone take's bars, or every place of a note in a fret window.
type answerKeyDocument struct {
	String      *int                 `bson:"string,omitempty"`
	Fret        *int                 `bson:"fret,omitempty"`
	NoteName    string               `bson:"note_name,omitempty"`
	Options     []bson.Raw           `bson:"options,omitempty"`
	ShapeFamily string               `bson:"shape_family,omitempty"`
	Shape       string               `bson:"shape,omitempty"`
	Interval    string               `bson:"interval,omitempty"`
	Cells       []answerCellDocument `bson:"cells,omitempty"`
	Choices     []string             `bson:"choices,omitempty"`
	FretWindow  *fretWindowDocument  `bson:"fret_window,omitempty"`
	Way         string               `bson:"way,omitempty"`
	MusicKey    string               `bson:"music_key,omitempty"`
	Answer      string               `bson:"answer,omitempty"`
	Offered     []string             `bson:"offered,omitempty"`
	Tone        string               `bson:"tone,omitempty"`
	Bars        []answerBarDocument  `bson:"bars,omitempty"`
}

// answerBarDocument is one bar of a guide-tone take, graded.
type answerBarDocument struct {
	Chord    string `bson:"chord"`
	NoteName string `bson:"note_name"`
	Hit      bool   `bson:"hit"`
}

// answerCellDocument is a string and fret where a right answer was.
type answerCellDocument struct {
	String int `bson:"string"`
	Fret   int `bson:"fret"`
}

func toAnswerKeyDocument(k *domain.AnswerKey) (*answerKeyDocument, error) {
	if k == nil {
		return nil, nil
	}
	doc := &answerKeyDocument{String: k.String, Fret: k.Fret, NoteName: k.NoteName, ShapeFamily: k.ShapeFamily, Shape: k.Shape, Interval: k.Interval,
		Cells: toCellDocuments(k.Cells), Choices: k.Choices, FretWindow: toFretWindowDocument(k.FretWindowFrom),
		Way: k.Way, MusicKey: k.MusicKey, Answer: k.Answer, Offered: k.Offered, Tone: k.Tone}
	for _, b := range k.Bars {
		doc.Bars = append(doc.Bars, answerBarDocument(b))
	}
	for _, o := range k.Options {
		raw := bson.Raw(o.Shown)
		if len(raw) == 0 {
			var err error
			if raw, err = bson.Marshal(bson.D{{Key: "option_id", Value: o.OptionID}, {Key: "is_correct", Value: o.IsCorrect}}); err != nil {
				return nil, err
			}
		}
		doc.Options = append(doc.Options, raw)
	}
	return doc, nil
}

func (d *answerKeyDocument) toDomain() (*domain.AnswerKey, error) {
	if d == nil {
		return nil, nil
	}
	options, err := answerOptions(d.Options)
	if err != nil {
		return nil, err
	}
	key := &domain.AnswerKey{String: d.String, Fret: d.Fret, NoteName: d.NoteName, Options: options, ShapeFamily: d.ShapeFamily, Shape: d.Shape, Interval: d.Interval,
		Cells: toDomainCells(d.Cells), Choices: d.Choices, FretWindowFrom: d.FretWindow.toDomain(),
		Way: d.Way, MusicKey: d.MusicKey, Answer: d.Answer, Offered: d.Offered, Tone: d.Tone}
	for _, b := range d.Bars {
		key.Bars = append(key.Bars, domain.AnswerBar(b))
	}
	return key, nil
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

func toEvidenceDocument(e domain.PracticeEvidence) (practiceEvidenceDocument, error) {
	answerKey, err := toAnswerKeyDocument(e.AnswerKey)
	if err != nil {
		return practiceEvidenceDocument{}, err
	}
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
			Shape:            r.Shape,
			Interval:         r.Interval,
			String:           r.String,
			Fret:             r.Fret,
			OptionIDs:        r.OptionIDs,
			LatencyMs:        r.LatencyMs,
			AudioMs:          r.AudioMs,
			Rating:           string(r.Rating),
			TempoBPM:         r.TempoBPM,
			NoteValue:        noteValueString(r.NoteValue),
			ChangesPerMinute: r.ChangesPerMinute,
			Choices:          r.Choices,
			Reason:           string(r.Reason),
			Way:              r.Way,
			MusicKey:         r.MusicKey,
			Options:          r.Options,
			Choice:           r.Choice,
			Tone:             r.Tone,
			Taps:             toTapDocuments(r.Taps),
			FretWindow:       toFretWindowDocument(r.FretWindowFrom),
			Cells:            toKeptCellDocuments(r.Cells),
		},
		Correct:          e.Correct,
		LatencyMs:        e.LatencyMs,
		AudioMs:          e.AudioMs,
		TapMs:            e.TapMs,
		Rating:           string(e.Rating),
		TempoBPM:         e.TempoBPM,
		NoteValue:        noteValueString(e.NoteValue),
		ChangesPerMinute: e.ChangesPerMinute,
		AnswerKey:        answerKey,
	}, nil
}

func (d practiceEvidenceDocument) toDomain() (domain.PracticeEvidence, error) {
	answerKey, err := d.AnswerKey.toDomain()
	if err != nil {
		return domain.PracticeEvidence{}, err
	}
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
			Shape:            r.Shape,
			Interval:         r.Interval,
			String:           r.String,
			Fret:             r.Fret,
			OptionIDs:        r.OptionIDs,
			LatencyMs:        r.LatencyMs,
			AudioMs:          r.AudioMs,
			Rating:           domain.SelfRating(r.Rating),
			TempoBPM:         r.TempoBPM,
			NoteValue:        domainNoteValue(r.NoteValue),
			ChangesPerMinute: r.ChangesPerMinute,
			Choices:          r.Choices,
			Reason:           domain.NotAnsweredReason(r.Reason),
			Way:              r.Way,
			MusicKey:         r.MusicKey,
			Options:          r.Options,
			Choice:           r.Choice,
			Tone:             r.Tone,
			Taps:             toDomainTaps(r.Taps),
			FretWindowFrom:   r.FretWindow.toDomain(),
			Cells:            toKeptDomainCells(r.Cells),
		},
		Correct:          d.Correct,
		LatencyMs:        d.LatencyMs,
		AudioMs:          d.AudioMs,
		TapMs:            d.TapMs,
		Rating:           domain.SelfRating(d.Rating),
		TempoBPM:         d.TempoBPM,
		NoteValue:        domainNoteValue(d.NoteValue),
		ChangesPerMinute: d.ChangesPerMinute,
		AnswerKey:        answerKey,
	}, nil
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
	BestCleanNoteValue   *string    `bson:"best_clean_note_value"`
	BestChangesPerMinute *int       `bson:"best_changes_per_minute"`
	// RightByResponse counts right answers by the way the item was asked.
	RightByResponse map[string]int `bson:"right_by_response,omitempty"`
	UpdatedAt       time.Time      `bson:"updated_at"`
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
		BestCleanNoteValue:   domainNoteValue(d.BestCleanNoteValue),
		BestChangesPerMinute: d.BestChangesPerMinute,
		RightByResponse:      d.RightByResponse,
	}
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// noteValueString is v as stored, or nil without a note value.
func noteValueString(v *domain.NoteValue) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

// domainNoteValue is a stored note value back in the domain, or nil when none was stored.
func domainNoteValue(s *string) *domain.NoteValue {
	if s == nil {
		return nil
	}
	v := domain.NoteValue(*s)
	return &v
}
