//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// releaseFixture is a published path whose lesson has a published version and
// contains one exercise, inside a published course version.
type releaseFixture struct {
	store    *EntCommercialStore
	path     domain.LearningPath
	lesson   domain.ContentNode
	exercise domain.Exercise
	version  domain.ContentNodeVersion
	course   domain.Course
	courseV  domain.CourseVersion
}

func seedReleaseFixture(t *testing.T, ctx context.Context) releaseFixture {
	t.Helper()
	client, _ := setupMigratedPostgres(t)
	nodes := NewEntContentNodeRepository(client)
	paths := NewEntLearningPathRepository(client)
	exercises := NewEntExerciseRepository(client)

	lesson := seedContentNode(t, ctx, nodes)
	exercise := instrumentExercise("Shell voicings in C", seedSkill(t, ctx, client, "shells-"+uuid.NewString()))
	exercise.Options = []domain.Option{{ID: uuid.NewString(), Label: strPtr("C E B"), IsCorrect: true}, {ID: uuid.NewString(), Label: strPtr("C E G")}}
	require.NoError(t, exercises.Create(ctx, exercise, testOwnership(t, domain.ContentExercise, exercise.ID)))
	require.NoError(t, exercises.LinkContentNode(ctx, exercise.ID, lesson.ID))
	version := domain.ContentNodeVersion{
		ID: uuid.NewString(), ContentNodeID: lesson.ID, VersionNumber: 1, Title: lesson.Title,
		ContentType: lesson.ContentType, MediaURL: strPtr("https://cdn.example.com/v.mp4"),
		PublishedBy: uuid.NewString(), PublishedAt: fixedAt,
	}
	require.NoError(t, NewEntContentNodeVersionRepository(client).Create(ctx, version))

	path := seedLearningPath(t, ctx, paths, lesson)
	require.NoError(t, paths.UpdateStatus(ctx, path.ID, domain.LearningPathStatusPublished))

	course := domain.Course{
		ID: uuid.NewString(), CreatedBy: uuid.NewString(), Title: "Guitar from zero", Summary: "Start here.",
		Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusDraft, CreatedAt: fixedAt,
		Checkpoints: []domain.CourseCheckpoint{{Position: 1, LearningPathID: path.ID, EffectiveTitle: path.Title}},
	}
	require.NoError(t, NewEntCourseRepository(client).Create(ctx, course, testOwnership(t, domain.ContentCourse, course.ID)))
	courseV := domain.CourseVersion{
		ID: uuid.NewString(), CourseID: course.ID, VersionNumber: 1, TitleSnapshot: course.Title,
		SummarySnapshot: course.Summary, LevelSnapshot: course.Level, PublishedAt: fixedAt, AvailableForNewEnrollments: true,
		Checkpoints: []domain.CourseVersionCheckpoint{{Position: 1, LearningPathID: path.ID, EffectiveTitle: "First chords"}},
	}
	require.NoError(t, NewEntCourseVersionRepository(client).Create(ctx, courseV))

	return releaseFixture{
		store: NewEntCommercialStore(client), path: path, lesson: lesson, exercise: exercise,
		version: version, course: course, courseV: courseV,
	}
}

func snapshotOf(t *testing.T, ctx context.Context, store *EntCommercialStore, content domain.ContentRef) (domain.ReleaseSnapshot, error) {
	t.Helper()
	var snapshot domain.ReleaseSnapshot
	err := store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		var err error
		snapshot, err = tx.Releases().Snapshot(ctx, content)
		return err
	})
	return snapshot, err
}

func TestEntCommercialStore_ReleaseSnapshots(t *testing.T) {
	ctx := context.Background()
	f := seedReleaseFixture(t, ctx)
	pathRef := domain.ContentRef{Kind: domain.ContentPath, ID: f.path.ID}
	lessonRef := domain.ContentRef{Kind: domain.ContentContentNode, ID: f.lesson.ID}
	exerciseRef := domain.ContentRef{Kind: domain.ContentExercise, ID: f.exercise.ID}
	courseRef := domain.ContentRef{Kind: domain.ContentCourse, ID: f.course.ID}

	t.Run("a published path lists its lessons in order", func(t *testing.T) {
		snapshot, err := snapshotOf(t, ctx, f.store, pathRef)

		require.NoError(t, err)
		assert.True(t, snapshot.Published)
		assert.Nil(t, snapshot.VersionRef)
		assert.Equal(t, f.path.Title, snapshot.Title)
		assert.Equal(t, []domain.ContentRef{lessonRef}, snapshot.Children)
		state, ok := snapshot.State.(domain.PathReleaseState)
		require.True(t, ok)
		assert.Equal(t, []domain.PathReleaseItem{{LessonID: f.lesson.ID}}, state.Items)
	})

	t.Run("a lesson is released at its latest published version with its exercises", func(t *testing.T) {
		snapshot, err := snapshotOf(t, ctx, f.store, lessonRef)

		require.NoError(t, err)
		assert.True(t, snapshot.Published)
		require.NotNil(t, snapshot.VersionRef)
		assert.Equal(t, f.version.ID, *snapshot.VersionRef)
		assert.Equal(t, []domain.ContentRef{exerciseRef}, snapshot.Children)
	})

	t.Run("an exercise covers its content and answer key", func(t *testing.T) {
		snapshot, err := snapshotOf(t, ctx, f.store, exerciseRef)

		require.NoError(t, err)
		assert.True(t, snapshot.Published)
		assert.Empty(t, snapshot.Children)
		state, ok := snapshot.State.(domain.ExerciseReleaseState)
		require.True(t, ok)
		assert.Equal(t, "Shell voicings in C", state.Title)
		require.Len(t, state.Options, 2)
		correct := 0
		for _, option := range state.Options {
			if option.IsCorrect {
				correct++
			}
		}
		assert.Equal(t, 1, correct)
	})

	t.Run("a course is released at its latest published version with its paths", func(t *testing.T) {
		snapshot, err := snapshotOf(t, ctx, f.store, courseRef)

		require.NoError(t, err)
		assert.True(t, snapshot.Published)
		require.NotNil(t, snapshot.VersionRef)
		assert.Equal(t, f.courseV.ID, *snapshot.VersionRef)
		assert.Equal(t, []domain.ContentRef{pathRef}, snapshot.Children)
	})

	t.Run("a draft path and a lesson without a published version are not published", func(t *testing.T) {
		client := f.store.client
		draft := seedLearningPath(t, ctx, NewEntLearningPathRepository(client), f.lesson)
		unpublished := seedContentNode(t, ctx, NewEntContentNodeRepository(client))

		path, err := snapshotOf(t, ctx, f.store, domain.ContentRef{Kind: domain.ContentPath, ID: draft.ID})
		require.NoError(t, err)
		lesson, err := snapshotOf(t, ctx, f.store, domain.ContentRef{Kind: domain.ContentContentNode, ID: unpublished.ID})
		require.NoError(t, err)

		assert.False(t, path.Published)
		assert.False(t, lesson.Published)
		assert.Nil(t, lesson.VersionRef)
	})

	t.Run("a deleted path is not found", func(t *testing.T) {
		paths := NewEntLearningPathRepository(f.store.client)
		gone := seedLearningPath(t, ctx, paths, f.lesson)
		require.NoError(t, paths.Delete(ctx, gone.ID))

		_, err := snapshotOf(t, ctx, f.store, domain.ContentRef{Kind: domain.ContentPath, ID: gone.ID})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestEntCommercialStore_Releases(t *testing.T) {
	ctx := context.Background()
	client, db := setupMigratedPostgres(t)
	store := NewEntCommercialStore(client)
	path := domain.ContentRef{Kind: domain.ContentPath, ID: uuid.NewString()}
	lesson := domain.ContentRef{Kind: domain.ContentContentNode, ID: uuid.NewString()}
	ownerID := uuid.NewString()
	version := "lesson-version"
	release := domain.CommercialRelease{
		ReleaseID: uuid.NewString(), Content: path, Revision: 1,
		ManifestSHA256: "a1", ContentSHA256: "c1", CoreVersionRef: &version,
		Ownership: domain.ContentOwnership{
			Content: path, OwnerKind: domain.OwnerTeacher, OwnerID: &ownerID, CreatorID: &ownerID,
			Source: domain.SourceNative, EvidenceEntryID: uuid.NewString(), Revision: 1,
		},
		Dependencies: []domain.ReleaseDependency{{
			Content: lesson, ContentSHA256: "d1", ReleaseID: uuid.NewString(), Permission: domain.PermissionOwn, EvidenceEntryID: uuid.NewString(),
		}},
		EvidenceEntryID: uuid.NewString(), CreatedAt: fixedAt.UTC(),
	}

	t.Run("an item never released is at commercial revision 1 with no release", func(t *testing.T) {
		state, err := store.CommercialState(ctx, path)

		require.NoError(t, err)
		assert.Equal(t, domain.NewCommercialState(path), state)
	})

	t.Run("a stored release reads back unchanged and advances the item's state", func(t *testing.T) {
		err := store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			states, err := tx.Releases().LockStates(ctx, []domain.ContentRef{path, lesson})
			if err != nil {
				return err
			}
			assert.Equal(t, []domain.CommercialState{domain.NewCommercialState(path), domain.NewCommercialState(lesson)}, states)
			if err := tx.Releases().Insert(ctx, release); err != nil {
				return err
			}
			return tx.Releases().SaveState(ctx, states[0].AdvancedTo(release.ReleaseID))
		})
		require.NoError(t, err)

		stored, err := store.Release(ctx, release.ReleaseID)
		require.NoError(t, err)
		assert.Equal(t, release, stored)
		state, err := store.CommercialState(ctx, path)
		require.NoError(t, err)
		assert.Equal(t, 2, state.Revision)
		assert.Equal(t, release.ReleaseID, *state.LatestReleaseID)
	})

	t.Run("a release revision already stored for the item is a conflict", func(t *testing.T) {
		duplicate := release
		duplicate.ReleaseID = uuid.NewString()

		err := store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			return tx.Releases().Insert(ctx, duplicate)
		})

		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("a stored release cannot be changed or removed", func(t *testing.T) {
		_, updateErr := db.ExecContext(ctx, `UPDATE commercial_releases SET content_sha256 = 'x' WHERE id = $1`, release.ReleaseID)
		_, deleteErr := db.ExecContext(ctx, `DELETE FROM commercial_releases WHERE id = $1`, release.ReleaseID)

		require.Error(t, updateErr)
		assert.Contains(t, updateErr.Error(), "append-only")
		require.Error(t, deleteErr)
	})

	t.Run("an unknown release is not found", func(t *testing.T) {
		_, err := store.Release(ctx, uuid.NewString())

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a second publication waits for the first to release its lock", func(t *testing.T) {
		locked := make(chan struct{})
		release := make(chan struct{})
		firstDone := make(chan error, 1)
		go func() {
			firstDone <- store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
				if _, err := tx.Releases().LockStates(ctx, []domain.ContentRef{lesson}); err != nil {
					return err
				}
				close(locked)
				<-release
				return nil
			})
		}()
		<-locked
		secondDone := make(chan error, 1)
		go func() {
			secondDone <- store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
				_, err := tx.Releases().LockStates(ctx, []domain.ContentRef{lesson})
				return err
			})
		}()

		select {
		case err := <-secondDone:
			t.Fatalf("the second lock did not wait: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		close(release)
		require.NoError(t, <-firstDone)
		require.NoError(t, <-secondDone)
	})
}
