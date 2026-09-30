package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// catalogFixture seeds a published and a draft path from two creators.
func catalogFixture(t *testing.T) (*application.PathCatalogService, *fakeLearningPathRepository) {
	t.Helper()
	users := newFakeUserRepository()
	users.put(domain.User{ID: "teacher-bob", Role: domain.RoleTeacher, DisplayName: "Bob"})
	users.put(domain.User{ID: "teacher-carol", Role: domain.RoleTeacher, DisplayName: "Carol"})
	users.put(domain.User{ID: "teacher-dave", Role: domain.RoleTeacher, DisplayName: "Dave"})
	paths := newFakeLearningPathRepository()
	paths.put(domain.LearningPath{ID: "open-chords", TeacherID: "teacher-bob", Title: "Open Chords", Status: domain.LearningPathStatusPublished, Language: strPtr("en")})
	paths.put(domain.LearningPath{ID: "acordes", TeacherID: "teacher-carol", Title: "Acordes", Status: domain.LearningPathStatusPublished, Language: strPtr("pt_BR")})
	paths.put(domain.LearningPath{ID: "theory", TeacherID: "teacher-dave", Title: "Theory", Status: domain.LearningPathStatusDraft})
	return application.NewPathCatalogService(paths, users), paths
}

func TestPathCatalogService_ListCatalogPaths(t *testing.T) {
	cases := []struct {
		name   string
		filter domain.LearningPathFilter
		want   []string
	}{
		{name: "only published paths are listed, ordered by title", want: []string{"acordes", "open-chords"}},
		{name: "a draft is never listed, even when asked for", filter: domain.LearningPathFilter{Status: domain.LearningPathStatusDraft}, want: []string{"acordes", "open-chords"}},
		{name: "other filters still narrow the list", filter: domain.LearningPathFilter{Language: "pt_BR"}, want: []string{"acordes"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := catalogFixture(t)

			page, err := svc.ListCatalogPaths(context.Background(), tc.filter, domain.PageRequest{Limit: 20})

			require.NoError(t, err)
			ids := make([]string, len(page.Items))
			for i, p := range page.Items {
				ids[i] = p.ID
			}
			assert.Equal(t, tc.want, ids)
			assert.Equal(t, len(tc.want), page.Total)
		})
	}
}

func TestPathCatalogService_GetCatalogPath(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr error
	}{
		{name: "a published path is returned", id: "open-chords"},
		{name: "a draft path is not found", id: "theory", wantErr: domain.ErrNotFound},
		{name: "an unknown path is not found", id: "missing", wantErr: domain.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := catalogFixture(t)

			path, err := svc.GetCatalogPath(context.Background(), tc.id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.id, path.ID)
		})
	}
}

func TestPathCatalogService_ListCatalogPathCreators(t *testing.T) {
	cases := []struct {
		name      string
		nameQuery string
		want      []string
	}{
		{name: "only creators of published paths are listed, by name", want: []string{"Bob", "Carol"}},
		{name: "the name query narrows them", nameQuery: "car", want: []string{"Carol"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := catalogFixture(t)

			creators, err := svc.ListCatalogPathCreators(context.Background(), tc.nameQuery)

			require.NoError(t, err)
			names := make([]string, len(creators))
			for i, c := range creators {
				names[i] = c.DisplayName
			}
			assert.Equal(t, tc.want, names)
		})
	}

	t.Run("a creator with several published paths is listed once", func(t *testing.T) {
		svc, paths := catalogFixture(t)
		paths.put(domain.LearningPath{ID: "strumming", TeacherID: "teacher-bob", Title: "Strumming", Status: domain.LearningPathStatusPublished})

		creators, err := svc.ListCatalogPathCreators(context.Background(), "")

		require.NoError(t, err)
		assert.Len(t, creators, 2)
	})
}
