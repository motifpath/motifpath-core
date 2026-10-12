package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestSoundOf(t *testing.T) {
	chord := domain.Diagram{
		ID: "a-minor",
		Playbacks: []domain.DiagramPlayback{
			{ID: "strum", Names: domain.LocalizedText{"en": "Strum down"}},
			{ID: "arpeggio", Names: domain.LocalizedText{"en": "Arpeggio"}},
		},
	}

	t.Run("plays the playback with that name, never looping", func(t *testing.T) {
		sound, err := soundOf(chord, "Arpeggio", nil)
		require.NoError(t, err)
		assert.Equal(t, "a-minor", sound.DiagramID)
		require.NotNil(t, sound.Playback)
		assert.Equal(t, "arpeggio", *sound.Playback.PlaybackID)
		assert.False(t, sound.Playback.Loop)
		assert.Nil(t, sound.Playback.TempoBPM)
	})

	t.Run("sets the tempo when given", func(t *testing.T) {
		tempo := 72
		sound, err := soundOf(chord, "Strum down", &tempo)
		require.NoError(t, err)
		assert.Equal(t, 72, *sound.Playback.TempoBPM)
	})

	t.Run("fails for a playback the diagram doesn't have", func(t *testing.T) {
		_, err := soundOf(chord, "Fingerstyle", nil)
		assert.ErrorContains(t, err, "Fingerstyle")
	})
}
