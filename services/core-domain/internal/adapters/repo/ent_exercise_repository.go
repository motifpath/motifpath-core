package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exerciseoption"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntExerciseRepository persists Exercise, its ExerciseOption children, and
// its many-to-many links to Challenge, via ent/Postgres.
type EntExerciseRepository struct {
	client *ent.Client
}

func NewEntExerciseRepository(client *ent.Client) *EntExerciseRepository {
	return &EntExerciseRepository{client: client}
}

func (r *EntExerciseRepository) Create(ctx context.Context, ex domain.Exercise) error {
	id, err := uuid.Parse(ex.ID)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	builder := tx.Exercise.Create().
		SetID(id).
		SetTitle(ex.Title).
		SetPrompt(ex.Prompt).
		SetExerciseType(exercise.ExerciseType(ex.ExerciseType)).
		SetSkillTags(ex.SkillTags).
		SetNillableImageURL(ex.ImageURL).
		SetNillableAudioURL(ex.AudioURL).
		SetCreatedAt(ex.CreatedAt)
	if _, err := builder.Save(ctx); err != nil {
		return rollback(tx, err)
	}

	optionBuilders := make([]*ent.ExerciseOptionCreate, len(ex.Options))
	for i, opt := range ex.Options {
		optionID, err := uuid.Parse(opt.ID)
		if err != nil {
			return rollback(tx, err)
		}
		optBuilder := tx.ExerciseOption.Create().
			SetID(optionID).
			SetExerciseID(id).
			SetIsCorrect(opt.IsCorrect).
			SetNillableLabel(opt.Label).
			SetNillableImageURL(opt.ImageURL)
		if opt.Region != nil {
			shape := exerciseoption.RegionShape(opt.Region.Shape)
			optBuilder = optBuilder.
				SetRegionX(opt.Region.X).
				SetRegionY(opt.Region.Y).
				SetRegionWidth(opt.Region.Width).
				SetRegionHeight(opt.Region.Height).
				SetRegionShape(shape)
		}
		optionBuilders[i] = optBuilder
	}
	if len(optionBuilders) > 0 {
		if _, err := tx.ExerciseOption.CreateBulk(optionBuilders...).Save(ctx); err != nil {
			return rollback(tx, err)
		}
	}

	return tx.Commit()
}

func (r *EntExerciseRepository) GetByID(ctx context.Context, id string) (domain.Exercise, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Exercise{}, domain.ErrNotFound
	}
	row, err := r.client.Exercise.Query().
		Where(exercise.ID(parsed)).
		WithChallenges().
		WithOptions().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Exercise{}, domain.ErrNotFound
		}
		return domain.Exercise{}, err
	}
	return toDomainExercise(row), nil
}

func (r *EntExerciseRepository) LinkChallenge(ctx context.Context, exerciseID, challengeID string) error {
	exID, err := uuid.Parse(exerciseID)
	if err != nil {
		return err
	}
	chID, err := uuid.Parse(challengeID)
	if err != nil {
		return err
	}
	return r.client.Exercise.UpdateOneID(exID).AddChallengeIDs(chID).Exec(ctx)
}

func (r *EntExerciseRepository) UnlinkChallenge(ctx context.Context, exerciseID, challengeID string) error {
	exID, err := uuid.Parse(exerciseID)
	if err != nil {
		return err
	}
	chID, err := uuid.Parse(challengeID)
	if err != nil {
		return err
	}
	return r.client.Exercise.UpdateOneID(exID).RemoveChallengeIDs(chID).Exec(ctx)
}

func toDomainExercise(row *ent.Exercise) domain.Exercise {
	challengeIDs := make([]string, len(row.Edges.Challenges))
	for i, c := range row.Edges.Challenges {
		challengeIDs[i] = c.ID.String()
	}

	options := make([]domain.Option, len(row.Edges.Options))
	for i, opt := range row.Edges.Options {
		options[i] = toDomainOption(opt)
	}

	return domain.Exercise{
		ID:           row.ID.String(),
		Title:        row.Title,
		Prompt:       row.Prompt,
		ExerciseType: domain.ExerciseType(row.ExerciseType),
		SkillTags:    row.SkillTags,
		ImageURL:     row.ImageURL,
		AudioURL:     row.AudioURL,
		Options:      options,
		ChallengeIDs: challengeIDs,
		CreatedAt:    row.CreatedAt,
	}
}

func toDomainOption(row *ent.ExerciseOption) domain.Option {
	opt := domain.Option{
		ID:        row.ID.String(),
		IsCorrect: row.IsCorrect,
		Label:     row.Label,
		ImageURL:  row.ImageURL,
	}
	if row.RegionShape != nil {
		opt.Region = &domain.OptionRegion{
			X:      valueOrZero(row.RegionX),
			Y:      valueOrZero(row.RegionY),
			Width:  valueOrZero(row.RegionWidth),
			Height: valueOrZero(row.RegionHeight),
			Shape:  domain.OptionRegionShape(*row.RegionShape),
		}
	}
	return opt
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
