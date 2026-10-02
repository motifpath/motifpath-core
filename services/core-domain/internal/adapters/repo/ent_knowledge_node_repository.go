package repo

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/challenge"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnode"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnodeconcept"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnodeskill"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagram"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagramconcept"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagramskill"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exerciseconcept"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exerciseskill"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/knowledgeedge"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/knowledgenode"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/predicate"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// EntKnowledgeNodeRepository persists knowledge nodes via ent/Postgres.
type EntKnowledgeNodeRepository struct {
	client *ent.Client
}

func NewEntKnowledgeNodeRepository(client *ent.Client) *EntKnowledgeNodeRepository {
	return &EntKnowledgeNodeRepository{client: client}
}

func (r *EntKnowledgeNodeRepository) Create(ctx context.Context, n domain.KnowledgeNode) error {
	id, err := uuid.Parse(n.ID)
	if err != nil {
		return err
	}
	parentID, err := parseOptionalUUID(n.ParentID)
	if err != nil {
		return err
	}
	instrumentIDs, err := parseUUIDs(n.InstrumentIDs)
	if err != nil {
		return err
	}
	builder := r.client.KnowledgeNode.Create().
		SetID(id).
		SetKind(knowledgenode.Kind(n.Kind)).
		SetKey(n.Key).
		SetNames(n.Names).
		SetNillableParentID(parentID).
		AddInstrumentIDs(instrumentIDs...)
	if n.Descriptions != nil {
		builder.SetDescriptions(n.Descriptions)
	}
	err = builder.Exec(ctx)
	if ent.IsConstraintError(err) && r.keyTaken(ctx, n.Key) {
		return domain.ErrAlreadyExists
	}
	return err
}

// keyTaken tells a duplicate key apart from the other constraints Create can
// trip (an unknown parent or instrument), which are bugs upstream rather
// than conflicts.
func (r *EntKnowledgeNodeRepository) keyTaken(ctx context.Context, key string) bool {
	taken, err := r.client.KnowledgeNode.Query().Where(knowledgenode.Key(key)).Exist(ctx)
	return err == nil && taken
}

func (r *EntKnowledgeNodeRepository) GetByID(ctx context.Context, id string) (domain.KnowledgeNode, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.KnowledgeNode{}, domain.ErrNotFound
	}
	row, err := r.query().Where(knowledgenode.ID(parsed)).Only(ctx)
	if ent.IsNotFound(err) {
		return domain.KnowledgeNode{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.KnowledgeNode{}, err
	}
	return toDomainKnowledgeNode(row), nil
}

func (r *EntKnowledgeNodeRepository) GetByIDs(ctx context.Context, ids []string) (map[string]domain.KnowledgeNode, error) {
	rows, err := r.query().Where(knowledgenode.IDIn(parseUUIDsSkippingInvalid(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]domain.KnowledgeNode, len(rows))
	for _, row := range rows {
		result[row.ID.String()] = toDomainKnowledgeNode(row)
	}
	return result, nil
}

func (r *EntKnowledgeNodeRepository) GetByKeys(ctx context.Context, keys []string) (map[string]domain.KnowledgeNode, error) {
	rows, err := r.query().Where(knowledgenode.KeyIn(keys...)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]domain.KnowledgeNode, len(rows))
	for _, row := range rows {
		result[row.Key] = toDomainKnowledgeNode(row)
	}
	return result, nil
}

func (r *EntKnowledgeNodeRepository) List(ctx context.Context, filter ports.KnowledgeNodeFilter) ([]domain.KnowledgeNode, error) {
	query := r.query().Order(knowledgenode.ByKey())
	if filter.Kind != nil {
		query.Where(knowledgenode.KindEQ(knowledgenode.Kind(*filter.Kind)))
	}
	if len(filter.InstrumentIDs) > 0 {
		instrumentIDs, err := parseUUIDs(filter.InstrumentIDs)
		if err != nil {
			return nil, err
		}
		query.Where(knowledgenode.Or(
			knowledgenode.Not(knowledgenode.HasInstruments()),
			knowledgenode.HasInstrumentsWith(instrument.IDIn(instrumentIDs...)),
		))
	}
	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.KnowledgeNode, len(rows))
	for i, row := range rows {
		result[i] = toDomainKnowledgeNode(row)
	}
	return result, nil
}

func (r *EntKnowledgeNodeRepository) Update(ctx context.Context, n domain.KnowledgeNode) error {
	id, err := uuid.Parse(n.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	parentID, err := parseOptionalUUID(n.ParentID)
	if err != nil {
		return err
	}
	instrumentIDs, err := parseUUIDs(n.InstrumentIDs)
	if err != nil {
		return err
	}
	update := r.client.KnowledgeNode.UpdateOneID(id).
		SetNames(n.Names).
		ClearInstruments().
		AddInstrumentIDs(instrumentIDs...)
	if n.Descriptions != nil {
		update.SetDescriptions(n.Descriptions)
	} else {
		update.ClearDescriptions()
	}
	if parentID != nil {
		update.SetParentID(*parentID)
	} else {
		update.ClearParentID()
	}
	err = update.Exec(ctx)
	if ent.IsNotFound(err) {
		return domain.ErrNotFound
	}
	return err
}

func (r *EntKnowledgeNodeRepository) Delete(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.KnowledgeNode.UpdateOneID(parsed).ClearInstruments().Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return rollback(tx, domain.ErrNotFound)
		}
		return rollback(tx, err)
	}
	if err := tx.KnowledgeNode.DeleteOneID(parsed).Exec(ctx); err != nil {
		return rollback(tx, err)
	}
	return tx.Commit()
}

func (r *EntKnowledgeNodeRepository) Children(ctx context.Context, id string) ([]domain.KnowledgeNode, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, nil
	}
	rows, err := r.query().Where(knowledgenode.ParentID(parsed)).Order(knowledgenode.ByKey()).All(ctx)
	if err != nil {
		return nil, err
	}
	return domainKnowledgeNodesFromEdges(rows), nil
}

func (r *EntKnowledgeNodeRepository) InSubtree(ctx context.Context, rootID, candidateID string) (bool, error) {
	root, err := uuid.Parse(rootID)
	if err != nil {
		return false, nil
	}
	candidate, err := uuid.Parse(candidateID)
	if err != nil {
		return false, nil
	}
	return r.client.KnowledgeNode.Query().
		Where(knowledgenode.ID(candidate), inSubtreeOf(root)).
		Exist(ctx)
}

// inSubtreeOf matches root and every node below it, at any depth.
func inSubtreeOf(root uuid.UUID) predicate.KnowledgeNode {
	return func(s *sql.Selector) {
		s.Where(sql.P(func(b *sql.Builder) {
			b.Ident(s.C(knowledgenode.FieldID)).
				WriteString(fmt.Sprintf(" IN (WITH RECURSIVE subtree(id) AS (SELECT id FROM %[1]s WHERE id = ", knowledgenode.Table)).
				Arg(root).
				WriteString(fmt.Sprintf(" UNION SELECT n.id FROM %[1]s n JOIN subtree ON n.parent_id = subtree.id) SELECT id FROM subtree)", knowledgenode.Table))
		}))
	}
}

// counter is any ent query Usage can count.
type counter interface {
	Count(ctx context.Context) (int, error)
}

func (r *EntKnowledgeNodeRepository) Usage(ctx context.Context, id string) (ports.KnowledgeNodeUsage, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return ports.KnowledgeNodeUsage{}, err
	}
	c := r.client
	var usage ports.KnowledgeNodeUsage
	// Each count adds to its field, so a field fed by both the skill and
	// the concept link table sums the two.
	counts := []struct {
		into  *int
		query counter
	}{
		{&usage.Children, c.KnowledgeNode.Query().Where(knowledgenode.ParentID(parsed))},
		{&usage.Edges, c.KnowledgeEdge.Query().Where(knowledgeedge.Or(knowledgeedge.FromID(parsed), knowledgeedge.ToID(parsed)))},
		{&usage.ContentNodes, c.ContentNodeSkill.Query().Where(contentnodeskill.SkillID(parsed))},
		{&usage.ContentNodes, c.ContentNodeConcept.Query().Where(contentnodeconcept.ConceptID(parsed))},
		{&usage.Exercises, c.ExerciseSkill.Query().Where(exerciseskill.SkillID(parsed))},
		{&usage.Exercises, c.ExerciseConcept.Query().Where(exerciseconcept.ConceptID(parsed))},
		{&usage.Diagrams, c.DiagramSkill.Query().Where(diagramskill.SkillID(parsed))},
		{&usage.Diagrams, c.DiagramConcept.Query().Where(diagramconcept.ConceptID(parsed))},
		{&usage.Challenges, c.Challenge.Query().Where(challenge.Or(challenge.SubjectSkillID(parsed), challenge.SubjectConceptID(parsed)))},
	}
	for _, count := range counts {
		n, err := count.query.Count(ctx)
		if err != nil {
			return ports.KnowledgeNodeUsage{}, err
		}
		*count.into += n
	}
	return usage, nil
}

func (r *EntKnowledgeNodeRepository) ClassifiedInstrumentSets(ctx context.Context, id string) ([][]string, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	classifiedBy := knowledgenode.ID(parsed)
	contentNodes, err := r.client.ContentNode.Query().
		Where(contentnode.Or(contentnode.HasSkillsWith(classifiedBy), contentnode.HasConceptsWith(classifiedBy))).
		WithInstruments().
		All(ctx)
	if err != nil {
		return nil, err
	}
	diagrams, err := r.client.Diagram.Query().
		Where(diagram.Or(diagram.HasSkillsWith(classifiedBy), diagram.HasConceptsWith(classifiedBy))).
		WithCompatibleInstruments().
		All(ctx)
	if err != nil {
		return nil, err
	}
	sets := make([][]string, 0, len(contentNodes)+len(diagrams))
	for _, row := range contentNodes {
		sets = append(sets, instrumentIDsOf(row.Edges.Instruments))
	}
	for _, row := range diagrams {
		sets = append(sets, instrumentIDsOf(row.Edges.CompatibleInstruments))
	}
	return sets, nil
}

func (r *EntKnowledgeNodeRepository) query() *ent.KnowledgeNodeQuery {
	return r.client.KnowledgeNode.Query().WithInstruments()
}

// toDomainKnowledgeNode converts a row loaded with its instruments.
func toDomainKnowledgeNode(row *ent.KnowledgeNode) domain.KnowledgeNode {
	n := domain.KnowledgeNode{
		ID:            row.ID.String(),
		Kind:          domain.KnowledgeNodeKind(row.Kind),
		Key:           row.Key,
		Names:         domain.LocalizedText(row.Names),
		InstrumentIDs: instrumentIDsOf(row.Edges.Instruments),
	}
	if row.Descriptions != nil {
		n.Descriptions = domain.LocalizedText(row.Descriptions)
	}
	if row.ParentID != nil {
		parentID := row.ParentID.String()
		n.ParentID = &parentID
	}
	return n
}

// domainKnowledgeNodesFromEdges converts an eager-loaded classification edge
// — skills or concepts, each loaded with its instruments — to its domain
// shape.
func domainKnowledgeNodesFromEdges(rows []*ent.KnowledgeNode) []domain.KnowledgeNode {
	result := make([]domain.KnowledgeNode, len(rows))
	for i, row := range rows {
		result[i] = toDomainKnowledgeNode(row)
	}
	return result
}
