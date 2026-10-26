package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func amazingGraceSong(bass bool) SongItem {
	return SongItem{
		LayoutInstrumentID: "layout-1", Bass: bass, ChartID: "chart-1", Title: "Amazing Grace", Artist: "John Newton",
		Progressions: []SongProgression{
			{Progression: Progression{Key: "I-IV-I-V", NodeIDs: []string{"node-cp"}}, SectionIndex: 0},
			{Progression: Progression{Key: "I-IV-V-I", NodeIDs: []string{"node-cp", "node-cad"}}, SectionIndex: 2},
		},
	}
}

func TestSongItem(t *testing.T) {
	song := amazingGraceSong(false)

	assert.Equal(t, "song:layout-1:chart-1", song.ItemKey())
	assert.Equal(t, PracticeItemKindSong, song.Kind())
	assert.Equal(t, 130, song.Seconds(), "2 minutes to play it, then 10 seconds to rate it")
	assert.True(t, song.SuitsSession(true))
	assert.False(t, song.SuitsSession(false), "a song is played, never imagined")
	assert.Equal(t, []string{"node-cp", "node-cad"}, song.NodeIDs(), "its progressions' nodes, each once")

	rung, fixed := fixedRung(PracticeItemKindSong)
	assert.True(t, fixed)
	assert.Equal(t, PracticeRungUse, rung)
	most, capped := kindSessionCap(PracticeItemKindSong)
	assert.True(t, capped)
	assert.Equal(t, 1, most)
	assert.True(t, EndsSessionOnly(PracticeItemKindSong))
	assert.False(t, EndsSessionOnly(PracticeItemKindProgressionKeys))
}

func TestSongItem_Plan(t *testing.T) {
	t.Run("on guitar the chords are played, for the progression that practises the subject", func(t *testing.T) {
		song := amazingGraceSong(false).ForSubject("node-cad")

		var item PracticeSessionItem
		item.Ask(song.Plan("self_rating", DrillPlanning{}))

		require.NotNil(t, item.Song)
		assert.Equal(t, PracticeItemKindSong, item.Kind)
		assert.Equal(t, PlannedSong{
			SongChartID: "chart-1", LayoutInstrumentID: "layout-1", Title: "Amazing Grace", Artist: "John Newton",
			ProgressionKey: "I-IV-V-I", SectionIndex: 2, Plays: SongPlaysChords,
		}, *item.Song)
		assert.Equal(t, PracticeRungUse, ItemRung(item, nil))
	})

	t.Run("on bass the roots are played", func(t *testing.T) {
		var item PracticeSessionItem
		item.Ask(amazingGraceSong(true).ForSubject("node-cp").Plan("self_rating", DrillPlanning{}))

		assert.Equal(t, SongPlaysRoots, item.Song.Plays)
		assert.Equal(t, "I-IV-I-V", item.Song.ProgressionKey, "the first progression in tagged order that practises the subject")
		assert.Equal(t, 0, item.Song.SectionIndex, "the first section tagged with it")
	})

	t.Run("a subject none of its progressions practises takes the first", func(t *testing.T) {
		var item PracticeSessionItem
		item.Ask(amazingGraceSong(false).ForSubject("node-other").Plan("self_rating", DrillPlanning{}))

		assert.Equal(t, "I-IV-I-V", item.Song.ProgressionKey)
		assert.Equal(t, 0, item.Song.SectionIndex)
	})
}

func TestSongItem_Serves(t *testing.T) {
	song := amazingGraceSong(false)

	assert.True(t, song.Serves("node-cad"))
	assert.False(t, song.Serves("node-blues"))
}
