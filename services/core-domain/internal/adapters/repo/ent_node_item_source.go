package repo

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagram"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntNodeItemSource lists practice items with their classification via
// ent/Postgres.
type EntNodeItemSource struct {
	client *ent.Client
}

func NewEntNodeItemSource(client *ent.Client) *EntNodeItemSource {
	return &EntNodeItemSource{client: client}
}

// ClassifiedItems returns the play-alongs, exercises and fretboard cells that
// suit instrumentID. A play-along is a basic diagram with playback — a
// teacher's custom diagrams are theirs alone, never offered for practice —
// and it suits every instrument it is linked to. A malformed id has no items.
func (s *EntNodeItemSource) ClassifiedItems(ctx context.Context, instrumentID string) ([]domain.ClassifiedItem, error) {
	if instrumentID == "" {
		return s.everyInstrumentItems(ctx)
	}
	parsed, err := uuid.Parse(instrumentID)
	if err != nil {
		return nil, nil
	}
	diagrams, err := s.client.Diagram.Query().
		Where(
			diagram.KindEQ(diagram.KindBasic),
			diagram.TempoBpmNotNil(),
			diagram.HasCompatibleInstrumentsWith(instrument.ID(parsed)),
		).
		WithSkills().
		WithConcepts().
		All(ctx)
	if err != nil {
		return nil, err
	}
	exercises, err := s.client.Exercise.Query().
		Where(exercise.Or(
			exercise.HasInstrumentsWith(instrument.ID(parsed)),
			exercise.Not(exercise.HasInstruments()),
		)).
		WithSkills().
		WithConcepts().
		All(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]domain.ClassifiedItem, 0, len(diagrams)+len(exercises))
	for _, row := range diagrams {
		items = append(items, domain.ClassifiedItem{
			ItemKey: domain.PlayAlongItemKey(row.ID.String()),
			NodeIDs: classifiedNodeIDs(row.Edges.Skills, row.Edges.Concepts),
		})
	}
	items = append(items, exerciseItems(exercises)...)
	cells, err := s.fretboardCells(ctx, parsed)
	if err != nil {
		return nil, err
	}
	return append(items, cells...), nil
}

// fretboardCells generates the catalog's fretboard cells that suit
// instrumentID: those of every range on a layout of the same geometry, so
// instruments sharing a fretboard share its cells and their item keys, for a
// skill that is for the instrument. A range that doesn't fit its layout is
// reported as domain.ErrInvalidDrillCatalog rather than half-generated.
func (s *EntNodeItemSource) fretboardCells(ctx context.Context, instrumentID uuid.UUID) ([]domain.ClassifiedItem, error) {
	row, err := s.client.Instrument.Get(ctx, instrumentID)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	played := toDomainInstrument(row)
	ranges, err := s.client.FretboardCellRange.Query().
		WithLayoutInstrument().
		WithSkill(func(q *ent.KnowledgeNodeQuery) { q.WithInstruments() }).
		All(ctx)
	if err != nil {
		return nil, err
	}
	var items []domain.ClassifiedItem
	for _, r := range ranges {
		layout := toDomainInstrument(r.Edges.LayoutInstrument)
		if !played.SameGeometry(layout) {
			continue
		}
		skill := toDomainKnowledgeNode(r.Edges.Skill)
		cellRange := domain.FretboardCellRange{
			ID: r.ID.String(), SkillID: skill.ID, LayoutInstrumentID: layout.ID,
			Strings: r.Strings, FromFret: r.FromFret, ToFret: r.ToFret,
		}
		if err := cellRange.CheckFits(layout, skill); err != nil {
			return nil, err
		}
		if len(skill.InstrumentIDs) > 0 && !slices.Contains(skill.InstrumentIDs, played.ID) {
			continue
		}
		items = append(items, cellRange.Items()...)
	}
	return items, nil
}

func classifiedNodeIDs(skills, concepts []*ent.KnowledgeNode) []string {
	ids := make([]string, 0, len(skills)+len(concepts))
	for _, rows := range [][]*ent.KnowledgeNode{skills, concepts} {
		for _, row := range rows {
			ids = append(ids, row.ID.String())
		}
	}
	return ids
}

// everyInstrumentItems lists the items for every instrument: exercises
// linked to no instrument. A play-along is always for some instrument.
func (s *EntNodeItemSource) everyInstrumentItems(ctx context.Context) ([]domain.ClassifiedItem, error) {
	exercises, err := s.client.Exercise.Query().
		Where(exercise.Not(exercise.HasInstruments())).
		WithSkills().
		WithConcepts().
		All(ctx)
	if err != nil {
		return nil, err
	}
	return exerciseItems(exercises), nil
}

// exerciseItems classifies exercises, read with their skills and concepts.
func exerciseItems(exercises []*ent.Exercise) []domain.ClassifiedItem {
	items := make([]domain.ClassifiedItem, 0, len(exercises))
	for _, row := range exercises {
		items = append(items, domain.ClassifiedItem{
			ItemKey: domain.ExerciseItemKey(row.ID.String()),
			NodeIDs: classifiedNodeIDs(row.Edges.Skills, row.Edges.Concepts),
		})
	}
	return items
}
