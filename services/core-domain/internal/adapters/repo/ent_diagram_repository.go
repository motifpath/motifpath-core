package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/concept"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagram"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/position"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/skill"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntDiagramRepository persists Diagram records, with their Positions and
// skill/concept links, via ent/Postgres.
type EntDiagramRepository struct {
	client *ent.Client
}

func NewEntDiagramRepository(client *ent.Client) *EntDiagramRepository {
	return &EntDiagramRepository{client: client}
}

func (r *EntDiagramRepository) Create(ctx context.Context, d domain.Diagram) error {
	id, err := uuid.Parse(d.ID)
	if err != nil {
		return err
	}
	instrumentID, err := uuid.Parse(d.InstrumentID)
	if err != nil {
		return err
	}
	skillIDs, err := parseUUIDs(d.SkillIDs())
	if err != nil {
		return err
	}
	conceptIDs, err := parseUUIDs(d.ConceptIDs())
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Diagram.Create().
		SetID(id).
		SetInstrumentID(instrumentID).
		SetName(d.Name).
		SetNillableRootNote(d.RootNote).
		SetNillableColor(d.Color).
		SetLabelDisplay(diagram.LabelDisplay(d.LabelDisplay)).
		SetCreatedAt(d.CreatedAt).
		AddSkillIDs(skillIDs...).
		AddConceptIDs(conceptIDs...).
		Save(ctx); err != nil {
		return rollback(tx, err)
	}
	if err := createPositions(ctx, tx, id, d.Positions); err != nil {
		return rollback(tx, err)
	}
	return tx.Commit()
}

func (r *EntDiagramRepository) GetByID(ctx context.Context, id string) (domain.Diagram, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.Diagram{}, domain.ErrNotFound
	}
	row, err := withDiagramEdges(r.client.Diagram.Query()).Where(diagram.ID(parsed)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.Diagram{}, domain.ErrNotFound
		}
		return domain.Diagram{}, err
	}
	return toDomainDiagram(row), nil
}

func (r *EntDiagramRepository) List(ctx context.Context, instrumentID, skillID, conceptID string) ([]domain.Diagram, error) {
	query := withDiagramEdges(r.client.Diagram.Query()).Order(ent.Asc(diagram.FieldID))
	if instrumentID != "" {
		parsed, err := uuid.Parse(instrumentID)
		if err != nil {
			return nil, err
		}
		query = query.Where(diagram.InstrumentID(parsed))
	}
	if skillID != "" {
		parsed, err := uuid.Parse(skillID)
		if err != nil {
			return nil, err
		}
		query = query.Where(diagram.HasSkillsWith(skill.ID(parsed)))
	}
	if conceptID != "" {
		parsed, err := uuid.Parse(conceptID)
		if err != nil {
			return nil, err
		}
		query = query.Where(diagram.HasConceptsWith(concept.ID(parsed)))
	}

	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Diagram, len(rows))
	for i, row := range rows {
		result[i] = toDomainDiagram(row)
	}
	return result, nil
}

func (r *EntDiagramRepository) Update(ctx context.Context, d domain.Diagram) error {
	id, err := uuid.Parse(d.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	skillIDs, err := parseUUIDs(d.SkillIDs())
	if err != nil {
		return err
	}
	conceptIDs, err := parseUUIDs(d.ConceptIDs())
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	// Checked up front: replacing the join rows for an id with no diagram
	// trips their foreign key before UpdateOneID can report not-found.
	exists, err := tx.Diagram.Query().Where(diagram.ID(id)).Exist(ctx)
	if err != nil {
		return rollback(tx, err)
	}
	if !exists {
		return rollback(tx, domain.ErrNotFound)
	}
	if _, err := tx.Diagram.UpdateOneID(id).
		SetName(d.Name).
		SetNillableRootNote(d.RootNote).
		SetNillableColor(d.Color).
		SetLabelDisplay(diagram.LabelDisplay(d.LabelDisplay)).
		ClearSkills().
		AddSkillIDs(skillIDs...).
		ClearConcepts().
		AddConceptIDs(conceptIDs...).
		Save(ctx); err != nil {
		return rollback(tx, err)
	}
	if _, err := tx.Position.Delete().Where(position.DiagramID(id)).Exec(ctx); err != nil {
		return rollback(tx, err)
	}
	if err := createPositions(ctx, tx, id, d.Positions); err != nil {
		return rollback(tx, err)
	}
	return tx.Commit()
}

// withDiagramEdges eager-loads everything toDomainDiagram reads, with
// positions in the order the author listed them.
func withDiagramEdges(q *ent.DiagramQuery) *ent.DiagramQuery {
	return q.
		WithPositions(func(pq *ent.PositionQuery) { pq.Order(ent.Asc(position.FieldOrdinal)) }).
		WithSkills().
		WithConcepts()
}

func createPositions(ctx context.Context, tx *ent.Tx, diagramID uuid.UUID, positions []domain.Position) error {
	builders := make([]*ent.PositionCreate, len(positions))
	for i, p := range positions {
		id, err := uuid.Parse(p.ID)
		if err != nil {
			return err
		}
		builders[i] = tx.Position.Create().
			SetID(id).
			SetDiagramID(diagramID).
			SetOrdinal(i).
			SetInterval(p.Interval).
			SetNoteName(p.NoteName).
			SetShape(position.Shape(p.Shape)).
			SetNillableColor(p.Color).
			SetNillableSequenceIndex(p.SequenceIndex).
			SetNillableStringNumber(p.String).
			SetNillableFret(p.Fret).
			SetNillableKey(p.Key)
	}
	_, err := tx.Position.CreateBulk(builders...).Save(ctx)
	if ent.IsConstraintError(err) {
		// The diagram row exists and ordinals are generated 0..n-1, so the
		// only constraint a caller can trip here is the global uniqueness of
		// a client-supplied position id already owned by another diagram.
		return domain.NewValidationError("positions", "a position_id is already used by another diagram")
	}
	return err
}

func toDomainDiagram(row *ent.Diagram) domain.Diagram {
	positions := make([]domain.Position, len(row.Edges.Positions))
	for i, p := range row.Edges.Positions {
		positions[i] = domain.Position{
			ID:            p.ID.String(),
			Interval:      p.Interval,
			NoteName:      p.NoteName,
			Shape:         domain.PositionShape(p.Shape),
			Color:         p.Color,
			SequenceIndex: p.SequenceIndex,
			String:        p.StringNumber,
			Fret:          p.Fret,
			Key:           p.Key,
		}
	}
	return domain.Diagram{
		ID:           row.ID.String(),
		InstrumentID: row.InstrumentID.String(),
		Name:         row.Name,
		RootNote:     row.RootNote,
		LabelDisplay: domain.LabelDisplay(row.LabelDisplay),
		Color:        row.Color,
		Positions:    positions,
		Skills:       domainSkillsFromEdges(row.Edges.Skills),
		Concepts:     domainConceptsFromEdges(row.Edges.Concepts),
		CreatedAt:    row.CreatedAt,
	}
}
