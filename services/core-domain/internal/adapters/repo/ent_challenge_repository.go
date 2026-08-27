package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
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

	builder := r.client.Challenge.Create().
		SetID(id).
		SetContentNodeID(contentNodeID).
		SetSubjectTag(challenge.SubjectTag).
		SetPassThreshold(challenge.PassThreshold).
		SetCreatedAt(challenge.CreatedAt)

	if challenge.RemediationTargetContentNodeID != nil {
		target, err := uuid.Parse(*challenge.RemediationTargetContentNodeID)
		if err != nil {
			return err
		}
		builder = builder.SetRemediationTargetContentNodeID(target)
	}

	_, err = builder.Save(ctx)
	return err
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

func toDomainChallenge(row *ent.Challenge) domain.Challenge {
	challenge := domain.Challenge{
		ID:            row.ID.String(),
		ContentNodeID: row.ContentNodeID.String(),
		SubjectTag:    row.SubjectTag,
		PassThreshold: row.PassThreshold,
		CreatedAt:     row.CreatedAt,
	}
	if row.RemediationTargetContentNodeID != nil {
		target := row.RemediationTargetContentNodeID.String()
		challenge.RemediationTargetContentNodeID = &target
	}
	return challenge
}
