package repo

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/drilltemplate"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntDrillTemplateRepository reads the drill_templates the drill catalog's
// migrations install.
type EntDrillTemplateRepository struct {
	client *ent.Client
}

func NewEntDrillTemplateRepository(client *ent.Client) *EntDrillTemplateRepository {
	return &EntDrillTemplateRepository{client: client}
}

func (r *EntDrillTemplateRepository) List(ctx context.Context) ([]domain.DrillTemplate, error) {
	rows, err := r.client.DrillTemplate.Query().Order(ent.Asc(drilltemplate.FieldKey)).All(ctx)
	if err != nil {
		return nil, err
	}
	templates := make([]domain.DrillTemplate, len(rows))
	for i, row := range rows {
		templates[i] = domain.DrillTemplate{
			ID:           row.ID.String(),
			Key:          row.Key,
			ItemKind:     domain.PracticeItemKind(row.ItemKind),
			ResponseType: row.ResponseType,
			Rung:         domain.PracticeRung(row.Rung),
			SecondWay:    row.SecondWay,
			Timed:        row.Timed,
			Names:        domain.LocalizedText(row.Names),
		}
	}
	return templates, nil
}
