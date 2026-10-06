package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/aggregation-worker/internal/application"
	"github.com/motifpath/aggregation-worker/internal/domain"
)

const (
	sessionOne = "5e551000-0000-4000-8000-000000000001"
	introNode  = "c0000000-0000-4000-8000-000000000001"
)

type fakeSessions struct {
	sessions map[string]domain.PracticeSession
	puts     int
	getErr   error
	putErr   error
}

func newFakeSessions() *fakeSessions {
	return &fakeSessions{sessions: map[string]domain.PracticeSession{}}
}

func (f *fakeSessions) Get(_ context.Context, studentID, sessionID string) (domain.PracticeSession, bool, error) {
	if f.getErr != nil {
		return domain.PracticeSession{}, false, f.getErr
	}
	s, ok := f.sessions[studentID+"|"+sessionID]
	return s, ok, nil
}

func (f *fakeSessions) Put(_ context.Context, s domain.PracticeSession) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.puts++
	f.sessions[s.StudentID+"|"+s.ID] = s
	return nil
}

type fakeTapChecks struct {
	stored    []domain.TapCheck
	insertErr error
}

func (f *fakeTapChecks) Insert(_ context.Context, c domain.TapCheck) (bool, error) {
	if f.insertErr != nil {
		return false, f.insertErr
	}
	for _, s := range f.stored {
		if s.EventID == c.EventID {
			return false, nil
		}
	}
	f.stored = append(f.stored, c)
	return true, nil
}

type fakeLearning struct {
	stored    []domain.LearningActivity
	insertErr error
}

func (f *fakeLearning) Insert(_ context.Context, a domain.LearningActivity) (bool, error) {
	if f.insertErr != nil {
		return false, f.insertErr
	}
	for _, s := range f.stored {
		if s.EventID == a.EventID {
			return false, nil
		}
	}
	f.stored = append(f.stored, a)
	return true, nil
}

func clock(hh, mm int) time.Time { return time.Date(2026, 10, 5, hh, mm, 0, 0, time.UTC) }

func sessionStartEvent(at time.Time) domain.TrackingEvent {
	return domain.TrackingEvent{
		EventType:         domain.EventTypePracticeSessionStarted,
		EventID:           "e0000000-0000-4000-8000-0000000000a1",
		StudentID:         alice,
		OccurredAt:        at,
		PracticeSessionID: sessionOne,
		SessionStart: &domain.PracticeSessionStart{
			EventID:           "e0000000-0000-4000-8000-0000000000a1",
			StudentID:         alice,
			PracticeSessionID: sessionOne,
			OccurredAt:        at,
			InstrumentID:      "1a500000-0000-4000-8000-000000000001",
			Minutes:           10,
			PlannedItems:      []domain.PlannedPracticeItem{{ItemKey: playAlongKey, Reason: "due"}},
		},
	}
}

func sessionEndEvent(eventID string, at time.Time, leftEarly bool, answered int) domain.TrackingEvent {
	return domain.TrackingEvent{
		EventType:         domain.EventTypePracticeSessionEnded,
		EventID:           eventID,
		StudentID:         alice,
		OccurredAt:        at,
		PracticeSessionID: sessionOne,
		SessionEnd: &domain.PracticeSessionEnd{
			EventID:           eventID,
			StudentID:         alice,
			PracticeSessionID: sessionOne,
			OccurredAt:        at,
			LeftEarly:         leftEarly,
			AnsweredCount:     answered,
		},
	}
}

func answeredEvent(eventID string, at time.Time, sessionID string) domain.TrackingEvent {
	answer := ratedTake(eventID, at, domain.SelfRatingClean, 90)
	answer.PracticeSessionID = sessionID
	return domain.TrackingEvent{
		EventType:         domain.EventTypePracticeItemAnswered,
		EventID:           eventID,
		StudentID:         alice,
		OccurredAt:        at,
		PracticeSessionID: sessionID,
		PracticeAnswer:    &answer,
	}
}

func lessonCompletedEvent(eventID string, at time.Time) domain.TrackingEvent {
	return domain.TrackingEvent{
		EventType:     domain.EventTypeLessonCompleted,
		EventID:       eventID,
		StudentID:     alice,
		OccurredAt:    at,
		ContentNodeID: introNode,
	}
}

func newActivity() *application.ActivityService {
	return application.NewActivityService(newFakeSessions(), &fakeLearning{}, &fakeTapChecks{})
}

type activityFixture struct {
	sessions  *fakeSessions
	learning  *fakeLearning
	tapChecks *fakeTapChecks
	practice  *practiceFixture
	service   *application.ProcessEventService
}

func newActivityFixture() *activityFixture {
	f := &activityFixture{sessions: newFakeSessions(), learning: &fakeLearning{}, tapChecks: &fakeTapChecks{}, practice: newPracticeFixture()}
	f.service = application.NewProcessEventService(newFakeRepository(), f.practice.service,
		application.NewActivityService(f.sessions, f.learning, f.tapChecks))
	return f
}

func (f *activityFixture) handle(t *testing.T, events ...domain.TrackingEvent) {
	t.Helper()
	for _, e := range events {
		require.NoError(t, f.service.Handle(context.Background(), e))
	}
}

func (f *activityFixture) session(t *testing.T) domain.PracticeSession {
	t.Helper()
	s, ok := f.sessions.sessions[alice+"|"+sessionOne]
	require.True(t, ok, "no session record")
	return s
}

func TestActivityService_KeepsEachPracticeSession(t *testing.T) {
	cases := []struct {
		name        string
		events      []domain.TrackingEvent
		readAt      time.Time
		wantStatus  domain.PracticeSessionStatus
		wantEndedAt *time.Time
		wantCount   *int
	}{
		{
			name: "ended without leaving early is finished",
			events: []domain.TrackingEvent{sessionStartEvent(clock(18, 0)),
				sessionEndEvent("e0000000-0000-4000-8000-0000000000e1", clock(18, 11), false, 6)},
			readAt: clock(23, 0), wantStatus: domain.PracticeSessionFinished,
			wantEndedAt: ptrTo(clock(18, 11)), wantCount: ptrTo(6),
		},
		{
			name: "left early ended early",
			events: []domain.TrackingEvent{sessionStartEvent(clock(18, 0)),
				sessionEndEvent("e0000000-0000-4000-8000-0000000000e1", clock(18, 4), true, 2)},
			readAt: clock(23, 0), wantStatus: domain.PracticeSessionEndedEarly,
			wantEndedAt: ptrTo(clock(18, 4)), wantCount: ptrTo(2),
		},
		{
			name: "an answer moves the session's last event, so silence is counted from it",
			events: []domain.TrackingEvent{sessionStartEvent(clock(18, 0)),
				answeredEvent("e0000000-0000-4000-8000-000000000001", clock(18, 3), sessionOne)},
			readAt: clock(18, 29), wantStatus: domain.PracticeSessionAbandoned, wantEndedAt: ptrTo(clock(18, 3)),
		},
		{
			name: "an answer the grader rejects still shows the student was there",
			events: []domain.TrackingEvent{sessionStartEvent(clock(18, 0)), func() domain.TrackingEvent {
				e := answeredEvent("e0000000-0000-4000-8000-000000000001", clock(18, 20), sessionOne)
				e.PracticeAnswer.Response.TempoBPM = nil
				return e
			}()},
			readAt: clock(18, 40), wantStatus: domain.PracticeSessionInProgress,
		},
		{
			name: "a late answer and end reopen an abandoned session",
			events: []domain.TrackingEvent{sessionStartEvent(clock(18, 0)),
				answeredEvent("e0000000-0000-4000-8000-000000000001", clock(18, 3), sessionOne),
				answeredEvent("e0000000-0000-4000-8000-000000000002", clock(18, 30), sessionOne),
				sessionEndEvent("e0000000-0000-4000-8000-0000000000e1", clock(18, 35), false, 2)},
			readAt: clock(23, 0), wantStatus: domain.PracticeSessionFinished,
			wantEndedAt: ptrTo(clock(18, 35)), wantCount: ptrTo(2),
		},
		{
			name: "the same end delivered twice keeps its first delivery",
			events: []domain.TrackingEvent{sessionStartEvent(clock(18, 0)),
				sessionEndEvent("e0000000-0000-4000-8000-0000000000e1", clock(18, 11), false, 6),
				sessionEndEvent("e0000000-0000-4000-8000-0000000000e1", clock(18, 11), false, 6)},
			readAt: clock(23, 0), wantStatus: domain.PracticeSessionFinished,
			wantEndedAt: ptrTo(clock(18, 11)), wantCount: ptrTo(6),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newActivityFixture()

			f.handle(t, c.events...)

			s := f.session(t)
			status, endedAt := s.StatusAt(c.readAt)
			assert.Equal(t, c.wantStatus, status)
			assert.Equal(t, c.wantEndedAt, endedAt)
			if c.wantCount != nil {
				require.NotNil(t, s.End)
				assert.Equal(t, *c.wantCount, s.End.AnsweredCount)
			}
		})
	}
}

func TestActivityService_KeepsThePlanAndInstrument(t *testing.T) {
	f := newActivityFixture()

	f.handle(t, sessionStartEvent(clock(18, 0)))

	s := f.session(t)
	assert.Equal(t, alice, s.StudentID)
	assert.Equal(t, sessionOne, s.ID)
	assert.Equal(t, "1a500000-0000-4000-8000-000000000001", s.InstrumentID)
	assert.Equal(t, 10, s.Minutes)
	assert.Equal(t, []domain.PlannedPracticeItem{{ItemKey: playAlongKey, Reason: "due"}}, s.PlannedItems)
}

func TestActivityService_AnswersOutsideASessionBelongToNoSession(t *testing.T) {
	f := newActivityFixture()

	f.handle(t, answeredEvent("e0000000-0000-4000-8000-000000000001", clock(18, 3), ""))

	assert.Empty(t, f.sessions.sessions)
	// The answer itself is still graded.
	assert.Len(t, f.practice.evidence.stored, 1)
}

func TestActivityService_KeepsEveryContentNodeCompletion(t *testing.T) {
	cases := []struct {
		name   string
		events []domain.TrackingEvent
		want   []time.Time
	}{
		{
			name:   "a completion is kept with when it was completed",
			events: []domain.TrackingEvent{lessonCompletedEvent("e0000000-0000-4000-8000-0000000000c1", clock(19, 30))},
			want:   []time.Time{clock(19, 30)},
		},
		{
			name: "completing it again is another completion",
			events: []domain.TrackingEvent{
				lessonCompletedEvent("e0000000-0000-4000-8000-0000000000c1", clock(19, 30).AddDate(0, 0, -1)),
				lessonCompletedEvent("e0000000-0000-4000-8000-0000000000c2", clock(19, 30)),
			},
			want: []time.Time{clock(19, 30).AddDate(0, 0, -1), clock(19, 30)},
		},
		{
			name: "the same event delivered twice is kept once",
			events: []domain.TrackingEvent{
				lessonCompletedEvent("e0000000-0000-4000-8000-0000000000c1", clock(19, 30)),
				lessonCompletedEvent("e0000000-0000-4000-8000-0000000000c1", clock(19, 30)),
			},
			want: []time.Time{clock(19, 30)},
		},
		{
			name: "starting or resuming a content node isn't a completion",
			events: []domain.TrackingEvent{
				{EventType: domain.EventTypeLessonStarted, EventID: "e0000000-0000-4000-8000-0000000000c1",
					StudentID: alice, OccurredAt: clock(19, 0), ContentNodeID: introNode},
				{EventType: domain.EventTypeLessonResumed, EventID: "e0000000-0000-4000-8000-0000000000c2",
					StudentID: alice, OccurredAt: clock(19, 10), ContentNodeID: introNode},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newActivityFixture()

			f.handle(t, c.events...)

			var got []time.Time
			for _, a := range f.learning.stored {
				assert.Equal(t, alice, a.StudentID)
				assert.Equal(t, introNode, a.ContentNodeID)
				got = append(got, a.CompletedAt)
			}
			assert.Equal(t, c.want, got)
		})
	}
}

func TestActivityService_ReturnsStorageFailuresForARetry(t *testing.T) {
	boom := errors.New("connection refused")
	cases := []struct {
		name    string
		breakIt func(f *activityFixture)
		event   domain.TrackingEvent
	}{
		{"reading a session", func(f *activityFixture) { f.sessions.getErr = boom }, sessionStartEvent(clock(18, 0))},
		{"writing a session", func(f *activityFixture) { f.sessions.putErr = boom },
			sessionEndEvent("e0000000-0000-4000-8000-0000000000e1", clock(18, 11), false, 6)},
		{"touching a session on an answer", func(f *activityFixture) { f.sessions.putErr = boom },
			answeredEvent("e0000000-0000-4000-8000-000000000001", clock(18, 3), sessionOne)},
		{"keeping a completion", func(f *activityFixture) { f.learning.insertErr = boom },
			lessonCompletedEvent("e0000000-0000-4000-8000-0000000000c1", clock(19, 30))},
		{"keeping a tap check", func(f *activityFixture) { f.tapChecks.insertErr = boom },
			tapCheckEvent("e0000000-0000-4000-8000-0000000000f1", clock(9, 0), 320)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newActivityFixture()
			c.breakIt(f)

			assert.ErrorIs(t, f.service.Handle(context.Background(), c.event), boom)
		})
	}
}

func ptrTo[T any](v T) *T { return &v }

func TestActivityService_KeepsEachSessionsPractisedDrillsAndFeltRatings(t *testing.T) {
	latency := 3500
	inSession := func(answer domain.PracticeAnswer) domain.TrackingEvent {
		answer.PracticeSessionID = sessionOne
		return domain.TrackingEvent{
			EventType: domain.EventTypePracticeItemAnswered, EventID: answer.EventID, StudentID: alice,
			OccurredAt: answer.OccurredAt, PracticeSessionID: sessionOne, PracticeAnswer: &answer,
		}
	}
	named := inSession(cellAnswer("e0000000-0000-4000-8000-0000000000c1", clock(18, 3),
		domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: &latency}))
	rejected := inSession(cellAnswer("e0000000-0000-4000-8000-0000000000c2", clock(18, 4),
		domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, LatencyMs: &latency}))
	take := answeredEvent("e0000000-0000-4000-8000-0000000000c3", clock(18, 5), sessionOne)
	ended := sessionEndEvent("e0000000-0000-4000-8000-0000000000e1", clock(18, 11), false, 3)
	ended.SessionEnd.FeltRatings = []domain.FeltRating{
		{DrillTemplateKey: "fretboard_cell:name_the_note", Felt: domain.FeltHard},
		{DrillTemplateKey: "exercise:image_choice", Felt: domain.FeltEasy},
	}

	f := newActivityFixture()
	f.handle(t, sessionStartEvent(clock(18, 0)), named, rejected, take, ended)

	s := f.session(t)
	assert.Equal(t, []string{"fretboard_cell:name_the_note"}, s.PractisedTemplates, "a rejected answer and a rated take practise no timed drill")
	assert.Equal(t, ended.SessionEnd.FeltRatings, s.End.FeltRatings)
	assert.Equal(t, []string{"fretboard_cell:name_the_note"}, s.FeltRatedTemplates())
	assert.Equal(t, clock(18, 11), s.LastEventAt)
}

func tapCheckEvent(eventID string, at time.Time, medianMs int) domain.TrackingEvent {
	return domain.TrackingEvent{
		EventType:  domain.EventTypePracticeTapCheckCompleted,
		EventID:    eventID,
		StudentID:  alice,
		OccurredAt: at,
		TapCheck:   &domain.TapCheck{EventID: eventID, StudentID: alice, DoneAt: at, MedianTapMs: medianMs, TapCount: 24},
	}
}

func TestActivityService_KeepsEveryTapCheck(t *testing.T) {
	cases := []struct {
		name   string
		events []domain.TrackingEvent
		want   []domain.TapCheck
	}{
		{
			name:   "a tap check is kept with when it was done, its median and its taps",
			events: []domain.TrackingEvent{tapCheckEvent("e0000000-0000-4000-8000-0000000000f1", clock(9, 0), 320)},
			want: []domain.TapCheck{
				{EventID: "e0000000-0000-4000-8000-0000000000f1", StudentID: alice, DoneAt: clock(9, 0), MedianTapMs: 320, TapCount: 24},
			},
		},
		{
			name: "doing it again is another tap check",
			events: []domain.TrackingEvent{
				tapCheckEvent("e0000000-0000-4000-8000-0000000000f1", clock(9, 0).AddDate(0, 0, -31), 380),
				tapCheckEvent("e0000000-0000-4000-8000-0000000000f2", clock(9, 0), 320),
			},
			want: []domain.TapCheck{
				{EventID: "e0000000-0000-4000-8000-0000000000f1", StudentID: alice, DoneAt: clock(9, 0).AddDate(0, 0, -31), MedianTapMs: 380, TapCount: 24},
				{EventID: "e0000000-0000-4000-8000-0000000000f2", StudentID: alice, DoneAt: clock(9, 0), MedianTapMs: 320, TapCount: 24},
			},
		},
		{
			name: "the same event delivered twice is kept once",
			events: []domain.TrackingEvent{
				tapCheckEvent("e0000000-0000-4000-8000-0000000000f1", clock(9, 0), 320),
				tapCheckEvent("e0000000-0000-4000-8000-0000000000f1", clock(9, 0), 320),
			},
			want: []domain.TapCheck{
				{EventID: "e0000000-0000-4000-8000-0000000000f1", StudentID: alice, DoneAt: clock(9, 0), MedianTapMs: 320, TapCount: 24},
			},
		},
		{
			name:   "a tap check event without its fields changes nothing",
			events: []domain.TrackingEvent{{EventType: domain.EventTypePracticeTapCheckCompleted, EventID: "e0000000-0000-4000-8000-0000000000f1", StudentID: alice}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newActivityFixture()

			f.handle(t, c.events...)

			assert.Equal(t, c.want, f.tapChecks.stored)
		})
	}
}
