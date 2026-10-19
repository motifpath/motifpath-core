package repo

import (
	"context"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/voice"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntVoiceRepository reads the platform's Voice records via ent/Postgres.
type EntVoiceRepository struct {
	client *ent.Client
}

func NewEntVoiceRepository(client *ent.Client) *EntVoiceRepository {
	return &EntVoiceRepository{client: client}
}

func (r *EntVoiceRepository) GetByID(ctx context.Context, id string) (domain.Voice, error) {
	row, err := r.client.Voice.Get(ctx, id)
	if ent.IsNotFound(err) {
		return domain.Voice{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Voice{}, err
	}
	return toDomainVoice(row), nil
}

func (r *EntVoiceRepository) List(ctx context.Context) ([]domain.Voice, error) {
	rows, err := r.client.Voice.Query().Order(ent.Asc(voice.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Voice, len(rows))
	for i, row := range rows {
		result[i] = toDomainVoice(row)
	}
	return result, nil
}

func toDomainVoice(row *ent.Voice) domain.Voice {
	return domain.Voice{
		ID:          row.ID,
		Names:       domain.LocalizedText(row.Names),
		Family:      domain.InstrumentFamily(row.Family),
		Pitches:     row.Pitches,
		Attribution: row.Attribution,
	}
}
