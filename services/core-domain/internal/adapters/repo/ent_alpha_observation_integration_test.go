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

func TestEntUserRepository_ListByRole(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntUserRepository(client)
	user := func(name string, role domain.Role) domain.User {
		u := domain.User{ID: uuid.NewString(), ClerkUserID: "clerk-" + name, Role: role, DisplayName: name, Locale: domain.Language{Code: "en", Name: "English"}, RegisteredAt: fixedAt}
		require.NoError(t, repo.Create(ctx, u))
		return u
	}
	alice, bruno := user("alice", domain.RoleStudent), user("bruno", domain.RoleStudent)
	user("tina", domain.RoleTeacher)

	got, err := repo.ListByRole(ctx, domain.RoleStudent)

	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.User{alice, bruno}, got)
}

func TestEntStudentPathRepository_FirstGivenByStudentIDs(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	versionRepo := NewEntContentNodeVersionRepository(client)
	repo := NewEntStudentPathRepository(client)
	template := seedLearningPath(t, ctx, NewEntLearningPathRepository(client), seedContentNode(t, ctx, nodeRepo))
	node := seedContentNode(t, ctx, nodeRepo)
	version := domain.ContentNodeVersion{
		ID: uuid.NewString(), ContentNodeID: node.ID, VersionNumber: 1,
		Title: node.Title, ContentType: node.ContentType, PublishedBy: uuid.NewString(), PublishedAt: fixedAt,
	}
	require.NoError(t, versionRepo.Create(ctx, version))

	give := func(studentID, title string, at time.Time, archived bool) domain.StudentPath {
		sp := domain.StudentPath{
			ID: uuid.NewString(), StudentID: studentID, SourceTemplateID: template.ID, Title: title,
			AssignedBy: studentID, AssignedAt: at,
			Items: []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID, ContentNodeVersionID: version.ID}},
		}
		require.NoError(t, repo.Create(ctx, sp))
		if archived {
			require.NoError(t, repo.Archive(ctx, sp.ID, at.Add(24*time.Hour)))
		}
		return sp
	}
	alice, bruno, carla := uuid.NewString(), uuid.NewString(), uuid.NewString()
	alicesFirst := give(alice, "Guitar foundations", fixedAt, true)
	give(alice, "Blues guitar basics", fixedAt.Add(48*time.Hour), false)
	brunosOnly := give(bruno, "Guitar foundations", fixedAt.Add(time.Hour), false)
	give(uuid.NewString(), "Not asked", fixedAt, false)

	got, err := repo.FirstGivenByStudentIDs(ctx, []string{alice, bruno, carla})

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, alicesFirst.ID, got[alice].ID, "the first path is kept even after it was archived")
	assert.Equal(t, "Guitar foundations", got[alice].Title)
	assert.True(t, fixedAt.Equal(got[alice].AssignedAt))
	assert.Equal(t, alicesFirst.Items, got[alice].Items)
	assert.Equal(t, brunosOnly.ID, got[bruno].ID)
}
