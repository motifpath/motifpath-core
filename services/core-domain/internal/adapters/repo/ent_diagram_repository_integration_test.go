//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func intPtr(n int) *int { return &n }

func frettedInstrument() domain.Instrument {
	return domain.Instrument{
		ID: uuid.NewString(), Names: domain.LocalizedText{"en": "6-string guitar", "pt_BR": "Violão de 6 cordas"}, Family: domain.InstrumentFamilyFretted,
		StringCount: intPtr(6), Tuning: []string{"E", "A", "D", "G", "B", "E"},
	}
}

func keyboardInstrument() domain.Instrument {
	return domain.Instrument{
		ID: uuid.NewString(), Names: domain.LocalizedText{"en": "Piano", "pt_BR": "Piano"}, Family: domain.InstrumentFamilyKeyboard,
		KeyRange: &domain.KeyRange{Lowest: "A0", Highest: "C8"},
	}
}

func TestEntInstrumentRepository_CreateGetAndList(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntInstrumentRepository(client)

	guitar, piano := frettedInstrument(), keyboardInstrument()
	require.NoError(t, repo.Create(ctx, guitar))
	require.NoError(t, repo.Create(ctx, piano))

	gotGuitar, err := repo.GetByID(ctx, guitar.ID)
	require.NoError(t, err)
	assert.Equal(t, guitar, gotGuitar)

	gotPiano, err := repo.GetByID(ctx, piano.ID)
	require.NoError(t, err)
	assert.Equal(t, piano, gotPiano)

	_, err = repo.GetByID(ctx, uuid.NewString())
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = repo.GetByID(ctx, "not-a-uuid")
	require.ErrorIs(t, err, domain.ErrNotFound)

	all, err := repo.List(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.Instrument{guitar, piano}, all)
}

func TestEntInstrumentRepository_UpdateNames(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntInstrumentRepository(client)
	guitar := frettedInstrument()
	require.NoError(t, repo.Create(ctx, guitar))

	renamed := domain.LocalizedText{"en": "Guitar", "pt_BR": "Violão"}
	require.NoError(t, repo.UpdateNames(ctx, guitar.ID, renamed))

	got, err := repo.GetByID(ctx, guitar.ID)
	require.NoError(t, err)
	assert.Equal(t, renamed, got.Names)
	assert.Equal(t, guitar.Family, got.Family)
	assert.Equal(t, guitar.Tuning, got.Tuning)

	require.ErrorIs(t, repo.UpdateNames(ctx, uuid.NewString(), renamed), domain.ErrNotFound)
	require.ErrorIs(t, repo.UpdateNames(ctx, "not-a-uuid", renamed), domain.ErrNotFound)
}

func TestEntDiagramRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments, diagrams := NewEntInstrumentRepository(client), NewEntDiagramRepository(client)

	guitar := frettedInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))
	skill := seedSkill(t, ctx, client, "minor-pentatonic-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "scale-construction-"+uuid.NewString())

	// Deliberately not in id order: the repository must return positions in
	// the order the author listed them, not in primary-key order.
	d := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: guitar.ID, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(), Names: domain.LocalizedText{"en": "Minor Pentatonic — Position 1"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{
			{ID: "ffffffff-0000-4000-8000-000000000001", Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5), SequenceIndex: intPtr(0)},
			{ID: "00000000-0000-4000-8000-000000000002", Interval: "b3", NoteName: "C", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(8)},
			{ID: "88888888-0000-4000-8000-000000000003", Interval: "4", NoteName: "D", Shape: domain.PositionShapeDot, String: intPtr(5), Fret: intPtr(5), SequenceIndex: intPtr(1)},
		},
		Skills: []domain.Skill{skill}, Concepts: []domain.Concept{concept}, CreatedAt: fixedAt,
	}
	require.NoError(t, diagrams.Create(ctx, d))

	got, err := diagrams.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, d, got)

	_, err = diagrams.GetByID(ctx, uuid.NewString())
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestEntDiagramRepository_KeyboardPositionsRoundTrip(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments, diagrams := NewEntInstrumentRepository(client), NewEntDiagramRepository(client)

	piano := keyboardInstrument()
	require.NoError(t, instruments.Create(ctx, piano))
	skill := seedSkill(t, ctx, client, "s-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "c-"+uuid.NewString())

	a3, c4 := "A3", "C4"
	d := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: piano.ID, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(), Names: domain.LocalizedText{"en": "Minor Pentatonic — Piano"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{
			{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, Key: &a3},
			{ID: uuid.NewString(), Interval: "b3", NoteName: "C", Shape: domain.PositionShapeDot, Key: &c4},
		},
		Skills: []domain.Skill{skill}, Concepts: []domain.Concept{concept}, CreatedAt: fixedAt,
	}
	require.NoError(t, diagrams.Create(ctx, d))

	got, err := diagrams.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, d, got)
	for _, p := range got.Positions {
		assert.Nil(t, p.String)
		assert.Nil(t, p.Fret)
	}
}

func TestEntDiagramRepository_List(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments, diagrams := NewEntInstrumentRepository(client), NewEntDiagramRepository(client)

	guitar, piano := frettedInstrument(), keyboardInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))
	require.NoError(t, instruments.Create(ctx, piano))
	skillA := seedSkill(t, ctx, client, "a-"+uuid.NewString())
	skillB := seedSkill(t, ctx, client, "b-"+uuid.NewString())
	conceptA := seedConcept(t, ctx, client, "a-"+uuid.NewString())
	conceptB := seedConcept(t, ctx, client, "b-"+uuid.NewString())
	key := "A3"
	admin, me, them := uuid.NewString(), uuid.NewString(), uuid.NewString()

	// Names are deliberately out of creation order, so the name ordering is
	// observable.
	basic := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: guitar.ID, Kind: domain.DiagramKindBasic, CreatedBy: admin, Names: domain.LocalizedText{"en": "C Basic"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)}},
		Skills:    []domain.Skill{skillA}, Concepts: []domain.Concept{conceptA}, CreatedAt: fixedAt,
	}
	mine := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: piano.ID, Kind: domain.DiagramKindCustom, CreatedBy: me, Names: domain.LocalizedText{"en": "A Mine"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, Key: &key}},
		Skills:    []domain.Skill{skillB}, Concepts: []domain.Concept{conceptB}, CreatedAt: fixedAt,
	}
	theirs := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: guitar.ID, Kind: domain.DiagramKindCustom, CreatedBy: them, Names: domain.LocalizedText{"en": "B Theirs", "pt_BR": "0 Deles"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(5), Fret: intPtr(7)}},
		Skills:    []domain.Skill{skillA}, Concepts: []domain.Concept{conceptB}, CreatedAt: fixedAt,
	}
	for _, d := range []domain.Diagram{basic, mine, theirs} {
		require.NoError(t, diagrams.Create(ctx, d))
	}
	all := domain.PageRequest{Limit: domain.MaxPageLimit}

	tests := []struct {
		name   string
		filter domain.DiagramListFilter
		want   []domain.Diagram
	}{
		{name: "no filter, ordered by name", want: []domain.Diagram{mine, theirs, basic}},
		{name: "by instrument", filter: domain.DiagramListFilter{InstrumentID: guitar.ID}, want: []domain.Diagram{theirs, basic}},
		{name: "by skill", filter: domain.DiagramListFilter{SkillID: skillB.ID}, want: []domain.Diagram{mine}},
		{name: "by concept", filter: domain.DiagramListFilter{ConceptID: conceptA.ID}, want: []domain.Diagram{basic}},
		{name: "by kind", filter: domain.DiagramListFilter{Kind: domain.DiagramKindCustom}, want: []domain.Diagram{mine, theirs}},
		{name: "by creator", filter: domain.DiagramListFilter{CreatedBy: them}, want: []domain.Diagram{theirs}},
		{name: "visible to one user: basic plus their own custom", filter: domain.DiagramListFilter{VisibleTo: me}, want: []domain.Diagram{mine, basic}},
		{name: "visibility combines with kind", filter: domain.DiagramListFilter{VisibleTo: me, Kind: domain.DiagramKindCustom}, want: []domain.Diagram{mine}},
		{name: "filters combine with AND", filter: domain.DiagramListFilter{InstrumentID: guitar.ID, SkillID: skillB.ID}, want: []domain.Diagram{}},
		{name: "by language: only diagrams named in it", filter: domain.DiagramListFilter{Language: "pt_BR"}, want: []domain.Diagram{theirs}},
		{name: "ordered by the names a pt_BR reader sees", filter: domain.DiagramListFilter{Locale: "pt_BR"}, want: []domain.Diagram{theirs, mine, basic}},
		{name: "a locale that is not a language code orders as English, never reaching the SQL", filter: domain.DiagramListFilter{Locale: "en'; DROP TABLE diagrams; --"}, want: []domain.Diagram{mine, theirs, basic}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := diagrams.List(ctx, tt.filter, all)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Items)
			assert.Equal(t, len(tt.want), got.Total)
		})
	}

	t.Run("a page is a window of the name order, with the full total", func(t *testing.T) {
		got, err := diagrams.List(ctx, domain.DiagramListFilter{}, domain.PageRequest{Limit: 1, Offset: 1})

		require.NoError(t, err)
		assert.Equal(t, []domain.Diagram{theirs}, got.Items)
		assert.Equal(t, 3, got.Total)
	})

	t.Run("an offset past the end is an empty page, not an error", func(t *testing.T) {
		got, err := diagrams.List(ctx, domain.DiagramListFilter{}, domain.PageRequest{Limit: 10, Offset: 10})

		require.NoError(t, err)
		assert.Empty(t, got.Items)
		assert.Equal(t, 3, got.Total)
	})
}

func TestEntDiagramRepository_Update(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments, diagrams := NewEntInstrumentRepository(client), NewEntDiagramRepository(client)

	guitar := frettedInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))
	skillA := seedSkill(t, ctx, client, "a-"+uuid.NewString())
	skillB := seedSkill(t, ctx, client, "b-"+uuid.NewString())
	conceptA := seedConcept(t, ctx, client, "a-"+uuid.NewString())
	conceptB := seedConcept(t, ctx, client, "b-"+uuid.NewString())

	original := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: guitar.ID, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(), Names: domain.LocalizedText{"en": "Original"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{
			{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)},
			{ID: uuid.NewString(), Interval: "b3", NoteName: "C", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(8)},
		},
		Skills: []domain.Skill{skillA}, Concepts: []domain.Concept{conceptA}, CreatedAt: fixedAt,
	}
	require.NoError(t, diagrams.Create(ctx, original))

	t.Run("replaces name, positions and classification but not instrument or creation time", func(t *testing.T) {
		updated := original
		updated.Names = domain.LocalizedText{"en": "Renamed", "pt_BR": "Renomeado"}
		updated.Positions = []domain.Position{
			{ID: uuid.NewString(), Interval: "5", NoteName: "E", Shape: domain.PositionShapeDot, String: intPtr(5), Fret: intPtr(7)},
		}
		updated.Skills, updated.Concepts = []domain.Skill{skillB}, []domain.Concept{conceptB}

		require.NoError(t, diagrams.Update(ctx, updated))

		got, err := diagrams.GetByID(ctx, original.ID)
		require.NoError(t, err)
		assert.Equal(t, updated, got)
		assert.Equal(t, original.InstrumentID, got.InstrumentID)
		assert.Equal(t, original.CreatedAt, got.CreatedAt)
	})

	t.Run("an unknown diagram is not found and leaves no rows behind", func(t *testing.T) {
		ghost := original
		ghost.ID = uuid.NewString()

		err := diagrams.Update(ctx, ghost)

		require.ErrorIs(t, err, domain.ErrNotFound)
		_, getErr := diagrams.GetByID(ctx, ghost.ID)
		require.ErrorIs(t, getErr, domain.ErrNotFound)
		count, countErr := client.Position.Query().Count(ctx)
		require.NoError(t, countErr)
		assert.Equal(t, 1, count, "only the one replaced position of the real diagram may exist")
	})
}

func TestEntDiagramRepository_PositionIDOwnedByAnotherDiagramIsRejected(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments, diagrams := NewEntInstrumentRepository(client), NewEntDiagramRepository(client)

	guitar := frettedInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))
	skill := seedSkill(t, ctx, client, "s-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "c-"+uuid.NewString())

	newDiagram := func(positionID string) domain.Diagram {
		return domain.Diagram{
			ID: uuid.NewString(), InstrumentID: guitar.ID, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(), Names: domain.LocalizedText{"en": "D"}, LabelDisplay: domain.LabelDisplayInterval,
			Positions: []domain.Position{{ID: positionID, Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)}},
			Skills:    []domain.Skill{skill}, Concepts: []domain.Concept{concept}, CreatedAt: fixedAt,
		}
	}
	sharedID := uuid.NewString()
	owner := newDiagram(sharedID)
	require.NoError(t, diagrams.Create(ctx, owner))

	requirePositionsRejection := func(t *testing.T, err error) {
		t.Helper()
		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "positions", valErr.Fields[0].Field)
	}

	t.Run("creating a diagram with another diagram's position id is a validation error", func(t *testing.T) {
		thief := newDiagram(sharedID)

		err := diagrams.Create(ctx, thief)

		requirePositionsRejection(t, err)
		_, getErr := diagrams.GetByID(ctx, thief.ID)
		require.ErrorIs(t, getErr, domain.ErrNotFound, "the rejected diagram must not be partly persisted")
	})

	t.Run("updating a diagram with another diagram's position id is a validation error and changes nothing", func(t *testing.T) {
		other := newDiagram(uuid.NewString())
		require.NoError(t, diagrams.Create(ctx, other))
		update := other
		update.Names = domain.LocalizedText{"en": "Renamed"}
		update.Positions = []domain.Position{{ID: sharedID, Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)}}

		err := diagrams.Update(ctx, update)

		requirePositionsRejection(t, err)
		got, getErr := diagrams.GetByID(ctx, other.ID)
		require.NoError(t, getErr)
		assert.Equal(t, other, got)
	})

	t.Run("a diagram may resend its own position ids on update", func(t *testing.T) {
		update := owner
		update.Names = domain.LocalizedText{"en": "Renamed"}

		require.NoError(t, diagrams.Update(ctx, update))

		got, err := diagrams.GetByID(ctx, owner.ID)
		require.NoError(t, err)
		assert.Equal(t, update, got)
	})
}

func TestEntDiagramRepository_ColorsRoundTrip(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments, diagrams := NewEntInstrumentRepository(client), NewEntDiagramRepository(client)

	guitar := frettedInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))
	skill := seedSkill(t, ctx, client, "s-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "c-"+uuid.NewString())
	strPtr := func(s string) *string { return &s }

	d := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: guitar.ID, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(), Names: domain.LocalizedText{"en": "Colored"}, LabelDisplay: domain.LabelDisplayInterval,
		Color: strPtr("#3B82F6"),
		Positions: []domain.Position{
			{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5), Color: strPtr("#EF4444")},
			{ID: uuid.NewString(), Interval: "b3", NoteName: "C", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(8)},
		},
		Skills: []domain.Skill{skill}, Concepts: []domain.Concept{concept}, CreatedAt: fixedAt,
	}
	require.NoError(t, diagrams.Create(ctx, d))

	got, err := diagrams.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, d, got)

	t.Run("update replaces the general color and clears a per-position color", func(t *testing.T) {
		updated := d
		updated.Color = strPtr("#22C55E")
		updated.Positions = []domain.Position{
			{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)},
		}

		require.NoError(t, diagrams.Update(ctx, updated))

		again, err := diagrams.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.Equal(t, updated, again)
		assert.Nil(t, again.Positions[0].Color)
	})
}

func TestEntDiagramRepository_AnnotationsRoundTrip(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	instruments, diagrams := NewEntInstrumentRepository(client), NewEntDiagramRepository(client)

	guitar, piano := frettedInstrument(), keyboardInstrument()
	require.NoError(t, instruments.Create(ctx, guitar))
	require.NoError(t, instruments.Create(ctx, piano))
	skill := seedSkill(t, ctx, client, "s-"+uuid.NewString())
	concept := seedConcept(t, ctx, client, "c-"+uuid.NewString())
	strPtr := func(s string) *string { return &s }

	// Regions deliberately not in id order: they come back in drawing order.
	d := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: guitar.ID, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(), Names: domain.LocalizedText{"en": "Two Boxes", "pt_BR": "Duas caixas"}, LabelDisplay: domain.LabelDisplayInterval,
		Positions: []domain.Position{
			{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)},
			{ID: uuid.NewString(), Interval: "b3", NoteName: "C", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(8),
				CustomLabel: domain.LocalizedText{"en": "Av", "pt_BR": "Ev"}, Note: domain.LocalizedText{"en": "Avoid it", "pt_BR": "Evite"}},
		},
		Regions: []domain.Region{
			{ID: "ffffffff-0000-4000-8000-000000000001", FretStart: intPtr(5), FretEnd: intPtr(8), Description: domain.LocalizedText{"en": "Box 1", "pt_BR": "Caixa 1"}},
			{ID: "00000000-0000-4000-8000-000000000002", FretStart: intPtr(7), FretEnd: intPtr(10), StringStart: intPtr(1), StringEnd: intPtr(3), Description: domain.LocalizedText{"en": "Box 2", "pt_BR": "Caixa 2"}, Color: strPtr("#22C55E")},
		},
		Skills: []domain.Skill{skill}, Concepts: []domain.Concept{concept}, CreatedAt: fixedAt,
	}
	require.NoError(t, diagrams.Create(ctx, d))

	got, err := diagrams.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, d, got)

	t.Run("keyboard regions round-trip their key range", func(t *testing.T) {
		k := domain.Diagram{
			ID: uuid.NewString(), InstrumentID: piano.ID, Kind: domain.DiagramKindCustom, CreatedBy: uuid.NewString(), Names: domain.LocalizedText{"en": "Octave"}, LabelDisplay: domain.LabelDisplayInterval,
			Positions: []domain.Position{{ID: uuid.NewString(), Interval: "R", NoteName: "C", Shape: domain.PositionShapeDot, Key: strPtr("C4")}},
			Regions:   []domain.Region{{ID: uuid.NewString(), KeyStart: strPtr("C4"), KeyEnd: strPtr("B4"), Description: domain.LocalizedText{"en": "Octave 4"}}},
			Skills:    []domain.Skill{skill}, Concepts: []domain.Concept{concept}, CreatedAt: fixedAt,
		}
		require.NoError(t, diagrams.Create(ctx, k))

		again, err := diagrams.GetByID(ctx, k.ID)
		require.NoError(t, err)
		assert.Equal(t, k, again)
	})

	t.Run("update replaces the regions and clears a position's annotations", func(t *testing.T) {
		updated := d
		updated.Positions = []domain.Position{d.Positions[0], {ID: d.Positions[1].ID, Interval: "b3", NoteName: "C", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(8)}}
		updated.Regions = []domain.Region{{ID: uuid.NewString(), FretStart: intPtr(0), FretEnd: intPtr(3), Description: domain.LocalizedText{"en": "Open", "pt_BR": "Solta"}}}

		require.NoError(t, diagrams.Update(ctx, updated))

		again, err := diagrams.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.Equal(t, updated, again)
	})

	t.Run("update with no regions removes them all", func(t *testing.T) {
		updated := d
		updated.Regions = nil

		require.NoError(t, diagrams.Update(ctx, updated))

		again, err := diagrams.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.Nil(t, again.Regions)
	})

	t.Run("a region id owned by another diagram is a validation error", func(t *testing.T) {
		other := d
		other.ID = uuid.NewString()
		other.Positions = []domain.Position{{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)}}
		other.Regions = []domain.Region{{ID: "ffffffff-0000-4000-8000-000000000009", FretStart: intPtr(5), FretEnd: intPtr(8), Description: domain.LocalizedText{"en": "Box", "pt_BR": "Caixa"}}}
		require.NoError(t, diagrams.Create(ctx, other))
		clash := d
		clash.ID = uuid.NewString()
		clash.Positions = []domain.Position{{ID: uuid.NewString(), Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, String: intPtr(6), Fret: intPtr(5)}}
		clash.Regions = other.Regions

		err := diagrams.Create(ctx, clash)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "regions", valErr.Fields[0].Field)
	})
}
