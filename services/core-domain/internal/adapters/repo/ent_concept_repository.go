package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/concept"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntConceptRepository persists Concept records via ent/Postgres. See
// EntSkillRepository's doc comment for the shared shape and rationale.
type EntConceptRepository struct {
	*entTaxonomyRepository[domain.Concept]
}

func NewEntConceptRepository(client *ent.Client) *EntConceptRepository {
	return &EntConceptRepository{entTaxonomyRepository: &entTaxonomyRepository[domain.Concept]{
		create:        func(ctx context.Context, row entTaxonomyRow) error { return conceptCreate(ctx, client, row) },
		getByID:       func(ctx context.Context, id uuid.UUID) (entTaxonomyRow, error) { return conceptGetByID(ctx, client, id) },
		getByIDs:      func(ctx context.Context, ids []uuid.UUID) ([]entTaxonomyRow, error) { return conceptGetByIDs(ctx, client, ids) },
		list:          func(ctx context.Context) ([]entTaxonomyRow, error) { return conceptList(ctx, client) },
		existsSibling: func(ctx context.Context, parentID *uuid.UUID, name string) (bool, error) { return conceptExistsSibling(ctx, client, parentID, name) },
		wrap:          conceptRowToDomain,
		unwrap:        conceptDomainToRow,
	}}
}

func conceptCreate(ctx context.Context, client *ent.Client, row entTaxonomyRow) error {
	builder := client.Concept.Create().SetID(row.ID).SetName(row.Name)
	if row.ParentID != nil {
		builder = builder.SetParentID(*row.ParentID)
	}
	return builder.Exec(ctx)
}

func conceptGetByID(ctx context.Context, client *ent.Client, id uuid.UUID) (entTaxonomyRow, error) {
	row, err := client.Concept.Query().Where(concept.ID(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return entTaxonomyRow{}, domain.ErrNotFound
		}
		return entTaxonomyRow{}, err
	}
	return entRowFromConcept(row), nil
}

func conceptGetByIDs(ctx context.Context, client *ent.Client, ids []uuid.UUID) ([]entTaxonomyRow, error) {
	rows, err := client.Concept.Query().Where(concept.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	return entRowsFromConcepts(rows), nil
}

func conceptList(ctx context.Context, client *ent.Client) ([]entTaxonomyRow, error) {
	rows, err := client.Concept.Query().Order(concept.ByName()).All(ctx)
	if err != nil {
		return nil, err
	}
	return entRowsFromConcepts(rows), nil
}

func conceptExistsSibling(ctx context.Context, client *ent.Client, parentID *uuid.UUID, name string) (bool, error) {
	query := client.Concept.Query().Where(concept.NameEQ(name))
	if parentID != nil {
		query = query.Where(concept.ParentIDEQ(*parentID))
	} else {
		query = query.Where(concept.ParentIDIsNil())
	}
	return query.Exist(ctx)
}

func conceptRowToDomain(row entTaxonomyRow) domain.Concept {
	c := domain.Concept{ID: row.ID.String(), Name: row.Name}
	if row.ParentID != nil {
		parentID := row.ParentID.String()
		c.ParentID = &parentID
	}
	return c
}

func conceptDomainToRow(c domain.Concept) (entTaxonomyRow, error) {
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return entTaxonomyRow{}, err
	}
	parentID, err := parseOptionalUUID(c.ParentID)
	if err != nil {
		return entTaxonomyRow{}, err
	}
	return entTaxonomyRow{ID: id, Name: c.Name, ParentID: parentID}, nil
}

func entRowFromConcept(row *ent.Concept) entTaxonomyRow {
	return entTaxonomyRow{ID: row.ID, Name: row.Name, ParentID: row.ParentID}
}

func entRowsFromConcepts(rows []*ent.Concept) []entTaxonomyRow {
	result := make([]entTaxonomyRow, len(rows))
	for i, row := range rows {
		result[i] = entRowFromConcept(row)
	}
	return result
}

func toDomainConcept(row *ent.Concept) domain.Concept {
	c := domain.Concept{ID: row.ID.String(), Name: row.Name}
	if row.ParentID != nil {
		parentID := row.ParentID.String()
		c.ParentID = &parentID
	}
	return c
}
