package http

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func (h *Handler) SearchChords(ctx context.Context, request generated.SearchChordsRequestObject) (generated.SearchChordsResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.SearchChords401JSONResponse(unauthorizedError()), nil
	}

	search, err := h.chordCatalog.SearchChords(ctx, caller, request.Params.Symbol)
	if err != nil {
		kind, valErr := classify(err)
		switch kind {
		case errKindValidation:
			return generated.SearchChords400JSONResponse(validationErrorResponse(valErr)), nil
		case errKindForbidden:
			return generated.SearchChords403JSONResponse(forbiddenError("only teachers and admins may search the chord catalog")), nil
		case errKindNotFound, errKindOther:
			return nil, err
		}
	}
	return generated.SearchChords200JSONResponse(toGeneratedChordSearch(search)), nil
}

func (h *Handler) GetChord(ctx context.Context, request generated.GetChordRequestObject) (generated.GetChordResponseObject, error) {
	caller, ok := h.resolveCaller(ctx)
	if !ok {
		return generated.GetChord401JSONResponse(unauthorizedError()), nil
	}

	chord, err := h.chordCatalog.GetChord(ctx, caller, request.ChordDefinitionId.String())
	if err != nil {
		kind, _ := classify(err)
		switch kind {
		case errKindForbidden:
			return generated.GetChord403JSONResponse(forbiddenError("only teachers and admins may read the chord catalog")), nil
		case errKindNotFound:
			return generated.GetChord404JSONResponse(notFoundError("no chord exists with the given chord_definition_id")), nil
		case errKindValidation, errKindOther:
			return nil, err
		}
	}
	return generated.GetChord200JSONResponse(toGeneratedChord(chord)), nil
}

func toGeneratedChordSearch(s application.ChordSearch) generated.ChordSearchResult {
	result := generated.ChordSearchResult{
		WrittenSymbol: s.WrittenSymbol,
		Status:        generated.ChordSearchResultStatus(s.Reading.Status),
	}
	if s.Reading.Warning != "" {
		warning := generated.ChordSearchResultWarning(s.Reading.Warning)
		result.Warning = &warning
	}
	if p := s.Reading.Parsed; p != nil {
		result.Parsed = &generated.ParsedChordSymbol{
			Root: p.Root, RootPitchClass: p.RootPitchClass, Quality: generated.ChordQuality(p.Quality),
			Bass: p.Bass, BassPitchClass: p.BassPitchClass, CanonicalSymbol: p.CanonicalSymbol,
		}
	}
	if s.Chord != nil {
		chord := toGeneratedChord(*s.Chord)
		result.Chord = &chord
	}
	if s.ChordWithoutBass != nil {
		chord := toGeneratedChord(*s.ChordWithoutBass)
		result.ChordWithoutBass = &chord
	}
	return result
}

func toGeneratedChord(c domain.ChordDefinition) generated.ChordDefinition {
	voicings := make([]generated.ChordVoicing, len(c.Voicings))
	for i, v := range c.Voicings {
		voicings[i] = toGeneratedVoicing(v)
	}
	aliases := c.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	return generated.ChordDefinition{
		ChordDefinitionId: mustUUID(c.ID),
		CanonicalSymbol:   c.CanonicalSymbol,
		Root:              c.Root,
		RootPitchClass:    c.RootPitchClass,
		Quality:           generated.ChordQuality(c.Quality),
		Formula:           toGeneratedIntervals(c.Formula),
		Bass:              c.Bass,
		Aliases:           aliases,
		Voicings:          voicings,
	}
}

func toGeneratedVoicing(v domain.ChordVoicing) generated.ChordVoicing {
	out := generated.ChordVoicing{
		ChordVoicingId:    mustUUID(v.ID),
		ChordDefinitionId: mustUUID(v.ChordDefinitionID),
		DiagramId:         mustUUID(v.DiagramID),
		InstrumentId:      mustUUID(v.InstrumentID),
		TuningFingerprint: v.TuningFingerprint,
		MutedStrings:      v.MutedStrings,
		OmittedIntervals:  toGeneratedIntervals(v.OmittedIntervals),
		Difficulty:        generated.ChordVoicingDifficulty(v.Difficulty),
		IsMovable:         v.IsMovable,
		RecommendedRank:   v.RecommendedRank,
		CatalogStatus:     generated.ChordVoicingCatalogStatus(v.Status),
	}
	if out.MutedStrings == nil {
		out.MutedStrings = []int{}
	}
	out.FretWindow.LowestFret, out.FretWindow.HighestFret = v.LowestFret, v.HighestFret
	out.Fingering = make([]struct {
		Finger     generated.ChordVoicingFingeringFinger `json:"finger"`
		PositionId string                                `json:"position_id"`
	}, len(v.Fingering))
	for i, f := range v.Fingering {
		out.Fingering[i].Finger = generated.ChordVoicingFingeringFinger(f.Finger)
		out.Fingering[i].PositionId = f.PositionID
	}
	out.TechniqueTags = make([]generated.ChordVoicingTechniqueTags, len(v.TechniqueTags))
	for i, tag := range v.TechniqueTags {
		out.TechniqueTags[i] = generated.ChordVoicingTechniqueTags(tag)
	}
	if v.ShapeFamily != nil {
		family := generated.ChordVoicingShapeFamily(*v.ShapeFamily)
		out.ShapeFamily = &family
	}
	out.Provenance.Source = generated.ChordVoicingProvenanceSource("hand_authored")
	if v.TemplateKey != nil {
		out.Provenance.Source = generated.ChordVoicingProvenanceSource("template")
		out.Provenance.TemplateKey = v.TemplateKey
	}
	return out
}

func toGeneratedIntervals(intervals []string) []generated.ChordInterval {
	out := make([]generated.ChordInterval, len(intervals))
	for i, interval := range intervals {
		out[i] = generated.ChordInterval(interval)
	}
	return out
}
