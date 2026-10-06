//go:build integration

package repo

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// playAlongDiagram is a diagram of kind on instrumentIDs (the first being
// its layout) classified under skills and concepts, with a one-note
// sequence when playable.
func playAlongDiagram(kind domain.DiagramKind, playable bool, skills, concepts []domain.KnowledgeNode, instrumentIDs ...string) domain.Diagram {
	position := uuid.NewString()
	d := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: instrumentIDs[0], InstrumentIDs: instrumentIDs, Kind: kind, CreatedBy: uuid.NewString(),
		Names: domain.LocalizedText{"en": "Lick"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{{ID: position, Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)}},
		Skills:    skills, Concepts: concepts, CreatedAt: fixedAt,
	}
	if playable {
		d.TempoBPM = intPtr(80)
		d.Sequence = []domain.SequenceStep{{PositionIDs: []string{position}, Value: domain.NoteValue{Num: 1, Den: 4}, Strum: domain.StrumNone}}
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
	for _, d := range []domain.Diagram{shared, silent, custom, bassLine} {
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
