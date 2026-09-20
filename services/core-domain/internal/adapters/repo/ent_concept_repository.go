package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/concept"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntConceptRepository persists Concept records via ent/Postgres.
type EntConceptRepository struct {
	client *ent.Client
}

func NewEntConceptRepository(client *ent.Client) *EntConceptRepository {
	return &EntConceptRepository{client: client}
}

func (r *EntConceptRepository) Create(ctx context.Context, c domain.Concept) error {
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return err
	}
	builder := r.client.Concept.Create().
		SetID(id).
		SetName(c.Name)
	if c.ParentID != nil && *c.ParentID != "" {
		parentID, err := uuid.Parse(*c.ParentID)
		if err != nil {
			return err
		}
		builder = builder.SetParentID(parentID)
	}
	return builder.Exec(ctx)
}

func (r *EntConceptRepository) GetByID(ctx context.Context, id string) (domain.Concept, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Concept{}, domain.ErrNotFound
	}
	row, err := r.client.Concept.Query().Where(concept.ID(parsed)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Concept{}, domain.ErrNotFound
		}
		return domain.Concept{}, err
	}
	return toDomainConcept(row), nil
}

func (r *EntConceptRepository) GetByIDs(ctx context.Context, ids []string) (map[string]domain.Concept, error) {
	result := map[string]domain.Concept{}
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
	rows, err := r.client.Concept.Query().Where(concept.IDIn(parsed...)).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID.String()] = toDomainConcept(row)
	}
	return result, nil
}

func (r *EntConceptRepository) List(ctx context.Context) ([]domain.Concept, error) {
	rows, err := r.client.Concept.Query().Order(concept.ByName()).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Concept, len(rows))
	for i, row := range rows {
		result[i] = toDomainConcept(row)
	}
	return result, nil
}

func (r *EntConceptRepository) ExistsSibling(ctx context.Context, parentID *string, name string) (bool, error) {
	query := r.client.Concept.Query().Where(concept.NameEQ(name))
	if parentID != nil && *parentID != "" {
		parsed, err := uuid.Parse(*parentID)
		if err != nil {
			return false, err
		}
		query = query.Where(concept.ParentIDEQ(parsed))
	} else {
		query = query.Where(concept.ParentIDIsNil())
	}
	return query.Exist(ctx)
}

func toDomainConcept(row *ent.Concept) domain.Concept {
	c := domain.Concept{ID: row.ID.String(), Name: row.Name}
	if row.ParentID != nil {
		parentID := row.ParentID.String()
		c.ParentID = &parentID
	}
	return c
}
