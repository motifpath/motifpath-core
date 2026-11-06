package repo

import (
	"context"
	"slices"

	"github.com/google/uuid"

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

// GetByID returns the progression with id, or domain.ErrNotFound.
func (r *EntProgressionRepository) GetByID(ctx context.Context, id string) (domain.Progression, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Progression{}, domain.ErrNotFound
	}
	row, err := r.client.Progression.Get(ctx, parsed)
	if ent.IsNotFound(err) {
		return domain.Progression{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Progression{}, err
	}
	return toDomainProgression(row), nil
}

// FindByNumerals returns the progression with this mode and these bars, or
// domain.ErrNotFound. A table this small is compared here rather than in SQL.
func (r *EntProgressionRepository) FindByNumerals(ctx context.Context, mode string, bars []string) (domain.Progression, error) {
	rows, err := r.client.Progression.Query().Where(progression.Mode(mode)).All(ctx)
	if err != nil {
		return domain.Progression{}, err
	}
	for _, row := range rows {
		if slices.Equal(row.Bars, bars) {
			return toDomainProgression(row), nil
		}
	}
	return domain.Progression{}, domain.ErrNotFound
}

// Create stores a new progression, or returns domain.ErrConflict when its mode
// and bars or its key are taken.
func (r *EntProgressionRepository) Create(ctx context.Context, p domain.Progression) error {
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return err
	}
	create := r.client.Progression.Create().
		SetID(id).SetKey(p.Key).SetMode(p.Mode).SetBars(p.Bars).
		SetNodeIds(p.NodeIDs).SetConfusable(p.Confusable).SetNames(p.Names).
		SetInDrills(p.InDrills).SetNicknames(p.Nicknames).SetNillableCreatedAt(p.CreatedAt)
	if p.CreatedBy != nil {
		createdBy, err := uuid.Parse(*p.CreatedBy)
		if err != nil {
			return err
		}
		create.SetCreatedBy(createdBy)
	}
	if err := create.Exec(ctx); err != nil {
		if ent.IsConstraintError(err) {
			return domain.ErrConflict
		}
		return err
	}
	return nil
}

// SaveNames replaces a progression's nicknames and names.
func (r *EntProgressionRepository) SaveNames(ctx context.Context, p domain.Progression) error {
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	update := r.client.Progression.UpdateOneID(id).SetNames(p.Names)
	if p.Nicknames == nil {
		update.ClearNicknames()
	} else {
		update.SetNicknames(p.Nicknames)
	}
	if err := update.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

// Delete removes the progression with id.
func (r *EntProgressionRepository) Delete(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	if err := r.client.Progression.DeleteOneID(parsed).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func toDomainProgressions(rows []*ent.Progression) []domain.Progression {
	progressions := make([]domain.Progression, len(rows))
	for i, row := range rows {
		progressions[i] = toDomainProgression(row)
	}
	return progressions
}

func toDomainProgression(row *ent.Progression) domain.Progression {
	p := domain.Progression{
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
		p.CreatedBy = &createdBy
	}
	return p
}
