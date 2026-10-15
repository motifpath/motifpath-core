package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

// The practice.* mappers validate what events.yaml and practice.yaml constrain beyond
// field presence — ranges, patterns, enums, array sizes — because nothing upstream of
// them checks a request against the schema. The answer response gets the strictest
// treatment: it is graded downstream, and its schema closes it (additionalProperties:
// false) so a client can never slip a verdict such as "correct" into it.
//
// A required field whose zero value is valid (0, false, an empty list) decodes the
// same whether it was sent or left out, so requirePresent checks those against the
// raw body: a client that drops one must be refused, not stored as a real zero.

const (
	maxPracticeMinutes = 60
	maxFeltRatings     = 2
	maxTempoBPM        = 400
	maxChangesPerMin   = 600
	noteChoices        = 4
	minOptions         = 2
	maxOptions         = 8
	maxOptionLength    = 120
	maxTaps            = 400
	maxTappedCells     = 72
)

var (
	noteNamePattern         = regexp.MustCompile(`^[A-G][#b]?$`)
	drillTemplateKeyPattern = regexp.MustCompile(`^[a-z][a-z_]*:[a-z][a-z_]*$`)
	// shapeKeyPattern is a shape family member's key, such as A, 2 or minor.
	shapeKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)
)

// generatedChoiceWays are the ways a generated question can be asked.
var generatedChoiceWays = map[string]bool{
	"numeral_to_chord": true, "chord_to_numeral": true, "chord_to_function": true, "function_to_chord": true,
	"chart_to_numbers": true, "numbers_to_chart": true, "cadence_then_chord": true, "progression_by_number": true,
}

// musicKeys are the major keys, spelled as the chord catalog spells roots.
var musicKeys = map[string]bool{
	"C": true, "Db": true, "D": true, "Eb": true, "E": true, "F": true,
	"F#": true, "G": true, "Ab": true, "A": true, "Bb": true, "B": true,
}

// fretWindowStarts are the lowest frets a fret window can start at; never 1, since
// a window at the nut takes the open string too.
var fretWindowStarts = map[int]bool{0: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true}

// shapeIntervals are the interval codes a diagram position can have.
var shapeIntervals = map[string]bool{
	"R": true, "b2": true, "2": true, "#2": true, "b3": true, "3": true, "4": true, "#4": true,
	"b5": true, "5": true, "#5": true, "b6": true, "6": true, "bb7": true, "b7": true, "7": true,
	"b9": true, "9": true, "#9": true, "11": true, "#11": true, "b13": true, "13": true,
}

func toPracticeSessionStartedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsPracticeSessionStartedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	practiceSessionID, err := requireUUID(v.PracticeSessionId, "practice_session_id")
	if err != nil {
		return nil, err
	}
	if err := requirePresent(body, "minutes"); err != nil {
		return nil, err
	}
	if v.Minutes < 1 || v.Minutes > maxPracticeMinutes {
		return nil, fmt.Errorf("%w: minutes must be between 1 and %d", domain.ErrInvalidField, maxPracticeMinutes)
	}
	if len(v.PlannedItems) == 0 {
		return nil, fmt.Errorf("%w: planned_items", domain.ErrMissingRequiredField)
	}
	planned := make([]domain.PlannedPracticeItem, 0, len(v.PlannedItems))
	for i, item := range v.PlannedItems {
		if !domain.ValidPracticeItemKey(item.ItemKey) {
			return nil, fmt.Errorf("%w: planned_items[%d].item_key", domain.ErrInvalidField, i)
		}
		reason := domain.PracticePickReason(item.Reason)
		if !validPickReason(reason) {
			return nil, fmt.Errorf("%w: planned_items[%d].reason", domain.ErrInvalidField, i)
		}
		planned = append(planned, domain.PlannedPracticeItem{ItemKey: item.ItemKey, Reason: reason})
	}
	var instrumentID string
	if v.InstrumentId != nil {
		instrumentID = v.InstrumentId.String()
	}
	return domain.PracticeSessionStartedEvent{
		TrackingEventBase: base,
		PracticeSessionID: practiceSessionID,
		InstrumentID:      instrumentID,
		Minutes:           v.Minutes,
		PlannedItems:      planned,
	}, nil
}

// toPracticeItemAnsweredEvent never carries over the body's tap_ms: it is readOnly in
// the spec, and the application stamps the server's value.
func toPracticeItemAnsweredEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsPracticeItemAnsweredEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	practiceSessionID, trigger, err := toAnswerContext(v.PracticeSessionId, v.TriggerContext)
	if err != nil {
		return nil, err
	}
	if v.ItemKey == "" {
		return nil, fmt.Errorf("%w: item_key", domain.ErrMissingRequiredField)
	}
	if !domain.ValidPracticeItemKey(v.ItemKey) {
		return nil, fmt.Errorf("%w: item_key matches no item kind", domain.ErrInvalidField)
	}
	raw, err := v.Response.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("%w: response", domain.ErrInvalidField)
	}
	response, err := toDomainPracticeResponse(raw)
	if err != nil {
		return nil, err
	}
	return domain.PracticeItemAnsweredEvent{
		TrackingEventBase: base,
		PracticeSessionID: practiceSessionID,
		TriggerContext:    trigger,
		ItemKey:           v.ItemKey,
		Response:          response,
	}, nil
}

// toAnswerContext reads the one context an answer belongs to: its practice session, or
// the trigger context of an exercise answered elsewhere on the platform.
func toAnswerContext(sessionID *openapi_types.UUID, tc *generated.TriggerContext) (string, *domain.TriggerContext, error) {
	switch {
	case sessionID != nil && tc != nil:
		return "", nil, fmt.Errorf("%w: trigger_context must be absent when practice_session_id is present", domain.ErrInvalidField)
	case tc != nil:
		trigger, err := toDomainTriggerContext(*tc)
		if err != nil {
			return "", nil, err
		}
		return "", &trigger, nil
	case sessionID != nil:
		id, err := requireUUID(*sessionID, "practice_session_id")
		return id, nil, err
	default:
		return "", nil, fmt.Errorf("%w: practice_session_id", domain.ErrMissingRequiredField)
	}
}

func toPracticeSessionEndedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsPracticeSessionEndedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	practiceSessionID, err := requireUUID(v.PracticeSessionId, "practice_session_id")
	if err != nil {
		return nil, err
	}
	if err := requirePresent(body, "answered_count", "left_early", "felt_ratings"); err != nil {
		return nil, err
	}
	if v.AnsweredCount < 0 {
		return nil, fmt.Errorf("%w: answered_count", domain.ErrInvalidField)
	}
	if len(v.FeltRatings) > maxFeltRatings {
		return nil, fmt.Errorf("%w: felt_ratings holds at most %d ratings", domain.ErrInvalidField, maxFeltRatings)
	}
	felt := make([]domain.FeltRating, 0, len(v.FeltRatings))
	for i, r := range v.FeltRatings {
		if !drillTemplateKeyPattern.MatchString(r.DrillTemplateKey) {
			return nil, fmt.Errorf("%w: felt_ratings[%d].drill_template_key", domain.ErrInvalidField, i)
		}
		f := domain.Felt(r.Felt)
		switch f {
		case domain.FeltEasy, domain.FeltAboutRight, domain.FeltHard:
		default:
			return nil, fmt.Errorf("%w: felt_ratings[%d].felt", domain.ErrInvalidField, i)
		}
		felt = append(felt, domain.FeltRating{DrillTemplateKey: r.DrillTemplateKey, Felt: f})
	}
	return domain.PracticeSessionEndedEvent{
		TrackingEventBase: base,
		PracticeSessionID: practiceSessionID,
		AnsweredCount:     v.AnsweredCount,
		LeftEarly:         v.LeftEarly,
		FeltRatings:       felt,
	}, nil
}

func toPracticeTapCheckCompletedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsPracticeTapCheckCompletedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	if err := requirePresent(body, "median_tap_ms", "tap_count"); err != nil {
		return nil, err
	}
	if v.MedianTapMs < 0 {
		return nil, fmt.Errorf("%w: median_tap_ms", domain.ErrInvalidField)
	}
	if v.TapCount < 1 {
		return nil, fmt.Errorf("%w: tap_count", domain.ErrInvalidField)
	}
	return domain.PracticeTapCheckCompletedEvent{
		TrackingEventBase: base,
		MedianTapMs:       v.MedianTapMs,
		TapCount:          v.TapCount,
	}, nil
}

// requirePresent reports the first of fields that the body leaves out or sends as null.
func requirePresent(body *generated.TrackingEvent, fields ...string) error {
	raw, err := body.MarshalJSON()
	if err != nil {
		return fmt.Errorf("%w: request body", domain.ErrMissingRequiredField)
	}
	var present map[string]json.RawMessage
	if err := json.Unmarshal(raw, &present); err != nil {
		return fmt.Errorf("%w: request body", domain.ErrMissingRequiredField)
	}
	for _, field := range fields {
		value, ok := present[field]
		if !ok || bytes.Equal(value, []byte("null")) {
			return fmt.Errorf("%w: %s", domain.ErrMissingRequiredField, field)
		}
	}
	return nil
}

func validPickReason(r domain.PracticePickReason) bool {
	switch r {
	case domain.PracticePickReasonTeacherSuggested, domain.PracticePickReasonDue, domain.PracticePickReasonWeak,
		domain.PracticePickReasonNew, domain.PracticePickReasonWarmUp, domain.PracticePickReasonApplication,
		domain.PracticePickReasonReviewAhead, domain.PracticePickReasonStretch:
		return true
	default:
		return false
	}
}

// rawPracticeResponse holds every property any response shape may carry, as pointers,
// so the decoder can tell an absent property from a zero one. Decoding rejects any
// other property; toDomainPracticeResponse then rejects a property that belongs to a
// different shape than response_type names.
type rawPracticeResponse struct {
	ResponseType     string   `json:"response_type"`
	NoteName         *string  `json:"note_name"`
	Shape            *string  `json:"shape"`
	Interval         *string  `json:"interval"`
	String           *int     `json:"string"`
	Fret             *int     `json:"fret"`
	OptionIDs        []string `json:"option_ids"`
	LatencyMs        *int     `json:"latency_ms"`
	AudioMs          *int     `json:"audio_ms"`
	Rating           *string  `json:"rating"`
	TempoBPM         *int     `json:"tempo_bpm"`
	NoteValue        *string  `json:"note_value"`
	ChangesPerMinute *int     `json:"changes_per_minute"`
	Choices          []string `json:"choices"`
	Reason           *string  `json:"reason"`
	Way              *string  `json:"way"`
	MusicKey         *string  `json:"music_key"`
	Options          []string `json:"options"`
	Choice           *string  `json:"choice"`
	Tone             *string  `json:"tone"`
	Taps             []rawTap `json:"taps"`
	FretWindow       *struct {
		FromFret *int `json:"from_fret"`
	} `json:"fret_window"`
	Cells []rawCell `json:"cells"`
}

// rawTap and rawCell hold a tap's properties as pointers, so a missing one is refused.
type rawTap struct {
	AtMs   *int `json:"at_ms"`
	String *int `json:"string"`
	Fret   *int `json:"fret"`
}

type rawCell struct {
	String *int `json:"string"`
	Fret   *int `json:"fret"`
}

// strayProperty is the first property r carries that isn't one of shape's
// properties, or "" when it carries none.
func (r rawPracticeResponse) strayProperty(shape ...string) string {
	carried := []struct {
		name    string
		present bool
	}{
		{"note_name", r.NoteName != nil}, {"shape", r.Shape != nil}, {"interval", r.Interval != nil},
		{"string", r.String != nil}, {"fret", r.Fret != nil}, {"option_ids", r.OptionIDs != nil},
		{"latency_ms", r.LatencyMs != nil}, {"audio_ms", r.AudioMs != nil}, {"rating", r.Rating != nil},
		{"tempo_bpm", r.TempoBPM != nil}, {"note_value", r.NoteValue != nil},
		{"changes_per_minute", r.ChangesPerMinute != nil},
		{"choices", r.Choices != nil}, {"reason", r.Reason != nil}, {"way", r.Way != nil},
		{"music_key", r.MusicKey != nil}, {"options", r.Options != nil}, {"choice", r.Choice != nil},
		{"tone", r.Tone != nil}, {"taps", r.Taps != nil}, {"fret_window", r.FretWindow != nil},
		{"cells", r.Cells != nil},
	}
	for _, p := range carried {
		if p.present && !slices.Contains(shape, p.name) {
			return p.name
		}
	}
	return ""
}

func toDomainPracticeResponse(raw []byte) (domain.PracticeResponse, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return domain.PracticeResponse{}, fmt.Errorf("%w: response", domain.ErrMissingRequiredField)
	}

	var r rawPracticeResponse
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return invalidResponse(err.Error())
	}
	if r.LatencyMs != nil && *r.LatencyMs < 0 {
		return invalidResponse("latency_ms must not be negative")
	}
	if r.AudioMs != nil && *r.AudioMs < 0 {
		return invalidResponse("audio_ms must not be negative")
	}
	if t := domain.PracticeResponseType(r.ResponseType); r.AudioMs != nil &&
		t != domain.PracticeResponseOptionChoice && t != domain.PracticeResponseGeneratedChoice {
		return invalidResponse("audio_ms belongs to option_choice and generated_choice only")
	}

	toShape, ok := responseShapes[domain.PracticeResponseType(r.ResponseType)]
	if !ok {
		return invalidResponse(fmt.Sprintf("response_type %q is not a known response", r.ResponseType))
	}
	return toShape(r)
}

// responseShapes reads each response shape, by its response_type.
var responseShapes = map[domain.PracticeResponseType]func(rawPracticeResponse) (domain.PracticeResponse, error){
	domain.PracticeResponseNameTheNote:     toNameTheNoteResponse,
	domain.PracticeResponseFindTheNote:     toFindTheNoteResponse,
	domain.PracticeResponseOptionChoice:    toOptionChoiceResponse,
	domain.PracticeResponseSelfRating:      toSelfRatingResponse,
	domain.PracticeResponseNameTheShape:    toNameTheShapeResponse,
	domain.PracticeResponseFindTheDegree:   toFindTheDegreeResponse,
	domain.PracticeResponseGeneratedChoice: toGeneratedChoiceResponse,
	domain.PracticeResponseTimedTaps:       toTimedTapsResponse,
	domain.PracticeResponseMultiTap:        toMultiTapResponse,
	domain.PracticeResponseNotAnswered:     toNotAnsweredResponse,
}

func invalidResponse(why string) (domain.PracticeResponse, error) {
	return domain.PracticeResponse{}, fmt.Errorf("%w: response %s", domain.ErrInvalidField, why)
}

func toNameTheNoteResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("note_name", "choices", "latency_ms"); property != "" {
		return invalidResponse(property + " is not a property of name_the_note")
	}
	if r.NoteName == nil || !noteNamePattern.MatchString(*r.NoteName) {
		return invalidResponse("note_name must be a note letter with an optional # or b")
	}
	if r.Choices != nil {
		if len(r.Choices) != noteChoices || !distinct(r.Choices) {
			return invalidResponse(fmt.Sprintf("choices must be %d different notes", noteChoices))
		}
		for _, c := range r.Choices {
			if !noteNamePattern.MatchString(c) {
				return invalidResponse("choices must be note letters with an optional # or b")
			}
		}
	}
	if r.LatencyMs == nil {
		return invalidResponse("latency_ms is required")
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: *r.NoteName, Choices: r.Choices, LatencyMs: r.LatencyMs}, nil
}

func toNotAnsweredResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("reason"); property != "" {
		return invalidResponse(property + " is not a property of not_answered")
	}
	if r.Reason == nil {
		return invalidResponse("reason is required")
	}
	reason := domain.NotAnsweredReason(*r.Reason)
	switch reason {
	case domain.NotAnsweredFailedToLoad, domain.NotAnsweredUnavailable:
	default:
		return invalidResponse("reason must be failed_to_load or unavailable")
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseNotAnswered, Reason: reason}, nil
}

func toGeneratedChoiceResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("way", "music_key", "options", "choice", "latency_ms", "audio_ms"); property != "" {
		return invalidResponse(property + " is not a property of generated_choice")
	}
	if r.Way == nil || !generatedChoiceWays[*r.Way] {
		return invalidResponse("way must be a way a generated question is asked")
	}
	if r.MusicKey != nil && !musicKeys[*r.MusicKey] {
		return invalidResponse("music_key must be a major key spelled as C, Db, D, Eb, E, F, F#, G, Ab, A, Bb or B")
	}
	if why := checkOptions(r.Options); why != "" {
		return invalidResponse(why)
	}
	if r.Choice == nil || *r.Choice == "" || len(*r.Choice) > maxOptionLength {
		return invalidResponse(fmt.Sprintf("choice must be 1 to %d characters long", maxOptionLength))
	}
	if r.LatencyMs == nil {
		return invalidResponse("latency_ms is required")
	}
	resp := domain.PracticeResponse{Type: domain.PracticeResponseGeneratedChoice, Way: *r.Way, Options: r.Options,
		Choice: *r.Choice, LatencyMs: r.LatencyMs, AudioMs: r.AudioMs}
	if r.MusicKey != nil {
		resp.MusicKey = *r.MusicKey
	}
	return resp, nil
}

// checkOptions is why a generated question's options are refused, or "".
func checkOptions(options []string) string {
	if len(options) < minOptions || len(options) > maxOptions || !distinct(options) {
		return fmt.Sprintf("options must be %d to %d different options", minOptions, maxOptions)
	}
	for _, o := range options {
		if o == "" || len(o) > maxOptionLength {
			return fmt.Sprintf("options must be 1 to %d characters long", maxOptionLength)
		}
	}
	return ""
}

func toTimedTapsResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("music_key", "tone", "tempo_bpm", "taps"); property != "" {
		return invalidResponse(property + " is not a property of timed_taps")
	}
	if r.MusicKey == nil || !musicKeys[*r.MusicKey] {
		return invalidResponse("music_key must be a major key spelled as C, Db, D, Eb, E, F, F#, G, Ab, A, Bb or B")
	}
	if r.Tone == nil || (*r.Tone != "3" && *r.Tone != "7") {
		return invalidResponse("tone must be 3 or 7")
	}
	if r.TempoBPM == nil || *r.TempoBPM < 1 || *r.TempoBPM > maxTempoBPM {
		return invalidResponse(fmt.Sprintf("tempo_bpm must be between 1 and %d", maxTempoBPM))
	}
	taps, why := toTimedTaps(r.Taps)
	if why != "" {
		return invalidResponse(why)
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseTimedTaps, MusicKey: *r.MusicKey, Tone: *r.Tone,
		TempoBPM: r.TempoBPM, Taps: taps}, nil
}

// toTimedTaps reads a take's taps, or says why they are refused.
func toTimedTaps(raw []rawTap) ([]domain.TimedTap, string) {
	if raw == nil || len(raw) > maxTaps {
		return nil, fmt.Sprintf("taps must list at most %d taps", maxTaps)
	}
	taps := make([]domain.TimedTap, 0, len(raw))
	for _, t := range raw {
		if t.AtMs == nil || *t.AtMs < 0 || t.String == nil || *t.String < 1 || t.Fret == nil || *t.Fret < 0 {
			return nil, "each tap needs at_ms of 0 or more, string of 1 or more and fret of 0 or more"
		}
		taps = append(taps, domain.TimedTap{AtMs: *t.AtMs, String: *t.String, Fret: *t.Fret})
	}
	return taps, ""
}

func toMultiTapResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("fret_window", "cells", "latency_ms"); property != "" {
		return invalidResponse(property + " is not a property of multi_tap")
	}
	if r.FretWindow == nil || r.FretWindow.FromFret == nil || !fretWindowStarts[*r.FretWindow.FromFret] {
		return invalidResponse("fret_window.from_fret must be 0 or 2 to 7")
	}
	if r.Cells == nil || len(r.Cells) > maxTappedCells {
		return invalidResponse(fmt.Sprintf("cells must list at most %d places", maxTappedCells))
	}
	cells := make([]domain.TappedCell, 0, len(r.Cells))
	seen := make(map[domain.TappedCell]bool, len(r.Cells))
	for _, c := range r.Cells {
		if c.String == nil || *c.String < 1 || c.Fret == nil || *c.Fret < 0 {
			return invalidResponse("each cell needs string of 1 or more and fret of 0 or more")
		}
		cell := domain.TappedCell{String: *c.String, Fret: *c.Fret}
		if seen[cell] {
			return invalidResponse("cells must not repeat a place")
		}
		seen[cell] = true
		cells = append(cells, cell)
	}
	if r.LatencyMs == nil {
		return invalidResponse("latency_ms is required")
	}
	from := *r.FretWindow.FromFret
	return domain.PracticeResponse{Type: domain.PracticeResponseMultiTap, FretWindowFrom: &from, Cells: cells, LatencyMs: r.LatencyMs}, nil
}

// distinct reports whether no value repeats.
func distinct(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

func toFindTheNoteResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("string", "fret", "latency_ms"); property != "" {
		return invalidResponse(property + " is not a property of find_the_note")
	}
	if r.String == nil || *r.String < 1 {
		return invalidResponse("string must be 1 or more")
	}
	if r.Fret == nil || *r.Fret < 0 {
		return invalidResponse("fret must be 0 or more")
	}
	if r.LatencyMs == nil {
		return invalidResponse("latency_ms is required")
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, String: r.String, Fret: r.Fret, LatencyMs: r.LatencyMs}, nil
}

func toNameTheShapeResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("shape", "latency_ms"); property != "" {
		return invalidResponse(property + " is not a property of name_the_shape")
	}
	if r.Shape == nil || !shapeKeyPattern.MatchString(*r.Shape) {
		return invalidResponse("shape must be a member key: letters, digits and dashes")
	}
	if r.LatencyMs == nil {
		return invalidResponse("latency_ms is required")
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseNameTheShape, Shape: *r.Shape, LatencyMs: r.LatencyMs}, nil
}

func toFindTheDegreeResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("interval", "string", "fret", "latency_ms"); property != "" {
		return invalidResponse(property + " is not a property of find_the_degree")
	}
	if r.Interval == nil || !shapeIntervals[*r.Interval] {
		return invalidResponse("interval must be an interval code such as R, b3 or 5")
	}
	if r.String == nil || *r.String < 1 {
		return invalidResponse("string must be 1 or more")
	}
	if r.Fret == nil || *r.Fret < 0 {
		return invalidResponse("fret must be 0 or more")
	}
	if r.LatencyMs == nil {
		return invalidResponse("latency_ms is required")
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseFindTheDegree, Interval: *r.Interval, String: r.String, Fret: r.Fret, LatencyMs: r.LatencyMs}, nil
}

func toOptionChoiceResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("option_ids", "latency_ms", "audio_ms"); property != "" {
		return invalidResponse(property + " is not a property of option_choice")
	}
	if len(r.OptionIDs) == 0 {
		return invalidResponse("option_ids must select at least one option")
	}
	ids := make([]string, 0, len(r.OptionIDs))
	seen := make(map[string]bool, len(r.OptionIDs))
	for _, raw := range r.OptionIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return invalidResponse("option_ids must be UUIDs")
		}
		if seen[id.String()] {
			return invalidResponse("option_ids must not repeat an option")
		}
		seen[id.String()] = true
		ids = append(ids, id.String())
	}
	if r.LatencyMs == nil {
		return invalidResponse("latency_ms is required")
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice, OptionIDs: ids, LatencyMs: r.LatencyMs, AudioMs: r.AudioMs}, nil
}

func toSelfRatingResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("rating", "tempo_bpm", "note_value", "changes_per_minute"); property != "" {
		return invalidResponse(property + " is not a property of self_rating")
	}
	if r.Rating == nil {
		return invalidResponse("rating is required")
	}
	rating := domain.SelfRating(*r.Rating)
	switch rating {
	case domain.SelfRatingStruggled, domain.SelfRatingAlmost, domain.SelfRatingClean:
	default:
		return invalidResponse("rating must be struggled, almost or clean")
	}
	if r.TempoBPM != nil && (*r.TempoBPM < 1 || *r.TempoBPM > maxTempoBPM) {
		return invalidResponse(fmt.Sprintf("tempo_bpm must be between 1 and %d", maxTempoBPM))
	}
	if r.ChangesPerMinute != nil && (*r.ChangesPerMinute < 0 || *r.ChangesPerMinute > maxChangesPerMin) {
		return invalidResponse(fmt.Sprintf("changes_per_minute must be between 0 and %d", maxChangesPerMin))
	}
	var noteValue *domain.NoteValue
	if r.NoteValue != nil {
		v := domain.NoteValue(*r.NoteValue)
		if !v.Valid() {
			return invalidResponse("note_value must be a note value, such as quarters or eighths")
		}
		noteValue = &v
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: rating, TempoBPM: r.TempoBPM, NoteValue: noteValue, ChangesPerMinute: r.ChangesPerMinute}, nil
}
