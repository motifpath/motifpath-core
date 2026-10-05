//go:build integration

package bdd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

func registerPracticeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" has (?:later )?submitted a practice\.tap_check_completed event with a median tap of (\d+) milliseconds$`, w.hasSubmittedTapCheck)
	sc.Step(`^"([^"]+)" has never submitted a practice\.tap_check_completed event$`, func(string) error { return nil })

	sc.Step(`^"([^"]+)" submits a practice\.session_started event for a (\d+)-minute session with guitar in hand and (\d+) planned items, each with a reason$`, w.submitSessionStartedWithGuitar)
	sc.Step(`^"([^"]+)" submits a practice\.session_started event for a (\d+)-minute session with no instrument in hand and (\d+) planned items, each with a reason$`, w.submitSessionStartedInTheHead)
	sc.Step(`^"([^"]+)" submits a practice\.session_started event for a (\d+)-minute session with no planned items$`, w.submitSessionStartedWithNoItems)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event naming the note "([^"]+)" for the guitar cell on string (\d+) at fret (\d+)$`, w.submitNameTheNote)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event rating a take of a play-along "([^"]+)" at (\d+) BPM$`, w.submitSelfRatedTake)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event naming a note that claims a tap time of (\d+) milliseconds$`, w.submitNameTheNoteClaimingTap)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event for the item key "([^"]+)"$`, w.submitAnswerForItemKey)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event whose response also claims it is correct$`, w.submitAnswerClaimingCorrect)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event selecting options of exercise "([^"]+)" in challenge "([^"]+)"$`, w.submitChallengeAnswer)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event carrying both a practice session and a challenge trigger context$`, w.submitAnswerWithSessionAndTrigger)
	sc.Step(`^"([^"]+)" submits a practice\.item_answered event with neither a practice session nor a trigger context$`, w.submitAnswerWithNoContext)
	sc.Step(`^"([^"]+)" submits a practice\.session_ended event with (\d+) items answered, not left early, and "([^"]+)" felt "([^"]+)"$`, w.submitSessionEndedWithFelt)
	sc.Step(`^"([^"]+)" submits a practice\.session_ended event with (\d+) items answered, left early, and no felt ratings$`, w.submitSessionEndedEarly)
	sc.Step(`^"([^"]+)" submits a practice\.session_ended event with (\d+) felt ratings$`, w.submitSessionEndedWithFeltCount)
	sc.Step(`^"([^"]+)" submits a practice\.tap_check_completed event with a median tap of (\d+) milliseconds over (\d+) taps$`, w.submitTapCheck)

	sc.Step(`^the stored and published practice\.item_answered event carries a tap time of (\d+) milliseconds$`, w.answerCarriesTap)
	sc.Step(`^the stored and published practice\.item_answered event carries no tap time$`, w.answerCarriesNoTap)
}

var guitarID = deterministicUUID("instrument", "guitar")

// practiceEvent is a practice.* payload under construction, marshalled to JSON and
// decoded the way the router decodes a request. It is its own type rather than the
// generated structs so a step can include what those cannot express: a property the
// schema forbids, or the readOnly tap_ms. Every event-specific field is omitempty,
// except where a scenario needs a zero value on the wire (pointers).
type practiceEvent struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	StudentID  string    `json:"student_id"`
	SessionID  string    `json:"session_id"`
	OccurredAt time.Time `json:"occurred_at"`

	PracticeSessionID string            `json:"practice_session_id,omitempty"`
	TriggerContext    *triggerContext   `json:"trigger_context,omitempty"`
	InstrumentID      string            `json:"instrument_id,omitempty"`
	Minutes           int               `json:"minutes,omitempty"`
	PlannedItems      *[]plannedItem    `json:"planned_items,omitempty"`
	ItemKey           string            `json:"item_key,omitempty"`
	Response          *practiceResponse `json:"response,omitempty"`
	TapMs             *int              `json:"tap_ms,omitempty"`
	AnsweredCount     *int              `json:"answered_count,omitempty"`
	LeftEarly         *bool             `json:"left_early,omitempty"`
	FeltRatings       *[]feltRating     `json:"felt_ratings,omitempty"`
	MedianTapMs       int               `json:"median_tap_ms,omitempty"`
	TapCount          int               `json:"tap_count,omitempty"`
}

type plannedItem struct {
	ItemKey string `json:"item_key"`
	Reason  string `json:"reason"`
}

type practiceResponse struct {
	ResponseType string   `json:"response_type"`
	NoteName     string   `json:"note_name,omitempty"`
	LatencyMs    *int     `json:"latency_ms,omitempty"`
	Rating       string   `json:"rating,omitempty"`
	TempoBPM     int      `json:"tempo_bpm,omitempty"`
	OptionIDs    []string `json:"option_ids,omitempty"`

	// Correct is not part of any response shape: a client claiming a verdict.
	Correct *bool `json:"correct,omitempty"`
}

type triggerContext struct {
	Source      string `json:"source"`
	ChallengeID string `json:"challenge_id,omitempty"`
}

type feltRating struct {
	DrillTemplateKey string `json:"drill_template_key"`
	Felt             string `json:"felt"`
}

// newPracticeEvent stamps the envelope with the next practice clock tick.
func (w *world) newPracticeEvent(name, eventType string) practiceEvent {
	at := w.practiceClock
	w.practiceClock = w.practiceClock.Add(time.Minute)
	return practiceEvent{
		EventID:    deterministicUUID("event", name, eventType, at.Format(time.RFC3339)).String(),
		EventType:  eventType,
		StudentID:  studentID(name),
		SessionID:  fixedSessionID.String(),
		OccurredAt: at,
	}
}

func practiceSessionID(name string) string {
	return deterministicUUID("practice-session", name).String()
}

func (w *world) submitPractice(e practiceEvent) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	body := &generated.TrackingEvent{}
	if err := body.UnmarshalJSON(raw); err != nil {
		return err
	}
	w.submit(body)
	return nil
}

func plannedItems(name string, n int) *[]plannedItem {
	reasons := []string{"warm_up", "due", "weak", "new"}
	items := make([]plannedItem, 0, n)
	for i := range n {
		items = append(items, plannedItem{
			ItemKey: "play_along:" + deterministicUUID("diagram", name, strconv.Itoa(i)).String(),
			Reason:  reasons[i%len(reasons)],
		})
	}
	return &items
}

func guitarCellKey(stringNumber, fret string) string {
	return fmt.Sprintf("fretboard_cell:%s:%s:%s", guitarID, stringNumber, fret)
}

func (w *world) answerEvent(name, itemKey string, response *practiceResponse) practiceEvent {
	e := w.newPracticeEvent(name, "practice.item_answered")
	e.PracticeSessionID = practiceSessionID(name)
	e.ItemKey = itemKey
	e.Response = response
	return e
}

func challengeTrigger(challengeName string) *triggerContext {
	return &triggerContext{Source: "challenge_sequence", ChallengeID: deterministicUUID("challenge", challengeName).String()}
}

func exerciseKey(exerciseName string) string {
	return "exercise:" + deterministicUUID("exercise", exerciseName).String()
}

func optionChoice(exerciseName string) *practiceResponse {
	latency := 3500
	return &practiceResponse{
		ResponseType: "option_choice",
		OptionIDs:    []string{deterministicUUID("option", exerciseName, "a").String()},
		LatencyMs:    &latency,
	}
}

func nameTheNote(note string) *practiceResponse {
	latency := 1800
	return &practiceResponse{ResponseType: "name_the_note", NoteName: note, LatencyMs: &latency}
}

// ── Given ──────────────────────────────────────────────────────────────────

func (w *world) hasSubmittedTapCheck(name, medianStr string) error {
	if err := w.submitTapCheck(name, medianStr, "24"); err != nil {
		return err
	}
	if _, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse); !ok {
		return fmt.Errorf("setup: expected the tap check to be accepted, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	return nil
}

// ── When ───────────────────────────────────────────────────────────────────

func (w *world) submitSessionStartedWithGuitar(name, minutesStr, itemsStr string) error {
	e, err := w.sessionStarted(name, minutesStr, itemsStr)
	if err != nil {
		return err
	}
	e.InstrumentID = guitarID.String()
	return w.submitPractice(e)
}

func (w *world) submitSessionStartedInTheHead(name, minutesStr, itemsStr string) error {
	e, err := w.sessionStarted(name, minutesStr, itemsStr)
	if err != nil {
		return err
	}
	return w.submitPractice(e)
}

func (w *world) submitSessionStartedWithNoItems(name, minutesStr string) error {
	e, err := w.sessionStarted(name, minutesStr, "0")
	if err != nil {
		return err
	}
	return w.submitPractice(e)
}

func (w *world) sessionStarted(name, minutesStr, itemsStr string) (practiceEvent, error) {
	minutes, err := strconv.Atoi(minutesStr)
	if err != nil {
		return practiceEvent{}, err
	}
	items, err := strconv.Atoi(itemsStr)
	if err != nil {
		return practiceEvent{}, err
	}
	e := w.newPracticeEvent(name, "practice.session_started")
	e.PracticeSessionID = practiceSessionID(name)
	e.Minutes = minutes
	e.PlannedItems = plannedItems(name, items)
	return e, nil
}

func (w *world) submitNameTheNote(name, note, stringNumber, fret string) error {
	return w.submitPractice(w.answerEvent(name, guitarCellKey(stringNumber, fret), nameTheNote(note)))
}

func (w *world) submitSelfRatedTake(name, rating, tempoStr string) error {
	tempo, err := strconv.Atoi(tempoStr)
	if err != nil {
		return err
	}
	itemKey := "play_along:" + deterministicUUID("diagram", name, "take").String()
	return w.submitPractice(w.answerEvent(name, itemKey, &practiceResponse{
		ResponseType: "self_rating", Rating: rating, TempoBPM: tempo,
	}))
}

func (w *world) submitNameTheNoteClaimingTap(name, tapStr string) error {
	tap, err := strconv.Atoi(tapStr)
	if err != nil {
		return err
	}
	e := w.answerEvent(name, guitarCellKey("5", "3"), nameTheNote("C"))
	e.TapMs = &tap
	return w.submitPractice(e)
}

func (w *world) submitAnswerForItemKey(name, itemKey string) error {
	return w.submitPractice(w.answerEvent(name, itemKey, nameTheNote("C")))
}

func (w *world) submitAnswerClaimingCorrect(name string) error {
	response := nameTheNote("C")
	correct := true
	response.Correct = &correct
	return w.submitPractice(w.answerEvent(name, guitarCellKey("5", "3"), response))
}

func (w *world) submitChallengeAnswer(name, exerciseName, challengeName string) error {
	e := w.answerEvent(name, exerciseKey(exerciseName), optionChoice(exerciseName))
	e.PracticeSessionID = ""
	e.TriggerContext = challengeTrigger(challengeName)
	return w.submitPractice(e)
}

func (w *world) submitAnswerWithSessionAndTrigger(name string) error {
	e := w.answerEvent(name, exerciseKey("minor-third-from-a"), optionChoice("minor-third-from-a"))
	e.TriggerContext = challengeTrigger("intervals-assessment")
	return w.submitPractice(e)
}

func (w *world) submitAnswerWithNoContext(name string) error {
	e := w.answerEvent(name, exerciseKey("minor-third-from-a"), optionChoice("minor-third-from-a"))
	e.PracticeSessionID = ""
	return w.submitPractice(e)
}

func (w *world) sessionEnded(name, answeredStr string, leftEarly bool, felt []feltRating) error {
	answered, err := strconv.Atoi(answeredStr)
	if err != nil {
		return err
	}
	e := w.newPracticeEvent(name, "practice.session_ended")
	e.PracticeSessionID = practiceSessionID(name)
	e.AnsweredCount = &answered
	e.LeftEarly = &leftEarly
	e.FeltRatings = &felt
	return w.submitPractice(e)
}

func (w *world) submitSessionEndedWithFelt(name, answeredStr, drillTemplateKey, felt string) error {
	return w.sessionEnded(name, answeredStr, false, []feltRating{{DrillTemplateKey: drillTemplateKey, Felt: felt}})
}

func (w *world) submitSessionEndedEarly(name, answeredStr string) error {
	return w.sessionEnded(name, answeredStr, true, []feltRating{})
}

func (w *world) submitSessionEndedWithFeltCount(name, countStr string) error {
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return err
	}
	drills := []string{"fretboard_cell:name_the_note", "fretboard_cell:find_the_note", "exercise:interval_recognition"}
	felt := make([]feltRating, 0, count)
	for i := range count {
		felt = append(felt, feltRating{DrillTemplateKey: drills[i%len(drills)], Felt: "about_right"})
	}
	return w.sessionEnded(name, "6", false, felt)
}

func (w *world) submitTapCheck(name, medianStr, countStr string) error {
	median, err := strconv.Atoi(medianStr)
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return err
	}
	e := w.newPracticeEvent(name, "practice.tap_check_completed")
	e.MedianTapMs = median
	e.TapCount = count
	return w.submitPractice(e)
}

// ── Then ───────────────────────────────────────────────────────────────────

// storedAndPublishedAnswer returns the accepted answer as the repository stored it
// and as the publisher sent it.
func (w *world) storedAndPublishedAnswer() (stored, published domain.PracticeItemAnsweredEvent, err error) {
	resp, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse)
	if !ok {
		return stored, published, fmt.Errorf("expected a 202 response, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	eventID := resp.EventId.String()

	w.repo.mu.Lock()
	saved, found := w.repo.saved[eventID]
	w.repo.mu.Unlock()
	if !found {
		return stored, published, fmt.Errorf("event %s was not stored", eventID)
	}
	if stored, ok = saved.(domain.PracticeItemAnsweredEvent); !ok {
		return stored, published, fmt.Errorf("stored event is a %T", saved)
	}

	sent, err := w.publisher.waitForPublished(eventID)
	if err != nil {
		return stored, published, err
	}
	if published, ok = sent.(domain.PracticeItemAnsweredEvent); !ok {
		return stored, published, fmt.Errorf("published event is a %T", sent)
	}
	return stored, published, nil
}

func (w *world) answerCarriesTap(tapStr string) error {
	want, err := strconv.Atoi(tapStr)
	if err != nil {
		return err
	}
	stored, published, err := w.storedAndPublishedAnswer()
	if err != nil {
		return err
	}
	for label, got := range map[string]*int{"stored": stored.TapMs, "published": published.TapMs} {
		if got == nil {
			return fmt.Errorf("the %s answer carries no tap time, want %d", label, want)
		}
		if *got != want {
			return fmt.Errorf("the %s answer carries a tap time of %d, want %d", label, *got, want)
		}
	}
	return nil
}

func (w *world) answerCarriesNoTap() error {
	stored, published, err := w.storedAndPublishedAnswer()
	if err != nil {
		return err
	}
	for label, got := range map[string]*int{"stored": stored.TapMs, "published": published.TapMs} {
		if got != nil {
			return fmt.Errorf("the %s answer carries a tap time of %d, want none", label, *got)
		}
	}
	return nil
}
