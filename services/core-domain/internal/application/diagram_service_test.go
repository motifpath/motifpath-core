package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

type diagramFixture struct {
	diagrams    *fakeDiagramRepository
	instruments *fakeInstrumentRepository
	svc         *application.DiagramService
}

func newDiagramFixture() diagramFixture {
	six := 6
	instruments := newFakeInstrumentRepository()
	instruments.put(domain.Instrument{ID: "guitar", Name: "Guitar", Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: []string{"E", "A", "D", "G", "B", "E"}})
	instruments.put(domain.Instrument{ID: "piano", Name: "Piano", Family: domain.InstrumentFamilyKeyboard, KeyRange: &domain.KeyRange{Lowest: "A0", Highest: "C8"}})
	diagrams := newFakeDiagramRepository()
	svc := application.NewDiagramService(diagrams, instruments, seededSkillRepository(), seededConceptRepository(), idSequence(), func() time.Time { return fixedCreatedAt })
	return diagramFixture{diagrams: diagrams, instruments: instruments, svc: svc}
}

func frettedPos(str, fret int) domain.Position {
	return domain.Position{Interval: "R", NoteName: "A", String: &str, Fret: &fret}
}

func TestDiagramService_CreateDiagram(t *testing.T) {
	ctx := context.Background()

	t.Run("a teacher creates a diagram, positions get server-assigned ids", func(t *testing.T) {
		f := newDiagramFixture()

		got, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "Minor Pentatonic", []domain.Position{frettedPos(6, 5), frettedPos(6, 8)}, []string{"skill-1"}, []string{"concept-1"}, nil, domain.LabelDisplayInterval)

		require.NoError(t, err)
		assert.NotEmpty(t, got.ID)
		assert.Equal(t, fixedCreatedAt, got.CreatedAt)
		require.Len(t, got.Positions, 2)
		assert.NotEmpty(t, got.Positions[0].ID)
		assert.NotEqual(t, got.Positions[0].ID, got.Positions[1].ID)
		_, err = f.diagrams.GetByID(ctx, got.ID)
		require.NoError(t, err)
	})

	t.Run("a client-supplied position id is kept", func(t *testing.T) {
		f := newDiagramFixture()
		pos := frettedPos(6, 5)
		pos.ID = "client-id"

		got, err := f.svc.CreateDiagram(ctx, adminCaller(), "guitar", "D", []domain.Position{pos}, []string{"skill-1"}, []string{"concept-1"}, nil, domain.LabelDisplayInterval)

		require.NoError(t, err)
		assert.Equal(t, "client-id", got.Positions[0].ID)
	})

	t.Run("a student cannot create a diagram", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := f.svc.CreateDiagram(ctx, studentCaller(), "guitar", "D", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, nil, domain.LabelDisplayInterval)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("root note and label display are carried onto the created diagram", func(t *testing.T) {
		f := newDiagramFixture()
		root := "A"

		got, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "D", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, &root, domain.LabelDisplayNote)

		require.NoError(t, err)
		require.NotNil(t, got.RootNote)
		assert.Equal(t, "A", *got.RootNote)
		assert.Equal(t, domain.LabelDisplayNote, got.LabelDisplay)
	})

	tests := []struct {
		name       string
		instrument string
		positions  []domain.Position
		skillIDs   []string
		conceptIDs []string
		wantField  string
	}{
		{name: "instrument does not exist", instrument: "nope", positions: []domain.Position{frettedPos(6, 5)}, skillIDs: []string{"skill-1"}, conceptIDs: []string{"concept-1"}, wantField: "instrument_id"},
		{name: "keyboard positions on a fretted instrument", instrument: "guitar", positions: []domain.Position{{Interval: "R", NoteName: "A", Key: func() *string { k := "A3"; return &k }()}}, skillIDs: []string{"skill-1"}, conceptIDs: []string{"concept-1"}, wantField: "positions"},
		{name: "no skills", instrument: "guitar", positions: []domain.Position{frettedPos(6, 5)}, conceptIDs: []string{"concept-1"}, wantField: "skill_ids"},
		{name: "unknown skill", instrument: "guitar", positions: []domain.Position{frettedPos(6, 5)}, skillIDs: []string{"missing"}, conceptIDs: []string{"concept-1"}, wantField: "skill_ids"},
		{name: "unknown concept", instrument: "guitar", positions: []domain.Position{frettedPos(6, 5)}, skillIDs: []string{"skill-1"}, conceptIDs: []string{"missing"}, wantField: "concept_ids"},
	}
	for _, tt := range tests {
		t.Run("rejected: "+tt.name, func(t *testing.T) {
			f := newDiagramFixture()

			_, err := f.svc.CreateDiagram(ctx, teacherCaller(), tt.instrument, "D", tt.positions, tt.skillIDs, tt.conceptIDs, nil, domain.LabelDisplayInterval)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
			list, listErr := f.diagrams.List(ctx, "", "", "")
			require.NoError(t, listErr)
			assert.Empty(t, list)
		})
	}
}

func TestDiagramService_GetAndList(t *testing.T) {
	ctx := context.Background()

	t.Run("get returns not found for an unknown id", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := f.svc.GetDiagram(ctx, "nope")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("list filters by instrument, skill and concept", func(t *testing.T) {
		f := newDiagramFixture()
		onGuitar, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "G", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, nil, domain.LabelDisplayInterval)
		require.NoError(t, err)
		key := "A3"
		onPiano, err := f.svc.CreateDiagram(ctx, teacherCaller(), "piano", "P", []domain.Position{{Interval: "R", NoteName: "A", Key: &key}}, []string{"skill-2"}, []string{"concept-2"}, nil, domain.LabelDisplayInterval)
		require.NoError(t, err)

		byInstrument, err := f.svc.ListDiagrams(ctx, "guitar", "", "")
		require.NoError(t, err)
		require.Len(t, byInstrument, 1)
		assert.Equal(t, onGuitar.ID, byInstrument[0].ID)

		bySkill, err := f.svc.ListDiagrams(ctx, "", "skill-2", "")
		require.NoError(t, err)
		require.Len(t, bySkill, 1)
		assert.Equal(t, onPiano.ID, bySkill[0].ID)

		all, err := f.svc.ListDiagrams(ctx, "", "", "")
		require.NoError(t, err)
		assert.Len(t, all, 2)
	})

	t.Run("list of an empty library is empty, not an error", func(t *testing.T) {
		f := newDiagramFixture()

		got, err := f.svc.ListDiagrams(ctx, "", "", "")

		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestDiagramService_UpdateDiagram(t *testing.T) {
	ctx := context.Background()

	seed := func(t *testing.T, f diagramFixture) domain.Diagram {
		t.Helper()
		d, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "Original", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, nil, domain.LabelDisplayInterval)
		require.NoError(t, err)
		return d
	}

	t.Run("renames without touching positions or classification", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f)
		name := "Renamed"

		got, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Name: &name})

		require.NoError(t, err)
		assert.Equal(t, "Renamed", got.Name)
		assert.Equal(t, d.Positions, got.Positions)
		assert.Equal(t, d.SkillIDs(), got.SkillIDs())
		assert.Equal(t, d.InstrumentID, got.InstrumentID)
		assert.Equal(t, d.CreatedAt, got.CreatedAt)
	})

	t.Run("replaces positions, validated against the diagram's own instrument", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f)

		got, err := f.svc.UpdateDiagram(ctx, adminCaller(), d.ID, application.DiagramUpdate{Positions: []domain.Position{frettedPos(5, 7), frettedPos(5, 10)}})

		require.NoError(t, err)
		require.Len(t, got.Positions, 2)
		assert.NotEmpty(t, got.Positions[0].ID)
	})

	t.Run("replaces classification", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f)

		got, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{SkillIDs: []string{"skill-2"}, ConceptIDs: []string{"concept-2"}})

		require.NoError(t, err)
		assert.Equal(t, []string{"skill-2"}, got.SkillIDs())
		assert.Equal(t, []string{"concept-2"}, got.ConceptIDs())
	})

	t.Run("wrong-shape positions are rejected and the diagram is unchanged", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f)
		key := "A3"

		_, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Positions: []domain.Position{{Interval: "R", NoteName: "A", Key: &key}}})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "positions", valErr.Fields[0].Field)
		stored, getErr := f.diagrams.GetByID(ctx, d.ID)
		require.NoError(t, getErr)
		assert.Equal(t, d.Positions, stored.Positions)
	})

	t.Run("an unknown skill in the new classification is rejected", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f)

		_, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{SkillIDs: []string{"missing"}, ConceptIDs: []string{"concept-1"}})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "skill_ids", valErr.Fields[0].Field)
	})

	t.Run("a student cannot update a diagram", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f)
		name := "X"

		_, err := f.svc.UpdateDiagram(ctx, studentCaller(), d.ID, application.DiagramUpdate{Name: &name})

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an unknown diagram is not found", func(t *testing.T) {
		f := newDiagramFixture()
		name := "X"

		_, err := f.svc.UpdateDiagram(ctx, teacherCaller(), "nope", application.DiagramUpdate{Name: &name})

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("replaces root note and label display, leaving them as-is when omitted", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f)
		root := "A"
		note := domain.LabelDisplayNote

		got, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{RootNote: &root, LabelDisplay: &note})
		require.NoError(t, err)
		require.NotNil(t, got.RootNote)
		assert.Equal(t, "A", *got.RootNote)
		assert.Equal(t, domain.LabelDisplayNote, got.LabelDisplay)

		name := "Renamed"
		again, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Name: &name})
		require.NoError(t, err)
		require.NotNil(t, again.RootNote)
		assert.Equal(t, "A", *again.RootNote)
		assert.Equal(t, domain.LabelDisplayNote, again.LabelDisplay)
	})
}
