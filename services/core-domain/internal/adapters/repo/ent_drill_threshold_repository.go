package repo

import (
	"context"
	"slices"
	"strings"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntDrillThresholdRepository reads the drill_thresholds the drill catalog's
// migrations install.
type EntDrillThresholdRepository struct {
	client *ent.Client
}

func NewEntDrillThresholdRepository(client *ent.Client) *EntDrillThresholdRepository {
	return &EntDrillThresholdRepository{client: client}
}

func (r *EntDrillThresholdRepository) List(ctx context.Context) ([]domain.DrillThreshold, error) {
	rows, err := r.client.DrillThreshold.Query().WithTemplate().All(ctx)
	if err != nil {
		return nil, err
	}
	thresholds := make([]domain.DrillThreshold, len(rows))
	for i, row := range rows {
		thresholds[i] = domain.DrillThreshold{
			ID:            row.ID.String(),
			TemplateKey:   row.Edges.Template.Key,
			Version:       row.Version,
			EffectiveFrom: row.EffectiveFrom.UTC(),
			FluentNetMs:   row.FluentNetMs,
			Source:        domain.DrillThresholdSource(row.Source),
		}
	}
	slices.SortFunc(thresholds, func(a, b domain.DrillThreshold) int {
		if c := strings.Compare(a.TemplateKey, b.TemplateKey); c != 0 {
			return c
		}
		return a.Version - b.Version
	})
	return thresholds, nil
}
