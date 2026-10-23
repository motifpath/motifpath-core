package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// The practice graders read diagrams only from the reference snapshot
// (ADR-047), so every path that creates or updates one must refresh it.
func TestDiagramService_PracticeReference(t *testing.T) {
	ctx := context.Background()
	eighth := domain.NoteValue{Num: 1, Den: 8}
	withPlayback := func() domain.DiagramOptions {
		return domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{{ID: "pb-1", Names: names("Run"), TempoBPM: 90, Steps: []domain.SequenceStep{{PositionIDs: []string{"p1"}, Value: eighth}}}}}
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

		_, err = f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Playbacks: []domain.DiagramPlayback{}})

		require.NoError(t, err)
		ref, ok := f.references.diagram(d.ID)
		require.True(t, ok)
		assert.Nil(t, ref.TempoBPM, "the playbacks were removed, so the reference has no tempo")
	})

	t.Run("a refused update leaves the reference as it was", func(t *testing.T) {
		f := newDiagramFixture()
		d, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", names("Lick"), []domain.Position{position()}, []string{"skill-1"}, []string{"concept-1"}, withPlayback())
		require.NoError(t, err)
		before, _ := f.references.diagram(d.ID)

		_, err = f.svc.UpdateDiagram(ctx, otherTeacherCaller(), d.ID, application.DiagramUpdate{Playbacks: []domain.DiagramPlayback{}})

		require.ErrorIs(t, err, domain.ErrForbidden)
		after, _ := f.references.diagram(d.ID)
		assert.Equal(t, before, after)
	})

	t.Run("the reference is written even when the request ends right after the save", func(t *testing.T) {
		f := newDiagramFixture()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		got, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", names("Lick"), []domain.Position{position()}, []string{"skill-1"}, []string{"concept-1"}, withPlayback())

		require.NoError(t, err)
		_, ok := f.references.diagram(got.ID)
		assert.True(t, ok, "a client leaving after the commit must not cost the snapshot its write")
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

// The exercise_option grader reads exercises only from the reference
// snapshot, so creating or updating one must refresh it.
func TestExerciseService_PracticeReference(t *testing.T) {
	ctx := context.Background()
	newService := func(references *fakePracticeReferenceWriter) (*application.ExerciseService, *fakeExerciseRepository) {
		exercises := newFakeExerciseRepository()
		return newExerciseServiceWithReferences(exercises, references), exercises
	}
	create := func(t *testing.T, ctx context.Context, svc *application.ExerciseService) domain.Exercise {
		t.Helper()
		e, err := svc.CreateExercise(ctx, teacherCaller(), "A minor or major?", domain.NewPlainTextPrompt("Which is it?"),
			domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)
		require.NoError(t, err)
		return e
	}
	update := func(ctx context.Context, svc *application.ExerciseService, caller domain.User, id string, options []domain.Option) error {
		_, err := svc.UpdateExercise(ctx, caller, id, "A minor or major?", domain.NewPlainTextPrompt("Which is it?"),
			[]string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, options, nil, nil, []string{"en"}, nil)
		return err
	}

	t.Run("creating an exercise writes its reference", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc, _ := newService(references)

		got := create(t, ctx, svc)

		ref, ok := references.exercise(got.ID)
		require.True(t, ok, "no reference written")
		assert.Equal(t, domain.NewExerciseReference(got), ref)
		assert.Equal(t, []string{"opt-1"}, ref.CorrectOptionIDs)
	})

	t.Run("updating an exercise rewrites its reference", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc, _ := newService(references)
		e := create(t, ctx, svc)
		swapped := textResponseOptions()
		swapped[0].IsCorrect, swapped[1].IsCorrect = false, true

		require.NoError(t, update(ctx, svc, teacherCaller(), e.ID, swapped))

		ref, ok := references.exercise(e.ID)
		require.True(t, ok)
		assert.Equal(t, []string{"opt-2"}, ref.CorrectOptionIDs, "the correct option changed")
	})

	t.Run("a refused update leaves the reference as it was", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc, _ := newService(references)
		e := create(t, ctx, svc)
		before, _ := references.exercise(e.ID)

		err := update(ctx, svc, studentCaller(), e.ID, textResponseOptions())

		require.ErrorIs(t, err, domain.ErrForbidden)
		after, _ := references.exercise(e.ID)
		assert.Equal(t, before, after)
	})

	t.Run("the reference is written even when the request ends right after the save", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc, _ := newService(references)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		got := create(t, ctx, svc)

		_, ok := references.exercise(got.ID)
		assert.True(t, ok, "a client leaving after the commit must not cost the snapshot its write")
	})

	t.Run("a failed reference write doesn't fail the saved exercise", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")
		svc, exercises := newService(references)

		e := create(t, ctx, svc)
		require.NoError(t, update(ctx, svc, teacherCaller(), e.ID, textResponseOptions()))

		_, err := exercises.GetByID(ctx, e.ID)
		require.NoError(t, err)
	})
}

func TestPracticeReferenceService_SyncExercises(t *testing.T) {
	ctx := context.Background()
	seed := func(t *testing.T, n int) *fakeExerciseRepository {
		t.Helper()
		exercises := newFakeExerciseRepository()
		for i := range n {
			require.NoError(t, exercises.Create(ctx, domain.Exercise{
				ID: fmt.Sprintf("e-%03d", i), Title: fmt.Sprintf("Exercise %03d", i),
				ExerciseType: domain.ExerciseTypeTextResponse, Options: textResponseOptions(),
			}))
		}
		return exercises
	}

	t.Run("writes every exercise's reference, across pages", func(t *testing.T) {
		exercises := seed(t, domain.MaxPageLimit+5)
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), exercises, newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncExercises(ctx)

		require.NoError(t, err)
		assert.Equal(t, domain.MaxPageLimit+5, synced)
		assert.Equal(t, 2, references.writeCount(), "one bulk write per page, not one per exercise")
		for i := range domain.MaxPageLimit + 5 {
			id := fmt.Sprintf("e-%03d", i)
			stored, err := exercises.GetByID(ctx, id)
			require.NoError(t, err)
			ref, ok := references.exercise(id)
			require.True(t, ok, "no reference for %s", id)
			assert.Equal(t, domain.NewExerciseReference(stored), ref)
		}
	})

	t.Run("reports a failed write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), seed(t, 2), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncExercises(ctx)

		require.Error(t, err)
		assert.Equal(t, 0, synced)
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
				InstrumentID: "guitar", InstrumentIDs: []string{"guitar"},
				Playbacks: []domain.DiagramPlayback{{ID: "pb-1", TempoBPM: tempo}}, DefaultPlaybackID: strPtr("pb-1"),
				Names: domain.LocalizedText{"en": fmt.Sprintf("Diagram %03d", i)},
			}))
		}
		return diagrams
	}

	t.Run("writes every diagram's reference, across pages", func(t *testing.T) {
		diagrams := seed(t, domain.MaxPageLimit+5)
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(diagrams, newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncDiagrams(ctx)

		require.NoError(t, err)
		assert.Equal(t, domain.MaxPageLimit+5, synced)
		assert.Equal(t, 2, references.writeCount(), "one bulk write per page, not one per diagram")
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
		svc := application.NewPracticeReferenceService(seed(t, 2), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncDiagrams(ctx)

		require.Error(t, err)
		assert.Equal(t, 0, synced)
	})
}

func TestPracticeReferenceService_SyncDrillThresholds(t *testing.T) {
	ctx := context.Background()
	october := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	installed := []domain.DrillThreshold{
		{ID: "th-1", TemplateKey: "exercise:text_response", Version: 1, EffectiveFrom: october, FluentNetMs: 6000, Source: domain.DrillThresholdSourceDefault},
		{ID: "th-2", TemplateKey: "exercise:audio_selection", Version: 1, EffectiveFrom: october, FluentNetMs: 4000, Source: domain.DrillThresholdSourceDefault},
	}

	t.Run("writes every installed version in one write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(installed), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncDrillThresholds(ctx)

		require.NoError(t, err)
		assert.Equal(t, 2, synced)
		assert.Equal(t, 1, references.writeCount())
		assert.Equal(t, installed, references.drillThresholds())
	})

	t.Run("reports a failed write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(installed), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncDrillThresholds(ctx)

		require.Error(t, err)
		assert.Equal(t, 0, synced)
	})
}

// A chord or root change is judged against the change target in force when it
// was played, and targets come only from migrations, so the sync is how they
// reach the snapshot.
func TestPracticeReferenceService_SyncChangeTargets(t *testing.T) {
	ctx := context.Background()
	installed := domain.ChordDrillCatalog{Targets: []domain.ChangeTarget{
		{ID: "ct-1", Version: 1, EffectiveFrom: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), ChangesPerMinute: 60, Source: domain.DrillThresholdSourceDefault},
	}}

	t.Run("writes every installed version in one write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills(installed), newFakeSongChartRepository(), references)

		synced, err := svc.SyncChangeTargets(ctx)

		require.NoError(t, err)
		assert.Equal(t, 1, synced)
		assert.Equal(t, 1, references.writeCount())
		assert.Equal(t, installed.Targets, references.changeTargets())
	})

	t.Run("a full sync writes them too", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills(installed), newFakeSongChartRepository(), references)

		synced, err := svc.Sync(ctx)

		require.NoError(t, err)
		assert.Equal(t, 1, synced.ChangeTargets)
		assert.Equal(t, installed.Targets, references.changeTargets())
	})

	t.Run("reports a failed write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills(installed), newFakeSongChartRepository(), references)

		synced, err := svc.SyncChangeTargets(ctx)

		require.Error(t, err)
		assert.Equal(t, 0, synced)
	})
}

// catalogProgressions are two confusable progressions as the progression
// catalog installs them.
func catalogProgressions() []domain.Progression {
	return []domain.Progression{
		{ID: "pr-pop", Key: "I-V-vi-IV", Mode: "major", Bars: []string{"I", "V", "vi", "IV"}, NodeIDs: []string{"node-cp"}, Confusable: []string{"I-vi-IV-V"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)"}},
		{ID: "pr-doo-wop", Key: "I-vi-IV-V", Mode: "major", Bars: []string{"I", "vi", "IV", "V"}, NodeIDs: []string{"node-cp"}, Confusable: []string{"I-V-vi-IV"}, Names: domain.LocalizedText{"en": "I–vi–IV–V (doo-wop)"}},
	}
}

// Harmony and ear drills are graded against the progression catalog, which
// comes only from migrations, so the sync is how it reaches the snapshot.
func TestPracticeReferenceService_SyncProgressions(t *testing.T) {
	ctx := context.Background()

	t.Run("writes every installed progression in one write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(catalogProgressions()), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncProgressions(ctx)

		require.NoError(t, err)
		assert.Equal(t, 2, synced)
		assert.Equal(t, 1, references.writeCount())
		assert.Equal(t, catalogProgressions(), references.progressions())
	})

	t.Run("writes nothing when none is installed", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncProgressions(ctx)

		require.NoError(t, err)
		assert.Equal(t, 0, synced)
		assert.Equal(t, 0, references.writeCount())
	})

	t.Run("reports a failed write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(catalogProgressions()), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncProgressions(ctx)

		require.Error(t, err)
		assert.Equal(t, 0, synced)
	})
}

// A song is graded against the charts ever published: a withdrawn chart's songs
// keep their evidence, and a chart never published has no songs.
func TestPracticeReferenceService_SyncSongCharts(t *testing.T) {
	ctx := context.Background()
	seed := func(t *testing.T) *fakeSongChartRepository {
		charts := newFakeSongChartRepository()
		for id, status := range map[string]domain.SongChartStatus{
			"amazing-grace": domain.SongChartPublished, "oh-susanna": domain.SongChartWithdrawn, "ciranda": domain.SongChartDraftStatus,
		} {
			require.NoError(t, charts.Create(ctx, domain.SongChart{ID: id, Status: status}))
		}
		return charts
	}
	newService := func(charts ports.SongChartRepository, references *fakePracticeReferenceWriter) *application.PracticeReferenceService {
		return application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), newFakeInstrumentRepository(), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills{}, charts, references)
	}

	t.Run("writes every published and withdrawn chart, never a draft", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()

		synced, err := newService(seed(t), references).SyncSongCharts(ctx)

		require.NoError(t, err)
		assert.Equal(t, 2, synced)
		assert.ElementsMatch(t, []string{"amazing-grace", "oh-susanna"}, references.songChartIDs())
	})

	t.Run("reports a failed write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")

		synced, err := newService(seed(t), references).SyncSongCharts(ctx)

		require.Error(t, err)
		assert.Equal(t, 0, synced)
	})

	t.Run("a failed listing of one status still writes the other, and is reported", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()

		synced, err := newService(failingPublishedCharts{seed(t)}, references).SyncSongCharts(ctx)

		require.Error(t, err)
		assert.Equal(t, 1, synced)
		assert.Equal(t, []string{"oh-susanna"}, references.songChartIDs())
	})
}

// failingPublishedCharts is a song chart repository whose listing of published
// charts always fails.
type failingPublishedCharts struct{ *fakeSongChartRepository }

func (f failingPublishedCharts) List(ctx context.Context, filter domain.SongChartFilter, page domain.PageRequest) (domain.Page[domain.SongChart], error) {
	if filter.Status != nil && *filter.Status == domain.SongChartPublished {
		return domain.Page[domain.SongChart]{}, errors.New("postgres down")
	}
	return f.fakeSongChartRepository.List(ctx, filter, page)
}

// failingExercises is an exercise repository whose listing always fails.
type failingExercises struct{ *fakeExerciseRepository }

func (failingExercises) List(context.Context, domain.ExerciseFilter, domain.PageRequest) (domain.Page[domain.Exercise], error) {
	return domain.Page[domain.Exercise]{}, errors.New("postgres down")
}

func TestPracticeReferenceService_Sync(t *testing.T) {
	ctx := context.Background()
	installed := []domain.DrillThreshold{{ID: "th-1", TemplateKey: "exercise:text_response", Version: 1,
		EffectiveFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), FluentNetMs: 6000, Source: domain.DrillThresholdSourceDefault}}

	t.Run("writes the fluent times even when another part fails", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), failingExercises{newFakeExerciseRepository()}, newFakeInstrumentRepository(), fakeDrillThresholds(installed), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.Sync(ctx)

		require.Error(t, err, "the failed part is still reported")
		assert.Equal(t, installed, references.drillThresholds(), "answers judged meanwhile need a fluent time, or they'd fold differently once rebuilt")
		assert.Equal(t, 1, synced.DrillThresholds)
	})

	t.Run("counts every part it wrote", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		require.NoError(t, exercises.Create(ctx, domain.Exercise{ID: "e-1", ExerciseType: domain.ExerciseTypeTextResponse, Options: textResponseOptions()}))
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), exercises, newFakeInstrumentRepository(), fakeDrillThresholds(installed), fakeProgressions(catalogProgressions()), fakeChordDrills{}, newFakeSongChartRepository(), newFakePracticeReferenceWriter())

		synced, err := svc.Sync(ctx)

		require.NoError(t, err)
		assert.Equal(t, application.PracticeReferenceSynced{Diagrams: 0, Exercises: 1, Instruments: 0, DrillThresholds: 1, Progressions: 2, SongCharts: 0}, synced)
	})
}

// The fretboard grader reads an instrument's strings and tuning only from the
// reference snapshot, so a created instrument writes its reference, and the
// sync on start covers the catalog instruments the migrations install. An
// update can't change strings or tuning, so it leaves the reference alone.
func TestInstrumentService_PracticeReference(t *testing.T) {
	ctx := context.Background()
	six := 6

	t.Run("creating an instrument writes its reference", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc := application.NewInstrumentService(newFakeInstrumentRepository(), newFakeVoiceRepository(), newFakeLanguageRepository(), references, idSequence())

		got, err := svc.CreateInstrument(ctx, teacherCaller(), bilingualGuitar, domain.InstrumentFamilyFretted, &six, guitarTuning, nil, "acoustic-guitar", nil)

		require.NoError(t, err)
		ref, ok := references.instrument(got.ID)
		require.True(t, ok)
		assert.Equal(t, domain.InstrumentReference{ID: got.ID, Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: guitarTuning}, ref)
	})

	t.Run("the reference is written even when the request ends right after the save", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		repo := newFakeInstrumentRepository()
		svc := application.NewInstrumentService(repo, newFakeVoiceRepository(), newFakeLanguageRepository(), references, idSequence())
		cancelled, cancel := context.WithCancel(ctx)
		cancel()

		got, err := svc.CreateInstrument(cancelled, teacherCaller(), bilingualGuitar, domain.InstrumentFamilyFretted, &six, guitarTuning, nil, "acoustic-guitar", nil)

		require.NoError(t, err)
		_, ok := references.instrument(got.ID)
		assert.True(t, ok)
	})

	t.Run("a failed reference write doesn't fail the saved instrument", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")
		repo := newFakeInstrumentRepository()
		svc := application.NewInstrumentService(repo, newFakeVoiceRepository(), newFakeLanguageRepository(), references, idSequence())

		got, err := svc.CreateInstrument(ctx, teacherCaller(), bilingualGuitar, domain.InstrumentFamilyFretted, &six, guitarTuning, nil, "acoustic-guitar", nil)

		require.NoError(t, err)
		_, err = repo.GetByID(ctx, got.ID)
		assert.NoError(t, err)
	})
}

func TestPracticeReferenceService_SyncInstruments(t *testing.T) {
	ctx := context.Background()
	six, four := 6, 4
	guitar := domain.Instrument{ID: "i-guitar", Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}}
	bass := domain.Instrument{ID: "i-bass", Family: domain.InstrumentFamilyFretted, StringCount: &four, Tuning: []string{"E1", "A1", "D2", "G2"}}
	seed := func(t *testing.T) *fakeInstrumentRepository {
		t.Helper()
		instruments := newFakeInstrumentRepository()
		for _, i := range []domain.Instrument{guitar, bass} {
			require.NoError(t, instruments.Create(ctx, i))
		}
		return instruments
	}

	t.Run("writes every instrument's reference in one write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), seed(t), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncInstruments(ctx)

		require.NoError(t, err)
		assert.Equal(t, 2, synced)
		assert.Equal(t, 1, references.writeCount())
		for _, i := range []domain.Instrument{guitar, bass} {
			ref, ok := references.instrument(i.ID)
			require.True(t, ok)
			assert.Equal(t, domain.NewInstrumentReference(i), ref)
		}
	})

	t.Run("reports a failed write", func(t *testing.T) {
		references := newFakePracticeReferenceWriter()
		references.err = errors.New("mongo down")
		svc := application.NewPracticeReferenceService(newFakeDiagramRepository(), newFakeExerciseRepository(), seed(t), fakeDrillThresholds(nil), fakeProgressions(nil), fakeChordDrills{}, newFakeSongChartRepository(), references)

		synced, err := svc.SyncInstruments(ctx)

		require.Error(t, err)
		assert.Equal(t, 0, synced)
	})
}
