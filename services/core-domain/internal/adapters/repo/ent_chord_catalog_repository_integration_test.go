//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/chordvoicing"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/schema"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntChordCatalogRepository(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments, diagrams, chords := NewEntInstrumentRepository(client), NewEntDiagramRepository(client), NewEntChordCatalogRepository(client)
	guitar := frettedInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))
	skill := seedSkill(t, ctx, client, "s-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "c-"+uuid.NewString())

	seedDiagram := func(name string) uuid.UUID {
		d := domain.Diagram{
			ID: uuid.NewString(), InstrumentID: guitar.ID, InstrumentIDs: []string{guitar.ID}, Kind: domain.DiagramKindBasic, Purpose: domain.DiagramPurposeChordVoicing, CreatedBy: uuid.NewString(), Names: domain.LocalizedText{"en": name}, LabelDisplay: domain.LabelDisplayInterval,
			Positions: []domain.Position{{ID: uuid.NewString(), Interval: "R", NoteName: "D", Shape: domain.PositionShapeDot, String: intPtr(4), Fret: intPtr(0)}},
			Skills:    []domain.KnowledgeNode{skill}, Concepts: []domain.KnowledgeNode{concept}, CreatedAt: fixedAt,
		}
		require.NoError(t, diagrams.Create(ctx, d))
		return uuid.MustParse(d.ID)
	}
	seedChord := func(symbol string, rootPC int, bass *string, bassPC *int) *ent.ChordDefinition {
		row, err := client.ChordDefinition.Create().SetID(uuid.New()).SetCanonicalSymbol(symbol).SetRoot(symbol[:1]).SetRootPitchClass(rootPC).
			SetQuality(string(domain.ChordQualityMajor)).SetFormula([]string{"R", "3", "5"}).SetOmittable([]string{}).
			SetNillableBass(bass).SetNillableBassPitchClass(bassPC).SetAliases([]string{symbol + "M"}).Save(ctx)
		require.NoError(t, err)
		return row
	}
	seedVoicing := func(chord *ent.ChordDefinition, rank int, status chordvoicing.Status) uuid.UUID {
		row, err := client.ChordVoicing.Create().SetID(uuid.New()).SetChordDefinitionID(chord.ID).SetDiagramID(seedDiagram(chord.CanonicalSymbol)).
			SetInstrumentID(uuid.MustParse(guitar.ID)).SetTuningFingerprint("E2-A2-D3-G3-B3-E4").SetLowestFret(0).SetHighestFret(3).
			SetFingering([]schema.VoicingFinger{{PositionID: "p1", Finger: "1"}}).SetMutedStrings([]int{6, 5}).SetOmittedIntervals([]string{}).
			SetDifficulty(chordvoicing.DifficultyBeginner).SetTechniqueTags([]string{"open"}).SetShapeFamily(chordvoicing.ShapeFamilyOpen).
			SetIsMovable(false).SetRecommendedRank(rank).SetStatus(status).Save(ctx)
		require.NoError(t, err)
		return row.ID
	}

	fSharp, six := "F#", 6
	d := seedChord("D", 2, nil, nil)
	dOverFSharp := seedChord("D/F#", 2, &fSharp, &six)
	second := seedVoicing(d, 2, chordvoicing.StatusActive)
	first := seedVoicing(d, 1, chordvoicing.StatusActive)
	withdrawn := seedVoicing(d, 3, chordvoicing.StatusWithdrawn)

	t.Run("a chord is read with its active voicings, best first", func(t *testing.T) {
		got, err := chords.GetChord(ctx, d.ID.String())

		require.NoError(t, err)
		assert.Equal(t, "D", got.CanonicalSymbol)
		assert.Equal(t, []string{"R", "3", "5"}, got.Formula)
		assert.Equal(t, []string{"DM"}, got.Aliases)
		require.Len(t, got.Voicings, 2)
		assert.Equal(t, first.String(), got.Voicings[0].ID)
		assert.Equal(t, second.String(), got.Voicings[1].ID)
		v := got.Voicings[0]
		assert.Equal(t, []domain.VoicingFinger{{PositionID: "p1", Finger: "1"}}, v.Fingering)
		assert.Equal(t, []int{6, 5}, v.MutedStrings)
		assert.Equal(t, "open", *v.ShapeFamily)
		assert.Equal(t, domain.ChordVoicingActive, v.Status)
		assert.Nil(t, v.TemplateKey)
	})

	t.Run("an unknown chord is not found", func(t *testing.T) {
		_, err := chords.GetChord(ctx, uuid.NewString())

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a chord is found by its pitch classes and quality", func(t *testing.T) {
		got, err := chords.FindChord(ctx, 2, domain.ChordQualityMajor, nil)

		require.NoError(t, err)
		assert.Equal(t, d.ID.String(), got.ID)
		assert.Len(t, got.Voicings, 2)
	})

	t.Run("a slash chord is found by its bass", func(t *testing.T) {
		got, err := chords.FindChord(ctx, 2, domain.ChordQualityMajor, &six)

		require.NoError(t, err)
		assert.Equal(t, dOverFSharp.ID.String(), got.ID)
		assert.Equal(t, "F#", *got.Bass)
		assert.Empty(t, got.Voicings)
	})

	t.Run("a chord the catalog doesn't have is not found", func(t *testing.T) {
		_, err := chords.FindChord(ctx, 2, domain.ChordQualityMinor, nil)

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("several chords are read at once with their active voicings, leaving out unknown ids", func(t *testing.T) {
		got, err := chords.GetChords(ctx, []string{d.ID.String(), dOverFSharp.ID.String(), uuid.NewString(), "not-a-uuid"})

		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Len(t, got[d.ID.String()].Voicings, 2)
		assert.Equal(t, first.String(), got[d.ID.String()].Voicings[0].ID)
		assert.Equal(t, "D/F#", got[dOverFSharp.ID.String()].CanonicalSymbol)
	})

	t.Run("voicings are read by id, withdrawn ones included", func(t *testing.T) {
		got, err := chords.GetVoicings(ctx, []string{first.String(), withdrawn.String(), uuid.NewString()})

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, domain.ChordVoicingWithdrawn, got[withdrawn.String()].Status)
		assert.Equal(t, d.ID.String(), got[first.String()].ChordDefinitionID)
	})

	t.Run("a voicing is found by its diagram, withdrawn or not; any other diagram has none", func(t *testing.T) {
		voicings, err := chords.GetVoicings(ctx, []string{first.String(), withdrawn.String()})
		require.NoError(t, err)

		for _, want := range []domain.ChordVoicing{voicings[first.String()], voicings[withdrawn.String()]} {
			got, err := chords.GetVoicingByDiagramID(ctx, want.DiagramID)

			require.NoError(t, err)
			assert.Equal(t, want, got)
		}
		for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
			_, err := chords.GetVoicingByDiagramID(ctx, id)
			require.ErrorIs(t, err, domain.ErrNotFound)
		}
	})
}
