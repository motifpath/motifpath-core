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
)

var fixedAt = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC).Truncate(time.Microsecond)

func TestEntUserRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntUserRepository(client)

	user := domain.User{ID: uuid.NewString(), ClerkUserID: "clerk-alice", Role: domain.RoleStudent, RegisteredAt: fixedAt}
	require.NoError(t, repo.Create(ctx, user))

	byClerk, err := repo.GetByClerkUserID(ctx, "clerk-alice")
	require.NoError(t, err)
	assert.Equal(t, user, byClerk)

	byID, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user, byID)

	err = repo.Create(ctx, domain.User{ID: uuid.NewString(), ClerkUserID: "clerk-alice", Role: domain.RoleTeacher, RegisteredAt: fixedAt})
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)

	_, err = repo.GetByID(ctx, uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestEntContentNodeRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntContentNodeRepository(client)

	teacherID := uuid.NewString()
	node := domain.ContentNode{
		ID: uuid.NewString(), TeacherID: teacherID, Title: "Intro", ContentType: domain.ContentTypeVideo,
		Classification: domain.Classification{Skill: "triad-shapes", Concept: "chord-theory", DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
		CreatedAt:      fixedAt,
	}
	require.NoError(t, repo.Create(ctx, node))

	got, err := repo.GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, node, got)

	node2 := node
	node2.ID = uuid.NewString()
	require.NoError(t, repo.Create(ctx, node2))

	byIDs, err := repo.GetByIDs(ctx, []string{node.ID, node2.ID, uuid.NewString()})
	require.NoError(t, err)
	assert.Len(t, byIDs, 2)
	assert.Contains(t, byIDs, node.ID)
	assert.Contains(t, byIDs, node2.ID)
}

func TestEntChallengeRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntChallengeRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	target := seedContentNode(t, ctx, nodeRepo).ID

	challenge := domain.Challenge{
		ID: uuid.NewString(), ContentNodeID: node.ID, SubjectTag: "triad-shapes", PassThreshold: 70,
		RemediationTargetContentNodeID: &target, CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, challenge))

	got, err := repo.GetByID(ctx, challenge.ID)
	require.NoError(t, err)
	assert.Equal(t, challenge, got)
}

func TestEntExerciseRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	challengeRepo := NewEntChallengeRepository(client)
	repo := NewEntExerciseRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	challenge := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectTag: "triad-shapes", PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challengeRepo.Create(ctx, challenge))

	exercise := domain.Exercise{ID: uuid.NewString(), ChallengeID: challenge.ID, ExerciseType: domain.ExerciseTypeFretboardRegion, Prompt: "Identify the chord", CreatedAt: fixedAt}
	require.NoError(t, repo.Create(ctx, exercise))

	got, err := repo.GetByID(ctx, exercise.ID)
	require.NoError(t, err)
	assert.Equal(t, exercise, got)
}

func TestEntExpandedContentRepository_CreateGetAndList(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntExpandedContentRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)

	first, second, third := 210, 90, 150
	hideFirst, hideSecond, hideThird := first+10, second+10, third+10
	items := []domain.ExpandedContent{
		{ID: uuid.NewString(), ContentNodeID: node.ID, ContentType: domain.ExpandedContentTypeImage, MediaURL: "https://cdn/a.png", TriggerAtSeconds: &first, HideAtSeconds: &hideFirst, CreatedAt: fixedAt},
		{ID: uuid.NewString(), ContentNodeID: node.ID, ContentType: domain.ExpandedContentTypeImage, MediaURL: "https://cdn/b.png", TriggerAtSeconds: &second, HideAtSeconds: &hideSecond, CreatedAt: fixedAt},
		{ID: uuid.NewString(), ContentNodeID: node.ID, ContentType: domain.ExpandedContentTypeImage, MediaURL: "https://cdn/c.png", TriggerAtSeconds: &third, HideAtSeconds: &hideThird, CreatedAt: fixedAt},
	}
	for _, item := range items {
		require.NoError(t, repo.Create(ctx, item))
	}

	got, err := repo.GetByID(ctx, items[0].ID)
	require.NoError(t, err)
	assert.Equal(t, items[0], got)

	listed, err := repo.ListByContentNode(ctx, node.ID)
	require.NoError(t, err)
	require.Len(t, listed, 3)
	assert.Equal(t, second, *listed[0].TriggerAtSeconds)
	assert.Equal(t, third, *listed[1].TriggerAtSeconds)
	assert.Equal(t, first, *listed[2].TriggerAtSeconds)
}

func TestEntLearningPathRepository_CreateAndGet(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntLearningPathRepository(client)

	node1 := seedContentNode(t, ctx, nodeRepo)
	node2 := seedContentNode(t, ctx, nodeRepo)

	sectionLabel := "Open chords"
	path := domain.LearningPath{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Week 1",
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: node1.ID, Title: node1.Title, ContentType: node1.ContentType, SectionLabel: &sectionLabel},
			{Position: 2, ContentNodeID: node2.ID, Title: node2.Title, ContentType: node2.ContentType},
		},
		CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, path))

	got, err := repo.GetByID(ctx, path.ID)
	require.NoError(t, err)
	assert.Equal(t, path, got)
	// section_label round-trips exactly — the set label on item 1 and the
	// absent label on item 2 (stored NULL, read back as nil).
	require.NotNil(t, got.Items[0].SectionLabel)
	assert.Equal(t, "Open chords", *got.Items[0].SectionLabel)
	assert.Nil(t, got.Items[1].SectionLabel)

	_, err = repo.GetByID(ctx, uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestEntPathAssignmentRepository_ReplaceActive(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	pathRepo := NewEntLearningPathRepository(client)
	repo := NewEntPathAssignmentRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	studentID := uuid.NewString()

	path1 := seedLearningPath(t, ctx, pathRepo, node)
	first := domain.PathAssignment{ID: uuid.NewString(), StudentID: studentID, LearningPathID: path1.ID, AssignedBy: uuid.NewString(), AssignedAt: fixedAt}
	require.NoError(t, repo.ReplaceActive(ctx, first))

	active, err := repo.GetActiveByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Equal(t, first, active)

	// Replacing an active assignment resets progress: the new assignment
	// gets a fresh id and the old one is gone entirely, not merely updated.
	path2 := seedLearningPath(t, ctx, pathRepo, node)
	second := domain.PathAssignment{ID: uuid.NewString(), StudentID: studentID, LearningPathID: path2.ID, AssignedBy: uuid.NewString(), AssignedAt: fixedAt}
	require.NoError(t, repo.ReplaceActive(ctx, second))

	active, err = repo.GetActiveByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Equal(t, second, active)
	assert.NotEqual(t, first.ID, active.ID)
}

func seedContentNode(t *testing.T, ctx context.Context, repo *EntContentNodeRepository) domain.ContentNode {
	t.Helper()
	node := domain.ContentNode{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Node " + uuid.NewString(), ContentType: domain.ContentTypeVideo,
		Classification: domain.Classification{Skill: "skill", Concept: "concept", DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
		CreatedAt:      fixedAt,
	}
	require.NoError(t, repo.Create(ctx, node))
	return node
}

func seedLearningPath(t *testing.T, ctx context.Context, repo *EntLearningPathRepository, node domain.ContentNode) domain.LearningPath {
	t.Helper()
	path := domain.LearningPath{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Path " + uuid.NewString(),
		Items:     []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
		CreatedAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, path))
	return path
}
