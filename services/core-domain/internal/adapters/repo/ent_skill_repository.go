package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/skill"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntSkillRepository persists Skill records via ent/Postgres.
type EntSkillRepository struct {
	client *ent.Client
}

func NewEntSkillRepository(client *ent.Client) *EntSkillRepository {
	return &EntSkillRepository{client: client}
}

func (r *EntSkillRepository) Create(ctx context.Context, s domain.Skill) error {
	id, err := uuid.Parse(s.ID)
	if err != nil {
		return err
	}
	builder := r.client.Skill.Create().
		SetID(id).
		SetName(s.Name)
	if s.ParentID != nil && *s.ParentID != "" {
		parentID, err := uuid.Parse(*s.ParentID)
		if err != nil {
			return err
		}
		builder = builder.SetParentID(parentID)
	}
	return builder.Exec(ctx)
}

func (r *EntSkillRepository) GetByID(ctx context.Context, id string) (domain.Skill, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Skill{}, domain.ErrNotFound
	}
	row, err := r.client.Skill.Query().Where(skill.ID(parsed)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Skill{}, domain.ErrNotFound
		}
		return domain.Skill{}, err
	}
	return toDomainSkill(row), nil
}

func (r *EntSkillRepository) GetByIDs(ctx context.Context, ids []string) (map[string]domain.Skill, error) {
	result := map[string]domain.Skill{}
	if len(ids) == 0 {
		return result, nil
	}
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := uuid.Parse(id)
		if err != nil {
			continue // not a valid id, so it can never match — left absent from result
		}
		parsed = append(parsed, u)
	}
	rows, err := r.client.Skill.Query().Where(skill.IDIn(parsed...)).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID.String()] = toDomainSkill(row)
	}
	return result, nil
}

func (r *EntSkillRepository) List(ctx context.Context) ([]domain.Skill, error) {
	rows, err := r.client.Skill.Query().Order(skill.ByName()).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Skill, len(rows))
	for i, row := range rows {
		result[i] = toDomainSkill(row)
	}
	return result, nil
}

func (r *EntSkillRepository) ExistsSibling(ctx context.Context, parentID *string, name string) (bool, error) {
	query := r.client.Skill.Query().Where(skill.NameEQ(name))
	if parentID != nil && *parentID != "" {
		parsed, err := uuid.Parse(*parentID)
		if err != nil {
			return false, err
		}
		query = query.Where(skill.ParentIDEQ(parsed))
	} else {
		query = query.Where(skill.ParentIDIsNil())
	}
	return query.Exist(ctx)
}

func toDomainSkill(row *ent.Skill) domain.Skill {
	s := domain.Skill{ID: row.ID.String(), Name: row.Name}
	if row.ParentID != nil {
		parentID := row.ParentID.String()
		s.ParentID = &parentID
	}
	return s
}
