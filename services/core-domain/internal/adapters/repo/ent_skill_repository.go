package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/skill"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntSkillRepository persists Skill records via ent/Postgres. It's a thin
// instantiation of entTaxonomyRepository, which holds the logic shared with
// EntConceptRepository — see entTaxonomyRepository's doc comment.
type EntSkillRepository struct {
	*entTaxonomyRepository[domain.Skill]
}

func NewEntSkillRepository(client *ent.Client) *EntSkillRepository {
	return &EntSkillRepository{entTaxonomyRepository: &entTaxonomyRepository[domain.Skill]{
		create:        func(ctx context.Context, row entTaxonomyRow) error { return skillCreate(ctx, client, row) },
		getByID:       func(ctx context.Context, id uuid.UUID) (entTaxonomyRow, error) { return skillGetByID(ctx, client, id) },
		getByIDs:      func(ctx context.Context, ids []uuid.UUID) ([]entTaxonomyRow, error) { return skillGetByIDs(ctx, client, ids) },
		list:          func(ctx context.Context) ([]entTaxonomyRow, error) { return skillList(ctx, client) },
		existsSibling: func(ctx context.Context, parentID *uuid.UUID, name string) (bool, error) { return skillExistsSibling(ctx, client, parentID, name) },
		wrap:          skillRowToDomain,
		unwrap:        skillDomainToRow,
	}}
}

func skillCreate(ctx context.Context, client *ent.Client, row entTaxonomyRow) error {
	builder := client.Skill.Create().SetID(row.ID).SetName(row.Name)
	if row.ParentID != nil {
		builder = builder.SetParentID(*row.ParentID)
	}
	return builder.Exec(ctx)
}

func skillGetByID(ctx context.Context, client *ent.Client, id uuid.UUID) (entTaxonomyRow, error) {
	row, err := client.Skill.Query().Where(skill.ID(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return entTaxonomyRow{}, domain.ErrNotFound
		}
		return entTaxonomyRow{}, err
	}
	return entRowFromSkill(row), nil
}

func skillGetByIDs(ctx context.Context, client *ent.Client, ids []uuid.UUID) ([]entTaxonomyRow, error) {
	rows, err := client.Skill.Query().Where(skill.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	return entRowsFromSkills(rows), nil
}

func skillList(ctx context.Context, client *ent.Client) ([]entTaxonomyRow, error) {
	rows, err := client.Skill.Query().Order(skill.ByName()).All(ctx)
	if err != nil {
		return nil, err
	}
	return entRowsFromSkills(rows), nil
}

func skillExistsSibling(ctx context.Context, client *ent.Client, parentID *uuid.UUID, name string) (bool, error) {
	query := client.Skill.Query().Where(skill.NameEQ(name))
	if parentID != nil {
		query = query.Where(skill.ParentIDEQ(*parentID))
	} else {
		query = query.Where(skill.ParentIDIsNil())
	}
	return query.Exist(ctx)
}

func skillRowToDomain(row entTaxonomyRow) domain.Skill {
	s := domain.Skill{ID: row.ID.String(), Name: row.Name}
	if row.ParentID != nil {
		parentID := row.ParentID.String()
		s.ParentID = &parentID
	}
	return s
}

func skillDomainToRow(s domain.Skill) (entTaxonomyRow, error) {
	id, err := uuid.Parse(s.ID)
	if err != nil {
		return entTaxonomyRow{}, err
	}
	parentID, err := parseOptionalUUID(s.ParentID)
	if err != nil {
		return entTaxonomyRow{}, err
	}
	return entTaxonomyRow{ID: id, Name: s.Name, ParentID: parentID}, nil
}

func entRowFromSkill(row *ent.Skill) entTaxonomyRow {
	return entTaxonomyRow{ID: row.ID, Name: row.Name, ParentID: row.ParentID}
}

func entRowsFromSkills(rows []*ent.Skill) []entTaxonomyRow {
	result := make([]entTaxonomyRow, len(rows))
	for i, row := range rows {
		result[i] = entRowFromSkill(row)
	}
	return result
}

func toDomainSkill(row *ent.Skill) domain.Skill {
	s := domain.Skill{ID: row.ID.String(), Name: row.Name}
	if row.ParentID != nil {
		parentID := row.ParentID.String()
		s.ParentID = &parentID
	}
	return s
}
