package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToGeneratedPracticeSessionItem_Song(t *testing.T) {
	bass, chart := "7b1b7c43-6d9f-5b6d-9b57-0c1e3c9f4a11", "c0000000-0000-4000-8000-0000000000c1"

	got := toGeneratedPracticeSessionItem(domain.PracticeSessionItem{
		ItemKey: "song:" + bass + ":" + chart, Kind: domain.PracticeItemKindSong, Rung: domain.PracticeRungUse,
		Reason: domain.PracticePickApplication, EstimatedSeconds: 130,
		Song: &domain.PlannedSong{
			SongChartID: chart, LayoutInstrumentID: bass, Title: "Amazing Grace", Artist: "John Newton",
			ProgressionKey: "I-IV-I-V", SectionIndex: 2, Plays: domain.SongPlaysRoots,
		},
	}, nil, "en")

	assert.Equal(t, generated.PracticeItemKindSong, got.Kind)
	require.NotNil(t, got.Song)
	assert.Equal(t, generated.PlannedSong{
		SongChartId: mustUUID(chart), LayoutInstrumentId: mustUUID(bass), Title: "Amazing Grace", Artist: "John Newton",
		ProgressionKey: "I-IV-I-V", SectionIndex: 2, Plays: generated.Roots,
	}, *got.Song)
}
