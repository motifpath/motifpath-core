package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// The practice graders read diagrams only from the reference snapshot
// (ADR-047), so every path that creates or updates one must refresh it.
func TestDiagramService_PracticeReference(t *testing.T) {
	ctx := context.Background()
	eighth := domain.NoteValue{Num: 1, Den: 8}
	withPlayback := func() domain.DiagramOptions {
		tempo := 90
		return domain.DiagramOptions{TempoBPM: &tempo, Sequence: []domain.SequenceStep{{PositionIDs: []string{"p1"}, Value: eighth}}}
	}
	position := func() domain.Position {
		p := frettedPos(6, 5)
		p.ID = "p1"
		return p
	}

	t.Run("creating a diagram writes its reference", func(t *testing.T) {
		f := newDiagramFixture()

		got, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", names("Lick"), []domain.Position{position()}, []string{"skill-1"}, []string{"concept-1"}, withPlayback())

		require.NoError(t, err)
		ref, ok := f.references.diagram(got.ID)
		require.True(t, ok, "no reference written")
		assert.Equal(t, domain.NewDiagramReference(got), ref)
		require.NotNil(t, ref.TempoBPM)
		assert.Equal(t, 90, *ref.TempoBPM)
	})

	t.Run("updating a diagram rewrites its reference", func(t *testing.T) {
		f := newDiagramFixture()
		d, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", names("Lick"), []domain.Position{position()}, []string{"skill-1"}, []string{"concept-1"}, withPlayback())
		require.NoError(t, err)

		_, err = f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Sequence: []domain.SequenceStep{}, TempoBPM: application.Nullable[int]{Set: true}})

		require.NoError(t, err)
		ref, ok := f.references.diagram(d.ID)
		require.True(t, ok)
		assert.Nil(t, ref.TempoBPM, "the playback was removed, so the reference has no tempo")
	})

	t.Run("a refused update leaves the reference as it was", func(t *testing.T) {
		f := newDiagramFixture()
		d, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", names("Lick"), []domain.Position{position()}, []string{"skill-1"}, []string{"concept-1"}, withPlayback())
		require.NoError(t, err)
		before, _ := f.references.diagram(d.ID)

		_, err = f.svc.UpdateDiagram(ctx, otherTeacherCaller(), d.ID, application.DiagramUpdate{Sequence: []domain.SequenceStep{}, TempoBPM: application.Nullable[int]{Set: true}})

		require.ErrorIs(t, err, domain.ErrForbidden)
		after, _ := f.references.diagram(d.ID)
		assert.Equal(t, before, after)
	})

	t.Run("a failed reference write doesn't fail the saved diagram", func(t *testing.T) {
		f := newDiagramFixture()
		f.references.err = errors.New("mongo down")

		created, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", names("Lick"), []domain.Position{position()}, []string{"skill-1"}, []string{"concept-1"}, withPlayback())
		require.NoError(t, err)
		_, err = f.svc.UpdateDiagram(ctx, teacherCaller(), created.ID, application.DiagramUpdate{Names: names("Renamed")})
		require.NoError(t, err)
	})
}

func TestPracticeReferenceService_SyncDiagrams(t *testing.T) {
	ctx := context.Background()
	seed := func(t *testing.T, n int) *fakeDiagramRepository {
		t.Helper()
		diagrams := newFakeDiagramRepository()
		for i := range n {
			tempo := 60 + i
			require.NoError(t, diagrams.Create(ctx, domain.Diagram{
				ID: fmt.Sprintf("d-%03d", i), CreatedBy: "admin-1", Kind: domain.DiagramKindBasic,
				InstrumentID: "guitar", InstrumentIDs: []string{"guitar"}, TempoBPM: &tempo,
				Names: domain.LocalizedText{"en": fmt.Sprintf("Diagram %03d", i)},
			}))
		}
		return diagrams
	}

	t.Run("writes every diagram's reference, across pages", func(t *testing.T) {
		diagrams := seed(t, domain.MaxPageLimit+5)
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(diagrams, references)

		synced, err := svc.SyncDiagrams(ctx)

		require.NoError(t, err)
		assert.Equal(t, domain.MaxPageLimit+5, synced)
		for i := range domain.MaxPageLimit + 5 {
			id := fmt.Sprintf("d-%03d", i)
			stored, err := diagrams.GetByID(ctx, id)
			require.NoError(t, err)
			ref, ok := references.diagram(id)
			require.True(t, ok, "no reference for %s", id)
			assert.Equal(t, domain.NewDiagramReference(stored), ref)
		}
	})

	t.Run("reports a failed write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")
		svc := application.NewPracticeReferenceService(seed(t, 2), references)

		synced, err := svc.SyncDiagrams(ctx)

		require.Error(t, err)
		assert.Equal(t, 0, synced)
	})
}
