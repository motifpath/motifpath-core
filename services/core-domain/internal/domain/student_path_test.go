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

func TestStudentPath_DiffersFromTemplate(t *testing.T) {
	assignedAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	versions := map[string]string{"node-01": "v-01", "node-02": "v-02", "node-03": "v-03"}
	other := "Something else"
	level := domain.DifficultyLevelBeginner
	otherLevel := domain.DifficultyLevelIntermediate

	tests := []struct {
		name   string
		change func(*domain.LearningPath)
		want   bool
	}{
		{"an unchanged template", func(*domain.LearningPath) {}, false},
		{"a renamed template", func(p *domain.LearningPath) { p.Title = "Renamed" }, true},
		{"a lesson added", func(p *domain.LearningPath) {
			p.Items = append(p.Items, domain.LearningPathItem{Position: 3, ContentNodeID: "node-03"})
		}, true},
		{"a lesson removed", func(p *domain.LearningPath) { p.Items = p.Items[:1] }, true},
		{"lessons reordered", func(p *domain.LearningPath) {
			p.Items[0].ContentNodeID, p.Items[1].ContentNodeID = p.Items[1].ContentNodeID, p.Items[0].ContentNodeID
		}, true},
		{"a section label changed", func(p *domain.LearningPath) { p.Items[0].SectionLabel = &other }, true},
		{"a section label removed", func(p *domain.LearningPath) { p.Items[0].SectionLabel = nil }, true},
		{"a lesson's title changed (lessons are live, not copied)", func(p *domain.LearningPath) { p.Items[0].Title = "New title" }, false},
		{"a summary added", func(p *domain.LearningPath) { p.Summary = &other }, true},
		{"a level changed", func(p *domain.LearningPath) { p.Level = &otherLevel }, true},
		{"a thumbnail added", func(p *domain.LearningPath) { p.ThumbnailURL = &other }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := twoItemTemplate()
			template.Level = &level
			sp, err := domain.NewStudentPathFromTemplate("sp-1", "student-1", template, "teacher-1", assignedAt, versions)
			require.NoError(t, err)

			changed := twoItemTemplate()
			changed.Level = &level
			tc.change(&changed)

			assert.Equal(t, tc.want, sp.DiffersFromTemplate(changed))
		})
	}
}

func TestStudentPath_RefreshedFrom(t *testing.T) {
	assignedAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	archivedAt := assignedAt.Add(time.Hour)
	enrollmentID := "enrollment-1"
	position := 2
	summary := "New summary"
	thumbnail := "https://cdn.example/new.png"
	level := domain.DifficultyLevelIntermediate

	sp, err := domain.NewStudentPathFromTemplate("sp-1", "student-1", twoItemTemplate(), "teacher-9", assignedAt, map[string]string{"node-01": "v-01", "node-02": "v-02"})
	require.NoError(t, err)
	sp.ArchivedAt = &archivedAt
	sp.SourceCourseEnrollmentID = &enrollmentID
	sp.CourseCheckpointPosition = &position

	latest := twoItemTemplate()
	latest.Title = "Beginner Guitar, revised"
	latest.Summary = &summary
	latest.Level = &level
	latest.ThumbnailURL = &thumbnail
	latest.Items = []domain.LearningPathItem{
		{Position: 1, ContentNodeID: "node-02"},
		{Position: 2, ContentNodeID: "node-03"},
	}

	t.Run("takes the template's items, pinned to the given versions, and its presentation", func(t *testing.T) {
		refreshed, err := sp.RefreshedFrom(latest, map[string]string{"node-02": "v-02b", "node-03": "v-03"})
		require.NoError(t, err)

		assert.Equal(t, "Beginner Guitar, revised", refreshed.Title)
		assert.Equal(t, &summary, refreshed.SummarySnapshot)
		assert.Equal(t, &level, refreshed.LevelSnapshot)
		assert.Equal(t, &thumbnail, refreshed.ThumbnailURLSnapshot)
		require.Len(t, refreshed.Items, 2)
		assert.Equal(t, domain.StudentPathItemRecord{Position: 1, ContentNodeID: "node-02", ContentNodeVersionID: "v-02b"}, refreshed.Items[0])
		assert.Equal(t, domain.StudentPathItemRecord{Position: 2, ContentNodeID: "node-03", ContentNodeVersionID: "v-03"}, refreshed.Items[1])
		assert.False(t, refreshed.DiffersFromTemplate(latest))
	})

	t.Run("keeps who owns, assigned and archived the copy, and which course part it is", func(t *testing.T) {
		refreshed, err := sp.RefreshedFrom(latest, map[string]string{"node-02": "v-02b", "node-03": "v-03"})
		require.NoError(t, err)

		assert.Equal(t, "sp-1", refreshed.ID)
		assert.Equal(t, "student-1", refreshed.StudentID)
		assert.Equal(t, "template-1", refreshed.SourceTemplateID)
		assert.Equal(t, "teacher-9", refreshed.AssignedBy)
		assert.Equal(t, assignedAt, refreshed.AssignedAt)
		assert.Equal(t, &archivedAt, refreshed.ArchivedAt)
		assert.Equal(t, &enrollmentID, refreshed.SourceCourseEnrollmentID)
		assert.Equal(t, &position, refreshed.CourseCheckpointPosition)
	})

	t.Run("a content node with no resolved published version fails the refresh", func(t *testing.T) {
		_, err := sp.RefreshedFrom(latest, map[string]string{"node-02": "v-02b"})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
	})

	t.Run("leaves the original copy untouched", func(t *testing.T) {
		_, err := sp.RefreshedFrom(latest, map[string]string{"node-02": "v-02b", "node-03": "v-03"})
		require.NoError(t, err)

		assert.Equal(t, "Beginner Guitar", sp.Title)
		assert.Equal(t, "node-01", sp.Items[0].ContentNodeID)
	})
}

func twoNodeItems() []domain.LearningPathItem {
	return []domain.LearningPathItem{
		{Position: 1, ContentNodeID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo},
		{Position: 2, ContentNodeID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo},
	}
}

func TestBuildStudentPathItems_LanguageLocking(t *testing.T) {
	english := []domain.Language{{Code: "en"}}

	t.Run("prerequisite met and language available leaves the item unlocked", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string][]domain.Language{})

		assert.Equal(t, domain.CompletionStatusNotStarted, items[0].Status)
		assert.Nil(t, items[0].LockReason)
		assert.Nil(t, items[0].AvailableLanguages)
	})

	t.Run("prerequisite met but language missing locks the item for language, with the languages it has", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string][]domain.Language{"node-01": english})

		assert.Equal(t, domain.CompletionStatusLocked, items[0].Status)
		require.NotNil(t, items[0].LockReason)
		assert.Equal(t, domain.LockReasonLanguage, *items[0].LockReason)
		assert.Equal(t, english, items[0].AvailableLanguages)
	})

	t.Run("prerequisite unmet but language available locks the item for the previous step", func(t *testing.T) {
		raw := map[string]domain.CompletionStatus{"node-01": domain.CompletionStatusNotStarted}
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), raw, map[string][]domain.Language{})

		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
		require.NotNil(t, items[1].LockReason)
		assert.Equal(t, domain.LockReasonPreviousStep, *items[1].LockReason)
		assert.Nil(t, items[1].AvailableLanguages)
	})

	t.Run("prerequisite unmet and language missing locks the item for the previous step, without languages", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string][]domain.Language{"node-02": english})

		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
		require.NotNil(t, items[1].LockReason)
		assert.Equal(t, domain.LockReasonPreviousStep, *items[1].LockReason)
		assert.Nil(t, items[1].AvailableLanguages)
	})

	t.Run("language lock on the first item locks the rest for the previous step and stays current", func(t *testing.T) {
		items, current := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string][]domain.Language{"node-01": english})

		require.Len(t, items, 2)
		assert.Equal(t, domain.CompletionStatusLocked, items[0].Status)
		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
		require.NotNil(t, items[1].LockReason)
		assert.Equal(t, domain.LockReasonPreviousStep, *items[1].LockReason)
		assert.Equal(t, 1, current)
	})

	t.Run("an item with no language-lock entry is never locked for language reasons", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, nil)

		assert.Equal(t, domain.CompletionStatusNotStarted, items[0].Status)
	})

	t.Run("a completed item stays completed, with no lock reason, even if language-locked in the map", func(t *testing.T) {
		raw := map[string]domain.CompletionStatus{"node-01": domain.CompletionStatusCompleted}
		items, current := domain.BuildStudentPathItems(twoNodeItems(), raw, map[string][]domain.Language{"node-01": english, "node-02": english})

		assert.Equal(t, domain.CompletionStatusCompleted, items[0].Status)
		assert.Nil(t, items[0].LockReason)
		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
		require.NotNil(t, items[1].LockReason)
		assert.Equal(t, domain.LockReasonLanguage, *items[1].LockReason)
		assert.Equal(t, 2, current)
	})
}
