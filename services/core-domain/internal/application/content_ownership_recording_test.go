package application_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

func testOwnershipRecorder(t *testing.T) *application.OwnershipRecorder {
	t.Helper()
	environment, err := domain.ParseEnvironment("test")
	require.NoError(t, err)
	return application.NewOwnershipRecorder(environment, idSequence())
}

// assertNativeOwnership checks that a record names author's content with the
// expected owner, and that its evidence is an applied entry for that same
// content, attributed to the author, describing the recorded ownership.
func assertNativeOwnership(t *testing.T, record ports.OwnershipRecord, content domain.ContentRef, author domain.User, wantOwnerKind domain.OwnerKind, wantOwnerID string) {
	t.Helper()
	ownership := record.Ownership
	assert.Equal(t, content, ownership.Content)
	assert.Equal(t, wantOwnerKind, ownership.OwnerKind)
	require.NotNil(t, ownership.OwnerID)
	assert.Equal(t, wantOwnerID, *ownership.OwnerID)
	require.NotNil(t, ownership.CreatorID)
	assert.Equal(t, author.ID, *ownership.CreatorID)
	assert.Equal(t, domain.SourceNative, ownership.Source)
	assert.Equal(t, 1, ownership.Revision)

	evidence := record.Evidence
	assert.Equal(t, ownership.EvidenceEntryID, evidence.AuditEntryID)
	assert.Equal(t, "test", evidence.Operation.Environment.String())
	assert.Equal(t, domain.AuditActorUser, evidence.Operation.ActorKind)
	assert.Equal(t, author.ID, evidence.Operation.ActorID)
	assert.Equal(t, application.ActionRecordContentOwnership, evidence.Operation.Action)
	assert.Equal(t, string(content.Kind), evidence.Operation.ResourceKind)
	assert.Equal(t, content.ID, evidence.Operation.ResourceID)
	assert.Equal(t, domain.AuditOutcomeApplied, evidence.Outcome)
	assert.Equal(t, fixedCreatedAt, evidence.OccurredAt, "evidence is dated when the content was created")
	assert.Nil(t, evidence.BeforeState, "nothing was owned before creation")

	var after map[string]any
	require.NoError(t, json.Unmarshal(evidence.AfterState, &after))
	assert.Equal(t, string(wantOwnerKind), after["owner_kind"])
	assert.Equal(t, wantOwnerID, after["owner_id"])
	assert.Equal(t, author.ID, after["creator_id"])
	assert.Equal(t, "native", after["source"])
	assert.EqualValues(t, 1, after["revision"])
}

func TestContentCreationRecordsOwnership(t *testing.T) {
	authors := map[string]struct {
		author        domain.User
		wantOwnerKind domain.OwnerKind
		wantOwnerID   string
	}{
		"an admin's content is owned by the platform":  {adminCaller(), domain.OwnerPlatform, domain.CatalogProfileID},
		"a teacher's content is owned by that teacher": {teacherCaller(), domain.OwnerTeacher, teacherCaller().ID},
	}

	for name, tc := range authors {
		t.Run("path: "+name, func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
			paths := newFakeLearningPathRepository()
			svc := newLearningPathService(nodes, paths)

			path, err := svc.CreateLearningPath(context.Background(), tc.author, application.LearningPathInput{
				Level: domain.DifficultyLevelBeginner, Title: "Jazz foundations", Items: pathItems("node-01"),
			})

			require.NoError(t, err)
			assertNativeOwnership(t, paths.ownership[path.ID], domain.ContentRef{Kind: domain.ContentPath, ID: path.ID}, tc.author, tc.wantOwnerKind, tc.wantOwnerID)
		})

		t.Run("content node: "+name, func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			svc := newContentService(nodes, newFakeExpandedContentRepository())

			node, err := svc.CreateContentNode(context.Background(), tc.author, application.ContentNodeInput{
				Title: "Shell voicings", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"},
				Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: videoMediaURL(),
			})

			require.NoError(t, err)
			assertNativeOwnership(t, nodes.ownership[node.ID], domain.ContentRef{Kind: domain.ContentContentNode, ID: node.ID}, tc.author, tc.wantOwnerKind, tc.wantOwnerID)
		})

		t.Run("exercise: "+name, func(t *testing.T) {
			exercises := newFakeExerciseRepository()
			svc := newExerciseService(newFakeChallengeRepository(), exercises)
			imageURL := "https://cdn.example.com/fretboard/c-major-triad.png"

			exercise, err := svc.CreateExercise(context.Background(), tc.author,
				"Root position of a C major triad", domain.NewPlainTextPrompt("Identify the root position of a C major triad"),
				domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, &imageURL, nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"}, nil)

			require.NoError(t, err)
			assertNativeOwnership(t, exercises.ownership[exercise.ID], domain.ContentRef{Kind: domain.ContentExercise, ID: exercise.ID}, tc.author, tc.wantOwnerKind, tc.wantOwnerID)
		})

		t.Run("course: "+name, func(t *testing.T) {
			paths := newFakeLearningPathRepository()
			paths.put(domain.LearningPath{Status: domain.LearningPathStatusPublished, ID: "own-path", TeacherID: tc.author.ID, Title: "Mine"})
			courses := newFakeCourseRepository()
			users := newFakeUserRepository()
			users.put(tc.author)
			svc := application.NewCourseService(testOwnershipRecorder(t), paths, courses, newFakeCourseVersionRepository(), users, newFakeLanguageRepository(), seededInstrumentRepository(), seededKnowledgeNodeRepository(), idSequence(), func() time.Time { return fixedCreatedAt })

			course, err := svc.CreateCourse(context.Background(), tc.author, application.CourseInput{
				Language: "en", Title: "Journey", Summary: "Summary", Level: domain.DifficultyLevelBeginner, Checkpoints: checkpointInputs("own-path"),
			})

			require.NoError(t, err)
			assertNativeOwnership(t, courses.ownership[course.ID], domain.ContentRef{Kind: domain.ContentCourse, ID: course.ID}, tc.author, tc.wantOwnerKind, tc.wantOwnerID)
		})
	}
}

// ownershipRecorderForTests records ownership evidence in the test
// environment, for services built outside a single test.
func ownershipRecorderForTests() *application.OwnershipRecorder {
	environment, err := domain.ParseEnvironment("test")
	if err != nil {
		panic(fmt.Sprintf("the test environment is invalid: %v", err))
	}
	return application.NewOwnershipRecorder(environment, idSequence())
}
