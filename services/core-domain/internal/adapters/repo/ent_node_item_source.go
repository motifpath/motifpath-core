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
	chords, err := s.chordItems(ctx, parsed)
	if err != nil {
		return nil, err
	}
	return slices.Concat(items, cells, shapes, harmony, chords), nil
}

// The chord drills' skills: single chords and changes on guitar, a chord's
// root and fifth and root changes on bass.
const (
	chordVoicingSkill = "play-open-chords"
	chordChangeSkill  = "change-chords"
	rootFifthSkill    = "play-root-fifth-lines"
	rootChangeSkill   = "play-root-notes"
)

// chordItems generates the chord drills' items that suit instrumentID. On an
// instrument their voicing diagrams are linked to, each chord drill is a
// single chord under play-open-chords and each pair a change under
// change-chords. On an instrument the root and fifth skill is for, each chord
// is a root and fifth under it and each pair a root change under
// play-root-notes, keyed by the bass layout the instrument shares, so
// instruments of one fretboard share them. A skill missing from the knowledge
// map leaves its items out.
func (s *EntNodeItemSource) chordItems(ctx context.Context, instrumentID uuid.UUID) ([]domain.ClassifiedItem, error) {
	catalog, err := NewEntChordDrillRepository(s.client).Catalog(ctx)
	if err != nil || len(catalog.Drills) == 0 {
		return nil, err
	}
	nodes, err := s.client.KnowledgeNode.Query().
		Where(knowledgenode.KeyIn(chordVoicingSkill, chordChangeSkill, rootFifthSkill, rootChangeSkill)).
		WithInstruments().
		All(ctx)
	if err != nil {
		return nil, err
	}
	skills := map[string]domain.KnowledgeNode{}
	for _, row := range nodes {
		skills[row.Key] = toDomainKnowledgeNode(row)
	}
	chords := map[string]domain.ChordDrill{}
	for _, d := range catalog.Drills {
		chords[d.ID] = d
	}

	items, err := s.guitarChordItems(ctx, catalog, chords, skills, instrumentID)
	if err != nil {
		return nil, err
	}
	layout, ok, err := s.bassLayout(ctx, instrumentID, skills[rootFifthSkill])
	if err != nil || !ok {
		return items, err
	}
	return append(items, bassChordItems(catalog, chords, skills, layout)...), nil
}

// guitarChordItems are the single chords and changes whose voicings are
// linked to instrumentID.
func (s *EntNodeItemSource) guitarChordItems(ctx context.Context, catalog domain.ChordDrillCatalog, chords map[string]domain.ChordDrill, skills map[string]domain.KnowledgeNode, instrumentID uuid.UUID) ([]domain.ClassifiedItem, error) {
	playable, err := s.voicingsPlayableOn(ctx, catalog, instrumentID)
	if err != nil {
		return nil, err
	}
	var items []domain.ClassifiedItem
	if skill, ok := skills[chordVoicingSkill]; ok {
		for _, d := range catalog.Drills {
			if playable[d.DiagramID] {
				items = append(items, domain.ClassifiedItem{ItemKey: domain.ChordVoicingItem{Drill: d}.ItemKey(), NodeIDs: []string{skill.ID}})
			}
		}
	}
	if skill, ok := skills[chordChangeSkill]; ok {
		for _, p := range catalog.Changes {
			from, to := chords[p.FromChordDrillID], chords[p.ToChordDrillID]
			if playable[from.DiagramID] && playable[to.DiagramID] {
				items = append(items, domain.ClassifiedItem{ItemKey: domain.ChordChangeItem{From: from, To: to}.ItemKey(), NodeIDs: []string{skill.ID}})
			}
		}
	}
	return items, nil
}

// bassChordItems are every chord's root and fifth and every pair's root
// change on the bass layout.
func bassChordItems(catalog domain.ChordDrillCatalog, chords map[string]domain.ChordDrill, skills map[string]domain.KnowledgeNode, layout string) []domain.ClassifiedItem {
	var items []domain.ClassifiedItem
	if skill, ok := skills[rootFifthSkill]; ok {
		for _, d := range catalog.Drills {
			items = append(items, domain.ClassifiedItem{ItemKey: domain.RootFifthItemKey(layout, d.Chord), NodeIDs: []string{skill.ID}})
		}
	}
	if skill, ok := skills[rootChangeSkill]; ok {
		for _, p := range catalog.Changes {
			items = append(items, domain.ClassifiedItem{
				ItemKey: domain.RootChangeItemKey(layout, chords[p.FromChordDrillID].Chord, chords[p.ToChordDrillID].Chord),
				NodeIDs: []string{skill.ID},
			})
		}
	}
	return items
}

// voicingsPlayableOn lists the chord drills' voicing diagrams linked to
// instrumentID.
func (s *EntNodeItemSource) voicingsPlayableOn(ctx context.Context, catalog domain.ChordDrillCatalog, instrumentID uuid.UUID) (map[string]bool, error) {
	ids := make([]uuid.UUID, 0, len(catalog.Drills))
	for _, d := range catalog.Drills {
		if id, err := uuid.Parse(d.DiagramID); err == nil {
			ids = append(ids, id)
		}
	}
	rows, err := s.client.Diagram.Query().
		Where(diagram.IDIn(ids...), diagram.HasCompatibleInstrumentsWith(instrument.ID(instrumentID))).
		IDs(ctx)
	if err != nil {
		return nil, err
	}
	playable := make(map[string]bool, len(rows))
	for _, id := range rows {
		playable[id.String()] = true
	}
	return playable, nil
}

// bassLayout is the layout instrument the bass chord drills of instrumentID
// are keyed by: the fretboard layout it shares, when the root and fifth
// skill is for it. False when it is not, or it shares no layout.
func (s *EntNodeItemSource) bassLayout(ctx context.Context, instrumentID uuid.UUID, rootFifth domain.KnowledgeNode) (string, bool, error) {
	if rootFifth.ID == "" || !slices.Contains(rootFifth.InstrumentIDs, instrumentID.String()) {
		return "", false, nil
	}
	layouts, err := s.FretboardLayouts(ctx, instrumentID.String())
	if err != nil || len(layouts) == 0 {
		return "", false, err
	}
	return layouts[0].ID, true, nil
}

// harmonyItems generates the harmony and ear drills' items, which suit
// every instrument: every numeral but the tonic in every major key, as a
// diatonic chord and as a chord function; every numeral but the tonic as a
// heard chord; every progression as a chart, and every 4-bar one as a heard
// progression, each under its family's nodes (a progression's item also
// under the progression's).
func (s *EntNodeItemSource) harmonyItems(ctx context.Context) ([]domain.ClassifiedItem, error) {
	nodes, err := s.harmonyNodes(ctx)
	if err != nil {
		return nil, err
	}
	var items []domain.ClassifiedItem
	for _, key := range domain.MusicKeys {
		for _, numeral := range domain.HarmonyItemNumerals {
			items = append(items,
				domain.ClassifiedItem{ItemKey: domain.DiatonicChordItem{Key: key, Numeral: numeral}.ItemKey(), NodeIDs: nodes[domain.PracticeItemKindDiatonicChord]},
				domain.ClassifiedItem{ItemKey: domain.ChordFunctionItem{Key: key, Numeral: numeral}.ItemKey(), NodeIDs: nodes[domain.PracticeItemKindChordFunction]},
			)
		}
	}
	for _, numeral := range domain.HarmonyItemNumerals {
		items = append(items, domain.ClassifiedItem{ItemKey: domain.HeardChordItem{Numeral: numeral}.ItemKey(), NodeIDs: nodes[domain.PracticeItemKindHeardChord]})
	}
	progressions, err := NewEntProgressionRepository(s.client).List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range progressions {
		items = append(items, domain.ClassifiedItem{
			ItemKey: domain.ChartProgressionItemKey(p.Key),
			NodeIDs: withNodes(nodes[domain.PracticeItemKindChartProgression], p.NodeIDs),
		})
		if len(p.Bars) == domain.HeardProgressionBars {
			items = append(items, domain.ClassifiedItem{
				ItemKey: domain.HeardProgressionItemKey(p.Key),
				NodeIDs: withNodes(nodes[domain.PracticeItemKindHeardProgression], p.NodeIDs),
			})
		}
	}
	return items, nil
}

// withNodes is a copy of familyNodes, then each of extra it doesn't hold.
func withNodes(familyNodes, extra []string) []string {
	out := slices.Clone(familyNodes)
	for _, id := range extra {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// harmonyNodes looks up each harmony family's knowledge nodes by key; a
// key the knowledge map doesn't have is left out.
func (s *EntNodeItemSource) harmonyNodes(ctx context.Context) (map[domain.PracticeItemKind][]string, error) {
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
	nodes := map[domain.PracticeItemKind][]string{}
	for kind, nodeKeys := range domain.HarmonyNodeKeys {
		for _, key := range nodeKeys {
			if id, ok := idOf[key]; ok {
				nodes[kind] = append(nodes[kind], id)
			}
		}
	}
	return nodes, nil
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
