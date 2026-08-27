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
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	_ "github.com/lib/pq"

	"github.com/motifpath/core-domain/internal/adapters/repo"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

type pipeline struct {
	content    *application.ContentService
	challenge  *application.ChallengeService
	path       *application.LearningPathService
	assignment *application.PathAssignmentService
	users      *repo.EntUserRepository
	mongoDB    *mongo.Database
}

func setupPipeline(t *testing.T) *pipeline {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("core_domain_pipeline_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, testcontainers.TerminateContainer(pgContainer)) })

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	entClient, err := ent.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, entClient.Close()) })

	// See repo.setupPostgres for why this retries: Schema.Create's first
	// act is a version query that can race the container's brief
	// post-ready connection reset window.
	for attempt := 0; ; attempt++ {
		err = entClient.Schema.Create(ctx)
		if err == nil || attempt >= 4 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err)

	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, testcontainers.TerminateContainer(mongoContainer)) })
	mongoConnStr, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoConnStr))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, mongoClient.Disconnect(context.Background())) })
	mongoDB := mongoClient.Database("motifpath_events_pipeline_test")

	newID := uuid.NewString
	now := func() time.Time { return time.Now().UTC() }

	nodes := repo.NewEntContentNodeRepository(entClient)
	challenges := repo.NewEntChallengeRepository(entClient)
	exercises := repo.NewEntExerciseRepository(entClient)
	expanded := repo.NewEntExpandedContentRepository(entClient)
	paths := repo.NewEntLearningPathRepository(entClient)
	assignments := repo.NewEntPathAssignmentRepository(entClient)
	users := repo.NewEntUserRepository(entClient)
	completion := repo.NewMongoCompletionStateReader(mongoDB)

	return &pipeline{
		content:    application.NewContentService(nodes, expanded, newID, now),
		challenge:  application.NewChallengeService(nodes, challenges, exercises, newID, now),
		path:       application.NewLearningPathService(nodes, paths, newID, now),
		assignment: application.NewPathAssignmentService(users, paths, assignments, completion, newID, now),
		users:      users,
		mongoDB:    mongoDB,
	}
}

func TestCoreDomainPipeline_CreateAssignAndViewPath(t *testing.T) {
	p := setupPipeline(t)
	ctx := context.Background()
	teacher := domain.User{ID: uuid.NewString(), Role: domain.RoleTeacher}
	student := domain.User{ID: uuid.NewString(), Role: domain.RoleStudent}

	node, err := p.content.CreateContentNode(ctx, teacher, "Intro to Triads", domain.ContentTypeVideo, "triad-shapes", "chord-theory", domain.DifficultyLevelBeginner)
	require.NoError(t, err)

	challenge, err := p.challenge.CreateChallenge(ctx, teacher, node.ID, "triad-shapes", 70, nil)
	require.NoError(t, err)
	assert.Equal(t, node.ID, challenge.ContentNodeID)

	learningPath, err := p.path.CreateLearningPath(ctx, teacher, "Week 1", []string{node.ID})
	require.NoError(t, err)

	// AssignLearningPath needs the student to exist in the same Postgres
	// database as a User row — seed it directly via the ent repo rather
	// than through IdentityService, which isn't part of this pipeline.
	seedStudentInto(t, ctx, p, student)

	assignment, err := p.assignment.AssignLearningPath(ctx, teacher, student.ID, learningPath.ID)
	require.NoError(t, err)
	assert.Equal(t, student.ID, assignment.StudentID)

	// Simulate the Aggregation Worker (ADR-011) marking this node completed.
	_, err = p.mongoDB.Collection("aggregates").InsertOne(ctx, bson.D{
		{Key: "student_id", Value: student.ID},
		{Key: "content_node_id", Value: node.ID},
		{Key: "status", Value: "completed"},
		{Key: "updated_at", Value: time.Now().UTC()},
	})
	require.NoError(t, err)

	view, err := p.assignment.GetMyPath(ctx, student)
	require.NoError(t, err)
	require.Len(t, view.Items, 1)
	assert.Equal(t, domain.CompletionStatusCompleted, view.Items[0].Status)
	assert.Equal(t, 1, view.CurrentPosition)
}

func TestCoreDomainPipeline_ReplacingAssignmentResetsProgress(t *testing.T) {
	p := setupPipeline(t)
	ctx := context.Background()
	teacher := domain.User{ID: uuid.NewString(), Role: domain.RoleTeacher}
	student := domain.User{ID: uuid.NewString(), Role: domain.RoleStudent}
	seedStudentInto(t, ctx, p, student)

	node1, err := p.content.CreateContentNode(ctx, teacher, "Node 1", domain.ContentTypeVideo, "s1", "c1", domain.DifficultyLevelBeginner)
	require.NoError(t, err)
	path1, err := p.path.CreateLearningPath(ctx, teacher, "Path 1", []string{node1.ID})
	require.NoError(t, err)

	first, err := p.assignment.AssignLearningPath(ctx, teacher, student.ID, path1.ID)
	require.NoError(t, err)

	// Mark the first path's node completed under the first assignment.
	_, err = p.mongoDB.Collection("aggregates").InsertOne(ctx, bson.D{
		{Key: "student_id", Value: student.ID},
		{Key: "content_node_id", Value: node1.ID},
		{Key: "status", Value: "completed"},
		{Key: "updated_at", Value: time.Now().UTC()},
	})
	require.NoError(t, err)

	node2, err := p.content.CreateContentNode(ctx, teacher, "Node 2", domain.ContentTypeVideo, "s2", "c2", domain.DifficultyLevelBeginner)
	require.NoError(t, err)
	path2, err := p.path.CreateLearningPath(ctx, teacher, "Path 2", []string{node2.ID})
	require.NoError(t, err)

	second, err := p.assignment.AssignLearningPath(ctx, teacher, student.ID, path2.ID)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID)

	view, err := p.assignment.GetMyPath(ctx, student)
	require.NoError(t, err)
	require.Len(t, view.Items, 1)
	assert.Equal(t, path2.ID, view.LearningPathID)
	// node2 has no aggregates document at all — not_started, not carried
	// over from the old path's completed node1.
	assert.Equal(t, domain.CompletionStatusNotStarted, view.Items[0].Status)
}

func seedStudentInto(t *testing.T, ctx context.Context, p *pipeline, student domain.User) {
	t.Helper()
	student.ClerkUserID = "clerk-" + student.ID
	student.RegisteredAt = time.Now().UTC()
	require.NoError(t, p.users.Create(ctx, student))
}
