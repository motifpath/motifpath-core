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
	instruments.put(domain.Instrument{ID: "guitar", Names: domain.LocalizedText{"en": "Guitar"}, Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: []string{"E", "A", "D", "G", "B", "E"}})
	instruments.put(domain.Instrument{ID: "piano", Names: domain.LocalizedText{"en": "Piano"}, Family: domain.InstrumentFamilyKeyboard, KeyRange: &domain.KeyRange{Lowest: "A0", Highest: "C8"}})
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

		got, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "Minor Pentatonic", []domain.Position{frettedPos(6, 5), frettedPos(6, 8)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval})

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

		got, err := f.svc.CreateDiagram(ctx, adminCaller(), "guitar", "D", []domain.Position{pos}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval})

		require.NoError(t, err)
		assert.Equal(t, "client-id", got.Positions[0].ID)
	})

	t.Run("a student cannot create a diagram", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := f.svc.CreateDiagram(ctx, studentCaller(), "guitar", "D", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval})

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("root note and label display are carried onto the created diagram", func(t *testing.T) {
		f := newDiagramFixture()
		root := "A"

		got, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "D", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{RootNote: &root, LabelDisplay: domain.LabelDisplayNote})

		require.NoError(t, err)
		require.NotNil(t, got.RootNote)
		assert.Equal(t, "A", *got.RootNote)
		assert.Equal(t, domain.LabelDisplayNote, got.LabelDisplay)
	})

	t.Run("general and per-position colors are carried onto the created diagram", func(t *testing.T) {
		f := newDiagramFixture()
		general, override := "#3B82F6", "#EF4444"
		colored := frettedPos(6, 5)
		colored.Color = &override

		got, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "D", []domain.Position{colored, frettedPos(6, 8)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval, Color: &general})

		require.NoError(t, err)
		require.NotNil(t, got.Color)
		assert.Equal(t, "#3B82F6", *got.Color)
		require.NotNil(t, got.Positions[0].Color)
		assert.Equal(t, "#EF4444", *got.Positions[0].Color)
		assert.Nil(t, got.Positions[1].Color)
	})

	t.Run("a malformed general color is rejected as color", func(t *testing.T) {
		f := newDiagramFixture()
		bad := "blue"

		_, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "D", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval, Color: &bad})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "color", valErr.Fields[0].Field)
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

			_, err := f.svc.CreateDiagram(ctx, teacherCaller(), tt.instrument, "D", tt.positions, tt.skillIDs, tt.conceptIDs, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval})

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
			list, listErr := f.diagrams.List(ctx, domain.DiagramListFilter{}, domain.PageRequest{Limit: domain.MaxPageLimit})
			require.NoError(t, listErr)
			assert.Empty(t, list.Items)
		})
	}
}

func TestDiagramService_GetDiagram(t *testing.T) {
	ctx := context.Background()

	t.Run("get returns not found for an unknown id", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := f.svc.GetDiagram(ctx, "nope")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

}

func TestDiagramService_UpdateDiagram(t *testing.T) {
	ctx := context.Background()

	seed := func(t *testing.T, f diagramFixture) domain.Diagram {
		t.Helper()
		d, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", "Original", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval})
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

	t.Run("replaces the general color and per-position colors, leaving the general color as-is when omitted", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f)
		general, override := "#22C55E", "#F59E0B"
		positions := []domain.Position{d.Positions[0]}
		positions[0].Color = &override

		got, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Color: &general, Positions: positions})
		require.NoError(t, err)
		require.NotNil(t, got.Color)
		assert.Equal(t, "#22C55E", *got.Color)
		require.NotNil(t, got.Positions[0].Color)
		assert.Equal(t, "#F59E0B", *got.Positions[0].Color)

		name := "Renamed"
		again, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Name: &name})
		require.NoError(t, err)
		require.NotNil(t, again.Color)
		assert.Equal(t, "#22C55E", *again.Color)

		cleared, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Positions: []domain.Position{d.Positions[0]}})
		require.NoError(t, err)
		assert.Nil(t, cleared.Positions[0].Color)
	})
}

func TestDiagramService_CreateDiagram_KindAndOwner(t *testing.T) {
	ctx := context.Background()
	create := func(f diagramFixture, caller domain.User, kind domain.DiagramKind) (domain.Diagram, error) {
		return f.svc.CreateDiagram(ctx, caller, "guitar", "D", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{Kind: kind})
	}

	t.Run("a teacher's diagram is custom by default and owned by the teacher", func(t *testing.T) {
		f := newDiagramFixture()

		got, err := create(f, teacherCaller(), "")

		require.NoError(t, err)
		assert.Equal(t, domain.DiagramKindCustom, got.Kind)
		assert.Equal(t, teacherCaller().ID, got.CreatedBy)
	})

	t.Run("an admin creates a basic diagram, owned by the admin", func(t *testing.T) {
		f := newDiagramFixture()

		got, err := create(f, adminCaller(), domain.DiagramKindBasic)

		require.NoError(t, err)
		assert.Equal(t, domain.DiagramKindBasic, got.Kind)
		assert.Equal(t, adminCaller().ID, got.CreatedBy)
	})

	t.Run("a teacher cannot create a basic diagram", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := create(f, teacherCaller(), domain.DiagramKindBasic)

		require.ErrorIs(t, err, domain.ErrForbidden)
		assert.Empty(t, f.diagrams.byID)
	})

	t.Run("an unrecognised kind is a validation error, not forbidden", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := create(f, teacherCaller(), domain.DiagramKind("shared"))

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "kind", valErr.Fields[0].Field)
	})
}

func TestDiagramService_UpdateDiagram_Ownership(t *testing.T) {
	ctx := context.Background()
	rename := func(f diagramFixture, caller domain.User, id string) (domain.Diagram, error) {
		name := "Renamed"
		return f.svc.UpdateDiagram(ctx, caller, id, application.DiagramUpdate{Name: &name})
	}
	seed := func(t *testing.T, f diagramFixture, owner domain.User, kind domain.DiagramKind) domain.Diagram {
		t.Helper()
		d, err := f.svc.CreateDiagram(ctx, owner, "guitar", "Original", []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{Kind: kind})
		require.NoError(t, err)
		return d
	}

	t.Run("a teacher updates their own custom diagram", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f, teacherCaller(), domain.DiagramKindCustom)

		got, err := rename(f, teacherCaller(), d.ID)

		require.NoError(t, err)
		assert.Equal(t, "Renamed", got.Name)
	})

	t.Run("a teacher cannot update another teacher's custom diagram", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f, otherTeacherCaller(), domain.DiagramKindCustom)

		_, err := rename(f, teacherCaller(), d.ID)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher cannot update a basic diagram", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f, adminCaller(), domain.DiagramKindBasic)

		_, err := rename(f, teacherCaller(), d.ID)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin updates a basic diagram", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f, adminCaller(), domain.DiagramKindBasic)

		got, err := rename(f, adminCaller(), d.ID)

		require.NoError(t, err)
		assert.Equal(t, domain.DiagramKindBasic, got.Kind)
	})

	t.Run("an admin's update leaves a teacher's custom diagram owned by that teacher", func(t *testing.T) {
		f := newDiagramFixture()
		d := seed(t, f, teacherCaller(), domain.DiagramKindCustom)

		got, err := rename(f, adminCaller(), d.ID)

		require.NoError(t, err)
		assert.Equal(t, domain.DiagramKindCustom, got.Kind)
		assert.Equal(t, teacherCaller().ID, got.CreatedBy)
		stored, getErr := f.diagrams.GetByID(ctx, d.ID)
		require.NoError(t, getErr)
		assert.Equal(t, teacherCaller().ID, stored.CreatedBy)
	})

	t.Run("an unknown diagram is not found before any ownership check", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := rename(f, teacherCaller(), "nope")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestDiagramService_ListDiagrams(t *testing.T) {
	ctx := context.Background()
	page := domain.PageRequest{Limit: domain.MaxPageLimit}

	// seedLibrary creates one basic diagram (admin), one custom diagram for
	// teacherCaller and one for otherTeacherCaller, all on guitar.
	seedLibrary := func(t *testing.T, f diagramFixture) (basic, mine, theirs domain.Diagram) {
		t.Helper()
		mk := func(owner domain.User, name string, kind domain.DiagramKind) domain.Diagram {
			d, err := f.svc.CreateDiagram(ctx, owner, "guitar", name, []domain.Position{frettedPos(6, 5)}, []string{"skill-1"}, []string{"concept-1"}, domain.DiagramOptions{Kind: kind})
			require.NoError(t, err)
			return d
		}
		return mk(adminCaller(), "Basic", domain.DiagramKindBasic), mk(teacherCaller(), "Mine", domain.DiagramKindCustom), mk(otherTeacherCaller(), "Theirs", domain.DiagramKindCustom)
	}
	ids := func(p domain.Page[domain.Diagram]) []string {
		out := make([]string, len(p.Items))
		for i, d := range p.Items {
			out[i] = d.ID
		}
		return out
	}

	t.Run("an unrecognised kind filter is a validation error", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := f.svc.ListDiagrams(ctx, adminCaller(), domain.DiagramListFilter{Kind: domain.DiagramKind("shared")}, page)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "kind", valErr.Fields[0].Field)
	})

	t.Run("a student cannot list diagrams", func(t *testing.T) {
		f := newDiagramFixture()

		_, err := f.svc.ListDiagrams(ctx, studentCaller(), domain.DiagramListFilter{}, page)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher sees every basic diagram and only their own custom ones", func(t *testing.T) {
		f := newDiagramFixture()
		basic, mine, _ := seedLibrary(t, f)

		got, err := f.svc.ListDiagrams(ctx, teacherCaller(), domain.DiagramListFilter{}, page)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{basic.ID, mine.ID}, ids(got))
		assert.Equal(t, 2, got.Total)
	})

	t.Run("a teacher narrows to basic diagrams", func(t *testing.T) {
		f := newDiagramFixture()
		basic, _, _ := seedLibrary(t, f)

		got, err := f.svc.ListDiagrams(ctx, teacherCaller(), domain.DiagramListFilter{Kind: domain.DiagramKindBasic}, page)

		require.NoError(t, err)
		assert.Equal(t, []string{basic.ID}, ids(got))
	})

	t.Run("a teacher narrowing to custom diagrams sees only their own", func(t *testing.T) {
		f := newDiagramFixture()
		_, mine, _ := seedLibrary(t, f)

		got, err := f.svc.ListDiagrams(ctx, teacherCaller(), domain.DiagramListFilter{Kind: domain.DiagramKindCustom}, page)

		require.NoError(t, err)
		assert.Equal(t, []string{mine.ID}, ids(got))
	})

	t.Run("a teacher may filter by their own creator id", func(t *testing.T) {
		f := newDiagramFixture()
		_, mine, _ := seedLibrary(t, f)

		got, err := f.svc.ListDiagrams(ctx, teacherCaller(), domain.DiagramListFilter{CreatedBy: teacherCaller().ID}, page)

		require.NoError(t, err)
		assert.Equal(t, []string{mine.ID}, ids(got))
	})

	t.Run("a teacher cannot filter by another creator", func(t *testing.T) {
		f := newDiagramFixture()
		seedLibrary(t, f)

		_, err := f.svc.ListDiagrams(ctx, teacherCaller(), domain.DiagramListFilter{CreatedBy: otherTeacherCaller().ID}, page)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin sees every diagram", func(t *testing.T) {
		f := newDiagramFixture()
		basic, mine, theirs := seedLibrary(t, f)

		got, err := f.svc.ListDiagrams(ctx, adminCaller(), domain.DiagramListFilter{}, page)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{basic.ID, mine.ID, theirs.ID}, ids(got))
	})

	t.Run("an admin narrows to one creator", func(t *testing.T) {
		f := newDiagramFixture()
		_, _, theirs := seedLibrary(t, f)

		got, err := f.svc.ListDiagrams(ctx, adminCaller(), domain.DiagramListFilter{CreatedBy: otherTeacherCaller().ID}, page)

		require.NoError(t, err)
		assert.Equal(t, []string{theirs.ID}, ids(got))
	})

	t.Run("instrument, skill and concept filters combine with the scoping", func(t *testing.T) {
		f := newDiagramFixture()
		basic, _, _ := seedLibrary(t, f)
		key := "A3"
		_, err := f.svc.CreateDiagram(ctx, teacherCaller(), "piano", "Piano", []domain.Position{{Interval: "R", NoteName: "A", Key: &key}}, []string{"skill-2"}, []string{"concept-2"}, domain.DiagramOptions{})
		require.NoError(t, err)

		byInstrument, err := f.svc.ListDiagrams(ctx, adminCaller(), domain.DiagramListFilter{InstrumentID: "piano"}, page)
		require.NoError(t, err)
		assert.Len(t, byInstrument.Items, 1)

		bySkill, err := f.svc.ListDiagrams(ctx, teacherCaller(), domain.DiagramListFilter{SkillID: "skill-1", Kind: domain.DiagramKindBasic}, page)
		require.NoError(t, err)
		assert.Equal(t, []string{basic.ID}, ids(bySkill))
	})

	t.Run("pages are ordered by name and report the full total", func(t *testing.T) {
		f := newDiagramFixture()
		seedLibrary(t, f)

		got, err := f.svc.ListDiagrams(ctx, adminCaller(), domain.DiagramListFilter{}, domain.PageRequest{Limit: 2, Offset: 1})

		require.NoError(t, err)
		require.Len(t, got.Items, 2)
		assert.Equal(t, "Mine", got.Items[0].Name)
		assert.Equal(t, "Theirs", got.Items[1].Name)
		assert.Equal(t, 3, got.Total)
	})
}
