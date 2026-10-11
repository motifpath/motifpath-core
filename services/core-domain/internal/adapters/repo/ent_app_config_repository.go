package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/appconfig"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntAppConfigRepository stores the app's settings via ent/Postgres, one row per key.
type EntAppConfigRepository struct {
	client *ent.Client
}

func NewEntAppConfigRepository(client *ent.Client) *EntAppConfigRepository {
	return &EntAppConfigRepository{client: client}
}

func (r *EntAppConfigRepository) Get(ctx context.Context, key string) (domain.AppConfigEntry, error) {
	row, err := r.client.AppConfig.Get(ctx, key)
	if ent.IsNotFound(err) {
		return domain.AppConfigEntry{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.AppConfigEntry{}, err
	}
	return toDomainAppConfigEntry(row), nil
}

// Put replaces the key's row, creating it the first time. When two writers create the same key at
// once, the one that loses the race updates the row the other created.
func (r *EntAppConfigRepository) Put(ctx context.Context, entry domain.AppConfigEntry) error {
	updatedBy, err := uuid.Parse(entry.UpdatedBy)
	if err != nil {
		return fmt.Errorf("app config %q updated_by: %w", entry.Key, err)
	}
	update := func() error {
		return r.client.AppConfig.UpdateOneID(entry.Key).
			SetValue(entry.Value).
			SetUpdatedAt(entry.UpdatedAt).
			SetUpdatedBy(updatedBy).
			Exec(ctx)
	}
	err = update()
	if !ent.IsNotFound(err) {
		return err
	}
	err = r.client.AppConfig.Create().
		SetID(entry.Key).
		SetValue(entry.Value).
		SetUpdatedAt(entry.UpdatedAt).
		SetUpdatedBy(updatedBy).
		Exec(ctx)
	if ent.IsConstraintError(err) {
		return update()
	}
	return err
}

func (r *EntAppConfigRepository) List(ctx context.Context) ([]domain.AppConfigEntry, error) {
	rows, err := r.client.AppConfig.Query().Order(ent.Asc(appconfig.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AppConfigEntry, len(rows))
	for i, row := range rows {
		result[i] = toDomainAppConfigEntry(row)
	}
	return result, nil
}

func toDomainAppConfigEntry(row *ent.AppConfig) domain.AppConfigEntry {
	return domain.AppConfigEntry{
		Key:       row.ID,
		Value:     row.Value,
		UpdatedAt: row.UpdatedAt,
		UpdatedBy: row.UpdatedBy.String(),
	}
}
