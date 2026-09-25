package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestInstrumentIDs(t *testing.T) {
	at := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	course := func(ids []string) (domain.Course, error) {
		return domain.NewCourse("course-1", "teacher-1", domain.CourseFields{
			Title: "Blues Rhythm", Summary: "S", Level: domain.DifficultyLevelBeginner, Language: "en",
			InstrumentIDs: ids, Checkpoints: []domain.NewCourseCheckpoint{{Path: openChordsPath()}},
		}, offered, at)
	}
	path := func(ids []string) (domain.LearningPath, error) {
		return domain.NewLearningPath("path-1", "teacher-1", domain.LearningPathFields{
			Title: "Open Chords", Level: domain.DifficultyLevelBeginner, InstrumentIDs: ids,
			Items: []domain.NewLearningPathItem{{Node: domain.ContentNode{ID: "node-1", Title: "One", ContentType: domain.ContentTypeVideo}}},
		}, at)
	}
	node := func(ids []string) (domain.ContentNode, error) {
		media := "https://cdn.motifpath.io/v.mp4"
		return domain.NewContentNode("node-1", "teacher-1", domain.ContentTypeVideo, domain.ContentNodeFields{
			Title: "Barre Chords", SkillIDs: []string{"s"}, ConceptIDs: []string{"c"}, Difficulty: domain.DifficultyLevelBeginner,
			LanguageCodes: []string{"en"}, MediaURL: &media, InstrumentIDs: ids,
		}, at)
	}

	t.Run("courses, paths and content nodes keep the instruments they are for", func(t *testing.T) {
		ids := []string{"guitar", "electric-guitar"}

		c, err := course(ids)
		require.NoError(t, err)
		p, err := path(ids)
		require.NoError(t, err)
		n, err := node(ids)
		require.NoError(t, err)

		assert.Equal(t, ids, c.InstrumentIDs)
		assert.Equal(t, ids, p.InstrumentIDs)
		assert.Equal(t, ids, n.InstrumentIDs)
	})

	t.Run("no instruments means every instrument", func(t *testing.T) {
		c, err := course(nil)
		require.NoError(t, err)

		assert.Empty(t, c.InstrumentIDs)
	})

	t.Run("an instrument listed twice is rejected on every kind of item", func(t *testing.T) {
		twice := []string{"guitar", "guitar"}
		_, courseErr := course(twice)
		_, pathErr := path(twice)
		_, nodeErr := node(twice)

		for _, err := range []error{courseErr, pathErr, nodeErr} {
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "instrument_ids", valErr.Fields[0].Field)
		}
	})

	t.Run("updating a content node replaces its instruments", func(t *testing.T) {
		n, err := node([]string{"guitar"})
		require.NoError(t, err)
		media := "https://cdn.motifpath.io/v.mp4"

		updated, err := n.Update(domain.ContentNodeFields{
			Title: "Barre Chords", SkillIDs: []string{"s"}, ConceptIDs: []string{"c"}, Difficulty: domain.DifficultyLevelBeginner,
			LanguageCodes: []string{"en"}, MediaURL: &media, InstrumentIDs: []string{"piano"},
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"piano"}, updated.InstrumentIDs)
	})

	t.Run("course and content node versions snapshot the instruments", func(t *testing.T) {
		c, err := course([]string{"guitar"})
		require.NoError(t, err)
		n, err := node([]string{"piano"})
		require.NoError(t, err)

		assert.Equal(t, []string{"guitar"}, domain.NewCourseVersionSnapshot("v-1", c, 1, at).InstrumentIDsSnapshot)
		assert.Equal(t, []string{"piano"}, domain.NewContentNodeVersionSnapshot("v-2", n, 1, "teacher-1", at).InstrumentIDsSnapshot)
	})
}
