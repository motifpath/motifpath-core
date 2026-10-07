//go:build integration

package repo

import (
	"context"
	"slices"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// playAlongDiagram is a diagram of kind on instrumentIDs (the first being
// its layout) classified under skills and concepts, with a one-note
// playback when playable.
func playAlongDiagram(kind domain.DiagramKind, playable bool, skills, concepts []domain.KnowledgeNode, instrumentIDs ...string) domain.Diagram {
	position := uuid.NewString()
	d := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: instrumentIDs[0], InstrumentIDs: instrumentIDs, Kind: kind, CreatedBy: uuid.NewString(),
		Names: domain.LocalizedText{"en": "Lick"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{{ID: position, Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)}},
		Skills:    skills, Concepts: concepts, CreatedAt: fixedAt,
	}
	if playable {
		playbackID := uuid.NewString()
		d.Playbacks = []domain.DiagramPlayback{{
			ID: playbackID, Names: domain.LocalizedText{"en": "Lick"}, TempoBPM: 80, TimeSignature: domain.DefaultTimeSignature,
			Steps: []domain.SequenceStep{{PositionIDs: []string{position}, Value: domain.NoteValue{Num: 1, Den: 4}, Strum: domain.StrumNone}},
		}}
		d.DefaultPlaybackID = &playbackID
	}
	return d
}

func TestEntNodeItemSource_ClassifiedItems(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments := NewEntInstrumentRepository(client)
	diagrams := NewEntDiagramRepository(client)
	exercises := NewEntExerciseRepository(client)
	source := NewEntNodeItemSource(client)

	guitar, electric, bass := frettedInstrument(), frettedInstrument(), frettedInstrument()
	for _, i := range []domain.Instrument{guitar, electric, bass} {
		require.NoError(t, instruments.Create(ctx, i))
	}
	skill := seedSkill(t, ctx, client, "low-strings-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "intervals-"+uuid.NewString())
	none := []domain.KnowledgeNode{}

	shared := playAlongDiagram(domain.DiagramKindBasic, true, []domain.KnowledgeNode{skill}, []domain.KnowledgeNode{concept}, guitar.ID, electric.ID)
	silent := playAlongDiagram(domain.DiagramKindBasic, false, []domain.KnowledgeNode{skill}, none, guitar.ID)
	custom := playAlongDiagram(domain.DiagramKindCustom, true, []domain.KnowledgeNode{skill}, none, guitar.ID)
	bassLine := playAlongDiagram(domain.DiagramKindBasic, true, []domain.KnowledgeNode{skill}, none, bass.ID)
	// A chord catalog voicing plays like a play-along but is not practised
	// on its own yet, so it never counts toward a node's level.
	voicing := playAlongDiagram(domain.DiagramKindBasic, true, []domain.KnowledgeNode{skill}, []domain.KnowledgeNode{concept}, guitar.ID, electric.ID)
	voicing.Purpose = domain.DiagramPurposeChordVoicing
	for _, d := range []domain.Diagram{shared, silent, custom, bassLine, voicing} {
		require.NoError(t, diagrams.Create(ctx, d))
	}
	anyDrill := instrumentExercise("Any-instrument drill", skill)
	anyDrill.Concepts = []domain.KnowledgeNode{concept}
	guitarDrill := instrumentExercise("Guitar drill", skill, guitar.ID)
	bassDrill := instrumentExercise("Bass drill", skill, bass.ID)
	for _, e := range []domain.Exercise{anyDrill, guitarDrill, bassDrill} {
		require.NoError(t, exercises.Create(ctx, e))
	}
	// mine keeps the items this test created: the migrations install
	// catalog items too.
	created := []string{
		domain.PlayAlongItemKey(shared.ID), domain.PlayAlongItemKey(silent.ID), domain.PlayAlongItemKey(custom.ID), domain.PlayAlongItemKey(bassLine.ID),
		domain.PlayAlongItemKey(voicing.ID),
		domain.ExerciseItemKey(anyDrill.ID), domain.ExerciseItemKey(guitarDrill.ID), domain.ExerciseItemKey(bassDrill.ID),
	}
	mine := func(items []domain.ClassifiedItem) []domain.ClassifiedItem {
		kept := []domain.ClassifiedItem{}
		for _, item := range items {
			if slices.Contains(created, item.ItemKey) {
				slices.Sort(item.NodeIDs)
				kept = append(kept, item)
			}
		}
		return kept
	}
	both := []string{skill.ID, concept.ID}
	slices.Sort(both)

	tests := []struct {
		name       string
		instrument string
		want       []domain.ClassifiedItem
	}{
		{
			name:       "playable basic diagrams for the instrument and exercises for it or for every instrument, with their nodes",
			instrument: guitar.ID,
			want: []domain.ClassifiedItem{
				{ItemKey: domain.PlayAlongItemKey(shared.ID), NodeIDs: both},
				{ItemKey: domain.ExerciseItemKey(anyDrill.ID), NodeIDs: both},
				{ItemKey: domain.ExerciseItemKey(guitarDrill.ID), NodeIDs: []string{skill.ID}},
			},
		},
		{
			name:       "a diagram counts for every instrument it is linked to",
			instrument: electric.ID,
			want: []domain.ClassifiedItem{
				{ItemKey: domain.PlayAlongItemKey(shared.ID), NodeIDs: both},
				{ItemKey: domain.ExerciseItemKey(anyDrill.ID), NodeIDs: both},
			},
		},
		{
			name:       "another layout keeps its own items",
			instrument: bass.ID,
			want: []domain.ClassifiedItem{
				{ItemKey: domain.PlayAlongItemKey(bassLine.ID), NodeIDs: []string{skill.ID}},
				{ItemKey: domain.ExerciseItemKey(anyDrill.ID), NodeIDs: both},
				{ItemKey: domain.ExerciseItemKey(bassDrill.ID), NodeIDs: []string{skill.ID}},
			},
		},
		{
			name:       "no instrument has only the items for every instrument",
			instrument: "",
			want: []domain.ClassifiedItem{
				{ItemKey: domain.ExerciseItemKey(anyDrill.ID), NodeIDs: both},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := source.ClassifiedItems(ctx, tt.instrument)

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, mine(got))
		})
	}

	t.Run("a malformed instrument id has no items", func(t *testing.T) {
		got, err := source.ClassifiedItems(ctx, "not-a-uuid")

		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

// TestEntNodeItemSource_FretboardCells reads the cells of the catalog the
// migrations install, so it migrates an empty database rather than creating
// the bare schema.
func TestEntNodeItemSource_FretboardCells(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	source := NewEntNodeItemSource(client)
	rootStrings := catalogID("knowledge-node/find-notes-root-strings")
	topStrings := catalogID("knowledge-node/find-notes-top-strings")

	// cells keeps the fretboard cells of items, by the skill they serve.
	cells := func(items []domain.ClassifiedItem) map[string][]string {
		bySkill := map[string][]string{}
		for _, item := range items {
			if strings.HasPrefix(item.ItemKey, string(domain.PracticeItemKindFretboardCell)+":") {
				require.Len(t, item.NodeIDs, 1, item.ItemKey)
				bySkill[item.NodeIDs[0]] = append(bySkill[item.NodeIDs[0]], item.ItemKey)
			}
		}
		return bySkill
	}

	t.Run("the guitar has the catalog's 72 cells, on its own layout, under their skills", func(t *testing.T) {
		got, err := source.ClassifiedItems(ctx, acousticGuitarID)

		require.NoError(t, err)
		bySkill := cells(got)
		assert.Len(t, bySkill[rootStrings], 24)
		assert.Len(t, bySkill[topStrings], 48)
		assert.Contains(t, bySkill[rootStrings], domain.FretboardCellItemKey(acousticGuitarID, 6, 0))
		assert.Contains(t, bySkill[topStrings], domain.FretboardCellItemKey(acousticGuitarID, 1, 11))
	})

	t.Run("an electric guitar shares the guitar layout's cells, with the same item keys", func(t *testing.T) {
		guitar, err := source.ClassifiedItems(ctx, acousticGuitarID)
		require.NoError(t, err)
		electric, err := source.ClassifiedItems(ctx, electricGuitarID)
		require.NoError(t, err)

		assert.Equal(t, cells(guitar), cells(electric))
	})

	t.Run("the bass has its own layout's 48 cells", func(t *testing.T) {
		got, err := source.ClassifiedItems(ctx, electricBassID)

		require.NoError(t, err)
		bySkill := cells(got)
		assert.Len(t, bySkill[rootStrings], 24)
		assert.Len(t, bySkill[topStrings], 24)
		assert.Contains(t, bySkill[rootStrings], domain.FretboardCellItemKey(electricBassID, 4, 0))
	})

	t.Run("an instrument of another geometry has no cells", func(t *testing.T) {
		seven := frettedInstrument()
		seven.StringCount = intPtr(7)
		seven.Tuning = []string{"B1", "E2", "A2", "D3", "G3", "B3", "E4"}
		require.NoError(t, NewEntInstrumentRepository(client).Create(ctx, seven))

		got, err := source.ClassifiedItems(ctx, seven.ID)

		require.NoError(t, err)
		assert.Empty(t, cells(got))
	})

	t.Run("no instrument has no cells", func(t *testing.T) {
		got, err := source.ClassifiedItems(ctx, "")

		require.NoError(t, err)
		assert.Empty(t, cells(got))
	})

	t.Run("a range on a string its layout doesn't have is skipped, and the layout's other cells stay", func(t *testing.T) {
		findOctaves := catalogID("knowledge-node/find-octaves")
		_, err := db.ExecContext(ctx, `INSERT INTO fretboard_cell_ranges (id, skill_id, layout_instrument_id, strings, from_fret, to_fret)
			VALUES ($1, $2, $3, '[5]', 0, 11)`, uuid.NewString(), findOctaves, electricBassID)
		require.NoError(t, err)

		got, err := source.ClassifiedItems(ctx, electricBassID)

		require.NoError(t, err)
		bySkill := cells(got)
		assert.Empty(t, bySkill[findOctaves])
		assert.Len(t, bySkill[rootStrings], 24)
		assert.Len(t, bySkill[topStrings], 24)
	})
}
