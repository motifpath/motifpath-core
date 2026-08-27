package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exercise"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntExerciseRepository persists Exercise records via ent/Postgres.
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
	challengeID, err := uuid.Parse(ex.ChallengeID)
	if err != nil {
		return err
	}
	_, err = r.client.Exercise.Create().
		SetID(id).
		SetChallengeID(challengeID).
		SetExerciseType(exercise.ExerciseType(ex.ExerciseType)).
		SetPrompt(ex.Prompt).
		SetCreatedAt(ex.CreatedAt).
		Save(ctx)
	return err
}

func (r *EntExerciseRepository) GetByID(ctx context.Context, id string) (domain.Exercise, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Exercise{}, domain.ErrNotFound
	}
	row, err := r.client.Exercise.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Exercise{}, domain.ErrNotFound
		}
		return domain.Exercise{}, err
	}
	return toDomainExercise(row), nil
}

func toDomainExercise(row *ent.Exercise) domain.Exercise {
	return domain.Exercise{
		ID:           row.ID.String(),
		ChallengeID:  row.ChallengeID.String(),
		ExerciseType: domain.ExerciseType(row.ExerciseType),
		Prompt:       row.Prompt,
		CreatedAt:    row.CreatedAt,
	}
}
