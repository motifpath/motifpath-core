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
