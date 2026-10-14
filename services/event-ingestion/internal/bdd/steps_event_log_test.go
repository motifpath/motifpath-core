//go:build integration

package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

// Steps for reading the event log. Events are seeded straight into the
// repository fake with when they were received; reads go through the
// handler and are checked on the JSON it writes.

// logDay is the day the seeded events were received.
var logDay = time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

// eventLogWorld is what the event log scenarios share.
type eventLogWorld struct {
	resp generated.ListLoggedEventsResponseObject
	page loggedEventPage
	// pages are every page read, for "reads twice".
	pages []loggedEventPage
}

// loggedEventPage is a 200 response as the client reads it.
type loggedEventPage struct {
	Items []struct {
		ReceivedAt time.Time      `json:"received_at"`
		Event      map[string]any `json:"event"`
	} `json:"items"`
	Total int `json:"total"`
}

func (p loggedEventPage) eventIDs() []string {
	ids := make([]string, len(p.Items))
	for i, item := range p.Items {
		ids[i], _ = item.Event["event_id"].(string)
	}
	return ids
}

func registerEventLogSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the event log holds (\d+) events of student "([^"]+)" and (\d+) of student "([^"]+)"$`, w.eventLogHolds)
	sc.Step(`^"([^"]+)"'s practice\.session_ended event "([^"]+)", not left early with (\d+) items answered, was received at (\d\d:\d\d:\d\d)$`, w.sessionEndedReceivedAt)
	sc.Step(`^"([^"]+)" has practice\.session_started, practice\.item_answered, practice\.session_ended and lesson\.started events in the log$`, w.hasEventsOfFourTypes)
	sc.Step(`^events "([^"]+)" and "([^"]+)" were received at the same instant$`, w.receivedAtTheSameInstant)
	sc.Step(`^the platform cannot be reached to establish the caller's role$`, w.roleCannotBeEstablished)

	sc.Step(`^the caller reads the event log$`, func() error { return w.readEventLog(generated.ListLoggedEventsParams{}) })
	sc.Step(`^the caller reads the event log twice$`, w.readEventLogTwice)
	sc.Step(`^the caller reads the event log, (\d+) at a time$`, func(n int) error {
		return w.readEventLog(generated.ListLoggedEventsParams{Limit: &n})
	})
	sc.Step(`^the caller reads the event log, (\d+) at a time, from the (\d+)(?:st|nd|rd|th) event$`, func(n, from int) error {
		offset := from - 1
		return w.readEventLog(generated.ListLoggedEventsParams{Limit: &n, Offset: &offset})
	})
	sc.Step(`^the caller reads the event log for student "([^"]+)"(?:, who has no events)?$`, func(name string) error {
		id := uuid.MustParse(studentID(name))
		return w.readEventLog(generated.ListLoggedEventsParams{StudentId: &id})
	})
	sc.Step(`^the caller reads the event log for student "([^"]+)" in the type group ([a-z_]+)$`, func(name, group string) error {
		id := uuid.MustParse(studentID(name))
		g := generated.ListLoggedEventsParamsTypeGroup(group)
		return w.readEventLog(generated.ListLoggedEventsParams{StudentId: &id, TypeGroup: &g})
	})
	sc.Step(`^the caller reads the event log in the type group "([^"]+)"$`, func(group string) error {
		g := generated.ListLoggedEventsParamsTypeGroup(group)
		return w.readEventLog(generated.ListLoggedEventsParams{TypeGroup: &g})
	})

	sc.Step(`^the (\d+) most recently received events are returned, newest first$`, w.mostRecentReturned)
	sc.Step(`^the total is (\d+)$`, w.totalIs)
	sc.Step(`^event "([^"]+)" is returned with its event type, student, occurred_at and every field it was submitted with$`, w.eventReturnedAsSubmitted)
	sc.Step(`^it shows it was received at (\d\d:\d\d:\d\d)$`, w.receivedAtShown)
	sc.Step(`^the (\d+)(?:st|nd|rd|th) to (\d+)(?:st|nd|rd|th) most recently received events are returned$`, w.rangeReturned)
	sc.Step(`^only "([^"]+)"'s events are returned$`, w.onlyStudentsEvents)
	sc.Step(`^only "([^"]+)"'s practice\.session_started and practice\.session_ended events are returned$`, w.onlySessionEvents)
	sc.Step(`^"([^"]+)" and "([^"]+)" are in the same order both times$`, w.sameOrderBothTimes)
	sc.Step(`^no events are returned$`, w.noEventsReturned)
	sc.Step(`^the request is refused for lacking permission$`, func() error { return w.eventLogStatusIs(403) })
	sc.Step(`^the request is refused as temporarily unavailable$`, func() error { return w.eventLogStatusIs(503) })
	sc.Step(`^the request is rejected as invalid$`, func() error { return w.eventLogStatusIs(400) })
}

// ── Seeding ──────────────────────────────────────────────────────────────

func lessonStartedBy(eventID, student string, occurredAt time.Time) domain.LessonStartedEvent {
	return domain.LessonStartedEvent{
		TrackingEventBase: domain.TrackingEventBase{
			EventID: eventID, EventType: domain.EventTypeLessonStarted, StudentID: studentID(student),
			SessionID: deterministicUUID("session", student).String(), OccurredAt: occurredAt,
		},
		ContentContext: domain.ContentContext{ContentNodeID: deterministicUUID("node", "intro").String(), ContentType: domain.ContentTypeArticle},
	}
}

// eventLogHolds seeds lesson.started events, a second apart from 06:00,
// every fourth one bruno's while his last.
func (w *world) eventLogHolds(nA int, studentA string, nB int, studentB string) error {
	start := logDay.Add(6 * time.Hour)
	for i := range nA + nB {
		owner := studentA
		if i%4 == 3 && nB > 0 {
			owner = studentB
			nB--
		}
		at := start.Add(time.Duration(i) * time.Second)
		w.repo.store(lessonStartedBy(deterministicUUID("logged", fmt.Sprint(i)).String(), owner, at), at)
	}
	return nil
}

func (w *world) sessionEndedReceivedAt(student, name string, answered int, hhmmss string) error {
	at, err := time.Parse(time.TimeOnly, hhmmss)
	if err != nil {
		return err
	}
	receivedAt := logDay.Add(time.Duration(at.Hour())*time.Hour + time.Duration(at.Minute())*time.Minute + time.Duration(at.Second())*time.Second)
	w.repo.store(domain.PracticeSessionEndedEvent{
		TrackingEventBase: domain.TrackingEventBase{
			EventID: deterministicUUID("event", name).String(), EventType: domain.EventTypePracticeSessionEnded,
			StudentID: studentID(student), SessionID: deterministicUUID("session", student).String(), OccurredAt: receivedAt.Add(-2 * time.Second),
		},
		PracticeSessionID: deterministicUUID("practice-session", name).String(), AnsweredCount: answered, FeltRatings: []domain.FeltRating{},
	}, receivedAt)
	return nil
}

func (w *world) hasEventsOfFourTypes(student string) error {
	at := logDay.Add(8 * time.Hour)
	base := func(n int, eventType domain.EventType) domain.TrackingEventBase {
		return domain.TrackingEventBase{
			EventID: deterministicUUID("typed", fmt.Sprint(n)).String(), EventType: eventType, StudentID: studentID(student),
			SessionID: deterministicUUID("session", student).String(), OccurredAt: at,
		}
	}
	session := deterministicUUID("practice-session", "typed").String()
	events := []domain.TrackingEvent{
		domain.PracticeSessionStartedEvent{TrackingEventBase: base(1, domain.EventTypePracticeSessionStarted), PracticeSessionID: session, Minutes: 10},
		domain.PracticeItemAnsweredEvent{TrackingEventBase: base(2, domain.EventTypePracticeItemAnswered), PracticeSessionID: session,
			ItemKey: "play_along:" + deterministicUUID("diagram", "d").String(), Response: domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: "clean"}},
		domain.PracticeSessionEndedEvent{TrackingEventBase: base(3, domain.EventTypePracticeSessionEnded), PracticeSessionID: session, AnsweredCount: 1, FeltRatings: []domain.FeltRating{}},
		lessonStartedBy(deterministicUUID("typed", "4").String(), student, at),
	}
	for i, e := range events {
		w.repo.store(e, at.Add(time.Duration(i)*time.Second))
	}
	return nil
}

func (w *world) receivedAtTheSameInstant(a, b string) error {
	at := logDay.Add(9 * time.Hour)
	for _, name := range []string{a, b} {
		w.repo.store(lessonStartedBy(deterministicUUID("event", name).String(), "alice", at), at)
	}
	return nil
}

// ── Reads ────────────────────────────────────────────────────────────────

func (w *world) readEventLog(params generated.ListLoggedEventsParams) error {
	resp, err := w.handler.ListLoggedEvents(w.adminCtx(), generated.ListLoggedEventsRequestObject{Params: params})
	if err != nil {
		return err
	}
	w.eventLog.resp = resp
	rec := httptest.NewRecorder()
	if err := resp.VisitListLoggedEventsResponse(rec); err != nil {
		return err
	}
	w.adminStatusCode = rec.Code
	w.eventLog.page = loggedEventPage{}
	if rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), &w.eventLog.page); err != nil {
			return err
		}
		w.eventLog.pages = append(w.eventLog.pages, w.eventLog.page)
	}
	return nil
}

func (w *world) readEventLogTwice() error {
	for range 2 {
		if err := w.readEventLog(generated.ListLoggedEventsParams{}); err != nil {
			return err
		}
	}
	return nil
}

// allReceived is every stored event's id, newest received first.
func (w *world) allReceived() ([]string, error) {
	all, _, err := w.repo.List(context.Background(), domain.EventLogQuery{Limit: 1000})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(all))
	for i, e := range all {
		ids[i] = e.Event.Base().EventID
	}
	return ids, nil
}

func (w *world) eventLogStatusIs(status int) error {
	if w.adminStatusCode != status {
		return fmt.Errorf("expected status %d, got %d (%#v)", status, w.adminStatusCode, w.eventLog.resp)
	}
	return nil
}

func (w *world) mostRecentReturned(n int) error {
	return w.rangeReturned(1, n)
}

func (w *world) rangeReturned(from, to int) error {
	all, err := w.allReceived()
	if err != nil {
		return err
	}
	got, want := w.eventLog.page.eventIDs(), all[from-1:to]
	if !slices.Equal(got, want) {
		return fmt.Errorf("events returned: got %v, want %v", got, want)
	}
	for i := 1; i < len(w.eventLog.page.Items); i++ {
		if w.eventLog.page.Items[i].ReceivedAt.After(w.eventLog.page.Items[i-1].ReceivedAt) {
			return fmt.Errorf("events are not newest received first")
		}
	}
	return nil
}

func (w *world) totalIs(n int) error {
	if w.eventLog.page.Total != n {
		return fmt.Errorf("total: got %d, want %d", w.eventLog.page.Total, n)
	}
	return nil
}

func (w *world) loggedItem(name string) (int, error) {
	id := deterministicUUID("event", name).String()
	for i, got := range w.eventLog.page.eventIDs() {
		if got == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("event %q was not returned", name)
}

func (w *world) eventReturnedAsSubmitted(name string) error {
	i, err := w.loggedItem(name)
	if err != nil {
		return err
	}
	got := w.eventLog.page.Items[i].Event
	want := map[string]any{
		"event_id":            deterministicUUID("event", name).String(),
		"event_type":          "practice.session_ended",
		"student_id":          studentID("alice"),
		"session_id":          deterministicUUID("session", "alice").String(),
		"occurred_at":         "2026-10-14T07:51:10Z",
		"practice_session_id": deterministicUUID("practice-session", name).String(),
		"answered_count":      float64(9),
		"left_early":          false,
		"felt_ratings":        []any{},
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		return err
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		return err
	}
	if string(gotJSON) != string(wantJSON) {
		return fmt.Errorf("event returned:\n got %s\nwant %s", gotJSON, wantJSON)
	}
	return nil
}

func (w *world) receivedAtShown(hhmmss string) error {
	i, err := w.loggedItem("evt-end-001")
	if err != nil {
		return err
	}
	if got := w.eventLog.page.Items[i].ReceivedAt.Format(time.TimeOnly); got != hhmmss {
		return fmt.Errorf("received at: got %s, want %s", got, hhmmss)
	}
	return nil
}

func (w *world) onlyStudentsEvents(name string) error {
	for _, item := range w.eventLog.page.Items {
		if item.Event["student_id"] != studentID(name) {
			return fmt.Errorf("an event of %v was returned", item.Event["student_id"])
		}
	}
	if len(w.eventLog.page.Items) == 0 {
		return fmt.Errorf("no events were returned")
	}
	return nil
}

func (w *world) onlySessionEvents(name string) error {
	if err := w.onlyStudentsEvents(name); err != nil {
		return err
	}
	var types []string
	for _, item := range w.eventLog.page.Items {
		eventType, _ := item.Event["event_type"].(string)
		types = append(types, eventType)
	}
	slices.Sort(types)
	if want := []string{"practice.session_ended", "practice.session_started"}; !slices.Equal(types, want) {
		return fmt.Errorf("event types: got %v, want %v", types, want)
	}
	return nil
}

func (w *world) sameOrderBothTimes(a, b string) error {
	if len(w.eventLog.pages) != 2 {
		return fmt.Errorf("expected two reads, got %d", len(w.eventLog.pages))
	}
	ids := []string{deterministicUUID("event", a).String(), deterministicUUID("event", b).String()}
	positions := func(p loggedEventPage) []int {
		var at []int
		for i, id := range p.eventIDs() {
			if slices.Contains(ids, id) {
				at = append(at, i)
			}
		}
		return at
	}
	first, second := w.eventLog.pages[0].eventIDs(), w.eventLog.pages[1].eventIDs()
	if len(positions(w.eventLog.pages[0])) != 2 || !slices.Equal(first, second) {
		return fmt.Errorf("orders differ or events missing: %v then %v", first, second)
	}
	return nil
}

func (w *world) noEventsReturned() error {
	if len(w.eventLog.page.Items) != 0 {
		return fmt.Errorf("expected no events, got %d", len(w.eventLog.page.Items))
	}
	return nil
}

func (w *world) eventLogRejectionIdentifies(field string) error {
	resp, ok := w.eventLog.resp.(generated.ListLoggedEvents400JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 400 response, got %#v", w.eventLog.resp)
	}
	for _, e := range resp.Errors {
		if e.Field == field {
			return nil
		}
	}
	return fmt.Errorf("no validation error identified %q; got %+v", field, resp.Errors)
}
