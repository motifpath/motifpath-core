package repo

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/knowledgeedge"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/predicate"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// EntKnowledgeEdgeRepository persists the typed links between knowledge
// nodes via ent/Postgres.
type EntKnowledgeEdgeRepository struct {
	client *ent.Client
}

func NewEntKnowledgeEdgeRepository(client *ent.Client) *EntKnowledgeEdgeRepository {
	return &EntKnowledgeEdgeRepository{client: client}
}

func (r *EntKnowledgeEdgeRepository) Create(ctx context.Context, e domain.KnowledgeEdge) error {
	id, err := uuid.Parse(e.ID)
	if err != nil {
		return err
	}
	fromID, err := uuid.Parse(e.FromID)
	if err != nil {
		return err
	}
	toID, err := uuid.Parse(e.ToID)
	if err != nil {
		return err
	}
	builder := r.client.KnowledgeEdge.Create().
		SetID(id).
		SetFromID(fromID).
		SetToID(toID).
		SetType(knowledgeedge.Type(e.Type))
	if e.Level != nil {
		builder.SetLevel(knowledgeedge.Level(*e.Level))
	}
	err = builder.Exec(ctx)
	if ent.IsConstraintError(err) {
		taken, existsErr := r.client.KnowledgeEdge.Query().
			Where(knowledgeedge.FromID(fromID), knowledgeedge.ToID(toID), knowledgeedge.TypeEQ(knowledgeedge.Type(e.Type))).
			Exist(ctx)
		if existsErr == nil && taken {
			return domain.ErrAlreadyExists
		}
	}
	return err
}

func (r *EntKnowledgeEdgeRepository) GetByID(ctx context.Context, id string) (domain.KnowledgeEdge, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.KnowledgeEdge{}, domain.ErrNotFound
	}
	row, err := r.client.KnowledgeEdge.Get(ctx, parsed)
	if ent.IsNotFound(err) {
		return domain.KnowledgeEdge{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.KnowledgeEdge{}, err
	}
	return toDomainKnowledgeEdge(row), nil
}

func (r *EntKnowledgeEdgeRepository) List(ctx context.Context, filter ports.KnowledgeEdgeFilter) ([]domain.KnowledgeEdge, error) {
	query := r.client.KnowledgeEdge.Query().Order(knowledgeedge.ByFromID(), knowledgeedge.ByToID(), knowledgeedge.ByType())
	if filter.Type != nil {
		query.Where(knowledgeedge.TypeEQ(knowledgeedge.Type(*filter.Type)))
	}
	for _, f := range []struct {
		id    *string
		match func(uuid.UUID) predicate.KnowledgeEdge
	}{
		{filter.FromID, knowledgeedge.FromID},
		{filter.ToID, knowledgeedge.ToID},
	} {
		if f.id == nil {
			continue
		}
		parsed, err := uuid.Parse(*f.id)
		if err != nil {
			return nil, err
		}
		query.Where(f.match(parsed))
	}
	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.KnowledgeEdge, len(rows))
	for i, row := range rows {
		result[i] = toDomainKnowledgeEdge(row)
	}
	return result, nil
}

func (r *EntKnowledgeEdgeRepository) UpdateLevel(ctx context.Context, e domain.KnowledgeEdge) error {
	id, err := uuid.Parse(e.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	update := r.client.KnowledgeEdge.UpdateOneID(id)
	if e.Level != nil {
		update.SetLevel(knowledgeedge.Level(*e.Level))
	} else {
		update.ClearLevel()
	}
	err = update.Exec(ctx)
	if ent.IsNotFound(err) {
		return domain.ErrNotFound
	}
	return err
}

func (r *EntKnowledgeEdgeRepository) Delete(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	err = r.client.KnowledgeEdge.DeleteOneID(parsed).Exec(ctx)
	if ent.IsNotFound(err) {
		return domain.ErrNotFound
	}
	return err
}

func (r *EntKnowledgeEdgeRepository) RequiresPathExists(ctx context.Context, fromID, toID string) (bool, error) {
	from, err := uuid.Parse(fromID)
	if err != nil {
		return false, nil
	}
	to, err := uuid.Parse(toID)
	if err != nil {
		return false, nil
	}
	// Selects the requires edges arriving at to whose source is reachable
	// from from by requires edges (from itself included).
	return r.client.KnowledgeEdge.Query().
		Where(
			knowledgeedge.ToID(to),
			knowledgeedge.TypeEQ(knowledgeedge.TypeRequires),
			func(s *sql.Selector) {
				s.Where(sql.P(func(b *sql.Builder) {
					b.Ident(s.C(knowledgeedge.FieldFromID)).
						WriteString(" IN (WITH RECURSIVE reachable(id) AS (SELECT CAST(").
						Arg(from).
						WriteString(fmt.Sprintf(" AS uuid) UNION SELECT e.%[3]s FROM %[1]s e JOIN reachable ON e.%[2]s = reachable.id WHERE e.type = 'requires') SELECT id FROM reachable)", knowledgeedge.Table, knowledgeedge.FieldFromID, knowledgeedge.FieldToID))
				}))
			},
		).
		Exist(ctx)
}

func toDomainKnowledgeEdge(row *ent.KnowledgeEdge) domain.KnowledgeEdge {
	e := domain.KnowledgeEdge{
		ID:     row.ID.String(),
		FromID: row.FromID.String(),
		ToID:   row.ToID.String(),
		Type:   domain.KnowledgeEdgeType(row.Type),
	}
	if row.Level != nil {
		level := domain.MasteryLevel(*row.Level)
		e.Level = &level
	}
	return e
}
