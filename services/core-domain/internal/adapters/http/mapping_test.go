package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToCourseCatalogEntry(t *testing.T) {
	publishedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	creator := "0b6b8b8e-6f5b-4f7a-9c53-2a5f7a3a1111"
	live := domain.Course{
		ID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f2222", CreatedBy: creator,
		Title: "Edited Title", Summary: "Edited summary.", Level: domain.DifficultyLevelExpert,
		Status: domain.CourseStatusPublished,
	}
	version := domain.CourseVersion{
		CourseID: live.ID, VersionNumber: 1, PublishedAt: publishedAt,
		TitleSnapshot: "Published Title", SummarySnapshot: "Published summary.", LevelSnapshot: domain.DifficultyLevelBeginner,
	}
	names := userNames{creator: "Ana Souza"}
	t.Run("the learner catalog shows the latest published version's title, summary and level", func(t *testing.T) {
		entry := toCourseCatalogEntry(live, learnerCatalogView, &version, names)

		assert.Equal(t, "Published Title", entry.Title)
		assert.Equal(t, "Published summary.", entry.Summary)
		assert.EqualValues(t, domain.DifficultyLevelBeginner, entry.Level)
		assert.Nil(t, entry.HasUnpublishedChanges)
	})

	t.Run("the authoring list shows the live draft and whether it has unpublished changes", func(t *testing.T) {
		entry := toCourseCatalogEntry(live, authoringListView, &version, names)

		assert.Equal(t, "Edited Title", entry.Title)
		assert.EqualValues(t, domain.DifficultyLevelExpert, entry.Level)
		require.NotNil(t, entry.HasUnpublishedChanges)
		assert.True(t, *entry.HasUnpublishedChanges)
	})

	t.Run("every entry names the course's creator", func(t *testing.T) {
		want := generated.UserRef{UserId: uuid.MustParse(creator), DisplayName: "Ana Souza"}
		assert.Equal(t, want, toCourseCatalogEntry(live, learnerCatalogView, &version, names).CreatedBy)
		assert.Equal(t, want, toCourseCatalogEntry(live, authoringListView, &version, names).CreatedBy)
	})

	t.Run("an unpublished course has no published_at", func(t *testing.T) {
		entry := toCourseCatalogEntry(live, authoringListView, nil, names)

		assert.Nil(t, entry.PublishedAt)
	})
}

// TestUserRefMapping covers every response field that points at a user:
// each carries the user's id and the name looked up for that id, never a
// bare id.
func TestUserRefMapping(t *testing.T) {
	teacherID, studentID := uuid.New(), uuid.New()
	names := userNames{teacherID.String(): "Bob Ferreira", studentID.String(): "Alice Martins"}
	teacher := generated.UserRef{UserId: teacherID, DisplayName: "Bob Ferreira"}
	student := generated.UserRef{UserId: studentID, DisplayName: "Alice Martins"}
	at := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	courseID := uuid.NewString()

	tests := []struct {
		name string
		got  func() generated.UserRef
		want generated.UserRef
	}{
		{
			name: "content node teacher",
			got: func() generated.UserRef {
				return toContentNode(domain.ContentNode{ID: uuid.NewString(), TeacherID: teacherID.String(), CreatedAt: at}, names).Teacher
			},
			want: teacher,
		},
		{
			name: "learning path teacher",
			got: func() generated.UserRef {
				return toLearningPath(domain.LearningPath{ID: uuid.NewString(), TeacherID: teacherID.String(), CreatedAt: at}, names).Teacher
			},
			want: teacher,
		},
		{
			name: "student path student",
			got: func() generated.UserRef {
				return toStudentPath(domain.StudentPath{ID: uuid.NewString(), StudentID: studentID.String(), SourceTemplateID: uuid.NewString(), AssignedBy: teacherID.String()}, names).Student
			},
			want: student,
		},
		{
			name: "student path assigner",
			got: func() generated.UserRef {
				return toStudentPath(domain.StudentPath{ID: uuid.NewString(), StudentID: studentID.String(), SourceTemplateID: uuid.NewString(), AssignedBy: teacherID.String()}, names).AssignedBy
			},
			want: teacher,
		},
		{
			name: "course creator",
			got: func() generated.UserRef {
				return toCourse(domain.Course{ID: courseID, CreatedBy: teacherID.String(), Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusDraft}, nil, names).CreatedBy
			},
			want: teacher,
		},
		{
			name: "diagram creator",
			got: func() generated.UserRef {
				return toGeneratedDiagram(domain.Diagram{ID: uuid.NewString(), InstrumentID: uuid.NewString(), Kind: domain.DiagramKindCustom, CreatedBy: teacherID.String(), LabelDisplay: domain.LabelDisplayInterval, CreatedAt: at}, names).CreatedBy
			},
			want: teacher,
		},
		{
			name: "course enrollment student",
			got: func() generated.UserRef {
				return toCourseEnrollment(domain.CourseEnrollment{ID: uuid.NewString(), StudentID: studentID.String(), CourseID: courseID, Status: domain.CourseEnrollmentStatusActive}, names).Student
			},
			want: student,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got())
		})
	}
}

func TestToUserProfile_IncludesDisplayName(t *testing.T) {
	user := domain.User{ID: uuid.NewString(), Role: domain.RoleTeacher, DisplayName: "Bob Ferreira", Locale: domain.Language{Code: "en"}}

	assert.Equal(t, "Bob Ferreira", toUserProfile(user).DisplayName)
}
