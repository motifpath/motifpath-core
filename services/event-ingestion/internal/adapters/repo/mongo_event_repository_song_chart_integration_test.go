//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/motifpath/event-ingestion/internal/domain"
)

var songChartRead = domain.SongChartContext{SongChartID: "55555555-5555-4555-8555-555555555555", RevisionNumber: 2}

func TestMongoEventRepository_FindByEventID_RoundTripsSongChartEvents(t *testing.T) {
	repo := setupMongoRepository(t)
	ctx := context.Background()

	events := []domain.TrackingEvent{
		domain.SongChartOpenedEvent{
			TrackingEventBase: practiceBase("b1000000-0000-4000-8000-000000000001", domain.EventTypeSongChartOpened, practiceStudentID, practiceAt),
			SongChartContext:  songChartRead,
		},
		domain.SongChartChordViewedEvent{
			TrackingEventBase: practiceBase("b1000000-0000-4000-8000-000000000002", domain.EventTypeSongChartChordViewed, practiceStudentID, practiceAt),
			SongChartContext:  songChartRead,
			AnchorID:          "a3",
			ChordDefinitionID: "66666666-6666-4666-8666-666666666666",
			ChordVoicingID:    "77777777-7777-4777-8777-777777777777",
		},
		domain.SongChartCompletedEvent{
			TrackingEventBase: practiceBase("b1000000-0000-4000-8000-000000000003", domain.EventTypeSongChartCompleted, practiceStudentID, practiceAt),
			SongChartContext:  songChartRead,
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

func TestMongoEventRepository_Save_WritesTheSongChartContext(t *testing.T) {
	repo := setupMongoRepository(t)
	ctx := context.Background()
	event := domain.SongChartCompletedEvent{
		TrackingEventBase: practiceBase("b2000000-0000-4000-8000-000000000001", domain.EventTypeSongChartCompleted, practiceStudentID, practiceAt),
		SongChartContext:  songChartRead,
	}

	_, _, err := repo.Save(ctx, event)
	require.NoError(t, err)

	raw, err := repo.collection.FindOne(ctx, bson.D{{Key: "event_id", Value: event.EventID}}).Raw()
	require.NoError(t, err)
	var chart bson.M
	require.NoError(t, bson.Unmarshal(raw.Lookup("song_chart_context").Document(), &chart))
	assert.Equal(t, bson.M{"song_chart_id": songChartRead.SongChartID, "revision_number": int32(2)}, chart)
	_, err = raw.LookupErr("section_index")
	assert.Error(t, err, "a completed song has no section")
}
