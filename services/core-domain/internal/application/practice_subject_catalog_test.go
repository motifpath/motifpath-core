package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// subjectCatalogFixture is a practice fixture with something of every kind a
// list tallies: know and hear exercises under progressions (the know ones
// accurate), guitar fretboard cells under notes (one of them asked the find
// way next, which a test template puts on the hear rung), a guitar
// play-along under riffs and a guitar diagram shape under grips. The student
// learns progressions on guitar and bass.
func subjectCatalogFixture(t *testing.T) *practiceFixture {
	f := newPracticeFixture(t)
	f.childSkill("progressions", "harmony")
	f.accurate(f.exercisesOf("know", "progressions", domain.ExerciseTypeTextResponse, 3)...)
	f.exercisesOf("hear", "progressions", domain.ExerciseTypeAudioRecognition, 2)
	f.childSkill("notes", "fretboard")
	cells := f.cellsOn(practiceGuitar, "notes", 3, 6)
	f.stateOf(cells[0], domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.inDays(1),
		RightByResponse: map[string]int{string(domain.FretboardDrillNameTheNote): 3}})
	f.templates.extra = append(f.templates.extra, domain.DrillTemplate{Key: "fretboard_cell:find_the_note", ItemKind: domain.PracticeItemKindFretboardCell, Rung: domain.PracticeRungHear, Timed: true})
	f.playAlong("riff", "Riff", practiceGuitar, "riffs", 100)
	f.shapeOn("grip", "grips", practiceGuitar)
	f.onPathFor([]string{practiceGuitar, practiceBass}, "progressions")
	return f
}

// sessions are the lists a test compares: in hand on guitar, in hand on
// bass, and in the head.
var sessions = map[string]*string{"guitar": ptrTo(practiceGuitar), "bass": ptrTo(practiceBass), "head": nil}

// listAll lists the subjects for every session.
func listAll(t *testing.T, f *practiceFixture) map[string]domain.PracticeSubjectList {
	t.Helper()
	lists := map[string]domain.PracticeSubjectList{}
	for name, instrumentID := range sessions {
		list, err := f.svc.ListSubjects(context.Background(), studentCaller(), instrumentID)
		require.NoError(t, err, name)
		lists[name] = list
	}
	return lists
}

// withCatalogs installs a practice subject catalog on f.
func withCatalogs(f *practiceFixture) *fakePracticeSubjectCatalogs {
	catalogs := newFakePracticeSubjectCatalogs()
	f.svc.WithSubjectCatalogs(catalogs)
	return catalogs
}

// itemReads is how many diagram and exercise reads f has seen.
func itemReads(f *practiceFixture) int {
	return f.diagrams.gets + f.diagrams.batchReads + f.exercises.gets + f.exercises.batchReads
}

// resetItemReads forgets the diagram and exercise reads f has seen.
func resetItemReads(f *practiceFixture) {
	f.diagrams.gets, f.diagrams.batchReads, f.exercises.gets, f.exercises.batchReads = 0, 0, 0, 0
}

func TestPracticeSessionService_ListSubjectsFromCatalog(t *testing.T) {
	ctx := context.Background()

	t.Run("the first list builds and stores the catalog; later ones read it, reading no item, and list the same", func(t *testing.T) {
		live := listAll(t, subjectCatalogFixture(t))
		f := subjectCatalogFixture(t)
		catalogs := withCatalogs(f)

		first := listAll(t, f)
		require.ElementsMatch(t, []string{practiceGuitar, practiceBass}, catalogs.scopes(), "a catalog for each instrument listed")
		resetItemReads(f)
		later := listAll(t, f)

		assert.Equal(t, live, first)
		assert.Equal(t, live, later)
		assert.Zero(t, itemReads(f), "no diagram or exercise is read")
	})

	t.Run("a content change makes the next list rebuild, showing it; the one after reads the catalog again", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		withCatalogs(f)
		_, err := f.svc.ListSubjects(ctx, studentCaller(), guitar())
		require.NoError(t, err)

		f.exercisesOf("more", "progressions", domain.ExerciseTypeTextResponse, 2)
		f.svc.InvalidateSubjectCatalogs(ctx)
		resetItemReads(f)
		changed, err := f.svc.ListSubjects(ctx, studentCaller(), guitar())
		require.NoError(t, err)
		rebuildReads := itemReads(f)
		resetItemReads(f)
		_, err = f.svc.ListSubjects(ctx, studentCaller(), guitar())
		require.NoError(t, err)

		progressions, _ := subjectOf(changed, "progressions")
		assert.Equal(t, 7, progressions.ItemCount)
		assert.NotZero(t, rebuildReads, "the changed list rebuilt")
		assert.Zero(t, itemReads(f), "the next one read the catalog")
	})

	t.Run("a catalog built from content that changed while it was built is not stored", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		catalogs := withCatalogs(f)
		catalogs.beforeStore = func() { require.NoError(t, catalogs.BumpContentVersion(ctx)) }

		_, err := f.svc.ListSubjects(ctx, studentCaller(), guitar())
		require.NoError(t, err)
		assert.Empty(t, catalogs.scopes())

		catalogs.beforeStore = nil
		_, err = f.svc.ListSubjects(ctx, studentCaller(), guitar())
		require.NoError(t, err)
		assert.Equal(t, []string{practiceGuitar}, catalogs.scopes(), "the next list builds it again")
	})

	t.Run("a catalog built by an older format is rebuilt, and lists the same", func(t *testing.T) {
		live := listAll(t, subjectCatalogFixture(t))
		f := subjectCatalogFixture(t)
		catalogs := withCatalogs(f)
		listAll(t, f)
		catalogs.ageFormat()
		resetItemReads(f)

		lists := listAll(t, f)

		assert.Equal(t, live, lists)
		assert.NotZero(t, itemReads(f), "rebuilt from the content")
	})

	t.Run("a catalog that fails to store still lists", func(t *testing.T) {
		live := listAll(t, subjectCatalogFixture(t))
		f := subjectCatalogFixture(t)
		catalogs := withCatalogs(f)
		catalogs.storeErr = errors.New("database down")

		assert.Equal(t, live, listAll(t, f))
	})

	t.Run("a node renamed shows at once, as nodes are read live", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		withCatalogs(f)
		_, err := f.svc.ListSubjects(ctx, studentCaller(), guitar())
		require.NoError(t, err)
		parent := "harmony"
		f.knowledgeNodes.put(domain.KnowledgeNode{ID: "progressions", Kind: domain.KnowledgeNodeKindSkill, Key: "progressions", ParentID: &parent, Names: domain.LocalizedText{"en": "Common progressions"}})

		list, err := f.svc.ListSubjects(ctx, studentCaller(), guitar())

		require.NoError(t, err)
		progressions, ok := subjectOf(list, "progressions")
		require.True(t, ok)
		assert.Equal(t, domain.LocalizedText{"en": "Common progressions"}, progressions.Node.Names)
	})

	t.Run("invalidating raises the content version by one", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		catalogs := withCatalogs(f)

		f.svc.InvalidateSubjectCatalogs(ctx)
		f.svc.InvalidateSubjectCatalogs(ctx)

		assert.Equal(t, int64(2), catalogs.version)
	})
}

// recordedInvalidations is an application.SubjectCatalogInvalidator that
// counts its invalidations.
type recordedInvalidations struct{ n int }

func (r *recordedInvalidations) InvalidateSubjectCatalogs(context.Context) { r.n++ }

// Every diagram, exercise, instrument and published song chart has its
// practice reference written after it is saved, so the reference writer
// invalidates the catalog there too.
func TestCatalogInvalidatingReferences(t *testing.T) {
	ctx := context.Background()
	writes := []struct {
		name          string
		write         func(w ports.PracticeReferenceWriter) error
		invalidations int
	}{
		{"a diagram's reference invalidates", func(w ports.PracticeReferenceWriter) error {
			return w.PutDiagrams(ctx, []domain.DiagramReference{{ID: "d"}})
		}, 1},
		{"an exercise's reference invalidates", func(w ports.PracticeReferenceWriter) error {
			return w.PutExercises(ctx, []domain.ExerciseReference{{ID: "e"}})
		}, 1},
		{"an instrument's reference invalidates", func(w ports.PracticeReferenceWriter) error {
			return w.PutInstruments(ctx, []domain.InstrumentReference{{ID: "i"}})
		}, 1},
		{"a song chart's reference invalidates", func(w ports.PracticeReferenceWriter) error {
			return w.PutSongCharts(ctx, []domain.SongChartReference{{ID: "s"}})
		}, 1},
		{"catalog data installed by migrations doesn't: the invalidation on start covers it", func(w ports.PracticeReferenceWriter) error {
			return w.PutProgressions(ctx, []domain.Progression{{Key: "I-IV-V-I"}})
		}, 0},
	}
	for _, tc := range writes {
		t.Run(tc.name, func(t *testing.T) {
			refs := newFakePracticeReferenceWriter()
			invalidations := &recordedInvalidations{}

			require.NoError(t, tc.write(application.CatalogInvalidatingReferences(refs, invalidations)))

			assert.Equal(t, tc.invalidations, invalidations.n)
			assert.Equal(t, 1, refs.writes, "the reference is still written")
		})
	}

	t.Run("a reference that failed to write doesn't invalidate", func(t *testing.T) {
		refs := newFakePracticeReferenceWriter()
		refs.err = errors.New("mongo down")
		invalidations := &recordedInvalidations{}

		err := application.CatalogInvalidatingReferences(refs, invalidations).PutDiagrams(ctx, []domain.DiagramReference{{ID: "d"}})

		require.Error(t, err)
		assert.Zero(t, invalidations.n)
	})
}

// A withdrawn song chart is no longer a song to practise, and a withdrawal
// writes no practice reference, so it invalidates the catalog itself.
func TestSongChartService_WithdrawInvalidatesTheCatalog(t *testing.T) {
	ctx := context.Background()
	f := newSongChartFixture()
	invalidations := &recordedInvalidations{}
	f.service.WithCatalogInvalidation(invalidations)
	chart, err := f.service.Create(ctx, adminCaller(), asaBranca())
	require.NoError(t, err)
	_, err = f.service.Publish(ctx, adminCaller(), chart.ID)
	require.NoError(t, err)
	invalidations.n = 0

	_, err = f.service.Withdraw(ctx, adminCaller(), chart.ID, "Rights disputed")

	require.NoError(t, err)
	assert.Equal(t, 1, invalidations.n)
}

func subjectOf(list domain.PracticeSubjectList, nodeID string) (domain.PracticeSubject, bool) {
	for _, g := range list.Groups {
		for _, s := range g.Subjects {
			if s.Node.ID == nodeID {
				return s, true
			}
		}
	}
	return domain.PracticeSubject{}, false
}
