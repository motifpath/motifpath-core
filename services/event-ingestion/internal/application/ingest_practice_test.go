package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
)

func TestIngestEventService_Ingest_PracticeEventsHappyPath(t *testing.T) {
	eventTypes := []domain.EventType{
		domain.EventTypePracticeSessionStarted,
		domain.EventTypePracticeItemAnswered,
		domain.EventTypePracticeSessionEnded,
		domain.EventTypePracticeTapCheckCompleted,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			repo := newFakeRepository()
			publisher := newFakePublisher(nil)
			svc := application.NewIngestEventService(repo, newFakeOutboxRepository(), publisher, &fakeTapBaselineReader{}, testLogger())
			event := newEvent(eventType)

			_, err := svc.Ingest(context.Background(), callerUserID, event)

			require.NoError(t, err)
			require.Len(t, repo.saved, 1)
			assert.Equal(t, event, repo.saved[0])
			assert.Equal(t, event, waitForPublish(t, publisher.calls))
		})
	}
}

func TestIngestEventService_Ingest_StampsTheLatestTapOnATimedAnswer(t *testing.T) {
	repo := newFakeRepository()
	publisher := newFakePublisher(nil)
	taps := &fakeTapBaselineReader{tapMs: 350, found: true}
	svc := application.NewIngestEventService(repo, newFakeOutboxRepository(), publisher, taps, testLogger())
	answer := newEvent(domain.EventTypePracticeItemAnswered).(domain.PracticeItemAnsweredEvent)

	_, err := svc.Ingest(context.Background(), callerUserID, answer)

	require.NoError(t, err)
	require.Len(t, taps.calls, 1)
	assert.Equal(t, callerUserID, taps.calls[0].studentID)
	assert.Equal(t, answer.OccurredAt, taps.calls[0].before, "the tap check must precede the answer")

	require.Len(t, repo.saved, 1)
	stored := repo.saved[0].(domain.PracticeItemAnsweredEvent)
	require.NotNil(t, stored.TapMs)
	assert.Equal(t, 350, *stored.TapMs)

	published := waitForPublish(t, publisher.calls).(domain.PracticeItemAnsweredEvent)
	require.NotNil(t, published.TapMs)
	assert.Equal(t, 350, *published.TapMs)
}

func TestIngestEventService_Ingest_TimedAnswerWithoutATapCheckCarriesNoTap(t *testing.T) {
	repo := newFakeRepository()
	publisher := newFakePublisher(nil)
	taps := &fakeTapBaselineReader{found: false}
	svc := application.NewIngestEventService(repo, newFakeOutboxRepository(), publisher, taps, testLogger())

	_, err := svc.Ingest(context.Background(), callerUserID, newEvent(domain.EventTypePracticeItemAnswered))

	require.NoError(t, err)
	assert.Equal(t, 1, taps.callCount())
	assert.Nil(t, repo.saved[0].(domain.PracticeItemAnsweredEvent).TapMs)
	assert.Nil(t, waitForPublish(t, publisher.calls).(domain.PracticeItemAnsweredEvent).TapMs)
}

func TestIngestEventService_Ingest_NeverStampsASelfRatedTake(t *testing.T) {
	repo := newFakeRepository()
	publisher := newFakePublisher(nil)
	taps := &fakeTapBaselineReader{tapMs: 350, found: true}
	svc := application.NewIngestEventService(repo, newFakeOutboxRepository(), publisher, taps, testLogger())
	take := newSelfRatedAnswer(newEvent(domain.EventTypePracticeItemAnswered).Base())

	_, err := svc.Ingest(context.Background(), callerUserID, take)

	require.NoError(t, err)
	assert.Equal(t, 0, taps.callCount(), "an untimed answer needs no tap lookup")
	assert.Nil(t, repo.saved[0].(domain.PracticeItemAnsweredEvent).TapMs)
	assert.Nil(t, waitForPublish(t, publisher.calls).(domain.PracticeItemAnsweredEvent).TapMs)
}

func TestIngestEventService_Ingest_ReplacesAnyTapAlreadyOnTheEvent(t *testing.T) {
	// The HTTP mapping drops a client's tap_ms; this guards the application rule on
	// its own, so an event built any other way still carries only the server's value.
	repo := newFakeRepository()
	taps := &fakeTapBaselineReader{found: false}
	svc := application.NewIngestEventService(repo, newFakeOutboxRepository(), newFakePublisher(nil), taps, testLogger())
	answer := newEvent(domain.EventTypePracticeItemAnswered).(domain.PracticeItemAnsweredEvent)
	claimed := 5
	answer.TapMs = &claimed

	_, err := svc.Ingest(context.Background(), callerUserID, answer)

	require.NoError(t, err)
	assert.Nil(t, repo.saved[0].(domain.PracticeItemAnsweredEvent).TapMs)
}

func TestIngestEventService_Ingest_TapLookupFailureFailsTheRequest(t *testing.T) {
	// The lookup reads the same MongoDB the event is written to: when it fails, the
	// write would very likely fail too, and the client retries with the same event_id.
	// Storing the answer without its tap would grade it as if the student had none.
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	taps := &fakeTapBaselineReader{err: errors.New("connection refused")}
	svc := application.NewIngestEventService(repo, outbox, publisher, taps, testLogger())

	_, err := svc.Ingest(context.Background(), callerUserID, newEvent(domain.EventTypePracticeItemAnswered))

	require.Error(t, err)
	assert.Empty(t, repo.saved)
	assert.Equal(t, 0, outbox.createCalls)
	select {
	case <-publisher.calls:
		t.Fatal("nothing must be published when the tap lookup fails")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestIngestEventService_Ingest_IdentityMismatchIsCheckedBeforeTheTapLookup(t *testing.T) {
	taps := &fakeTapBaselineReader{tapMs: 350, found: true}
	svc := application.NewIngestEventService(newFakeRepository(), newFakeOutboxRepository(), newFakePublisher(nil), taps, testLogger())

	_, err := svc.Ingest(context.Background(), "someone-else", newEvent(domain.EventTypePracticeItemAnswered))

	assert.ErrorIs(t, err, domain.ErrIdentityMismatch)
	assert.Equal(t, 0, taps.callCount(), "another student's tap time must never be read")
}
