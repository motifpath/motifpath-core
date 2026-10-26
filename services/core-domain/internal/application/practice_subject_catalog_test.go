package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// subjectCatalogFixture is a practice fixture with something of every kind a list
// tallies: know and hear exercises under progressions (the know ones
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

// withCatalogs installs the catalog on f, its rebuilds settling and polling
// as given.
func withCatalogs(f *practiceFixture, settle, poll time.Duration) (*fakePracticeSubjectCatalogs, *application.PracticeSubjectCatalogService) {
	catalogs := newFakePracticeSubjectCatalogs()
	service := application.NewPracticeSubjectCatalogService(f.svc, catalogs, settle, poll)
	f.svc.WithSubjectCatalogs(catalogs, service)
	return catalogs, service
}

func TestPracticeSessionService_ListSubjectsFromCatalog(t *testing.T) {
	ctx := context.Background()

	t.Run("a built catalog lists what the live list does, reading no item", func(t *testing.T) {
		live := listAll(t, subjectCatalogFixture(t))
		f := subjectCatalogFixture(t)
		catalogs, service := withCatalogs(f, 0, time.Hour)
		service.Signal(ctx, domain.PracticeCatalogSignalStart)
		rebuilt, err := service.RebuildPending(ctx)
		require.NoError(t, err)
		require.True(t, rebuilt)
		f.diagrams.gets, f.diagrams.batchReads, f.exercises.gets, f.exercises.batchReads = 0, 0, 0, 0

		lists := listAll(t, f)

		assert.Equal(t, live, lists)
		assert.Zero(t, f.diagrams.gets+f.diagrams.batchReads, "no diagram is read")
		assert.Zero(t, f.exercises.gets+f.exercises.batchReads, "no exercise is read")
		assert.Empty(t, catalogs.pending(), "nothing was missing")
	})

	t.Run("a scope with no catalog is listed live and queues a rebuild", func(t *testing.T) {
		live := listAll(t, subjectCatalogFixture(t))
		f := subjectCatalogFixture(t)
		catalogs, _ := withCatalogs(f, 0, time.Hour)

		lists := listAll(t, f)

		assert.Equal(t, live, lists)
		assert.Contains(t, catalogs.pending(), domain.PracticeCatalogSignalMissing)
	})

	t.Run("a catalog built by an older format is ignored, listed live, and rebuilt", func(t *testing.T) {
		live := listAll(t, subjectCatalogFixture(t))
		f := subjectCatalogFixture(t)
		catalogs, service := withCatalogs(f, 0, time.Hour)
		service.Signal(ctx, domain.PracticeCatalogSignalStart)
		_, err := service.RebuildPending(ctx)
		require.NoError(t, err)
		catalogs.ageFormat()

		lists := listAll(t, f)

		assert.Equal(t, live, lists)
		assert.Contains(t, catalogs.pending(), domain.PracticeCatalogSignalMissing)
	})

	t.Run("new content shows once the catalog is rebuilt, not before", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		_, service := withCatalogs(f, 0, time.Hour)
		service.Signal(ctx, domain.PracticeCatalogSignalStart)
		_, err := service.RebuildPending(ctx)
		require.NoError(t, err)

		f.exercisesOf("more", "progressions", domain.ExerciseTypeTextResponse, 2)
		before, err := f.svc.ListSubjects(ctx, studentCaller(), guitar())
		require.NoError(t, err)
		service.Signal(ctx, domain.PracticeCatalogSignalExercise)
		_, err = service.RebuildPending(ctx)
		require.NoError(t, err)
		after, err := f.svc.ListSubjects(ctx, studentCaller(), guitar())
		require.NoError(t, err)

		progressions, _ := subjectOf(before, "progressions")
		assert.Equal(t, 5, progressions.ItemCount)
		progressions, _ = subjectOf(after, "progressions")
		assert.Equal(t, 7, progressions.ItemCount)
	})

	t.Run("a node renamed shows at once, as nodes are read live", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		_, service := withCatalogs(f, 0, time.Hour)
		service.Signal(ctx, domain.PracticeCatalogSignalStart)
		_, err := service.RebuildPending(ctx)
		require.NoError(t, err)
		parent := "harmony"
		f.knowledgeNodes.put(domain.KnowledgeNode{ID: "progressions", Kind: domain.KnowledgeNodeKindSkill, Key: "progressions", ParentID: &parent, Names: domain.LocalizedText{"en": "Common progressions"}})

		list, err := f.svc.ListSubjects(ctx, studentCaller(), guitar())

		require.NoError(t, err)
		progressions, ok := subjectOf(list, "progressions")
		require.True(t, ok)
		assert.Equal(t, domain.LocalizedText{"en": "Common progressions"}, progressions.Node.Names)
	})
}

func TestPracticeSubjectCatalogService_Rebuild(t *testing.T) {
	ctx := context.Background()

	t.Run("signals pending at once cause one rebuild, which clears them", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		catalogs, service := withCatalogs(f, 0, time.Hour)
		service.Signal(ctx, domain.PracticeCatalogSignalDiagram)
		service.Signal(ctx, domain.PracticeCatalogSignalExercise)
		service.Signal(ctx, domain.PracticeCatalogSignalExercise)

		rebuilt, err := service.RebuildPending(ctx)

		require.NoError(t, err)
		assert.True(t, rebuilt)
		assert.Equal(t, 1, catalogs.builds)
		assert.Empty(t, catalogs.pending())
		assert.ElementsMatch(t, []string{practiceGuitar, practiceBass, ""}, catalogs.scopes(), "a catalog for every instrument, and for none")
	})

	t.Run("with nothing pending, nothing is rebuilt", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		catalogs, service := withCatalogs(f, 0, time.Hour)

		rebuilt, err := service.RebuildPending(ctx)

		require.NoError(t, err)
		assert.False(t, rebuilt)
		assert.Zero(t, catalogs.builds)
	})

	t.Run("a signal queued during a rebuild stays for one more", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		catalogs, service := withCatalogs(f, 0, time.Hour)
		service.Signal(ctx, domain.PracticeCatalogSignalStart)
		catalogs.duringBuild = func() { service.Signal(ctx, domain.PracticeCatalogSignalDiagram) }

		_, err := service.RebuildPending(ctx)
		require.NoError(t, err)

		assert.Equal(t, []domain.PracticeCatalogSignalReason{domain.PracticeCatalogSignalDiagram}, catalogs.pending())
	})

	t.Run("a failed rebuild keeps its signals and the catalogs it had", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		catalogs, service := withCatalogs(f, 0, time.Hour)
		service.Signal(ctx, domain.PracticeCatalogSignalStart)
		_, err := service.RebuildPending(ctx)
		require.NoError(t, err)
		service.Signal(ctx, domain.PracticeCatalogSignalDiagram)
		f.templates.err = errors.New("database down")

		_, err = service.RebuildPending(ctx)

		require.Error(t, err)
		assert.Equal(t, []domain.PracticeCatalogSignalReason{domain.PracticeCatalogSignalDiagram}, catalogs.pending())
		assert.Len(t, catalogs.scopes(), 3)
	})

	t.Run("running, it rebuilds on start, then once per burst of signals, until stopped", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		catalogs, service := withCatalogs(f, 5*time.Millisecond, time.Hour)
		runCtx, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			service.Run(runCtx)
			close(done)
		}()

		require.Eventually(t, func() bool { return catalogs.buildCount() == 1 }, time.Second, time.Millisecond, "the start rebuild")
		service.Signal(ctx, domain.PracticeCatalogSignalDiagram)
		service.Signal(ctx, domain.PracticeCatalogSignalExercise)
		require.Eventually(t, func() bool { return catalogs.buildCount() == 2 }, time.Second, time.Millisecond, "one rebuild for the burst")
		stop()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Run didn't stop")
		}
		assert.Equal(t, 2, catalogs.buildCount())
	})

	t.Run("running, it picks up a signal another instance queued at the next poll", func(t *testing.T) {
		f := subjectCatalogFixture(t)
		catalogs, service := withCatalogs(f, 0, 10*time.Millisecond)
		runCtx, stop := context.WithCancel(ctx)
		defer stop()
		go service.Run(runCtx)
		require.Eventually(t, func() bool { return catalogs.buildCount() == 1 }, time.Second, time.Millisecond)

		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalDiagram))

		require.Eventually(t, func() bool { return catalogs.buildCount() == 2 }, time.Second, time.Millisecond)
	})
}

// recordedSignals is an application.CatalogSignaller that records each
// signal.
type recordedSignals struct{ reasons []domain.PracticeCatalogSignalReason }

func (r *recordedSignals) Signal(_ context.Context, reason domain.PracticeCatalogSignalReason) {
	r.reasons = append(r.reasons, reason)
}

// Every diagram, exercise, instrument and published song chart has its
// practice reference written after it is saved, so the reference writer
// queues the catalog's rebuild there too.
func TestCatalogSignallingReferences(t *testing.T) {
	ctx := context.Background()
	writes := []struct {
		name   string
		write  func(w ports.PracticeReferenceWriter) error
		reason []domain.PracticeCatalogSignalReason
	}{
		{"a diagram's reference queues a diagram signal", func(w ports.PracticeReferenceWriter) error {
			return w.PutDiagrams(ctx, []domain.DiagramReference{{ID: "d"}})
		}, []domain.PracticeCatalogSignalReason{domain.PracticeCatalogSignalDiagram}},
		{"an exercise's reference queues an exercise signal", func(w ports.PracticeReferenceWriter) error {
			return w.PutExercises(ctx, []domain.ExerciseReference{{ID: "e"}})
		}, []domain.PracticeCatalogSignalReason{domain.PracticeCatalogSignalExercise}},
		{"an instrument's reference queues an instrument signal", func(w ports.PracticeReferenceWriter) error {
			return w.PutInstruments(ctx, []domain.InstrumentReference{{ID: "i"}})
		}, []domain.PracticeCatalogSignalReason{domain.PracticeCatalogSignalInstrument}},
		{"a song chart's reference queues a song chart signal", func(w ports.PracticeReferenceWriter) error {
			return w.PutSongCharts(ctx, []domain.SongChartReference{{ID: "s"}})
		}, []domain.PracticeCatalogSignalReason{domain.PracticeCatalogSignalSongChart}},
		{"catalog data installed by migrations queues nothing: the start signal covers it", func(w ports.PracticeReferenceWriter) error {
			return w.PutProgressions(ctx, []domain.Progression{{Key: "I-IV-V-I"}})
		}, nil},
	}
	for _, tc := range writes {
		t.Run(tc.name, func(t *testing.T) {
			refs := newFakePracticeReferenceWriter()
			signals := &recordedSignals{}

			require.NoError(t, tc.write(application.CatalogSignallingReferences(refs, signals)))

			assert.Equal(t, tc.reason, signals.reasons)
			assert.Equal(t, 1, refs.writes, "the reference is still written")
		})
	}

	t.Run("a reference that failed to write queues nothing", func(t *testing.T) {
		refs := newFakePracticeReferenceWriter()
		refs.err = errors.New("mongo down")
		signals := &recordedSignals{}

		err := application.CatalogSignallingReferences(refs, signals).PutDiagrams(ctx, []domain.DiagramReference{{ID: "d"}})

		require.Error(t, err)
		assert.Empty(t, signals.reasons)
	})
}

// A withdrawn song chart is no longer a song to practise, and a withdrawal
// writes no practice reference, so it queues the catalog's rebuild itself.
func TestSongChartService_WithdrawSignalsTheCatalog(t *testing.T) {
	ctx := context.Background()
	f := newSongChartFixture()
	signals := &recordedSignals{}
	f.service.WithCatalogSignals(signals)
	chart, err := f.service.Create(ctx, adminCaller(), asaBranca())
	require.NoError(t, err)
	_, err = f.service.Publish(ctx, adminCaller(), chart.ID)
	require.NoError(t, err)
	signals.reasons = nil

	_, err = f.service.Withdraw(ctx, adminCaller(), chart.ID, "Rights disputed")

	require.NoError(t, err)
	assert.Equal(t, []domain.PracticeCatalogSignalReason{domain.PracticeCatalogSignalSongChart}, signals.reasons)
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
