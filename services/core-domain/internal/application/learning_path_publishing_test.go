package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// publishingFixture is a learning path service over fakes a publishing test
// can seed: content nodes, their published versions, paths, and course
// versions.
type publishingFixture struct {
	nodes          *fakeContentNodeRepository
	versions       *fakeContentNodeVersionRepository
	paths          *fakeLearningPathRepository
	courseVersions *fakeCourseVersionRepository
	svc            *application.LearningPathService
}

func newPublishingFixture() publishingFixture {
	f := publishingFixture{
		nodes:          newFakeContentNodeRepository(),
		versions:       newFakeContentNodeVersionRepository(),
		paths:          newFakeLearningPathRepository(),
		courseVersions: newFakeCourseVersionRepository(),
	}
	f.svc = newLearningPathServiceWith(f.nodes, f.paths, f.courseVersions, f.versions)
	return f
}

// publishedNode seeds a content node with one published version.
func (f publishingFixture) publishedNode(t *testing.T, id string) {
	t.Helper()
	f.nodes.put(domain.ContentNode{ID: id, Title: id, ContentType: domain.ContentTypeVideo})
	require.NoError(t, f.versions.Create(context.Background(), domain.ContentNodeVersion{ID: id + "-v1", ContentNodeID: id, VersionNumber: 1}))
}

// completePath seeds a draft path, owned by teacher-1, that meets every
// publishing requirement.
func (f publishingFixture) completePath(t *testing.T, id string) domain.LearningPath {
	t.Helper()
	f.publishedNode(t, id+"-node")
	level := domain.DifficultyLevelBeginner
	path := domain.LearningPath{
		ID:        id,
		TeacherID: "teacher-1",
		Title:     "Open Chords",
		Summary:   strPtr("Your first chords"),
		Language:  strPtr("en"),
		Level:     &level,
		Status:    domain.LearningPathStatusDraft,
		Items:     []domain.LearningPathItem{{Position: 1, ContentNodeID: id + "-node", Title: id + "-node", ContentType: domain.ContentTypeVideo}},
	}
	f.paths.put(path)
	return path
}

func (f publishingFixture) published(t *testing.T, id string) domain.LearningPath {
	t.Helper()
	path := f.completePath(t, id)
	path.Status = domain.LearningPathStatusPublished
	f.paths.put(path)
	return path
}

func notPublishable(t *testing.T, err error) *domain.LearningPathNotPublishableError {
	t.Helper()
	require.ErrorIs(t, err, domain.ErrConflict)
	var target *domain.LearningPathNotPublishableError
	require.True(t, errors.As(err, &target), "want a LearningPathNotPublishableError, got %v", err)
	return target
}

func TestLearningPathService_SummaryAndLanguage(t *testing.T) {
	cases := []struct {
		name     string
		summary  *string
		language *string
		wantErr  string
	}{
		{name: "a path is created with a summary and a language", summary: strPtr("Seus primeiros acordes"), language: strPtr("pt_BR")},
		{name: "a path can be created without a summary or a language"},
		{name: "the language-agnostic marker is not a language", language: strPtr(domain.LanguageCodeAny), wantErr: "language"},
		{name: "a language MotifPath does not offer is rejected", language: strPtr("xx"), wantErr: "language"},
		{name: "a blank summary is rejected", summary: strPtr("  "), wantErr: "summary"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPublishingFixture()
			f.publishedNode(t, "node-01")

			path, err := f.svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{
				Title: "Acordes Abertos", Level: domain.DifficultyLevelBeginner, Summary: tc.summary, Language: tc.language, Items: pathItems("node-01"),
			})

			if tc.wantErr != "" {
				var verr *domain.ValidationError
				require.True(t, errors.As(err, &verr), "want a validation error, got %v", err)
				assert.Equal(t, tc.wantErr, verr.Fields[0].Field)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.summary, path.Summary)
			assert.Equal(t, tc.language, path.Language)
			assert.Equal(t, domain.LearningPathStatusDraft, path.Status, "every new path starts as a draft")
		})
	}
}

func TestLearningPathService_PublishLearningPath(t *testing.T) {
	t.Run("an admin publishes a complete path", func(t *testing.T) {
		f := newPublishingFixture()
		f.completePath(t, "open-chords")

		path, err := f.svc.PublishLearningPath(context.Background(), adminCaller(), "open-chords")

		require.NoError(t, err)
		assert.Equal(t, domain.LearningPathStatusPublished, path.Status)
		stored, err := f.paths.GetByID(context.Background(), "open-chords")
		require.NoError(t, err)
		assert.Equal(t, domain.LearningPathStatusPublished, stored.Status)
	})

	t.Run("publishing a published path leaves it published", func(t *testing.T) {
		f := newPublishingFixture()
		f.published(t, "open-chords")

		path, err := f.svc.PublishLearningPath(context.Background(), adminCaller(), "open-chords")

		require.NoError(t, err)
		assert.Equal(t, domain.LearningPathStatusPublished, path.Status)
	})

	t.Run("a teacher cannot publish a path, even their own", func(t *testing.T) {
		f := newPublishingFixture()
		f.completePath(t, "open-chords")

		_, err := f.svc.PublishLearningPath(context.Background(), teacherCaller(), "open-chords")

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("publishing a path that does not exist is not found", func(t *testing.T) {
		f := newPublishingFixture()

		_, err := f.svc.PublishLearningPath(context.Background(), adminCaller(), "missing")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	missing := []struct {
		name        string
		mutate      func(*domain.LearningPath)
		wantMissing []domain.PublishRequirement
	}{
		{name: "without a summary", mutate: func(p *domain.LearningPath) { p.Summary = nil }, wantMissing: []domain.PublishRequirement{domain.PublishRequirementSummary}},
		{name: "without a language", mutate: func(p *domain.LearningPath) { p.Language = nil }, wantMissing: []domain.PublishRequirement{domain.PublishRequirementLanguage}},
		{name: "recorded without a level", mutate: func(p *domain.LearningPath) { p.Level = nil }, wantMissing: []domain.PublishRequirement{domain.PublishRequirementLevel}},
		{name: "without items", mutate: func(p *domain.LearningPath) { p.Items = nil }, wantMissing: []domain.PublishRequirement{domain.PublishRequirementItems}},
		{
			name:        "listing every missing piece at once",
			mutate:      func(p *domain.LearningPath) { p.Summary, p.Language = nil, nil },
			wantMissing: []domain.PublishRequirement{domain.PublishRequirementSummary, domain.PublishRequirementLanguage},
		},
	}
	for _, tc := range missing {
		t.Run("publishing is refused "+tc.name, func(t *testing.T) {
			f := newPublishingFixture()
			path := f.completePath(t, "open-chords")
			tc.mutate(&path)
			f.paths.put(path)

			_, err := f.svc.PublishLearningPath(context.Background(), adminCaller(), "open-chords")

			assert.Equal(t, tc.wantMissing, notPublishable(t, err).Missing)
			stored, getErr := f.paths.GetByID(context.Background(), "open-chords")
			require.NoError(t, getErr)
			assert.Equal(t, domain.LearningPathStatusDraft, stored.Status, "a refused publish leaves the path a draft")
		})
	}

	t.Run("publishing is refused while a lesson was never published, naming it", func(t *testing.T) {
		f := newPublishingFixture()
		path := f.completePath(t, "open-chords")
		f.nodes.put(domain.ContentNode{ID: "draft-node", Title: "Draft", ContentType: domain.ContentTypeVideo})
		path.Items = append(path.Items, domain.LearningPathItem{Position: 2, ContentNodeID: "draft-node"})
		f.paths.put(path)

		_, err := f.svc.PublishLearningPath(context.Background(), adminCaller(), "open-chords")

		refusal := notPublishable(t, err)
		assert.Equal(t, []domain.PublishRequirement{domain.PublishRequirementUnpublishedContent}, refusal.Missing)
		assert.Equal(t, []string{"draft-node"}, refusal.UnpublishedContentNodeIDs)
	})
}

func TestLearningPathService_UnpublishLearningPath(t *testing.T) {
	t.Run("an admin unpublishes a path", func(t *testing.T) {
		f := newPublishingFixture()
		f.published(t, "open-chords")

		path, err := f.svc.UnpublishLearningPath(context.Background(), adminCaller(), "open-chords")

		require.NoError(t, err)
		assert.Equal(t, domain.LearningPathStatusDraft, path.Status)
	})

	t.Run("unpublishing a draft leaves it a draft", func(t *testing.T) {
		f := newPublishingFixture()
		f.completePath(t, "open-chords")

		path, err := f.svc.UnpublishLearningPath(context.Background(), adminCaller(), "open-chords")

		require.NoError(t, err)
		assert.Equal(t, domain.LearningPathStatusDraft, path.Status)
	})

	t.Run("a teacher cannot unpublish a path", func(t *testing.T) {
		f := newPublishingFixture()
		f.published(t, "open-chords")

		_, err := f.svc.UnpublishLearningPath(context.Background(), teacherCaller(), "open-chords")

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a path used by a published course version cannot be unpublished", func(t *testing.T) {
		f := newPublishingFixture()
		f.published(t, "open-chords")
		require.NoError(t, f.courseVersions.Create(context.Background(), domain.CourseVersion{
			ID: "v1", CourseID: "course-1", VersionNumber: 1,
			Checkpoints: []domain.CourseVersionCheckpoint{{Position: 1, LearningPathID: "open-chords"}},
		}))

		_, err := f.svc.UnpublishLearningPath(context.Background(), adminCaller(), "open-chords")

		require.ErrorIs(t, err, domain.ErrConflict)
		stored, getErr := f.paths.GetByID(context.Background(), "open-chords")
		require.NoError(t, getErr)
		assert.Equal(t, domain.LearningPathStatusPublished, stored.Status)
	})
}

func TestLearningPathService_KeepsPublishedPathsComplete(t *testing.T) {
	t.Run("a valid edit keeps a published path published", func(t *testing.T) {
		f := newPublishingFixture()
		f.published(t, "open-chords")
		f.publishedNode(t, "node-04")

		path, err := f.svc.ReplaceLearningPath(context.Background(), teacherCaller(), "open-chords", application.LearningPathInput{
			Title: "Open Chords", Level: domain.DifficultyLevelBeginner, Summary: strPtr("Your first chords"), Language: strPtr("en"),
			Items: pathItems("open-chords-node", "node-04"),
		})

		require.NoError(t, err)
		assert.Equal(t, domain.LearningPathStatusPublished, path.Status)
	})

	t.Run("removing the summary of a published path is refused", func(t *testing.T) {
		f := newPublishingFixture()
		f.published(t, "open-chords")

		_, err := f.svc.ReplaceLearningPath(context.Background(), teacherCaller(), "open-chords", application.LearningPathInput{
			Title: "Open Chords", Level: domain.DifficultyLevelBeginner, Language: strPtr("en"), Items: pathItems("open-chords-node"),
		})

		assert.Equal(t, []domain.PublishRequirement{domain.PublishRequirementSummary}, notPublishable(t, err).Missing)
		stored, getErr := f.paths.GetByID(context.Background(), "open-chords")
		require.NoError(t, getErr)
		assert.Equal(t, strPtr("Your first chords"), stored.Summary, "a refused replace changes nothing")
	})

	t.Run("adding a never-published lesson to a published path is refused", func(t *testing.T) {
		f := newPublishingFixture()
		f.published(t, "open-chords")
		f.nodes.put(domain.ContentNode{ID: "draft-node", Title: "Draft", ContentType: domain.ContentTypeVideo})

		_, err := f.svc.ReplaceLearningPath(context.Background(), teacherCaller(), "open-chords", application.LearningPathInput{
			Title: "Open Chords", Level: domain.DifficultyLevelBeginner, Summary: strPtr("Your first chords"), Language: strPtr("en"),
			Items: pathItems("open-chords-node", "draft-node"),
		})

		assert.Equal(t, []string{"draft-node"}, notPublishable(t, err).UnpublishedContentNodeIDs)
	})

	t.Run("a draft path can be saved incomplete", func(t *testing.T) {
		f := newPublishingFixture()
		f.completePath(t, "open-chords")

		path, err := f.svc.ReplaceLearningPath(context.Background(), teacherCaller(), "open-chords", application.LearningPathInput{
			Title: "Open Chords", Level: domain.DifficultyLevelBeginner, Items: pathItems("open-chords-node"),
		})

		require.NoError(t, err)
		assert.Nil(t, path.Summary)
		assert.Equal(t, domain.LearningPathStatusDraft, path.Status)
	})

	t.Run("a published path cannot be deleted", func(t *testing.T) {
		f := newPublishingFixture()
		f.published(t, "open-chords")

		err := f.svc.DeleteLearningPath(context.Background(), teacherCaller(), "open-chords")

		require.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("an unpublished path can be deleted", func(t *testing.T) {
		f := newPublishingFixture()
		f.completePath(t, "open-chords")

		require.NoError(t, f.svc.DeleteLearningPath(context.Background(), teacherCaller(), "open-chords"))
	})
}

func TestLearningPathService_LibraryFilters(t *testing.T) {
	f := newPublishingFixture()
	published := f.published(t, "open-chords")
	draft := f.completePath(t, "acordes")
	draft.Language = strPtr("pt_BR")
	f.paths.put(draft)

	cases := []struct {
		name   string
		filter domain.LearningPathFilter
		want   []string
	}{
		{name: "narrowed to published paths", filter: domain.LearningPathFilter{Status: domain.LearningPathStatusPublished}, want: []string{published.ID}},
		{name: "narrowed to drafts", filter: domain.LearningPathFilter{Status: domain.LearningPathStatusDraft}, want: []string{draft.ID}},
		{name: "narrowed to one language", filter: domain.LearningPathFilter{Language: "pt_BR"}, want: []string{draft.ID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := f.svc.ListLearningPaths(context.Background(), teacherCaller(), tc.filter, domain.PageRequest{Limit: 20})

			require.NoError(t, err)
			ids := make([]string, len(page.Items))
			for i, p := range page.Items {
				ids[i] = p.ID
			}
			assert.Equal(t, tc.want, ids)
		})
	}

	t.Run("an unknown status filter is rejected", func(t *testing.T) {
		_, err := f.svc.ListLearningPaths(context.Background(), teacherCaller(), domain.LearningPathFilter{Status: "archived"}, domain.PageRequest{Limit: 20})

		var verr *domain.ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, "status", verr.Fields[0].Field)
	})
}
