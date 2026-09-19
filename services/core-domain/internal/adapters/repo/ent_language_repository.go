package repo

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/language"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntLanguageRepository looks up the Language lookup rows via ent/Postgres.
type EntLanguageRepository struct {
	client *ent.Client
}

func NewEntLanguageRepository(client *ent.Client) *EntLanguageRepository {
	return &EntLanguageRepository{client: client}
}

func (r *EntLanguageRepository) GetByCode(ctx context.Context, code string) (domain.Language, error) {
	row, err := r.client.Language.Query().Where(language.Code(code)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Language{}, domain.ErrNotFound
		}
		return domain.Language{}, err
	}
	return domain.Language{Code: row.Code, Name: row.Name}, nil
}
