package repo

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/progression"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntProgressionRepository keeps the progressions: the catalog's, which
// migrations install, and those admins create.
type EntProgressionRepository struct {
	client *ent.Client
}

func NewEntProgressionRepository(client *ent.Client) *EntProgressionRepository {
	return &EntProgressionRepository{client: client}
}

// List returns every progression, by key.
func (r *EntProgressionRepository) List(ctx context.Context) ([]domain.Progression, error) {
	rows, err := r.client.Progression.Query().Order(ent.Asc(progression.FieldKey)).All(ctx)
	if err != nil {
		return nil, err
	}
	return toDomainProgressions(rows), nil
}

// ListInDrills returns the progressions the drills use, by key.
func (r *EntProgressionRepository) ListInDrills(ctx context.Context) ([]domain.Progression, error) {
	rows, err := r.client.Progression.Query().
		Where(progression.InDrills(true)).
		Order(ent.Asc(progression.FieldKey)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return toDomainProgressions(rows), nil
}

func toDomainProgressions(rows []*ent.Progression) []domain.Progression {
	progressions := make([]domain.Progression, len(rows))
	for i, row := range rows {
		progressions[i] = domain.Progression{
			ID:         row.ID.String(),
			Key:        row.Key,
			Mode:       row.Mode,
			Bars:       row.Bars,
			NodeIDs:    row.NodeIds,
			Confusable: row.Confusable,
			Names:      domain.LocalizedText(row.Names),
			InDrills:   row.InDrills,
			Nicknames:  row.Nicknames,
			CreatedAt:  row.CreatedAt,
		}
		if row.CreatedBy != nil {
			createdBy := row.CreatedBy.String()
			progressions[i].CreatedBy = &createdBy
		}
	}
	return progressions
}
