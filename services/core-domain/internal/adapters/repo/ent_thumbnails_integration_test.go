//go:build integration

package repo

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestThumbnails_Repositories(t *testing.T) {
	f := newCourseListFixture(t)
	nodeVersions := NewEntContentNodeVersionRepository(f.nodes.client)
	enrollments := NewEntCourseEnrollmentRepository(f.nodes.client)
	thumbnail := func(name string) *string {
		url := "https://cdn.motifpath.io/thumbnails/" + name + ".png"
		return &url
	}
	path := f.pathWith(nil, nil)

	t.Run("a course keeps its thumbnail, loses it on a replace without one, and versions snapshot it", func(t *testing.T) {
		c := domain.Course{
			ID: uuid.NewString(), CreatedBy: uuid.NewString(), Title: "Fingerstyle", Summary: "S", Level: domain.DifficultyLevelBeginner,
			Language: "en", ThumbnailURL: thumbnail("course"), Status: domain.CourseStatusDraft, CreatedAt: fixedAt, Checkpoints: checkpointsOf(path),
		}
		require.NoError(t, f.courses.Create(f.ctx, c))
		require.NoError(t, f.versions.Create(f.ctx, domain.NewCourseVersionSnapshot(uuid.NewString(), c, 1, fixedAt)))

		got, err := f.courses.GetByID(f.ctx, c.ID)
		require.NoError(t, err)
		assert.Equal(t, c.ThumbnailURL, got.ThumbnailURL)
		version, err := f.versions.GetLatestByCourseID(f.ctx, c.ID)
		require.NoError(t, err)
		assert.Equal(t, c.ThumbnailURL, version.ThumbnailURLSnapshot)

		c.ThumbnailURL = nil
		require.NoError(t, f.courses.Replace(f.ctx, c))
		got, err = f.courses.GetByID(f.ctx, c.ID)
		require.NoError(t, err)
		assert.Nil(t, got.ThumbnailURL)

		enrollment := domain.CourseEnrollment{
			ID: uuid.NewString(), StudentID: uuid.NewString(), CourseID: c.ID, CourseTitle: c.Title, CourseThumbnailURL: version.ThumbnailURLSnapshot,
			CourseVersionNumber: 1, Status: domain.CourseEnrollmentStatusActive, EnrolledAt: fixedAt,
		}
		require.NoError(t, enrollments.Create(f.ctx, enrollment))
		stored, err := enrollments.GetByID(f.ctx, enrollment.ID)
		require.NoError(t, err)
		assert.Equal(t, version.ThumbnailURLSnapshot, stored.CourseThumbnailURL)
	})

	t.Run("a learning path keeps its thumbnail and loses it on a replace without one", func(t *testing.T) {
		level := domain.DifficultyLevelBeginner
		p := path
		p.Level, p.ThumbnailURL, p.UpdatedAt = &level, thumbnail("path"), fixedAt
		require.NoError(t, f.paths.Replace(f.ctx, p))
		got, err := f.paths.GetByID(f.ctx, p.ID)
		require.NoError(t, err)
		assert.Equal(t, p.ThumbnailURL, got.ThumbnailURL)

		p.ThumbnailURL = nil
		require.NoError(t, f.paths.Replace(f.ctx, p))
		got, err = f.paths.GetByID(f.ctx, p.ID)
		require.NoError(t, err)
		assert.Nil(t, got.ThumbnailURL)
	})

	t.Run("a content node keeps its thumbnail, loses it on an update without one, and versions snapshot it", func(t *testing.T) {
		node := seedContentNode(t, f.ctx, f.nodes)
		node.ThumbnailURL = thumbnail("node")
		require.NoError(t, f.nodes.Update(f.ctx, node))
		got, err := f.nodes.GetByID(f.ctx, node.ID)
		require.NoError(t, err)
		assert.Equal(t, node.ThumbnailURL, got.ThumbnailURL)
		require.NoError(t, nodeVersions.Create(f.ctx, domain.NewContentNodeVersionSnapshot(uuid.NewString(), got, 1, uuid.NewString(), fixedAt)))
		latest, err := nodeVersions.GetLatestByContentNodeID(f.ctx, node.ID)
		require.NoError(t, err)
		assert.Equal(t, node.ThumbnailURL, latest.ThumbnailURLSnapshot)

		got.ThumbnailURL = nil
		require.NoError(t, f.nodes.Update(f.ctx, got))
		again, err := f.nodes.GetByID(f.ctx, node.ID)
		require.NoError(t, err)
		assert.Nil(t, again.ThumbnailURL)
	})
}
