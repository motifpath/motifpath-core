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
)

var (
	noteNamePattern         = regexp.MustCompile(`^[A-G][#b]?$`)
	drillTemplateKeyPattern = regexp.MustCompile(`^[a-z][a-z_]*:[a-z][a-z_]*$`)
	// shapeKeyPattern is a shape family member's key, such as A, 2 or minor.
	shapeKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)
)

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
	ChangesPerMinute *int     `json:"changes_per_minute"`
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
		{"tempo_bpm", r.TempoBPM != nil}, {"changes_per_minute", r.ChangesPerMinute != nil},
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
	if r.AudioMs != nil && domain.PracticeResponseType(r.ResponseType) != domain.PracticeResponseOptionChoice {
		return invalidResponse("audio_ms belongs to option_choice only")
	}

	toShape, ok := responseShapes[domain.PracticeResponseType(r.ResponseType)]
	if !ok {
		return invalidResponse(fmt.Sprintf("response_type %q is not a known response", r.ResponseType))
	}
	return toShape(r)
}

// responseShapes reads each response shape, by its response_type.
var responseShapes = map[domain.PracticeResponseType]func(rawPracticeResponse) (domain.PracticeResponse, error){
	domain.PracticeResponseNameTheNote:   toNameTheNoteResponse,
	domain.PracticeResponseFindTheNote:   toFindTheNoteResponse,
	domain.PracticeResponseOptionChoice:  toOptionChoiceResponse,
	domain.PracticeResponseSelfRating:    toSelfRatingResponse,
	domain.PracticeResponseNameTheShape:  toNameTheShapeResponse,
	domain.PracticeResponseFindTheDegree: toFindTheDegreeResponse,
}

func invalidResponse(why string) (domain.PracticeResponse, error) {
	return domain.PracticeResponse{}, fmt.Errorf("%w: response %s", domain.ErrInvalidField, why)
}

func toNameTheNoteResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if property := r.strayProperty("note_name", "latency_ms"); property != "" {
		return invalidResponse(property + " is not a property of name_the_note")
	}
	if r.NoteName == nil || !noteNamePattern.MatchString(*r.NoteName) {
		return invalidResponse("note_name must be a note letter with an optional # or b")
	}
	if r.LatencyMs == nil {
		return invalidResponse("latency_ms is required")
	}
	return domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: *r.NoteName, LatencyMs: r.LatencyMs}, nil
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
	if property := r.strayProperty("rating", "tempo_bpm", "changes_per_minute"); property != "" {
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
	return domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: rating, TempoBPM: r.TempoBPM, ChangesPerMinute: r.ChangesPerMinute}, nil
}
