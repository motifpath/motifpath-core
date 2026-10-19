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
		ref := domain.DiagramRef{DiagramID: uuid.NewString(), Playback: &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionReversed, TempoBPM: &tempo, VoiceID: &voice, Loop: true}}

		got := toGeneratedDiagramRef(ref)

		require.NotNil(t, got.Playback)
		assert.Equal(t, generated.DiagramRefPlaybackDirection("reversed"), *got.Playback.Direction)
		assert.Equal(t, 72, *got.Playback.TempoBpm)
		assert.Equal(t, "acoustic-guitar", *got.Playback.VoiceId)
		assert.True(t, *got.Playback.Loop)
	})
}

func TestDiagramPlaybackMapping(t *testing.T) {
	p1, p2 := uuid.New(), uuid.New()
	strumID, arpID := uuid.New(), uuid.New()
	minor := domain.DiagramModeMinor
	d := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: uuid.NewString(), Names: domain.LocalizedText{"en": "Lick"}, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(),
		LabelDisplay: domain.LabelDisplayInterval, CreatedAt: time.Now(), Mode: &minor,
		Playbacks: []domain.DiagramPlayback{
			{
				ID: strumID.String(), Names: domain.LocalizedText{"en": "Strum"}, TempoBPM: 90, TimeSignature: domain.TimeSignature{Beats: 6, BeatValue: 8},
				Steps: []domain.SequenceStep{
					{PositionIDs: []string{p1.String(), p2.String()}, Value: domain.NoteValue{Num: 3, Den: 8}, Strum: domain.StrumDown},
					{PositionIDs: []string{}, Value: domain.NoteValue{Num: 1, Den: 12}, Strum: domain.StrumNone},
				},
			},
			{ID: arpID.String(), Names: domain.LocalizedText{"en": "Arpeggio"}, TempoBPM: 70, TimeSignature: domain.DefaultTimeSignature,
				Steps: []domain.SequenceStep{{PositionIDs: []string{p1.String()}, Value: domain.NoteValue{Num: 1, Den: 8}, Strum: domain.StrumNone}}},
		},
		DefaultPlaybackID: strPtr(arpID.String()),
	}

	t.Run("a diagram's key, playbacks and default are returned", func(t *testing.T) {
		got := toGeneratedDiagram(d, userNames{})

		assert.Equal(t, generated.DiagramMode("minor"), *got.Mode)
		require.Len(t, got.Playbacks, 2)
		strum := got.Playbacks[0]
		assert.Equal(t, strumID, strum.PlaybackId)
		assert.Equal(t, generated.LocalizedNames{"en": "Strum"}, strum.Names)
		assert.Equal(t, 90, strum.TempoBpm)
		assert.Equal(t, generated.TimeSignature{Beats: 6, BeatValue: 8}, strum.TimeSignature)
		require.Len(t, strum.Steps, 2)
		assert.Equal(t, []uuid.UUID{p1, p2}, strum.Steps[0].PositionIds)
		assert.Equal(t, generated.NoteValue{Num: 3, Den: 8}, strum.Steps[0].Value)
		assert.Equal(t, generated.SequenceStepStrum("down"), *strum.Steps[0].Strum)
		assert.Empty(t, strum.Steps[1].PositionIds)
		assert.Equal(t, arpID, got.Playbacks[1].PlaybackId)
		require.NotNil(t, got.DefaultPlaybackId)
		assert.Equal(t, arpID, *got.DefaultPlaybackId)
	})

	t.Run("a diagram that doesn't play returns an empty list and a null default", func(t *testing.T) {
		silent := d
		silent.Playbacks, silent.DefaultPlaybackID = nil, nil

		data, err := json.Marshal(toGeneratedDiagram(silent, userNames{}))

		require.NoError(t, err)
		assert.Contains(t, string(data), `"playbacks":[]`)
		assert.Contains(t, string(data), `"default_playback_id":null`)
	})

	t.Run("request playbacks become domain playbacks, leaving defaults to the domain", func(t *testing.T) {
		strum := generated.SequenceStepStrum("up")
		signature := generated.TimeSignature{Beats: 3, BeatValue: 4}
		playbacks := []generated.DiagramPlaybackInput{
			{
				PlaybackId: &strumID, Names: generated.LocalizedNames{"en": "Strum"}, TempoBpm: 90, TimeSignature: &signature,
				Steps: []generated.SequenceStep{
					{PositionIds: []uuid.UUID{p1, p2}, Value: generated.NoteValue{Num: 1, Den: 4}, Strum: &strum},
					{PositionIds: []uuid.UUID{}, Value: generated.NoteValue{Num: 1, Den: 4}},
				},
			},
			{Names: generated.LocalizedNames{"en": "Arpeggio"}, TempoBpm: 70, Steps: []generated.SequenceStep{{PositionIds: []uuid.UUID{p1}, Value: generated.NoteValue{Num: 1, Den: 8}}}},
		}

		got := toDomainPlaybacks(&playbacks)

		assert.Equal(t, []domain.DiagramPlayback{
			{
				ID: strumID.String(), Names: domain.LocalizedText{"en": "Strum"}, TempoBPM: 90, TimeSignature: domain.TimeSignature{Beats: 3, BeatValue: 4},
				Steps: []domain.SequenceStep{
					{PositionIDs: []string{p1.String(), p2.String()}, Value: domain.NoteValue{Num: 1, Den: 4}, Strum: domain.StrumUp},
					{PositionIDs: []string{}, Value: domain.NoteValue{Num: 1, Den: 4}},
				},
			},
			{Names: domain.LocalizedText{"en": "Arpeggio"}, TempoBPM: 70, Steps: []domain.SequenceStep{{PositionIDs: []string{p1.String()}, Value: domain.NoteValue{Num: 1, Den: 8}}}},
		}, got, "a missing id, time signature or strum is left for the service and domain to fill in")
		assert.Nil(t, toDomainPlaybacks(nil), "omitted playbacks stay nil")
		assert.Equal(t, []domain.DiagramPlayback{}, toDomainPlaybacks(&[]generated.DiagramPlaybackInput{}), "an empty list stays empty")
	})

	t.Run("an update's default playback keeps whether it was left out, cleared or set", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			body string
			want application.Nullable[string]
		}{
			{name: "left out", body: `{}`, want: application.Nullable[string]{}},
			{name: "cleared", body: `{"default_playback_id":null}`, want: application.Nullable[string]{Set: true}},
			{name: "set", body: `{"default_playback_id":"` + arpID.String() + `"}`, want: application.Nullable[string]{Set: true, Value: strPtr(arpID.String())}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				var body generated.UpdateDiagramRequest
				require.NoError(t, json.Unmarshal([]byte(tt.body), &body))

				assert.Equal(t, tt.want, toDiagramUpdate(&body).DefaultPlaybackID)
			})
		}
	})

	t.Run("a usage's chosen playback goes both ways", func(t *testing.T) {
		var ref generated.DiagramRef
		require.NoError(t, json.Unmarshal([]byte(`{"diagram_id":"`+uuid.NewString()+`","layers":{},"playback":{"playback_id":"`+arpID.String()+`"}}`), &ref))

		got := toDomainDiagramRef(ref)

		require.NotNil(t, got.Playback.PlaybackID)
		assert.Equal(t, arpID.String(), *got.Playback.PlaybackID)
		back := toGeneratedDiagramRef(got)
		require.NotNil(t, back.Playback.PlaybackId)
		assert.Equal(t, arpID, *back.Playback.PlaybackId)
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
