package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/challenge"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntChallengeRepository persists Challenge records via ent/Postgres.
type EntChallengeRepository struct {
	client *ent.Client
}

func NewEntChallengeRepository(client *ent.Client) *EntChallengeRepository {
	return &EntChallengeRepository{client: client}
}

func (r *EntChallengeRepository) Create(ctx context.Context, challenge domain.Challenge) error {
	id, err := uuid.Parse(challenge.ID)
	if err != nil {
		return err
	}
	contentNodeID, err := uuid.Parse(challenge.ContentNodeID)
	if err != nil {
		return err
	}
	subjectSkillID, err := parseUUIDPtr(challenge.SubjectSkillID)
	if err != nil {
		return err
	}
	subjectConceptID, err := parseUUIDPtr(challenge.SubjectConceptID)
	if err != nil {
		return err
	}

	builder := r.client.Challenge.Create().
		SetID(id).
		SetContentNodeID(contentNodeID).
		SetNillableSubjectSkillID(subjectSkillID).
		SetNillableSubjectConceptID(subjectConceptID).
		SetPassThreshold(challenge.PassThreshold).
		SetNillableTimeThresholdMs(challenge.TimeThresholdMS).
		SetShuffleExercises(challenge.ShuffleExercises).
		SetShuffleOptions(challenge.ShuffleOptions).
		SetCreatedAt(challenge.CreatedAt)

	_, err = builder.Save(ctx)
	return err
}

func (r *EntChallengeRepository) Update(ctx context.Context, challenge domain.Challenge) error {
	id, err := uuid.Parse(challenge.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	subjectSkillID, err := parseUUIDPtr(challenge.SubjectSkillID)
	if err != nil {
		return err
	}
	subjectConceptID, err := parseUUIDPtr(challenge.SubjectConceptID)
	if err != nil {
		return err
	}

	builder := r.client.Challenge.UpdateOneID(id).
		SetPassThreshold(challenge.PassThreshold).
		SetShuffleExercises(challenge.ShuffleExercises).
		SetShuffleOptions(challenge.ShuffleOptions)

	if subjectSkillID != nil {
		builder = builder.SetSubjectSkillID(*subjectSkillID)
	} else {
		builder = builder.ClearSubjectSkillID()
	}
	if subjectConceptID != nil {
		builder = builder.SetSubjectConceptID(*subjectConceptID)
	} else {
		builder = builder.ClearSubjectConceptID()
	}

	if challenge.TimeThresholdMS != nil {
		builder = builder.SetTimeThresholdMs(*challenge.TimeThresholdMS)
	} else {
		builder = builder.ClearTimeThresholdMs()
	}

	_, err = builder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *EntChallengeRepository) GetByID(ctx context.Context, id string) (domain.Challenge, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Challenge{}, domain.ErrNotFound
	}
	row, err := r.client.Challenge.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Challenge{}, domain.ErrNotFound
		}
		return domain.Challenge{}, err
	}
	return toDomainChallenge(row), nil
}

func (r *EntChallengeRepository) ListByContentNodeID(ctx context.Context, contentNodeID string) ([]domain.Challenge, error) {
	parsed, err := uuid.Parse(contentNodeID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	rows, err := r.client.Challenge.Query().
		Where(challenge.ContentNodeID(parsed)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Challenge, len(rows))
	for i, row := range rows {
		result[i] = toDomainChallenge(row)
	}
	return result, nil
}

func toDomainChallenge(row *ent.Challenge) domain.Challenge {
	c := domain.Challenge{
		ID:               row.ID.String(),
		ContentNodeID:    row.ContentNodeID.String(),
		PassThreshold:    row.PassThreshold,
		TimeThresholdMS:  row.TimeThresholdMs,
		ShuffleExercises: row.ShuffleExercises,
		ShuffleOptions:   row.ShuffleOptions,
		CreatedAt:        row.CreatedAt,
	}
	if row.SubjectSkillID != nil {
		id := row.SubjectSkillID.String()
		c.SubjectSkillID = &id
	}
	if row.SubjectConceptID != nil {
		id := row.SubjectConceptID.String()
		c.SubjectConceptID = &id
	}
	return c
}

// parseUUIDPtr parses id if non-nil and non-empty, otherwise returns nil —
// the shared shape Challenge's optional subject_skill_id/subject_concept_id
// columns need on both read and write.
func parseUUIDPtr(id *string) (*uuid.UUID, error) {
	if id == nil || *id == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(*id)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
