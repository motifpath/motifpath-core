//go:build integration

// Package pipeline exercises the Core Domain Service end to end — real
// Postgres via ent, real MongoDB completion state, real application
// services — matching the Phase 4.8 validation criteria: create node →
// create challenge → create learning path → assign to student → get
// student path, and that replacing an active assignment resets progress
// rather than carrying it over.
package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	_ "github.com/lib/pq"

	"github.com/motifpath/core-domain/internal/adapters/repo"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

type pipeline struct {
	content     *application.ContentService
	challenge   *application.ChallengeService
	path        *application.LearningPathService
	studentPath *application.StudentPathService
	skills      *application.SkillService
	concepts    *application.ConceptService
	users       *repo.EntUserRepository
	mongoDB     *mongo.Database
}

func setupPipeline(t *testing.T) *pipeline {
	t.Helper()
	ctx := context.Background()

	// Postgres and Mongo are one shared container each, waited out once in
	// TestMain; every test gets its own fresh database off them.
	entClient, err := ent.Open("postgres", newPostgresDSN(t))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, entClient.Close()) })
	require.NoError(t, entClient.Schema.Create(ctx))

	// entClient.Schema.Create only creates DDL — the en/pt_BR/any system rows
	// normally seeded by the Atlas migration itself (see ADR-024) don't exist
	// here, so this pipeline seeds the one row ("en") its own User/ContentNode
	// creation needs.
	_, err = entClient.Language.Create().SetCode("en").SetName("English").Save(ctx)
	require.NoError(t, err)

	mongoDB := mongoDatabase(t)

	newID := uuid.NewString
	now := func() time.Time { return time.Now().UTC() }

	nodes := repo.NewEntContentNodeRepository(entClient)
	challenges := repo.NewEntChallengeRepository(entClient)
	exercises := repo.NewEntExerciseRepository(entClient)
	expanded := repo.NewEntExpandedContentRepository(entClient)
	paths := repo.NewEntLearningPathRepository(entClient)
	studentPaths := repo.NewEntStudentPathRepository(entClient)
	versions := repo.NewEntContentNodeVersionRepository(entClient)
	learningState := repo.NewEntStudentLearningStateRepository(entClient)
	courseEnrollments := repo.NewEntCourseEnrollmentRepository(entClient)
	courseVersions := repo.NewEntCourseVersionRepository(entClient)
	users := repo.NewEntUserRepository(entClient)
	completion := repo.NewMongoCompletionStateReader(mongoDB)
	skillRepo := repo.NewEntSkillRepository(entClient)
	conceptRepo := repo.NewEntConceptRepository(entClient)

	return &pipeline{
		content:     application.NewContentService(nodes, expanded, skillRepo, conceptRepo, versions, newID, now),
		challenge:   application.NewChallengeService(nodes, challenges, exercises, newID, now),
		path:        application.NewLearningPathService(nodes, paths, courseVersions, newID, now),
		studentPath: application.NewStudentPathService(users, paths, studentPaths, versions, learningState, courseEnrollments, courseVersions, nodes, exercises, completion, newID, now),
		skills:      application.NewSkillService(skillRepo, newID),
		concepts:    application.NewConceptService(conceptRepo, newID),
		users:       users,
		mongoDB:     mongoDB,
	}
}

// seedClassification creates a fresh root Skill and Concept (owned by
// teacher) for a test's content node/challenge to reference — the pipeline
// exercises real membership validation end to end, so ids must reference
// real rows, not arbitrary uuids.
func seedClassification(t *testing.T, ctx context.Context, p *pipeline, teacher domain.User, name string) (skillID, conceptID string) {
	t.Helper()
	skill, err := p.skills.CreateSkill(ctx, teacher, name+"-skill-"+uuid.NewString(), nil)
	require.NoError(t, err)
	concept, err := p.concepts.CreateConcept(ctx, teacher, name+"-concept-"+uuid.NewString(), nil)
	require.NoError(t, err)
	return skill.ID, concept.ID
}

func TestCoreDomainPipeline_CreateAssignAndViewPath(t *testing.T) {
	p := setupPipeline(t)
	ctx := context.Background()
	teacher := domain.User{ID: uuid.NewString(), Role: domain.RoleTeacher}
	student := domain.User{ID: uuid.NewString(), Role: domain.RoleStudent, Locale: domain.Language{Code: "en"}}

	skillID, conceptID := seedClassification(t, ctx, p, teacher, "triads")
	node, err := p.content.CreateContentNode(ctx, teacher, "Intro to Triads", domain.ContentTypeVideo, []string{skillID}, []string{conceptID}, domain.DifficultyLevelBeginner, []string{"en"}, testVideoURL(), nil)
	require.NoError(t, err)

	challenge, err := p.challenge.CreateChallenge(ctx, teacher, node.ID, &skillID, nil, 70, nil, false, false)
	require.NoError(t, err)
	assert.Equal(t, node.ID, challenge.ContentNodeID)

	learningPath, err := p.path.CreateLearningPath(ctx, teacher, "Week 1", []application.PathItemInput{{ContentNodeID: node.ID}})
	require.NoError(t, err)

	// AssignLearningPath needs the student to exist in the same Postgres
	// database as a User row — seed it directly via the ent repo rather
	// than through IdentityService, which isn't part of this pipeline.
	seedStudentInto(t, ctx, p, student)

	// Every content node a learning path's items reference must already
	// have a published version before it can be copied into a StudentPath.
	_, err = p.content.PublishContentNode(ctx, teacher, node.ID)
	require.NoError(t, err)

	studentPath, err := p.studentPath.AssignLearningPath(ctx, teacher, student.ID, learningPath.ID)
	require.NoError(t, err)
	assert.Equal(t, student.ID, studentPath.StudentID)

	// Simulate the Aggregation Worker (ADR-011) marking this node completed.
	_, err = p.mongoDB.Collection("aggregates").InsertOne(ctx, bson.D{
		{Key: "student_id", Value: student.ID},
		{Key: "content_node_id", Value: node.ID},
		{Key: "status", Value: "completed"},
		{Key: "updated_at", Value: time.Now().UTC()},
	})
	require.NoError(t, err)

	view, err := p.studentPath.GetMyPath(ctx, student)
	require.NoError(t, err)
	require.Len(t, view.Items, 1)
	assert.Equal(t, domain.CompletionStatusCompleted, view.Items[0].Status)
	assert.Equal(t, 1, view.CurrentPosition)
}

func TestCoreDomainPipeline_AssigningANewPathIsAdditiveAndMovesCurrent(t *testing.T) {
	p := setupPipeline(t)
	ctx := context.Background()
	teacher := domain.User{ID: uuid.NewString(), Role: domain.RoleTeacher}
	student := domain.User{ID: uuid.NewString(), Role: domain.RoleStudent, Locale: domain.Language{Code: "en"}}
	seedStudentInto(t, ctx, p, student)

	skillID1, conceptID1 := seedClassification(t, ctx, p, teacher, "n1")
	node1, err := p.content.CreateContentNode(ctx, teacher, "Node 1", domain.ContentTypeVideo, []string{skillID1}, []string{conceptID1}, domain.DifficultyLevelBeginner, []string{"en"}, testVideoURL(), nil)
	require.NoError(t, err)
	_, err = p.content.PublishContentNode(ctx, teacher, node1.ID)
	require.NoError(t, err)
	path1, err := p.path.CreateLearningPath(ctx, teacher, "Path 1", []application.PathItemInput{{ContentNodeID: node1.ID}})
	require.NoError(t, err)

	first, err := p.studentPath.AssignLearningPath(ctx, teacher, student.ID, path1.ID)
	require.NoError(t, err)

	// Mark the first path's node completed under the first StudentPath.
	_, err = p.mongoDB.Collection("aggregates").InsertOne(ctx, bson.D{
		{Key: "student_id", Value: student.ID},
		{Key: "content_node_id", Value: node1.ID},
		{Key: "status", Value: "completed"},
		{Key: "updated_at", Value: time.Now().UTC()},
	})
	require.NoError(t, err)

	skillID2, conceptID2 := seedClassification(t, ctx, p, teacher, "n2")
	node2, err := p.content.CreateContentNode(ctx, teacher, "Node 2", domain.ContentTypeVideo, []string{skillID2}, []string{conceptID2}, domain.DifficultyLevelBeginner, []string{"en"}, testVideoURL(), nil)
	require.NoError(t, err)
	_, err = p.content.PublishContentNode(ctx, teacher, node2.ID)
	require.NoError(t, err)
	path2, err := p.path.CreateLearningPath(ctx, teacher, "Path 2", []application.PathItemInput{{ContentNodeID: node2.ID}})
	require.NoError(t, err)

	second, err := p.studentPath.AssignLearningPath(ctx, teacher, student.ID, path2.ID)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID)

	// The current pointer now resolves to the newly assigned copy.
	view, err := p.studentPath.GetMyPath(ctx, student)
	require.NoError(t, err)
	require.Len(t, view.Items, 1)
	assert.Equal(t, path2.ID, view.SourceTemplateID)
	// node2 has no aggregates document at all — not_started, not carried
	// over from the old path's completed node1.
	assert.Equal(t, domain.CompletionStatusNotStarted, view.Items[0].Status)

	// The earlier copy of path1 still exists, untouched — assigning is
	// additive, never a replace.
	stillThere, err := p.studentPath.ArchiveStandaloneStudentPath(ctx, student, first.ID)
	require.NoError(t, err)
	require.NotNil(t, stillThere.ArchivedAt)
}

// seedStudentInto persists student (whose Locale the caller must already
// have set to a seeded code — see setupPipeline) into the same Postgres
// database as a User row. Note this only writes a copy: it does not mutate
// the caller's student variable, so ClerkUserID/RegisteredAt aren't visible
// back at the call site, only the persisted row.
func seedStudentInto(t *testing.T, ctx context.Context, p *pipeline, student domain.User) {
	t.Helper()
	student.ClerkUserID = "clerk-" + student.ID
	student.RegisteredAt = time.Now().UTC()
	require.NoError(t, p.users.Create(ctx, student))
}

func testVideoURL() *string {
	url := "https://cdn.motifpath.io/videos/pipeline-test.mp4"
	return &url
}
