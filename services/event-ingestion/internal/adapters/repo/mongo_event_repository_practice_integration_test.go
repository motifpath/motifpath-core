//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/motifpath/event-ingestion/internal/domain"
)

const (
	practiceStudentID = "22222222-2222-2222-2222-222222222222"
	practiceSessionID = "44444444-4444-4444-4444-444444444444"
	cellItemKey       = "fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:5:3"
)

var practiceAt = time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)

func practiceBase(eventID string, eventType domain.EventType, studentID string, at time.Time) domain.TrackingEventBase {
	return domain.TrackingEventBase{
		EventID:    eventID,
		EventType:  eventType,
		StudentID:  studentID,
		SessionID:  "33333333-3333-3333-3333-333333333333",
		OccurredAt: at,
	}
}

func intRef(v int) *int { return &v }

func TestMongoEventRepository_FindByEventID_RoundTripsPracticeEvents(t *testing.T) {
	repo := setupMongoRepository(t)
	ctx := context.Background()

	events := []domain.TrackingEvent{
		domain.PracticeSessionStartedEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000001", domain.EventTypePracticeSessionStarted, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			InstrumentID:      "6ea2d087-ab9c-59dc-9657-8546025414d2",
			Minutes:           10,
			PlannedItems: []domain.PlannedPracticeItem{
				{ItemKey: "play_along:55555555-5555-4555-8555-555555555555", Reason: domain.PracticePickReasonWarmUp},
				{ItemKey: cellItemKey, Reason: domain.PracticePickReasonNew},
			},
		},
		domain.PracticeSessionStartedEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000002", domain.EventTypePracticeSessionStarted, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			Minutes:           5,
			PlannedItems:      []domain.PlannedPracticeItem{{ItemKey: cellItemKey, Reason: domain.PracticePickReasonDue}},
		},
		domain.PracticeItemAnsweredEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000003", domain.EventTypePracticeItemAnswered, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			ItemKey:           cellItemKey,
			Response:          domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: intRef(1800)},
			TapMs:             intRef(350),
		},
		domain.PracticeItemAnsweredEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000004", domain.EventTypePracticeItemAnswered, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			ItemKey:           cellItemKey,
			Response:          domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, String: intRef(6), Fret: intRef(0), LatencyMs: intRef(0)},
		},
		domain.PracticeItemAnsweredEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-0000000000a1", domain.EventTypePracticeItemAnswered, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			ItemKey:           "diagram_shape:55555555-5555-4555-8555-555555555555",
			Response:          domain.PracticeResponse{Type: domain.PracticeResponseNameTheShape, Shape: "A", LatencyMs: intRef(2600)},
		},
		domain.PracticeItemAnsweredEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-0000000000a2", domain.EventTypePracticeItemAnswered, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			ItemKey:           "diagram_shape:55555555-5555-4555-8555-555555555555",
			Response:          domain.PracticeResponse{Type: domain.PracticeResponseFindTheDegree, Interval: "b3", String: intRef(2), Fret: intRef(5), LatencyMs: intRef(2400)},
		},
		domain.PracticeItemAnsweredEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000005", domain.EventTypePracticeItemAnswered, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			ItemKey:           "exercise:55555555-5555-4555-8555-555555555555",
			Response: domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice,
				OptionIDs: []string{"66666666-6666-4666-8666-666666666666", "77777777-7777-4777-8777-777777777777"}, LatencyMs: intRef(4000)},
		},
		domain.PracticeItemAnsweredEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-00000000000a", domain.EventTypePracticeItemAnswered, practiceStudentID, practiceAt),
			TriggerContext: &domain.TriggerContext{
				Source:        domain.TriggerSourceChallengeSequence,
				ChallengeID:   "88888888-8888-4888-8888-888888888888",
				ContentNodeID: "99999999-9999-4999-8999-999999999999",
			},
			ItemKey: "exercise:55555555-5555-4555-8555-555555555555",
			Response: domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice,
				OptionIDs: []string{"66666666-6666-4666-8666-666666666666"}, LatencyMs: intRef(9500), AudioMs: intRef(5000)},
		},
		domain.PracticeItemAnsweredEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000006", domain.EventTypePracticeItemAnswered, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			ItemKey:           "play_along:55555555-5555-4555-8555-555555555555",
			Response:          domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingStruggled, TempoBPM: intRef(60)},
		},
		domain.PracticeSessionEndedEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000007", domain.EventTypePracticeSessionEnded, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			AnsweredCount:     6,
			FeltRatings:       []domain.FeltRating{{DrillTemplateKey: "fretboard_cell:name_the_note", Felt: domain.FeltAboutRight}},
		},
		domain.PracticeSessionEndedEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000008", domain.EventTypePracticeSessionEnded, practiceStudentID, practiceAt),
			PracticeSessionID: practiceSessionID,
			AnsweredCount:     0,
			LeftEarly:         true,
			FeltRatings:       []domain.FeltRating{},
		},
		domain.PracticeTapCheckCompletedEvent{
			TrackingEventBase: practiceBase("a1000000-0000-4000-8000-000000000009", domain.EventTypePracticeTapCheckCompleted, practiceStudentID, practiceAt),
			MedianTapMs:       350,
			TapCount:          24,
		},
	}

	for _, event := range events {
		t.Run(event.Base().EventID, func(t *testing.T) {
			_, _, err := repo.Save(ctx, event)
			require.NoError(t, err)

			found, err := repo.FindByEventID(ctx, event.Base().EventID)

			require.NoError(t, err)
			assert.Equal(t, event, found)
		})
	}
}

func TestMongoEventRepository_Save_WritesPracticeAnswerFields(t *testing.T) {
	repo := setupMongoRepository(t)
	ctx := context.Background()
	event := domain.PracticeItemAnsweredEvent{
		TrackingEventBase: practiceBase("a2000000-0000-4000-8000-000000000001", domain.EventTypePracticeItemAnswered, practiceStudentID, practiceAt),
		PracticeSessionID: practiceSessionID,
		ItemKey:           cellItemKey,
		Response:          domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: intRef(1800)},
		TapMs:             intRef(350),
	}

	_, _, err := repo.Save(ctx, event)
	require.NoError(t, err)

	raw, err := repo.collection.FindOne(ctx, bson.D{{Key: "event_id", Value: event.EventID}}).Raw()
	require.NoError(t, err)
	assert.Equal(t, practiceSessionID, raw.Lookup("practice_session_id").StringValue())
	assert.Equal(t, cellItemKey, raw.Lookup("item_key").StringValue())
	assert.EqualValues(t, 350, raw.Lookup("tap_ms").AsInt64())

	// The response is an embedded document holding only its shape's properties.
	var response bson.M
	require.NoError(t, bson.Unmarshal(raw.Lookup("response").Document(), &response))
	assert.Equal(t, bson.M{"response_type": "name_the_note", "note_name": "C", "latency_ms": int32(1800)}, response)
}

func TestMongoEventRepository_LatestTap(t *testing.T) {
	const other = "99999999-9999-9999-9999-999999999999"
	answerAt := practiceAt

	tapCheck := func(id, studentID string, at time.Time, medianTapMs int) domain.TrackingEvent {
		return domain.PracticeTapCheckCompletedEvent{
			TrackingEventBase: practiceBase(id, domain.EventTypePracticeTapCheckCompleted, studentID, at),
			MedianTapMs:       medianTapMs,
			TapCount:          24,
		}
	}

	t.Run("no tap check", func(t *testing.T) {
		repo := setupMongoRepository(t)

		_, found, err := repo.LatestTap(context.Background(), practiceStudentID, answerAt)

		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("newest tap check before the answer wins", func(t *testing.T) {
		repo := setupMongoRepository(t)
		ctx := context.Background()
		for _, e := range []domain.TrackingEvent{
			tapCheck("a3000000-0000-4000-8000-000000000001", practiceStudentID, answerAt.Add(-2*time.Hour), 420),
			tapCheck("a3000000-0000-4000-8000-000000000002", practiceStudentID, answerAt.Add(-time.Hour), 350),
			// Later than the answer: a tap check the student had not done yet.
			tapCheck("a3000000-0000-4000-8000-000000000003", practiceStudentID, answerAt.Add(time.Minute), 300),
			// Exactly at the answer: not before it.
			tapCheck("a3000000-0000-4000-8000-000000000004", practiceStudentID, answerAt, 310),
			// Another student's, newer.
			tapCheck("a3000000-0000-4000-8000-000000000005", other, answerAt.Add(-time.Minute), 200),
		} {
			_, _, err := repo.Save(ctx, e)
			require.NoError(t, err)
		}
		// A newer event of another type for the same student.
		_, _, err := repo.Save(ctx, domain.PracticeSessionEndedEvent{
			TrackingEventBase: practiceBase("a3000000-0000-4000-8000-000000000006", domain.EventTypePracticeSessionEnded, practiceStudentID, answerAt.Add(-time.Minute)),
			PracticeSessionID: practiceSessionID,
			FeltRatings:       []domain.FeltRating{},
		})
		require.NoError(t, err)

		tapMs, found, err := repo.LatestTap(ctx, practiceStudentID, answerAt)

		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, 350, tapMs)
	})
}

func TestMongoEventRepository_EnsureIndexes_CreatesTheLatestTapIndex(t *testing.T) {
	repo := setupMongoRepository(t)
	ctx := context.Background()

	cursor, err := repo.collection.Indexes().List(ctx)
	require.NoError(t, err)
	var indexes []bson.M
	require.NoError(t, cursor.All(ctx, &indexes))

	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		names = append(names, idx["name"].(string)) //nolint:forcetypeassert // index name is always a string
	}
	assert.Contains(t, names, "student_id_1_event_type_1_occurred_at_-1")
}
