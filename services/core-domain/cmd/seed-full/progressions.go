package main

import (
	"context"
	"fmt"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// createdProgressionSpec is a progression the seed creates as an admin would.
type createdProgressionSpec struct {
	mode      string
	bars      []string
	nicknames map[string]string
	inDrills  bool
}

// createdProgressionSpecs are the progressions an admin makes, so the admin list
// shows both kinds next to the catalog's and a student meets the drill ones in
// practice: one with a borrowed chord for song charts only; in the drills, a
// major one with a borrowed chord, a minor one, and a three-bar one, whose wrong
// answers the drills make up.
func createdProgressionSpecs() []createdProgressionSpec {
	return []createdProgressionSpec{
		{
			mode: string(domain.ProgressionModeMajor), bars: []string{"I", "♭VII", "IV", "I"},
			nicknames: map[string]string{"en": "Mixolydian rock", "pt_BR": "rock mixolídio"},
		},
		{mode: string(domain.ProgressionModeMajor), bars: []string{"I", "♭VII", "IV", "V"}, inDrills: true},
		{
			mode: string(domain.ProgressionModeMinor), bars: []string{"i", "♭VII", "♭VI", "V"},
			nicknames: map[string]string{"en": "Andalusian", "pt_BR": "andaluza"}, inDrills: true,
		},
		{mode: string(domain.ProgressionModeMajor), bars: []string{"I", "IV", "V"}, inDrills: true},
	}
}

// seedCreatedProgressions creates createdProgressionSpecs, those in the drills
// practising common-progressions.
func seedCreatedProgressions(ctx context.Context, progressions *application.ProgressionCatalogService, admin domain.User, classifier *classificationSeeder) error {
	concept, err := classifier.nodeID(ctx, domain.KnowledgeNodeKindConcept, "common-progressions")
	if err != nil {
		return err
	}
	for _, s := range createdProgressionSpecs() {
		in := application.ProgressionInput{Mode: s.mode, Bars: s.bars, Nicknames: s.nicknames, InDrills: s.inDrills}
		if s.inDrills {
			in.ConceptIDs = []string{concept}
		}
		if _, err := progressions.Create(ctx, admin, in); err != nil {
			return fmt.Errorf("create the %s progression %v: %w", s.mode, s.bars, err)
		}
	}
	return nil
}
