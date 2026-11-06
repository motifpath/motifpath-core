package main

import (
	"context"
	"fmt"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// seedCreatedProgressions creates the two kinds of progression an admin makes, so
// the admin list shows both next to the catalog's and a student meets the second in
// practice: one with a borrowed chord for song charts only, and one in the drills.
func seedCreatedProgressions(ctx context.Context, progressions *application.ProgressionCatalogService, admin domain.User, classifier *classificationSeeder) error {
	if _, err := progressions.Create(ctx, admin, application.ProgressionInput{
		Mode: string(domain.ProgressionModeMajor), Bars: []string{"I", "♭VII", "IV", "I"},
		Nicknames: map[string]string{"en": "Mixolydian rock", "pt_BR": "rock mixolídio"},
	}); err != nil {
		return fmt.Errorf("create the chart-only progression: %w", err)
	}
	concept, err := classifier.nodeID(ctx, domain.KnowledgeNodeKindConcept, "common-progressions")
	if err != nil {
		return err
	}
	if _, err := progressions.Create(ctx, admin, application.ProgressionInput{
		Mode: string(domain.ProgressionModeMajor), Bars: []string{"I", "♭VII", "IV", "V"},
		InDrills: true, ConceptIDs: []string{concept},
	}); err != nil {
		return fmt.Errorf("create the drill progression: %w", err)
	}
	return nil
}
