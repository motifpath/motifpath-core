package repo

import (
	"context"
	"log/slog"
	"slices"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagram"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/diagramshape"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/knowledgenode"
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

// ClassifiedItems returns the play-alongs, exercises, fretboard cells and
// diagram shapes that suit instrumentID. A play-along is a general basic diagram with playback —
// a teacher's custom diagrams are theirs alone, never offered for practice,
// and a chord catalog voicing isn't practised on its own — and it suits every
// instrument it is linked to. A malformed id has no items.
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
			diagram.PurposeEQ(diagram.PurposeGeneral),
			diagram.DefaultPlaybackIDNotNil(),
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
	shapes, err := s.diagramShapes(ctx, parsed)
	if err != nil {
		return nil, err
	}
	harmony, err := s.harmonyItems(ctx)
	if err != nil {
		return nil, err
	}
	return slices.Concat(items, cells, shapes, harmony), nil
}

// harmonyItems generates the harmony drills' items, which suit every
// instrument: every numeral but the tonic in every major key, as a diatonic
// chord and as a chord function, and every progression as a chart, each
// under its family's nodes (a chart also under its progression's).
func (s *EntNodeItemSource) harmonyItems(ctx context.Context) ([]domain.ClassifiedItem, error) {
	var keys []string
	for _, nodeKeys := range domain.HarmonyNodeKeys {
		keys = append(keys, nodeKeys...)
	}
	rows, err := s.client.KnowledgeNode.Query().Where(knowledgenode.KeyIn(keys...)).All(ctx)
	if err != nil {
		return nil, err
	}
	idOf := make(map[string]string, len(rows))
	for _, row := range rows {
		idOf[row.Key] = row.ID.String()
	}
	nodesOf := func(kind domain.PracticeItemKind) []string {
		var ids []string
		for _, key := range domain.HarmonyNodeKeys[kind] {
			if id, ok := idOf[key]; ok {
				ids = append(ids, id)
			}
		}
		return ids
	}
	var items []domain.ClassifiedItem
	for _, key := range domain.MusicKeys {
		for _, numeral := range domain.HarmonyItemNumerals {
			items = append(items,
				domain.ClassifiedItem{ItemKey: domain.DiatonicChordItem{Key: key, Numeral: numeral}.ItemKey(), NodeIDs: nodesOf(domain.PracticeItemKindDiatonicChord)},
				domain.ClassifiedItem{ItemKey: domain.ChordFunctionItem{Key: key, Numeral: numeral}.ItemKey(), NodeIDs: nodesOf(domain.PracticeItemKindChordFunction)},
			)
		}
	}
	progressions, err := NewEntProgressionRepository(s.client).List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range progressions {
		nodes := slices.Clone(nodesOf(domain.PracticeItemKindChartProgression))
		for _, id := range p.NodeIDs {
			if !slices.Contains(nodes, id) {
				nodes = append(nodes, id)
			}
		}
		items = append(items, domain.ClassifiedItem{ItemKey: domain.ChartProgressionItemKey(p.Key), NodeIDs: nodes})
	}
	return items, nil
}

// FretboardLayouts lists the layout instruments of the drill catalog's
// fretboard cell ranges that instrumentID shares its geometry with, each
// once; none for an instrument that doesn't exist.
func (s *EntNodeItemSource) FretboardLayouts(ctx context.Context, instrumentID string) ([]domain.Instrument, error) {
	parsed, err := uuid.Parse(instrumentID)
	if err != nil {
		return nil, nil
	}
	row, err := s.client.Instrument.Get(ctx, parsed)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	played := toDomainInstrument(row)
	ranges, err := s.client.FretboardCellRange.Query().WithLayoutInstrument().All(ctx)
	if err != nil {
		return nil, err
	}
	var layouts []domain.Instrument
	for _, r := range ranges {
		layout := toDomainInstrument(r.Edges.LayoutInstrument)
		if played.SameGeometry(layout) && !slices.ContainsFunc(layouts, func(l domain.Instrument) bool { return l.ID == layout.ID }) {
			layouts = append(layouts, layout)
		}
	}
	return layouts, nil
}

// diagramShapes lists the catalog's drill shapes whose diagram is linked to
// instrumentID, each classified under its diagram's skills and concepts.
func (s *EntNodeItemSource) diagramShapes(ctx context.Context, instrumentID uuid.UUID) ([]domain.ClassifiedItem, error) {
	rows, err := s.client.DiagramShape.Query().
		Where(diagramshape.HasDiagramWith(diagram.HasCompatibleInstrumentsWith(instrument.ID(instrumentID)))).
		WithDiagram(func(q *ent.DiagramQuery) { q.WithSkills().WithConcepts() }).
		Order(ent.Asc(diagramshape.FieldDiagramID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.ClassifiedItem, len(rows))
	for i, row := range rows {
		d := row.Edges.Diagram
		items[i] = domain.ClassifiedItem{
			ItemKey: domain.DiagramShapeItemKey(d.ID.String()),
			NodeIDs: classifiedNodeIDs(d.Edges.Skills, d.Edges.Concepts),
		}
	}
	return items, nil
}

// fretboardCells generates the catalog's fretboard cells that suit
// instrumentID: those of every range on a layout of the same geometry, so
// instruments sharing a fretboard share its cells and their item keys, for a
// skill that is for the instrument. A range that doesn't fit its layout is
// logged and left out whole, never half-generated: one bad catalog row must
// not take every student's practice on that layout down with it.
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
			slog.ErrorContext(ctx, "skip a fretboard cell range that doesn't fit its layout", "range_id", cellRange.ID, "error", err)
			continue
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
// linked to no instrument and the harmony drills' items. A play-along is
// always for some instrument.
func (s *EntNodeItemSource) everyInstrumentItems(ctx context.Context) ([]domain.ClassifiedItem, error) {
	exercises, err := s.client.Exercise.Query().
		Where(exercise.Not(exercise.HasInstruments())).
		WithSkills().
		WithConcepts().
		All(ctx)
	if err != nil {
		return nil, err
	}
	harmony, err := s.harmonyItems(ctx)
	if err != nil {
		return nil, err
	}
	return append(exerciseItems(exercises), harmony...), nil
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
