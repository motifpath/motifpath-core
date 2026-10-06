//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

func registerFeltSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]*)"'s session practised only "([^"]*)"$`, w.sessionPractisedOnly)
	sc.Step(`^the session ends with "([^"]*)" rated "([^"]*)"$`, w.sessionEndsWithRating)
	sc.Step(`^the rating is not used to calibrate "([^"]*)"$`, w.ratingIsNotUsedToCalibrate)
}

// sessionPractisedOnly starts a session and answers the current cell in it, the
// way the drill template asks it, through the event handler like the consumer.
func (w *world) sessionPractisedOnly(student, template string) error {
	asked, ok := strings.CutPrefix(template, string(domain.PracticeItemKindFretboardCell)+":")
	if !ok || asked != string(domain.PracticeResponseNameTheNote) {
		return fmt.Errorf("only %q sessions are set up here, not %q", "fretboard_cell:name_the_note", template)
	}
	if err := w.startedASession(student, 10, "18:00"); err != nil {
		return err
	}
	at, err := clockAt("18:03")
	if err != nil {
		return err
	}
	note, err := noteOf(w.currentCell(), 0)
	if err != nil {
		return err
	}
	latency := cellLatency
	answer := domain.PracticeAnswer{
		EventID:           w.nextEventID(),
		StudentID:         w.studentID(student),
		OccurredAt:        at,
		PracticeSessionID: w.activity.sessionID,
		ItemKey:           w.currentCell(),
		Response:          domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: note, LatencyMs: &latency},
	}
	return w.handle(domain.TrackingEvent{
		EventType: domain.EventTypePracticeItemAnswered, EventID: answer.EventID, StudentID: answer.StudentID,
		OccurredAt: at, PracticeSessionID: answer.PracticeSessionID, PracticeAnswer: &answer,
	})
}

func (w *world) sessionEndsWithRating(template, felt string) error {
	end, err := w.endOf(w.lastStudentName(), "18:11", false, 1)
	if err != nil {
		return err
	}
	end.FeltRatings = []domain.FeltRating{{DrillTemplateKey: template, Felt: domain.Felt(felt)}}
	return w.deliverEnd(end)
}

// ratingIsNotUsedToCalibrate checks the session keeps the rating but doesn't
// count it as a felt-rated session of the template.
func (w *world) ratingIsNotUsedToCalibrate(template string) error {
	s, err := w.session(w.lastStudentName())
	if err != nil {
		return err
	}
	if s.End == nil || !slices.ContainsFunc(s.End.FeltRatings, func(r domain.FeltRating) bool { return r.DrillTemplateKey == template }) {
		return fmt.Errorf("the session's end doesn't keep the rating of %q", template)
	}
	if slices.Contains(s.FeltRatedTemplates(), template) {
		return fmt.Errorf("the session counts as felt-rated for %q, which it didn't practise", template)
	}
	return nil
}
