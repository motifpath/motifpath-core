package repo

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/progression"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntProgressionRepository reads the progressions the progression catalog's
// migration installs.
type EntProgressionRepository struct {
	client *ent.Client
}

func NewEntProgressionRepository(client *ent.Client) *EntProgressionRepository {
	return &EntProgressionRepository{client: client}
}

func (r *EntProgressionRepository) List(ctx context.Context) ([]domain.Progression, error) {
	rows, err := r.client.Progression.Query().Order(ent.Asc(progression.FieldKey)).All(ctx)
	if err != nil {
		return nil, err
	}
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
		}
	}
	return progressions, nil
}
