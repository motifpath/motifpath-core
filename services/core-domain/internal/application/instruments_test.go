package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// seededInstrumentRepository holds a guitar and a piano, the instruments the
// instrument-tagging tests reference.
func seededInstrumentRepository() *fakeInstrumentRepository {
	instruments := newFakeInstrumentRepository()
	for _, id := range []string{"guitar", "piano"} {
		instruments.byID[id] = domain.Instrument{ID: id, Names: domain.LocalizedText{"en": id}}
	}
	return instruments
}

func TestInstrumentTagging(t *testing.T) {
	ctx := context.Background()
	nodes := newFakeContentNodeRepository()
	nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
	paths := newFakeLearningPathRepository()
	paths.put(domain.LearningPath{ID: "path-01", Title: "Open Chords"})
	media := "https://cdn.motifpath.io/v.mp4"

	createCourse := func(ids []string) (domain.Course, error) {
		return newCourseService(paths, newFakeCourseRepository()).CreateCourse(ctx, teacherCaller(), application.CourseInput{
			Title: "Blues Rhythm", Summary: "S", Level: domain.DifficultyLevelBeginner, Language: "en",
			InstrumentIDs: ids, Checkpoints: checkpointInputs("path-01"),
		})
	}
	createPath := func(ids []string) (domain.LearningPath, error) {
		return newLearningPathService(nodes, newFakeLearningPathRepository()).CreateLearningPath(ctx, teacherCaller(), application.LearningPathInput{
			Title: "Open Chords", Level: domain.DifficultyLevelBeginner, InstrumentIDs: ids, Items: pathItems("node-01"),
		})
	}
	nodeInput := func(ids []string) application.ContentNodeInput {
		return application.ContentNodeInput{
			Title: "Barre Chords", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"},
			Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: &media, InstrumentIDs: ids,
		}
	}

	t.Run("courses, paths and content nodes are created for the instruments given", func(t *testing.T) {
		course, err := createCourse([]string{"guitar"})
		require.NoError(t, err)
		path, err := createPath([]string{"guitar", "piano"})
		require.NoError(t, err)
		node, err := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository()).CreateContentNode(ctx, teacherCaller(), nodeInput([]string{"piano"}))
		require.NoError(t, err)

		assert.Equal(t, []string{"guitar"}, course.InstrumentIDs)
		assert.Equal(t, []string{"guitar", "piano"}, path.InstrumentIDs)
		assert.Equal(t, []string{"piano"}, node.InstrumentIDs)
	})

	t.Run("an instrument that does not exist is rejected on every kind of item, create and update alike", func(t *testing.T) {
		unknown := []string{"banjo"}
		_, courseErr := createCourse(unknown)
		_, pathErr := createPath(unknown)
		contents := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())
		_, createNodeErr := contents.CreateContentNode(ctx, teacherCaller(), nodeInput(unknown))
		node, err := contents.CreateContentNode(ctx, teacherCaller(), nodeInput(nil))
		require.NoError(t, err)
		_, updateNodeErr := contents.UpdateContentNode(ctx, teacherCaller(), node.ID, nodeInput(unknown))

		for _, err := range []error{courseErr, pathErr, createNodeErr, updateNodeErr} {
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "instrument_ids", valErr.Fields[0].Field)
		}
	})
}
