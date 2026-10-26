package http

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// The generated tour key, voicing and chord tone types are anonymous; these
// aliases name them.
type (
	generatedTourKey = struct {
		Chart      []string               `json:"chart"`
		ChordTones []generatedChordTones  `json:"chord_tones"`
		Clean      bool                   `json:"clean"`
		MusicKey   generated.MusicKey     `json:"music_key"`
		Voicings   []generatedTourVoicing `json:"voicings"`
	}
	generatedTourVoicing = struct {
		Chord     string             `json:"chord"`
		DiagramId openapi_types.UUID `json:"diagram_id"`
	}
	generatedChordTones = struct {
		Chord string              `json:"chord"`
		Fifth generated.FretPlace `json:"fifth"`
		Root  generated.FretPlace `json:"root"`
		Third generated.FretPlace `json:"third"`
	}
)

// toGeneratedProgressionKeys is a tour as the session API sends it, its
// progression named in locale. Lists are empty, never null.
func toGeneratedProgressionKeys(tour domain.PlannedProgressionKeys, locale string) *generated.PlannedProgressionKeys {
	out := &generated.PlannedProgressionKeys{
		LayoutInstrumentId: mustUUID(tour.LayoutInstrumentID),
		TimesThrough:       tour.TimesThrough,
		Takes:              tour.Takes,
		StartMusicKey:      generated.MusicKey(tour.StartMusicKey),
		TourTempoBpm:       tour.TourTempoBPM,
		StartNoteValue:     generated.NoteValue2(tour.StartNoteValue),
		TargetTempoBpm:     tour.TargetTempoBPM,
		MinTempoBpm:        tour.MinTempoBPM,
		Keys:               make([]generatedTourKey, len(tour.Keys)),
	}
	out.Progression.Key = tour.ProgressionKey
	out.Progression.Name = tour.ProgressionNames.Resolve(locale)
	for i, k := range tour.Keys {
		key := generatedTourKey{
			Chart: k.Chart, Clean: k.Clean, MusicKey: generated.MusicKey(k.MusicKey),
			Voicings:   make([]generatedTourVoicing, len(k.Voicings)),
			ChordTones: make([]generatedChordTones, len(k.ChordTones)),
		}
		for j, v := range k.Voicings {
			key.Voicings[j] = generatedTourVoicing{Chord: v.Chord, DiagramId: mustUUID(v.DiagramID)}
		}
		for j, c := range k.ChordTones {
			key.ChordTones[j] = generatedChordTones{
				Chord: c.Chord, Root: toGeneratedFretPlace(c.Root), Third: toGeneratedFretPlace(c.Third), Fifth: toGeneratedFretPlace(c.Fifth),
			}
		}
		out.Keys[i] = key
	}
	return out
}
