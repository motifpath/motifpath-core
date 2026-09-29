package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestDiagramRefPlaybackMapping(t *testing.T) {
	t.Run("a playback without a direction plays as authored", func(t *testing.T) {
		var ref generated.DiagramRef
		require.NoError(t, json.Unmarshal([]byte(`{"diagram_id":"`+uuid.NewString()+`","layers":{},"playback":{"tempo_bpm":60}}`), &ref))

		got := toDomainDiagramRef(ref)

		require.NotNil(t, got.Playback)
		assert.Equal(t, domain.DiagramPlaybackDirectionAsAuthored, got.Playback.Direction)
		assert.Equal(t, 60, *got.Playback.TempoBPM)
		assert.Nil(t, got.Playback.VoiceID)
		assert.False(t, got.Playback.Loop)
	})

	t.Run("a stack entry's playback without a direction plays as authored too", func(t *testing.T) {
		var stack generated.DiagramStackRef
		require.NoError(t, json.Unmarshal([]byte(`{"stack":[{"diagram_id":"`+uuid.NewString()+`","layers":{},"playback":{}}]}`), &stack))

		got := toDomainDiagramStackRef(stack)

		assert.Equal(t, domain.DiagramPlaybackDirectionAsAuthored, got.Stack[0].Playback.Direction)
	})

	t.Run("tempo, voice, direction and loop come back as stored", func(t *testing.T) {
		voice, tempo := "acoustic-guitar", 72
		ref := domain.DiagramRef{DiagramID: uuid.NewString(), Playback: &domain.DiagramPlayback{Direction: domain.DiagramPlaybackDirectionReversed, TempoBPM: &tempo, VoiceID: &voice, Loop: true}}

		got := toGeneratedDiagramRef(ref)

		require.NotNil(t, got.Playback)
		assert.Equal(t, generated.DiagramRefPlaybackDirection("reversed"), *got.Playback.Direction)
		assert.Equal(t, 72, *got.Playback.TempoBpm)
		assert.Equal(t, "acoustic-guitar", *got.Playback.VoiceId)
		assert.True(t, *got.Playback.Loop)
	})
}

func TestDiagramSequenceMapping(t *testing.T) {
	p1, p2 := uuid.New(), uuid.New()
	minor, tempo := domain.DiagramModeMinor, 90
	d := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: uuid.NewString(), Names: domain.LocalizedText{"en": "Lick"}, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(),
		LabelDisplay: domain.LabelDisplayInterval, CreatedAt: time.Now(),
		Mode: &minor, TempoBPM: &tempo, TimeSignature: domain.TimeSignature{Beats: 6, BeatValue: 8},
		Sequence: []domain.SequenceStep{
			{PositionIDs: []string{p1.String(), p2.String()}, Value: domain.NoteValue{Num: 3, Den: 8}, Strum: domain.StrumDown},
			{PositionIDs: []string{}, Value: domain.NoteValue{Num: 1, Den: 12}, Strum: domain.StrumNone},
		},
	}

	t.Run("a diagram's key, meter, tempo and steps are returned", func(t *testing.T) {
		got := toGeneratedDiagram(d, userNames{})

		assert.Equal(t, generated.DiagramMode("minor"), *got.Mode)
		assert.Equal(t, 90, *got.TempoBpm)
		assert.Equal(t, generated.TimeSignature{Beats: 6, BeatValue: 8}, got.TimeSignature)
		require.Len(t, got.Sequence, 2)
		assert.Equal(t, []uuid.UUID{p1, p2}, got.Sequence[0].PositionIds)
		assert.Equal(t, generated.NoteValue{Num: 3, Den: 8}, got.Sequence[0].Value)
		assert.Equal(t, generated.SequenceStepStrum("down"), *got.Sequence[0].Strum)
		assert.Empty(t, got.Sequence[1].PositionIds)
	})

	t.Run("a diagram that doesn't play returns an empty sequence, not null", func(t *testing.T) {
		silent := d
		silent.Sequence, silent.TempoBPM = nil, nil

		data, err := json.Marshal(toGeneratedDiagram(silent, userNames{}))

		require.NoError(t, err)
		assert.Contains(t, string(data), `"sequence":[]`)
		assert.Contains(t, string(data), `"tempo_bpm":null`)
	})

	t.Run("request steps become domain steps, a missing strum left for the domain to default", func(t *testing.T) {
		strum := generated.SequenceStepStrum("up")
		steps := []generated.SequenceStep{
			{PositionIds: []uuid.UUID{p1}, Value: generated.NoteValue{Num: 1, Den: 8}},
			{PositionIds: []uuid.UUID{p1, p2}, Value: generated.NoteValue{Num: 1, Den: 4}, Strum: &strum},
			{PositionIds: []uuid.UUID{}, Value: generated.NoteValue{Num: 1, Den: 4}},
		}

		got := toDomainSequence(&steps)

		assert.Equal(t, []domain.SequenceStep{
			{PositionIDs: []string{p1.String()}, Value: domain.NoteValue{Num: 1, Den: 8}},
			{PositionIDs: []string{p1.String(), p2.String()}, Value: domain.NoteValue{Num: 1, Den: 4}, Strum: domain.StrumUp},
			{PositionIDs: []string{}, Value: domain.NoteValue{Num: 1, Den: 4}},
		}, got)
		assert.Nil(t, toDomainSequence(nil), "an omitted sequence stays nil")
		assert.Equal(t, []domain.SequenceStep{}, toDomainSequence(&[]generated.SequenceStep{}), "an empty sequence stays empty")
	})
}

func TestVoiceMapping(t *testing.T) {
	voices := []application.PlayableVoice{{
		Voice:   domain.Voice{ID: "piano", Names: domain.LocalizedText{"pt_BR": "Piano", "en": "Piano"}, Family: domain.InstrumentFamilyKeyboard, Pitches: []int{21}, Attribution: "CC BY 3.0"},
		Samples: []application.VoiceSample{{Pitch: 21, URL: "https://media.example.com/audio/voices/piano/21.mp3"}},
	}}

	got := toGeneratedVoices(voices)

	assert.Equal(t, []generated.Voice{{
		VoiceId: "piano", Names: generated.LocalizedNames{"en": "Piano", "pt_BR": "Piano"}, Languages: []string{"en", "pt_BR"},
		Family: generated.VoiceFamily("keyboard"), Attribution: "CC BY 3.0",
		Samples: []generated.VoiceSample{{Pitch: 21, Url: "https://media.example.com/audio/voices/piano/21.mp3"}},
	}}, got)
}
