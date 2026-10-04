package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

// The practice.* mappers validate what events.yaml and practice.yaml constrain beyond
// field presence — ranges, patterns, enums, array sizes — because nothing upstream of
// them checks a request against the schema. The answer response gets the strictest
// treatment: it is graded downstream, and its schema closes it (additionalProperties:
// false) so a client can never slip a verdict such as "correct" into it.

const (
	maxPracticeMinutes = 60
	maxFeltRatings     = 2
	maxTempoBPM        = 400
	maxChangesPerMin   = 600
)

var (
	noteNamePattern         = regexp.MustCompile(`^[A-G][#b]?$`)
	drillTemplateKeyPattern = regexp.MustCompile(`^[a-z][a-z_]*:[a-z][a-z_]*$`)
)

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
	practiceSessionID, err := requireUUID(v.PracticeSessionId, "practice_session_id")
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
		ItemKey:           v.ItemKey,
		Response:          response,
	}, nil
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
	String           *int     `json:"string"`
	Fret             *int     `json:"fret"`
	OptionIDs        []string `json:"option_ids"`
	LatencyMs        *int     `json:"latency_ms"`
	Rating           *string  `json:"rating"`
	TempoBPM         *int     `json:"tempo_bpm"`
	ChangesPerMinute *int     `json:"changes_per_minute"`
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

	switch domain.PracticeResponseType(r.ResponseType) {
	case domain.PracticeResponseNameTheNote:
		return toNameTheNoteResponse(r)
	case domain.PracticeResponseFindTheNote:
		return toFindTheNoteResponse(r)
	case domain.PracticeResponseOptionChoice:
		return toOptionChoiceResponse(r)
	case domain.PracticeResponseSelfRating:
		return toSelfRatingResponse(r)
	default:
		return invalidResponse(fmt.Sprintf("response_type %q is not a known response", r.ResponseType))
	}
}

func invalidResponse(why string) (domain.PracticeResponse, error) {
	return domain.PracticeResponse{}, fmt.Errorf("%w: response %s", domain.ErrInvalidField, why)
}

func toNameTheNoteResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if r.String != nil || r.Fret != nil || r.OptionIDs != nil || r.Rating != nil || r.TempoBPM != nil || r.ChangesPerMinute != nil {
		return invalidResponse("carries a property name_the_note does not have")
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
	if r.NoteName != nil || r.OptionIDs != nil || r.Rating != nil || r.TempoBPM != nil || r.ChangesPerMinute != nil {
		return invalidResponse("carries a property find_the_note does not have")
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

func toOptionChoiceResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if r.NoteName != nil || r.String != nil || r.Fret != nil || r.Rating != nil || r.TempoBPM != nil || r.ChangesPerMinute != nil {
		return invalidResponse("carries a property option_choice does not have")
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
	return domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice, OptionIDs: ids, LatencyMs: r.LatencyMs}, nil
}

func toSelfRatingResponse(r rawPracticeResponse) (domain.PracticeResponse, error) {
	if r.NoteName != nil || r.String != nil || r.Fret != nil || r.OptionIDs != nil || r.LatencyMs != nil {
		return invalidResponse("carries a property self_rating does not have")
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
