//go:build integration

package bdd

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// activityWorld is what the session and learning activity scenarios share: the
// session under test and the day the scenario calls "today".
type activityWorld struct {
	sessionID string
	// firstEnd is the end a duplicated practice.session_ended carried.
	firstEnd *domain.PracticeSessionEnd
	readAt   time.Time
}

// activityDay is "today" in the session and learning scenarios.
var activityDay = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

func registerActivitySteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^student "([^"]*)" is authenticated with a valid session$`, w.studentIsAuthenticated)

	// ── Practice sessions ──────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" started a (\d+)-minute practice session at (\d\d:\d\d)$`, w.startedASession)
	sc.Step(`^"([^"]*)"'s (\d+)-minute session started at (\d\d:\d\d) was abandoned after its last answer at (\d\d:\d\d)$`, w.sessionWasAbandoned)
	sc.Step(`^"([^"]*)"'s last practice\.item_answered for it arrived at (\d\d:\d\d)$`, w.answeredInTheSessionAt)
	sc.Step(`^"([^"]*)"'s practice\.session_ended for it arrives at (\d\d:\d\d), (not left early|left early), with (\d+) items answered$`, w.sessionEndedArrives)
	sc.Step(`^"([^"]*)"'s practice\.item_answered for it arrives at (\d\d:\d\d) and practice\.session_ended arrives at (\d\d:\d\d), not left early$`, w.answerAndEndArrive)
	sc.Step(`^the same practice\.session_ended event for it, with one event id, is delivered twice$`, w.sessionEndedDeliveredTwice)
	sc.Step(`^"([^"]*)" answers a challenge exercise of the content node "([^"]*)"$`, w.answersAChallengeExercise)
	sc.Step(`^the session is read at (\d\d:\d\d)$`, w.sessionIsReadAt)

	sc.Step(`^the session is finished at (\d\d:\d\d) with (\d+) items answered$`, w.sessionIsFinishedWithAnswers)
	sc.Step(`^the session is finished at (\d\d:\d\d)$`, w.sessionIsFinishedAt)
	sc.Step(`^the session ended early at (\d\d:\d\d)$`, w.sessionEndedEarlyAt)
	sc.Step(`^the session is abandoned, ended early at (\d\d:\d\d)$`, w.sessionIsAbandonedAt)
	sc.Step(`^the session is in progress$`, w.sessionIsInProgress)
	sc.Step(`^the session is finished once, with its first delivery's end$`, w.sessionIsFinishedOnce)
	sc.Step(`^no practice session gains an answer$`, w.noSessionGainsAnAnswer)

	// ── Learning activity ──────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)"'s lesson\.completed for the content node "([^"]*)" arrives, completed at (\d\d:\d\d)$`, w.lessonCompletedArrives)
	sc.Step(`^"([^"]*)" completed "([^"]*)" yesterday$`, w.completedYesterday)
	sc.Step(`^"([^"]*)" completes "([^"]*)" again today$`, w.completesToday)
	sc.Step(`^the same lesson\.completed event for "([^"]*)", with one event id, is delivered twice$`, w.lessonCompletedDeliveredTwice)
	sc.Step(`^"([^"]*)"'s learning activity shows "([^"]*)" completed at (\d\d:\d\d)$`, w.learningShowsCompletedAt)
	sc.Step(`^"([^"]*)"'s learning activity shows "([^"]*)" completed yesterday and today$`, w.learningShowsYesterdayAndToday)
	sc.Step(`^"([^"]*)"'s learning activity shows "([^"]*)" completed once$`, w.learningShowsOnce)
}

func clockAt(hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	return activityDay.Add(time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute), nil
}

func (w *world) nextEventID() string {
	w.eventNo++
	return fmt.Sprintf("e0000000-0000-4000-8000-%012d", w.eventNo)
}

func (w *world) handle(e domain.TrackingEvent) error {
	return w.events.Handle(context.Background(), e)
}

func (w *world) studentIsAuthenticated(student string) error {
	w.studentID(student)
	return nil
}

// ── Practice sessions ────────────────────────────────────────────────────────

func (w *world) startedASession(student string, minutes int, hhmm string) error {
	at, err := clockAt(hhmm)
	if err != nil {
		return err
	}
	w.activity.sessionID = stableUUID("session", student+"@"+hhmm)
	start := domain.PracticeSessionStart{
		EventID:           w.nextEventID(),
		StudentID:         w.studentID(student),
		PracticeSessionID: w.activity.sessionID,
		OccurredAt:        at,
		InstrumentID:      stableUUID("instrument", "guitar"),
		Minutes:           minutes,
		PlannedItems:      []domain.PlannedPracticeItem{{ItemKey: w.playAlongKey("pentatonic-run"), Reason: "new"}},
	}
	return w.handle(domain.TrackingEvent{
		EventType: domain.EventTypePracticeSessionStarted, EventID: start.EventID, StudentID: start.StudentID,
		OccurredAt: at, PracticeSessionID: start.PracticeSessionID, SessionStart: &start,
	})
}

func (w *world) sessionWasAbandoned(student string, minutes int, startedAt, lastAnswerAt string) error {
	if err := w.startedASession(student, minutes, startedAt); err != nil {
		return err
	}
	if err := w.answeredInTheSessionAt(student, lastAnswerAt); err != nil {
		return err
	}
	last, err := clockAt(lastAnswerAt)
	if err != nil {
		return err
	}
	// Check the premise: read after its silence, the session counts as abandoned.
	s, err := w.session(student)
	if err != nil {
		return err
	}
	if status, _ := s.StatusAt(last.Add(time.Duration(minutes)*time.Minute + 16*time.Minute)); status != domain.PracticeSessionAbandoned {
		return fmt.Errorf("the session should have been abandoned, but it is %s", status)
	}
	return nil
}

// answeredInTheSessionAt sends a rated take of a play-along in the session.
func (w *world) answeredInTheSessionAt(student, hhmm string) error {
	at, err := clockAt(hhmm)
	if err != nil {
		return err
	}
	return w.answerAt(student, w.activity.sessionID, w.playAlongKey("pentatonic-run"), at)
}

func (w *world) answerAt(student, sessionID, itemKey string, at time.Time) error {
	bpm := playAlongTargetBPM
	answer := domain.PracticeAnswer{
		EventID:           w.nextEventID(),
		StudentID:         w.studentID(student),
		OccurredAt:        at,
		PracticeSessionID: sessionID,
		ItemKey:           itemKey,
		Response:          domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingClean, TempoBPM: &bpm},
	}
	return w.handle(domain.TrackingEvent{
		EventType: domain.EventTypePracticeItemAnswered, EventID: answer.EventID, StudentID: answer.StudentID,
		OccurredAt: at, PracticeSessionID: sessionID, PracticeAnswer: &answer,
	})
}

func (w *world) endOf(student, hhmm string, leftEarly bool, answered int) (domain.PracticeSessionEnd, error) {
	at, err := clockAt(hhmm)
	if err != nil {
		return domain.PracticeSessionEnd{}, err
	}
	return domain.PracticeSessionEnd{
		EventID:           w.nextEventID(),
		StudentID:         w.studentID(student),
		PracticeSessionID: w.activity.sessionID,
		OccurredAt:        at,
		LeftEarly:         leftEarly,
		AnsweredCount:     answered,
	}, nil
}

func (w *world) deliverEnd(end domain.PracticeSessionEnd) error {
	return w.handle(domain.TrackingEvent{
		EventType: domain.EventTypePracticeSessionEnded, EventID: end.EventID, StudentID: end.StudentID,
		OccurredAt: end.OccurredAt, PracticeSessionID: end.PracticeSessionID, SessionEnd: &end,
	})
}

func (w *world) sessionEndedArrives(student, hhmm, leftEarly string, answered int) error {
	end, err := w.endOf(student, hhmm, leftEarly == "left early", answered)
	if err != nil {
		return err
	}
	return w.deliverEnd(end)
}

func (w *world) answerAndEndArrive(student, answerAt, endAt string) error {
	if err := w.answeredInTheSessionAt(student, answerAt); err != nil {
		return err
	}
	end, err := w.endOf(student, endAt, false, 2)
	if err != nil {
		return err
	}
	return w.deliverEnd(end)
}

func (w *world) sessionEndedDeliveredTwice() error {
	student := w.lastStudentName()
	end, err := w.endOf(student, "18:11", false, 6)
	if err != nil {
		return err
	}
	w.activity.firstEnd = &end
	if err := w.deliverEnd(end); err != nil {
		return err
	}
	return w.deliverEnd(end)
}

func (w *world) answersAChallengeExercise(student, contentNode string) error {
	at, err := clockAt("18:03")
	if err != nil {
		return err
	}
	// Answered outside a session, the answer carries its trigger context, not a
	// practice session id.
	exerciseKey := "exercise:" + stableUUID("exercise", contentNode)
	return w.answerAt(student, "", exerciseKey, at)
}

func (w *world) sessionIsReadAt(hhmm string) error {
	at, err := clockAt(hhmm)
	w.activity.readAt = at
	return err
}

// lastStudentName is the one student the session scenarios have.
func (w *world) lastStudentName() string {
	for name := range w.students {
		return name
	}
	return ""
}

func (w *world) session(student string) (domain.PracticeSession, error) {
	s, found, err := w.sessionRecords.Get(context.Background(), w.studentID(student), w.activity.sessionID)
	if err != nil {
		return domain.PracticeSession{}, err
	}
	if !found {
		return domain.PracticeSession{}, fmt.Errorf("no record of the session")
	}
	return s, nil
}

// statusNow reads the session when the scenario reads it, or else at the end of
// the day, long after any silence has run out.
func (w *world) statusNow() (domain.PracticeSessionStatus, *time.Time, domain.PracticeSession, error) {
	s, err := w.session(w.lastStudentName())
	if err != nil {
		return "", nil, s, err
	}
	readAt := w.activity.readAt
	if readAt.IsZero() {
		readAt = activityDay.Add(23 * time.Hour)
	}
	status, endedAt := s.StatusAt(readAt)
	return status, endedAt, s, nil
}

func (w *world) expectStatus(want domain.PracticeSessionStatus, hhmm string) error {
	status, endedAt, _, err := w.statusNow()
	if err != nil {
		return err
	}
	if status != want {
		return fmt.Errorf("the session is %s, want %s", status, want)
	}
	if hhmm == "" {
		return nil
	}
	at, err := clockAt(hhmm)
	if err != nil {
		return err
	}
	if endedAt == nil || !endedAt.Equal(at) {
		return fmt.Errorf("the session ended at %v, want %v", endedAt, at)
	}
	return nil
}

func (w *world) sessionIsFinishedAt(hhmm string) error {
	return w.expectStatus(domain.PracticeSessionFinished, hhmm)
}

func (w *world) sessionIsFinishedWithAnswers(hhmm string, answered int) error {
	if err := w.sessionIsFinishedAt(hhmm); err != nil {
		return err
	}
	_, _, s, err := w.statusNow()
	if err != nil {
		return err
	}
	if s.End.AnsweredCount != answered {
		return fmt.Errorf("the session has %d items answered, want %d", s.End.AnsweredCount, answered)
	}
	return nil
}

func (w *world) sessionEndedEarlyAt(hhmm string) error {
	return w.expectStatus(domain.PracticeSessionEndedEarly, hhmm)
}

func (w *world) sessionIsAbandonedAt(hhmm string) error {
	return w.expectStatus(domain.PracticeSessionAbandoned, hhmm)
}

func (w *world) sessionIsInProgress() error {
	return w.expectStatus(domain.PracticeSessionInProgress, "")
}

func (w *world) sessionIsFinishedOnce() error {
	if err := w.expectStatus(domain.PracticeSessionFinished, "18:11"); err != nil {
		return err
	}
	_, _, s, err := w.statusNow()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(*s.End, *w.activity.firstEnd) {
		return fmt.Errorf("the session's end is %+v, want the first delivery's %+v", *s.End, *w.activity.firstEnd)
	}
	return nil
}

func (w *world) noSessionGainsAnAnswer() error {
	if len(w.sessionRecords.sessions) > 0 {
		return fmt.Errorf("an answer outside a session was recorded in %d sessions", len(w.sessionRecords.sessions))
	}
	return nil
}

// ── Learning activity ────────────────────────────────────────────────────────

func (w *world) completion(student, contentNode string, at time.Time) domain.TrackingEvent {
	return domain.TrackingEvent{
		EventType:     domain.EventTypeLessonCompleted,
		EventID:       w.nextEventID(),
		StudentID:     w.studentID(student),
		OccurredAt:    at,
		ContentNodeID: stableUUID("content-node", contentNode),
	}
}

func (w *world) lessonCompletedArrives(student, contentNode, hhmm string) error {
	at, err := clockAt(hhmm)
	if err != nil {
		return err
	}
	return w.handle(w.completion(student, contentNode, at))
}

func (w *world) completedYesterday(student, contentNode string) error {
	return w.handle(w.completion(student, contentNode, activityDay.Add(-4*time.Hour)))
}

func (w *world) completesToday(student, contentNode string) error {
	return w.handle(w.completion(student, contentNode, activityDay.Add(19*time.Hour)))
}

func (w *world) lessonCompletedDeliveredTwice(contentNode string) error {
	e := w.completion(w.lastStudentName(), contentNode, activityDay.Add(19*time.Hour))
	if err := w.handle(e); err != nil {
		return err
	}
	return w.handle(e)
}

func (w *world) completionsOf(student, contentNode string) []time.Time {
	var at []time.Time
	for _, a := range w.learning.stored {
		if a.StudentID == w.studentID(student) && a.ContentNodeID == stableUUID("content-node", contentNode) {
			at = append(at, a.CompletedAt)
		}
	}
	slices.SortFunc(at, time.Time.Compare)
	return at
}

func (w *world) expectCompletions(student, contentNode string, want ...time.Time) error {
	got := w.completionsOf(student, contentNode)
	if !slices.EqualFunc(got, want, time.Time.Equal) {
		return fmt.Errorf("%s's completions of %q are %v, want %v", student, contentNode, got, want)
	}
	return nil
}

func (w *world) learningShowsCompletedAt(student, contentNode, hhmm string) error {
	at, err := clockAt(hhmm)
	if err != nil {
		return err
	}
	return w.expectCompletions(student, contentNode, at)
}

func (w *world) learningShowsYesterdayAndToday(student, contentNode string) error {
	return w.expectCompletions(student, contentNode, activityDay.Add(-4*time.Hour), activityDay.Add(19*time.Hour))
}

func (w *world) learningShowsOnce(student, contentNode string) error {
	if n := len(w.completionsOf(student, contentNode)); n != 1 {
		return fmt.Errorf("%s completed %q %d times, want once", student, contentNode, n)
	}
	return nil
}
