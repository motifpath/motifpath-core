package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func twoItemTemplate() domain.LearningPath {
	label := "Open chords"
	return domain.LearningPath{
		ID:        "template-1",
		TeacherID: "teacher-1",
		Title:     "Beginner Guitar",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo, SectionLabel: &label},
			{Position: 2, ContentNodeID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo},
		},
	}
}

func TestNewStudentPathFromTemplate(t *testing.T) {
	assignedAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	versions := map[string]string{"node-01": "version-node-01-v1", "node-02": "version-node-02-v1"}

	t.Run("copies every template item with matching content_node_id, position, and section_label", func(t *testing.T) {
		sp, err := domain.NewStudentPathFromTemplate("sp-1", "student-1", twoItemTemplate(), "teacher-1", assignedAt, versions)
		require.NoError(t, err)

		assert.Equal(t, "sp-1", sp.ID)
		assert.Equal(t, "student-1", sp.StudentID)
		assert.Equal(t, "template-1", sp.SourceTemplateID)
		assert.Equal(t, "Beginner Guitar", sp.Title)
		assert.Equal(t, "teacher-1", sp.AssignedBy)
		assert.Equal(t, assignedAt, sp.AssignedAt)
		assert.Nil(t, sp.ArchivedAt)
		require.Len(t, sp.Items, 2)

		assert.Equal(t, 1, sp.Items[0].Position)
		assert.Equal(t, "node-01", sp.Items[0].ContentNodeID)
		assert.Equal(t, "version-node-01-v1", sp.Items[0].ContentNodeVersionID)
		require.NotNil(t, sp.Items[0].SectionLabel)
		assert.Equal(t, "Open chords", *sp.Items[0].SectionLabel)

		assert.Equal(t, 2, sp.Items[1].Position)
		assert.Equal(t, "node-02", sp.Items[1].ContentNodeID)
		assert.Equal(t, "version-node-02-v1", sp.Items[1].ContentNodeVersionID)
		assert.Nil(t, sp.Items[1].SectionLabel)
	})

	t.Run("editing the copy afterwards does not touch the template it came from", func(t *testing.T) {
		template := twoItemTemplate()
		sp, err := domain.NewStudentPathFromTemplate("sp-1", "student-1", template, "teacher-1", assignedAt, versions)
		require.NoError(t, err)

		sp.Title = "My personal copy"
		sp.Items[0].SectionLabel = nil

		assert.Equal(t, "Beginner Guitar", template.Title)
		require.NotNil(t, template.Items[0].SectionLabel)
		assert.Equal(t, "Open chords", *template.Items[0].SectionLabel)
	})

	t.Run("a content node with no resolved published version fails the whole copy", func(t *testing.T) {
		incomplete := map[string]string{"node-01": "version-node-01-v1"}

		_, err := domain.NewStudentPathFromTemplate("sp-1", "student-1", twoItemTemplate(), "teacher-1", assignedAt, incomplete)

		require.Error(t, err)
		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
	})
}

func twoNodeItems() []domain.LearningPathItem {
	return []domain.LearningPathItem{
		{Position: 1, ContentNodeID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo},
		{Position: 2, ContentNodeID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo},
	}
}

func TestBuildStudentPathItems_LanguageLocking(t *testing.T) {
	t.Run("prerequisite met and language available leaves the item unlocked", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string]bool{})

		assert.Equal(t, domain.CompletionStatusNotStarted, items[0].Status)
	})

	t.Run("prerequisite met but language missing locks the item", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string]bool{"node-01": true})

		assert.Equal(t, domain.CompletionStatusLocked, items[0].Status)
	})

	t.Run("prerequisite unmet but language available still locks the item", func(t *testing.T) {
		raw := map[string]domain.CompletionStatus{"node-01": domain.CompletionStatusNotStarted}
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), raw, map[string]bool{})

		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
	})

	t.Run("language lock on the first item cascades to lock the rest of the path", func(t *testing.T) {
		items, current := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string]bool{"node-01": true})

		require.Len(t, items, 2)
		assert.Equal(t, domain.CompletionStatusLocked, items[0].Status)
		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
		assert.Equal(t, 1, current)
	})

	t.Run("an item with no langLocked entry is never locked for language reasons", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, nil)

		assert.Equal(t, domain.CompletionStatusNotStarted, items[0].Status)
	})

	t.Run("a completed item stays completed even if later marked language-locked in the map", func(t *testing.T) {
		raw := map[string]domain.CompletionStatus{"node-01": domain.CompletionStatusCompleted}
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), raw, map[string]bool{"node-02": true})

		assert.Equal(t, domain.CompletionStatusCompleted, items[0].Status)
		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
	})
}
