//go:build integration

package repo

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestInstrumentTagging_Repositories(t *testing.T) {
	f := newCourseListFixture(t)
	instruments := NewEntInstrumentRepository(f.nodes.client)
	nodeVersions := NewEntContentNodeVersionRepository(f.nodes.client)
	guitar, piano := frettedInstrument(), keyboardInstrument()
	require.NoError(t, instruments.Create(f.ctx, guitar))
	require.NoError(t, instruments.Create(f.ctx, piano))
	path := f.pathWith(nil, nil)

	course := func(title string, ids ...string) domain.Course {
		c := domain.Course{
			ID: uuid.NewString(), CreatedBy: uuid.NewString(), Title: title, Summary: "S", Level: domain.DifficultyLevelBeginner,
			Language: "en", InstrumentIDs: ids, Status: domain.CourseStatusDraft, CreatedAt: fixedAt, Checkpoints: checkpointsOf(path),
		}
		require.NoError(t, f.courses.Create(f.ctx, c))
		return c
	}
	publishWith := func(c domain.Course, ids ...string) {
		require.NoError(t, f.versions.Create(f.ctx, domain.CourseVersion{
			ID: uuid.NewString(), CourseID: c.ID, VersionNumber: 1, TitleSnapshot: c.Title, SummarySnapshot: "S",
			LevelSnapshot: domain.DifficultyLevelBeginner, LanguageSnapshot: "en", InstrumentIDsSnapshot: ids,
			Checkpoints: versionCheckpointsOf(path), PublishedAt: fixedAt, AvailableForNewEnrollments: true,
		}))
		require.NoError(t, f.courses.UpdateStatus(f.ctx, c.ID, domain.CourseStatusPublished))
	}
	guitarCourse, theoryCourse, pianoCourse := course("Guitar", guitar.ID), course("Theory"), course("Piano", piano.ID)

	t.Run("a course keeps its instruments through create and replace", func(t *testing.T) {
		got, err := f.courses.GetByID(f.ctx, guitarCourse.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{guitar.ID}, got.InstrumentIDs)

		guitarCourse.InstrumentIDs = []string{guitar.ID, piano.ID}
		require.NoError(t, f.courses.Replace(f.ctx, guitarCourse))
		got, err = f.courses.GetByID(f.ctx, guitarCourse.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{guitar.ID, piano.ID}, got.InstrumentIDs)

		guitarCourse.InstrumentIDs = []string{guitar.ID}
		require.NoError(t, f.courses.Replace(f.ctx, guitarCourse))
	})

	t.Run("the authoring list keeps courses for the instrument and for every instrument", func(t *testing.T) {
		page := f.list(domain.CourseListFilter{InstrumentID: guitar.ID}, domain.PageRequest{Limit: 50})

		assert.ElementsMatch(t, []string{guitarCourse.ID, theoryCourse.ID}, courseIDs(page))
	})

	t.Run("the catalog filters on the published version's instruments, not the draft's", func(t *testing.T) {
		publishWith(guitarCourse, guitar.ID)
		publishWith(theoryCourse)
		publishWith(pianoCourse, piano.ID)
		// The piano course's draft moves to guitar, but its published version stays piano.
		pianoCourse.InstrumentIDs = []string{guitar.ID}
		require.NoError(t, f.courses.Replace(f.ctx, pianoCourse))

		page := f.list(domain.CourseListFilter{InstrumentID: guitar.ID, PublishedView: true}, domain.PageRequest{Limit: 50})

		assert.ElementsMatch(t, []string{guitarCourse.ID, theoryCourse.ID}, courseIDs(page))
		version, err := f.versions.GetLatestByCourseID(f.ctx, pianoCourse.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{piano.ID}, version.InstrumentIDsSnapshot)
	})

	t.Run("learning paths keep their instruments and filter the same way", func(t *testing.T) {
		node := seedContentNode(t, f.ctx, f.nodes)
		pathFor := func(ids ...string) domain.LearningPath {
			level := domain.DifficultyLevelBeginner
			p := domain.LearningPath{
				ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Path " + uuid.NewString(), Level: &level, InstrumentIDs: ids,
				Items:     []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
				CreatedAt: fixedAt, UpdatedAt: fixedAt,
			}
			require.NoError(t, f.paths.Create(f.ctx, p))
			return p
		}
		guitarPath, everyPath, pianoPath := pathFor(guitar.ID), pathFor(), pathFor(piano.ID)

		got, err := f.paths.GetByID(f.ctx, guitarPath.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{guitar.ID}, got.InstrumentIDs)
		page, err := f.paths.List(f.ctx, domain.LearningPathFilter{InstrumentID: guitar.ID}, domain.PageRequest{Limit: 50})
		require.NoError(t, err)
		var ids []string
		for _, p := range page.Items {
			ids = append(ids, p.ID)
		}
		assert.Contains(t, ids, guitarPath.ID)
		assert.Contains(t, ids, everyPath.ID)
		assert.NotContains(t, ids, pianoPath.ID)
	})

	t.Run("content nodes keep, update and filter on their instruments, and versions snapshot them", func(t *testing.T) {
		node := seedContentNode(t, f.ctx, f.nodes)
		node.InstrumentIDs = []string{piano.ID}
		require.NoError(t, f.nodes.Update(f.ctx, node))
		got, err := f.nodes.GetByID(f.ctx, node.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{piano.ID}, got.InstrumentIDs)

		page, err := f.nodes.List(f.ctx, domain.ContentNodeFilter{InstrumentID: guitar.ID}, domain.PageRequest{Limit: 100})
		require.NoError(t, err)
		for _, n := range page.Items {
			assert.NotEqual(t, node.ID, n.ID, "a piano-only node is not listed for guitar")
		}

		require.NoError(t, nodeVersions.Create(f.ctx, domain.NewContentNodeVersionSnapshot(uuid.NewString(), got, 1, uuid.NewString(), fixedAt)))
		latest, err := nodeVersions.GetLatestByContentNodeID(f.ctx, node.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{piano.ID}, latest.InstrumentIDsSnapshot)
	})
}
