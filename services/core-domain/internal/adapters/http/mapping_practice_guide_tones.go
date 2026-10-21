package http

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// The generated guide-tone bar and voicing types are anonymous; these aliases
// name them.
type (
	generatedGuideToneBar = struct {
		Chord   string `json:"chord"`
		Numeral string `json:"numeral"`
	}
	generatedGuideToneVoicing = struct {
		Chord     string             `json:"chord"`
		DiagramId openapi_types.UUID `json:"diagram_id"`
	}
)

// toGeneratedGuideTone is a guide tone as the session API sends it, its
// progression named in locale. Lists are empty, never null.
func toGeneratedGuideTone(g domain.PlannedGuideTone, locale string) *generated.PlannedGuideTone {
	out := &generated.PlannedGuideTone{
		LayoutInstrumentId: mustUUID(g.LayoutInstrumentID),
		MusicKey:           generated.MusicKey(g.MusicKey),
		Tone:               generated.PlannedGuideToneTone(g.Tone),
		TempoBpm:           g.TempoBPM,
		TargetTempoBpm:     g.TargetTempoBPM,
		Takes:              g.Takes,
		FretWindow:         generated.FretWindow{FromFret: generated.FretWindowFromFret(g.FretWindow.FromFret)},
		Bars:               make([]generatedGuideToneBar, len(g.Bars)),
		Voicings:           make([]generatedGuideToneVoicing, len(g.Voicings)),
	}
	out.Progression.Key = g.ProgressionKey
	out.Progression.Name = g.ProgressionNames.Resolve(locale)
	for i, b := range g.Bars {
		out.Bars[i] = generatedGuideToneBar{Chord: b.Chord, Numeral: b.Numeral}
	}
	for i, v := range g.Voicings {
		out.Voicings[i] = generatedGuideToneVoicing{Chord: v.Chord, DiagramId: mustUUID(v.DiagramID)}
	}
	return out
}
