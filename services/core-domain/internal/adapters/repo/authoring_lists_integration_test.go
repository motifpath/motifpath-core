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

var (
	earlierAt = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	laterAt   = time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
)

func TestEntKnowledgeNodeRepository_SubtreeIDs(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntKnowledgeNodeRepository(client)
	create := func(key string, parent *domain.KnowledgeNode) domain.KnowledgeNode {
		node := knowledgeNode(domain.KnowledgeNodeKindSkill, key, parent)
		require.NoError(t, repo.Create(ctx, node))
		return node
	}
	picking := create("picking", nil)
	alternate := create("alternate-picking", &picking)
	outside := create("outside-picking", &alternate)
	sweep := create("sweep-picking", &picking)
	legato := create("legato", nil)

	tests := []struct {
		name  string
		roots []string
		want  []string
	}{
		{name: "every node below an area, at any depth", roots: []string{picking.ID}, want: []string{alternate.ID, outside.ID, sweep.ID}},
		{name: "a sub-skill's own subtree only", roots: []string{alternate.ID}, want: []string{outside.ID}},
		{name: "a leaf adds nothing", roots: []string{legato.ID}, want: []string{}},
		{name: "several roots", roots: []string{alternate.ID, sweep.ID}, want: []string{outside.ID}},
		{name: "an unknown or malformed id adds nothing", roots: []string{uuid.NewString(), "not-a-uuid"}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.SubtreeIDs(ctx, tt.roots)

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestEntContentNodeRepository_AuthoringFilters(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntContentNodeRepository(client)
	skillA := seedSkill(t, ctx, client, "skill-a-"+uuid.NewString())
	skillB := seedSkill(t, ctx, client, "skill-b-"+uuid.NewString())
	bob, carol := uuid.NewString(), uuid.NewString()
	one := 1

	node := func(title, teacher string, skill domain.KnowledgeNode, languages []string, updatedAt time.Time, version *int, changed bool) domain.ContentNode {
		langs := make([]domain.Language, len(languages))
		for i, code := range languages {
			langs[i] = domain.Language{Code: code}
		}
		n := domain.ContentNode{
			ID: uuid.NewString(), TeacherID: teacher, Title: title, ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{Skills: []domain.KnowledgeNode{skill}, DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
			Languages:      langs, CreatedAt: fixedAt, UpdatedAt: updatedAt,
			LatestPublishedVersion: version, HasUnpublishedChanges: changed,
		}
		require.NoError(t, repo.Create(ctx, n))
		return n
	}
	draft := node("Draft lesson", bob, skillA, []string{"en"}, laterAt, nil, true)
	published := node("Published lesson", bob, skillB, []string{"pt_BR"}, earlierAt, &one, false)
	edited := node("Edited lesson", carol, skillA, []string{"en", "pt_BR"}, earlierAt.Add(time.Hour), &one, true)
	agnostic := node("Agnostic lesson", carol, skillB, []string{"any"}, earlierAt, nil, true)
	title := func(n domain.ContentNode) string { return n.Title }

	tests := []struct {
		name   string
		filter domain.ContentNodeFilter
		want   []string
	}{
		{name: "sorted by title by default", filter: domain.ContentNodeFilter{}, want: []string{agnostic.Title, draft.Title, edited.Title, published.Title}},
		{name: "sorted by most recent change first, then id", filter: domain.ContentNodeFilter{Sort: domain.ListSortUpdated}, want: []string{draft.Title, edited.Title, idOrder(agnostic, published)[0].Title, idOrder(agnostic, published)[1].Title}},
		{name: "several skills match any of them", filter: domain.ContentNodeFilter{SkillIDs: []string{skillA.ID, skillB.ID}}, want: []string{agnostic.Title, draft.Title, edited.Title, published.Title}},
		{name: "by author", filter: domain.ContentNodeFilter{CreatedBy: carol}, want: []string{agnostic.Title, edited.Title}},
		{name: "by language, never matching a language-agnostic node", filter: domain.ContentNodeFilter{Language: "pt_BR"}, want: []string{edited.Title, published.Title}},
		{name: "drafts are the never-published nodes", filter: domain.ContentNodeFilter{Status: domain.ContentNodeStatusDraft}, want: []string{agnostic.Title, draft.Title}},
		{name: "published leaves out nodes with unpublished changes", filter: domain.ContentNodeFilter{Status: domain.ContentNodeStatusPublished}, want: []string{published.Title}},
		{name: "unpublished changes are published nodes edited since", filter: domain.ContentNodeFilter{Status: domain.ContentNodeStatusUnpublishedChanges}, want: []string{edited.Title}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.List(ctx, tt.filter, domain.PageRequest{Limit: 20})

			require.NoError(t, err)
			assert.Equal(t, tt.want, titlesOf(got.Items, title))
			assert.Equal(t, len(tt.want), got.Total)
		})
	}

	t.Run("reads back the publishing state and the last change", func(t *testing.T) {
		got, err := repo.GetByID(ctx, edited.ID)

		require.NoError(t, err)
		assert.Equal(t, &one, got.LatestPublishedVersion)
		assert.True(t, got.HasUnpublishedChanges)
		assert.True(t, edited.UpdatedAt.Equal(got.UpdatedAt))
	})

	t.Run("marking a node published records the version and clears its changes", func(t *testing.T) {
		require.NoError(t, repo.MarkPublished(ctx, draft.ID, 1))

		got, err := repo.GetByID(ctx, draft.ID)
		require.NoError(t, err)
		assert.Equal(t, &one, got.LatestPublishedVersion)
		assert.False(t, got.HasUnpublishedChanges)
		assert.True(t, draft.UpdatedAt.Equal(got.UpdatedAt))
	})

	t.Run("marking an unknown node published is not found", func(t *testing.T) {
		assert.ErrorIs(t, repo.MarkPublished(ctx, uuid.NewString(), 1), domain.ErrNotFound)
	})

	t.Run("an update persists the last change and the publishing state", func(t *testing.T) {
		changed, err := repo.GetByID(ctx, published.ID)
		require.NoError(t, err)
		changed.UpdatedAt = laterAt.Add(time.Hour)
		changed.HasUnpublishedChanges = true

		require.NoError(t, repo.Update(ctx, changed))

		got, err := repo.GetByID(ctx, published.ID)
		require.NoError(t, err)
		assert.True(t, changed.UpdatedAt.Equal(got.UpdatedAt))
		assert.True(t, got.HasUnpublishedChanges)
		assert.Equal(t, &one, got.LatestPublishedVersion)
	})

	t.Run("creator counts", func(t *testing.T) {
		got, err := repo.ListCreatorCounts(ctx)

		require.NoError(t, err)
		assert.ElementsMatch(t, []domain.CreatorCount{{UserID: bob, Count: 2}, {UserID: carol, Count: 2}}, got)
	})
}

// idOrder returns a and b ordered by id, the tiebreak every list uses.
func idOrder(a, b domain.ContentNode) []domain.ContentNode {
	if a.ID < b.ID {
		return []domain.ContentNode{a, b}
	}
	return []domain.ContentNode{b, a}
}

func TestEntExerciseRepository_AuthoringSortAndCounts(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntExerciseRepository(client)
	bob, carol := uuid.NewString(), uuid.NewString()
	exercise := func(title, creator string, updatedAt time.Time) domain.Exercise {
		ex := domain.Exercise{
			ID: uuid.NewString(), Title: title, Prompt: domain.NewPlainTextPrompt("Say it"),
			ExerciseType: domain.ExerciseTypeTextResponse, Options: []domain.Option{},
			ChallengeIDs: []string{}, ContentNodeIDs: []string{}, Languages: []domain.Language{{Code: "any"}},
			CreatedBy: creator, CreatedAt: fixedAt, UpdatedAt: updatedAt,
		}
		require.NoError(t, repo.Create(ctx, ex))
		return ex
	}
	sweep := exercise("Arpeggio sweep", bob, earlierAt)
	triads := exercise("Triads on the top strings", carol, laterAt)
	exercise("Muting drill", carol, earlierAt.Add(time.Hour))
	title := func(e domain.Exercise) string { return e.Title }

	t.Run("most recently changed first", func(t *testing.T) {
		got, err := repo.List(ctx, domain.ExerciseFilter{Sort: domain.ListSortUpdated}, domain.PageRequest{Limit: 20})

		require.NoError(t, err)
		assert.Equal(t, []string{triads.Title, "Muting drill", sweep.Title}, titlesOf(got.Items, title))
	})

	t.Run("an update persists the last change", func(t *testing.T) {
		changed, err := repo.GetByID(ctx, sweep.ID)
		require.NoError(t, err)
		changed.UpdatedAt = laterAt.Add(time.Hour)
		require.NoError(t, repo.Update(ctx, changed))

		got, err := repo.GetByID(ctx, sweep.ID)
		require.NoError(t, err)
		assert.True(t, changed.UpdatedAt.Equal(got.UpdatedAt))
	})

	t.Run("creator counts", func(t *testing.T) {
		got, err := repo.ListCreatorCounts(ctx)

		require.NoError(t, err)
		assert.ElementsMatch(t, []domain.CreatorCount{{UserID: bob, Count: 1}, {UserID: carol, Count: 2}}, got)
	})
}

func TestEntDiagramRepository_AuthoringSortAndCounts(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntDiagramRepository(client)
	older := seedDiagram(t, ctx, client, "Guitar A")
	newer := seedDiagram(t, ctx, client, "Guitar B")
	// Named so their name order is the opposite of their change order: a
	// listing sorted by change must not fall back on names.
	for _, d := range []struct {
		diagram domain.Diagram
		name    string
		at      time.Time
	}{{older, "A older", earlierAt}, {newer, "Z newer", laterAt}} {
		got, err := repo.GetByID(ctx, d.diagram.ID)
		require.NoError(t, err)
		got.Names = domain.LocalizedText{"en": d.name}
		got.UpdatedAt = d.at
		require.NoError(t, repo.Update(ctx, got))
	}

	t.Run("most recently changed first", func(t *testing.T) {
		got, err := repo.List(ctx, domain.DiagramListFilter{Sort: domain.ListSortUpdated, Purpose: domain.DiagramPurposeFilterGeneral}, domain.PageRequest{Limit: 20})

		require.NoError(t, err)
		require.Len(t, got.Items, 2)
		assert.Equal(t, []string{newer.ID, older.ID}, []string{got.Items[0].ID, got.Items[1].ID})
		assert.True(t, laterAt.Equal(got.Items[0].UpdatedAt))
	})

	t.Run("creator counts", func(t *testing.T) {
		got, err := repo.ListCreatorCounts(ctx, domain.DiagramListFilter{Purpose: domain.DiagramPurposeFilterGeneral})

		require.NoError(t, err)
		assert.ElementsMatch(t, []domain.CreatorCount{{UserID: older.CreatedBy, Count: 1}, {UserID: newer.CreatedBy, Count: 1}}, got)
	})
}

func TestEntCourseRepository_AuthoringSortAndCounts(t *testing.T) {
	f := newCourseListFixture(t)
	bob := uuid.NewString()
	path := f.pathWith(nil, nil)
	blues := f.draft("Blues Basics", "s", domain.DifficultyLevelBeginner, bob, path)
	acoustic := f.draft("Acoustic Start", "s", domain.DifficultyLevelBeginner, bob, path)
	require.NoError(t, f.courses.UpdateStatus(f.ctx, blues.ID, domain.CourseStatusDraft, earlierAt))
	require.NoError(t, f.courses.UpdateStatus(f.ctx, acoustic.ID, domain.CourseStatusDraft, laterAt))
	title := func(c domain.Course) string { return c.Title }

	t.Run("most recently changed first", func(t *testing.T) {
		got := f.list(domain.CourseListFilter{Sort: domain.ListSortUpdated}, domain.PageRequest{Limit: 20})

		assert.Equal(t, []string{acoustic.Title, blues.Title}, titlesOf(got.Items, title))
		assert.True(t, laterAt.Equal(got.Items[0].UpdatedAt))
	})

	t.Run("replacing a course persists its last change", func(t *testing.T) {
		replaced, err := f.courses.GetByID(f.ctx, blues.ID)
		require.NoError(t, err)
		replaced.UpdatedAt = laterAt.Add(time.Hour)
		require.NoError(t, f.courses.Replace(f.ctx, replaced))

		got, err := f.courses.GetByID(f.ctx, blues.ID)
		require.NoError(t, err)
		assert.True(t, replaced.UpdatedAt.Equal(got.UpdatedAt))
	})

	t.Run("creator counts", func(t *testing.T) {
		got, err := f.courses.ListCreatorCounts(f.ctx, domain.CourseListFilter{})

		require.NoError(t, err)
		assert.Equal(t, []domain.CreatorCount{{UserID: bob, Count: 2}}, got)
	})
}

func TestEntLearningPathRepository_CreatorCounts(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodes := NewEntContentNodeRepository(client)
	repo := NewEntLearningPathRepository(client)
	node := seedContentNode(t, ctx, nodes)
	first := seedLearningPath(t, ctx, repo, node)
	second := seedLearningPath(t, ctx, repo, node)

	got, err := repo.ListCreatorCounts(ctx, domain.LearningPathFilter{})

	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.CreatorCount{{UserID: first.TeacherID, Count: 1}, {UserID: second.TeacherID, Count: 1}}, got)
}

// listCreatorIDs keeps only the user ids of a ListCreatorCounts result, for
// the tests that check which creators are listed.
func listCreatorIDs(counts []domain.CreatorCount, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(counts))
	for i, c := range counts {
		ids[i] = c.UserID
	}
	return ids, nil
}
