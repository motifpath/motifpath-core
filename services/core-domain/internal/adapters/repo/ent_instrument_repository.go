package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntInstrumentRepository persists Instrument records via ent/Postgres.
type EntInstrumentRepository struct {
	client *ent.Client
}

func NewEntInstrumentRepository(client *ent.Client) *EntInstrumentRepository {
	return &EntInstrumentRepository{client: client}
}

func (r *EntInstrumentRepository) Create(ctx context.Context, i domain.Instrument) error {
	id, err := uuid.Parse(i.ID)
	if err != nil {
		return err
	}
	builder := r.client.Instrument.Create().
		SetID(id).
		SetNames(i.Names).
		SetFamily(instrument.Family(i.Family)).
		SetNillableStringCount(i.StringCount)
	if len(i.Tuning) > 0 {
		builder = builder.SetTuning(i.Tuning)
	}
	if i.KeyRange != nil {
		builder = builder.SetKeyRangeLowest(i.KeyRange.Lowest).SetKeyRangeHighest(i.KeyRange.Highest)
	}
	_, err = builder.Save(ctx)
	return err
}

func (r *EntInstrumentRepository) GetByID(ctx context.Context, id string) (domain.Instrument, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Instrument{}, domain.ErrNotFound
	}
	row, err := r.client.Instrument.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Instrument{}, domain.ErrNotFound
		}
		return domain.Instrument{}, err
	}
	return toDomainInstrument(row), nil
}

func (r *EntInstrumentRepository) List(ctx context.Context) ([]domain.Instrument, error) {
	rows, err := r.client.Instrument.Query().Order(ent.Asc(instrument.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Instrument, len(rows))
	for i, row := range rows {
		result[i] = toDomainInstrument(row)
	}
	return result, nil
}

func (r *EntInstrumentRepository) UpdateNames(ctx context.Context, id string, names domain.LocalizedText) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	err = r.client.Instrument.UpdateOneID(parsed).SetNames(names).Exec(ctx)
	if ent.IsNotFound(err) {
		return domain.ErrNotFound
	}
	return err
}

func toDomainInstrument(row *ent.Instrument) domain.Instrument {
	i := domain.Instrument{
		ID:          row.ID.String(),
		Names:       domain.LocalizedText(row.Names),
		Family:      domain.InstrumentFamily(row.Family),
		StringCount: row.StringCount,
		Tuning:      row.Tuning,
	}
	if row.KeyRangeLowest != nil && row.KeyRangeHighest != nil {
		i.KeyRange = &domain.KeyRange{Lowest: *row.KeyRangeLowest, Highest: *row.KeyRangeHighest}
	}
	return i
}
