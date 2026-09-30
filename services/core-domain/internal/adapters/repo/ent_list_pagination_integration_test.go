//go:build integration

package repo

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func titlesOf[T any](items []T, title func(T) string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = title(item)
	}
	return out
}

func seedTitledContentNode(t *testing.T, ctx context.Context, repo *EntContentNodeRepository, title string, contentType domain.ContentType) {
	t.Helper()
	node := domain.ContentNode{
		ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: title, ContentType: contentType,
		Classification: domain.Classification{DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
		CreatedAt:      fixedAt,
	}
	require.NoError(t, repo.Create(ctx, node))
}

func TestEntContentNodeRepository_List_PagesSearchesAndCombinesFilters(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntContentNodeRepository(client)
	nodeTitle := func(n domain.ContentNode) string { return n.Title }

	for i := 1; i <= 5; i++ {
		seedTitledContentNode(t, ctx, repo, fmt.Sprintf("Chords %d", i), domain.ContentTypeArticle)
	}
	seedTitledContentNode(t, ctx, repo, "Chords video", domain.ContentTypeVideo)
	seedTitledContentNode(t, ctx, repo, "Scales", domain.ContentTypeArticle)
	seedTitledContentNode(t, ctx, repo, "100% legato", domain.ContentTypeArticle)

	t.Run("orders by title and pages with a total of every match", func(t *testing.T) {
		got, err := repo.List(ctx, domain.ContentNodeFilter{}, domain.PageRequest{Limit: 3, Offset: 3})

		require.NoError(t, err)
		assert.Equal(t, 8, got.Total)
		assert.Equal(t, []string{"Chords 3", "Chords 4", "Chords 5"}, titlesOf(got.Items, nodeTitle))
	})

	t.Run("an offset past the end returns an empty page with the total", func(t *testing.T) {
		got, err := repo.List(ctx, domain.ContentNodeFilter{}, domain.PageRequest{Limit: 20, Offset: 100})

		require.NoError(t, err)
		assert.NotNil(t, got.Items)
		assert.Empty(t, got.Items)
		assert.Equal(t, 8, got.Total)
	})

	t.Run("search is a case-insensitive substring match", func(t *testing.T) {
		got, err := repo.List(ctx, domain.ContentNodeFilter{Query: "CHORDS"}, domain.PageRequest{Limit: 20})

		require.NoError(t, err)
		assert.Equal(t, 6, got.Total)
	})

	t.Run("search treats percent as a literal character", func(t *testing.T) {
		got, err := repo.List(ctx, domain.ContentNodeFilter{Query: "100%"}, domain.PageRequest{Limit: 20})

		require.NoError(t, err)
		assert.Equal(t, []string{"100% legato"}, titlesOf(got.Items, nodeTitle))
	})

	t.Run("search, filters and paging combine, and the total reflects the filters", func(t *testing.T) {
		got, err := repo.List(ctx,
			domain.ContentNodeFilter{Query: "chords", ContentType: domain.ContentTypeArticle},
			domain.PageRequest{Limit: 2, Offset: 0})

		require.NoError(t, err)
		assert.Equal(t, 5, got.Total)
		assert.Equal(t, []string{"Chords 1", "Chords 2"}, titlesOf(got.Items, nodeTitle))
	})
}

func TestEntLearningPathRepository_List_PagesAndSearches(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	repo := NewEntLearningPathRepository(client)
	pathTitle := func(p domain.LearningPath) string { return p.Title }
	node := seedContentNode(t, ctx, nodeRepo)

	for _, title := range []string{"Open Chords Path", "Barre Chords Path", "Strumming Path"} {
		require.NoError(t, repo.Create(ctx, domain.LearningPath{
			ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: title,
			Items:     []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
			CreatedAt: fixedAt,
		}))
	}

	t.Run("orders by title and pages", func(t *testing.T) {
		got, err := repo.List(ctx, domain.LearningPathFilter{}, domain.PageRequest{Limit: 2, Offset: 1})

		require.NoError(t, err)
		assert.Equal(t, 3, got.Total)
		assert.Equal(t, []string{"Open Chords Path", "Strumming Path"}, titlesOf(got.Items, pathTitle))
		require.Len(t, got.Items[0].Items, 1)
	})

	t.Run("searches by title and reports the filtered total", func(t *testing.T) {
		got, err := repo.List(ctx, domain.LearningPathFilter{Query: "chords"}, domain.PageRequest{Limit: 1})

		require.NoError(t, err)
		assert.Equal(t, 2, got.Total)
		assert.Equal(t, []string{"Barre Chords Path"}, titlesOf(got.Items, pathTitle))
	})
}

func TestEntExerciseRepository_List_PagesAndFilters(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntExerciseRepository(client)
	skill := seedSkill(t, ctx, client, "skill-"+uuid.NewString())

	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		exType := domain.ExerciseTypeTextResponse
		var skills []domain.Skill
		if i%2 == 0 {
			skills = []domain.Skill{skill}
		}
		ex := domain.Exercise{
			ID: uuid.NewString(), Title: fmt.Sprintf("Exercise %d", i), Prompt: domain.NewPlainTextPrompt("Say it"),
			ExerciseType: exType, Skills: skills, Options: []domain.Option{},
			ChallengeIDs: []string{}, ContentNodeIDs: []string{},
			Languages: []domain.Language{{Code: "any"}}, CreatedAt: fixedAt,
		}
		require.NoError(t, repo.Create(ctx, ex))
		ids = append(ids, ex.ID)
	}
	exerciseID := func(e domain.Exercise) string { return e.ID }

	t.Run("orders by id and pages with a total", func(t *testing.T) {
		first, err := repo.List(ctx, domain.ExerciseFilter{}, domain.PageRequest{Limit: 3, Offset: 0})
		require.NoError(t, err)
		second, err := repo.List(ctx, domain.ExerciseFilter{}, domain.PageRequest{Limit: 3, Offset: 3})
		require.NoError(t, err)

		assert.Equal(t, 5, first.Total)
		assert.Len(t, first.Items, 3)
		assert.Len(t, second.Items, 2)
		all := append(titlesOf(first.Items, exerciseID), titlesOf(second.Items, exerciseID)...)
		assert.ElementsMatch(t, ids, all)
		assert.IsIncreasing(t, all)
	})

	t.Run("the skill filter narrows the total, not just the page", func(t *testing.T) {
		got, err := repo.List(ctx, domain.ExerciseFilter{SkillID: skill.ID}, domain.PageRequest{Limit: 1})

		require.NoError(t, err)
		assert.Equal(t, 3, got.Total)
		assert.Len(t, got.Items, 1)
	})
}

func TestEntExerciseRepository_List_TextLanguageConceptAndCreatorFilters(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntExerciseRepository(client)
	triad := seedConcept(t, ctx, client, "triad-"+uuid.NewString())
	fifth := seedConcept(t, ctx, client, "fifth-"+uuid.NewString())
	bob, carol := uuid.NewString(), uuid.NewString()

	exercise := func(title, lang, createdBy string, concept domain.Concept) domain.Exercise {
		ex := domain.Exercise{
			ID: uuid.NewString(), Title: title, Prompt: domain.NewPlainTextPrompt("Say it"),
			ExerciseType: domain.ExerciseTypeTextResponse, Concepts: []domain.Concept{concept}, Options: []domain.Option{},
			ChallengeIDs: []string{}, ContentNodeIDs: []string{},
			Languages: []domain.Language{{Code: lang}}, CreatedBy: createdBy, CreatedAt: fixedAt,
		}
		require.NoError(t, repo.Create(ctx, ex))
		return ex
	}
	bobsTriad := exercise("Major Triad Shapes", "en", bob, triad)
	carolsTriad := exercise("Tríade invertida triad", "pt_BR", carol, triad)
	carolsFifth := exercise("Perfect fifth drill", "pt_BR", carol, fifth)
	legacy := exercise("Legacy triad", "en", "", triad)
	all := domain.PageRequest{Limit: domain.MaxPageLimit}
	exerciseID := func(e domain.Exercise) string { return e.ID }

	tests := []struct {
		name   string
		filter domain.ExerciseFilter
		want   []domain.Exercise
	}{
		{name: "title search ignores case", filter: domain.ExerciseFilter{Query: "TRIAD"}, want: []domain.Exercise{bobsTriad, carolsTriad, legacy}},
		{name: "by concept", filter: domain.ExerciseFilter{ConceptID: fifth.ID}, want: []domain.Exercise{carolsFifth}},
		{name: "by language", filter: domain.ExerciseFilter{Language: "pt_BR"}, want: []domain.Exercise{carolsTriad, carolsFifth}},
		{name: "by creator, never matching an exercise with no recorded creator", filter: domain.ExerciseFilter{CreatedBy: bob}, want: []domain.Exercise{bobsTriad}},
		{name: "filters combine", filter: domain.ExerciseFilter{Query: "triad", CreatedBy: carol}, want: []domain.Exercise{carolsTriad}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.List(ctx, tt.filter, all)

			require.NoError(t, err)
			assert.ElementsMatch(t, titlesOf(tt.want, exerciseID), titlesOf(got.Items, exerciseID))
			assert.Equal(t, len(tt.want), got.Total)
		})
	}

	t.Run("the creator is read back", func(t *testing.T) {
		got, err := repo.GetByID(ctx, bobsTriad.ID)

		require.NoError(t, err)
		assert.Equal(t, bob, got.CreatedBy)
	})

	t.Run("an exercise with no recorded creator reads back with none", func(t *testing.T) {
		got, err := repo.GetByID(ctx, legacy.ID)

		require.NoError(t, err)
		assert.Empty(t, got.CreatedBy)
	})

	t.Run("creator ids are distinct and skip exercises with none", func(t *testing.T) {
		got, err := repo.ListCreatorIDs(ctx)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{bob, carol}, got)
	})
}
