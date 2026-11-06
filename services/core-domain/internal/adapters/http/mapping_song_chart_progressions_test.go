package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToGeneratedNotPublishable_UnplayedProgressions(t *testing.T) {
	got := toGeneratedNotPublishable(&domain.SongChartNotPublishableError{
		Reasons:              []domain.NotPublishableReason{domain.ProgressionsNotPlayed},
		UnplayedProgressions: []domain.SectionProgression{{SectionIndex: 1, ProgressionKey: "I-V-vi-IV"}},
	})

	assert.Equal(t, []generated.SongChartUnplayedProgression{{SectionIndex: 1, ProgressionKey: "I-V-vi-IV"}}, got.UnplayedProgressions)
	assert.NotNil(t, toGeneratedNotPublishable(&domain.SongChartNotPublishableError{}).UnplayedProgressions, "empty, never null")
}

func TestToGeneratedSongChartRevision_ProgressionKeys(t *testing.T) {
	admin := "a0000000-0000-4000-8000-0000000000a1"
	rev := domain.SongChartRevision{
		SongChartID: "c0000000-0000-4000-8000-0000000000c1", Number: 1,
		RightsConfirmation: domain.RightsConfirmation{ConfirmedBy: admin}, PublishedBy: admin,
		SectionProgressions: []domain.SectionProgression{
			{SectionIndex: 0, ProgressionKey: "I-IV-I-V"}, {SectionIndex: 1, ProgressionKey: "I-V-vi-IV"},
			{SectionIndex: 2, ProgressionKey: "I-IV-I-V"},
		},
	}

	got, err := toGeneratedSongChartRevision(rev, userNames{})

	require.NoError(t, err)
	assert.Equal(t, []string{"I-IV-I-V", "I-V-vi-IV"}, got.ProgressionKeys, "every section's tags, each once, in document order")

	untagged, err := toGeneratedSongChartRevision(domain.SongChartRevision{
		SongChartID: rev.SongChartID, RightsConfirmation: rev.RightsConfirmation, PublishedBy: admin,
	}, userNames{})
	require.NoError(t, err)
	assert.NotNil(t, untagged.ProgressionKeys, "empty, never null")
}

func TestToGeneratedProgression_Creator(t *testing.T) {
	creator := "11111111-1111-1111-1111-111111111111"
	progression := domain.Progression{ID: "22222222-2222-2222-2222-222222222222", Key: "major/I", Mode: "major", Bars: []string{"I"}, CreatedBy: &creator}

	t.Run("a known creator is named", func(t *testing.T) {
		got := toGeneratedProgression(progression, userNames{creator: "Ana Souza"})

		require.NotNil(t, got.CreatedBy)
		assert.Equal(t, "Ana Souza", got.CreatedBy.DisplayName)
	})

	t.Run("a creator who can't be found is left out", func(t *testing.T) {
		got := toGeneratedProgression(progression, userNames{})

		assert.Nil(t, got.CreatedBy)
	})
}
