package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const (
	releaseCourseID   = "0c1a0000-0000-4000-8000-0000000001c0"
	releaseLessonID   = "0c1a0000-0000-4000-8000-0000000001c2"
	releaseExerciseID = "0c1a0000-0000-4000-8000-0000000001c3"
)

var (
	releasePath     = domain.ContentRef{Kind: domain.ContentPath, ID: contentPathID}
	releaseLesson   = domain.ContentRef{Kind: domain.ContentContentNode, ID: releaseLessonID}
	releaseExercise = domain.ContentRef{Kind: domain.ContentExercise, ID: releaseExerciseID}
	releaseCourse   = domain.ContentRef{Kind: domain.ContentCourse, ID: releaseCourseID}
)

// switchContentVersions turns publication of releases on or off for everyone.
// Releases need ownership to be rolled out first.
func (f commercialContentFixture) switchContentVersions(on bool) {
	for _, flag := range []domain.FlagName{"content_ownership", "content_versions"} {
		f.store.state.flags[flag] = domain.FlagSetting{Enabled: on, Audience: domain.RolloutAudience{AllUsers: on}}
	}
}

// publishedPath stores Bea's published path "title", whose lesson "Shells"
// contains her exercise "Shell voicings in C".
func (f commercialContentFixture) publishedPath(t *testing.T, title string, author domain.User) {
	t.Helper()
	f.own(t, domain.ContentPath, releasePath.ID, author)
	f.own(t, domain.ContentContentNode, releaseLesson.ID, author)
	f.own(t, domain.ContentExercise, releaseExercise.ID, author)
	f.setPathTitle(title)
	version := "lesson-version-1"
	f.store.snapshots[releaseLesson] = domain.ReleaseSnapshot{
		Content: releaseLesson, Title: "Shells", Published: true, VersionRef: &version,
		State:    domain.LessonReleaseState{VersionNumber: 1, Title: "Shells", ContentType: "text"},
		Children: []domain.ContentRef{releaseExercise},
	}
	f.store.snapshots[releaseExercise] = domain.ReleaseSnapshot{
		Content: releaseExercise, Title: "Shell voicings in C", Published: true,
		State: domain.ExerciseReleaseState{Title: "Shell voicings in C", Prompt: "Play the shells", ExerciseType: "multiple_choice"},
	}
}

func (f commercialContentFixture) setPathTitle(title string) {
	f.store.snapshots[releasePath] = domain.ReleaseSnapshot{
		Content: releasePath, Title: title, Published: true,
		State:    domain.PathReleaseState{Title: title, Items: []domain.PathReleaseItem{{LessonID: releaseLessonID}}},
		Children: []domain.ContentRef{releaseLesson},
	}
}

func (f commercialContentFixture) publish(ctx context.Context, caller domain.User, content domain.ContentRef, expectedRevision int) (domain.CommercialRelease, error) {
	return f.service.PublishRelease(ctx, caller, string(content.Kind), content.ID, uuid.NewString(),
		application.PublishReleaseRequest{ExpectedRevision: expectedRevision, Reason: "Offer for sale"})
}

func (f commercialContentFixture) releasesOf(content domain.ContentRef) []domain.CommercialRelease {
	var found []domain.CommercialRelease
	for _, release := range f.store.state.releases {
		if release.Content == content {
			found = append(found, release)
		}
	}
	return found
}

func TestCommercialContentService_PublishRelease(t *testing.T) {
	ctx := context.Background()

	t.Run("the owner releases a path with its lesson and exercise", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "Jazz foundations", contentBea)

		release, err := f.publish(ctx, contentBea, releasePath, 1)

		require.NoError(t, err)
		assert.Equal(t, 1, release.Revision)
		assert.Equal(t, releasePath, release.Content)
		assert.Equal(t, contentBeaID, *release.Ownership.OwnerID)
		assert.Nil(t, release.CoreVersionRef)
		require.Len(t, release.Dependencies, 2)
		assert.Equal(t, releaseLesson, release.Dependencies[0].Content)
		assert.Equal(t, releaseExercise, release.Dependencies[1].Content)
		for _, dependency := range release.Dependencies {
			assert.Equal(t, domain.PermissionOwn, dependency.Permission)
			stored, err := f.store.Release(ctx, dependency.ReleaseID)
			require.NoError(t, err, "each dependency has its own release")
			assert.Equal(t, 1, stored.Revision)
			assert.Equal(t, dependency.ContentSHA256, stored.ContentSHA256)
		}
		lessonRelease := f.releasesOf(releaseLesson)[0]
		require.Len(t, lessonRelease.Dependencies, 1, "a lesson's release lists its exercise")

		content, err := f.service.Get(ctx, contentBea, "path", releasePath.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, content.Revision)
		require.NotNil(t, content.Release)
		assert.Equal(t, release.ReleaseID, content.Release.ReleaseID)
		assert.Contains(t, f.store.auditOutcomes(), domain.AuditOutcomeApplied)
	})

	t.Run("an admin releases a teacher's path; it records the teacher as owner", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "Jazz foundations", contentBea)

		release, err := f.publish(ctx, contentAdmin, releasePath, 1)

		require.NoError(t, err)
		assert.Equal(t, contentBeaID, *release.Ownership.OwnerID)
	})

	t.Run("a course is released at its published version with its paths", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "First chords", contentAdmin)
		f.own(t, domain.ContentCourse, releaseCourseID, contentAdmin)
		version := "course-version-2"
		f.store.snapshots[releaseCourse] = domain.ReleaseSnapshot{
			Content: releaseCourse, Title: "Guitar from zero", Published: true, VersionRef: &version,
			State:    domain.CourseReleaseState{VersionNumber: 2, Title: "Guitar from zero", Checkpoints: []domain.CourseReleaseCheckpoint{{PathID: releasePath.ID, EffectiveTitle: "First chords"}}},
			Children: []domain.ContentRef{releasePath},
		}

		release, err := f.publish(ctx, contentAdmin, releaseCourse, 1)

		require.NoError(t, err)
		require.NotNil(t, release.CoreVersionRef)
		assert.Equal(t, version, *release.CoreVersionRef)
		assert.Equal(t, domain.OwnerPlatform, release.Ownership.OwnerKind)
		require.Len(t, release.Dependencies, 3)
		assert.Equal(t, releasePath, release.Dependencies[0].Content)
	})

	t.Run("releasing unchanged content returns the latest release whatever revision was observed", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "Jazz foundations", contentBea)
		first, err := f.publish(ctx, contentBea, releasePath, 1)
		require.NoError(t, err)

		again, err := f.publish(ctx, contentAdmin, releasePath, 1)

		require.NoError(t, err)
		assert.Equal(t, first, again)
		assert.Len(t, f.releasesOf(releasePath), 1)
		content, err := f.service.Get(ctx, contentBea, "path", releasePath.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, content.Revision)
	})

	t.Run("an edited path gets the next revision, keeping the earlier one and reusing unchanged dependencies", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "Jazz foundations", contentBea)
		first, err := f.publish(ctx, contentBea, releasePath, 1)
		require.NoError(t, err)
		f.setPathTitle("Jazz foundations I")

		second, err := f.publish(ctx, contentBea, releasePath, 2)

		require.NoError(t, err)
		assert.Equal(t, 2, second.Revision)
		assert.NotEqual(t, first.ContentSHA256, second.ContentSHA256)
		assert.Equal(t, first.DependencyReleaseIDs(), second.DependencyReleaseIDs(), "unchanged dependencies keep their releases")
		stored, err := f.store.Release(ctx, first.ReleaseID)
		require.NoError(t, err)
		assert.Equal(t, first, stored, "the earlier release is unchanged")
		assert.Len(t, f.releasesOf(releaseLesson), 1)
	})

	t.Run("a release after an ownership correction is a new manifest", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "Jazz foundations", contentAdmin)
		first, err := f.publish(ctx, contentAdmin, releasePath, 1)
		require.NoError(t, err)
		for _, ref := range []domain.ContentRef{releasePath, releaseLesson, releaseExercise} {
			corrected := f.store.state.ownership[ref][0]
			corrected.OwnerKind, corrected.OwnerID, corrected.Revision = domain.OwnerTeacher, ptrTo(contentBeaID), 2
			f.store.state.ownership[ref] = append(f.store.state.ownership[ref], corrected)
		}

		second, err := f.publish(ctx, contentBea, releasePath, 2)

		require.NoError(t, err)
		assert.Equal(t, 2, second.Revision)
		assert.Equal(t, domain.CatalogProfileID, *first.Ownership.OwnerID)
		assert.Equal(t, contentBeaID, *second.Ownership.OwnerID)
	})

	t.Run("a same-intent replay returns the original release", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "Jazz foundations", contentBea)
		key := uuid.NewString()
		request := application.PublishReleaseRequest{ExpectedRevision: 1, Reason: "Offer for sale"}
		first, err := f.service.PublishRelease(ctx, contentBea, "path", releasePath.ID, key, request)
		require.NoError(t, err)
		f.setPathTitle("Jazz foundations I")

		replayed, err := f.service.PublishRelease(ctx, contentBea, "path", releasePath.ID, key, request)

		require.NoError(t, err)
		assert.Equal(t, first, replayed)
		assert.Len(t, f.releasesOf(releasePath), 1)
	})

	refusals := map[string]struct {
		arrange func(t *testing.T, f commercialContentFixture)
		caller  domain.User
		expect  int
		wantErr error
		message string
	}{
		"another teacher": {
			caller: contentDan, expect: 1, wantErr: domain.ErrForbidden,
		},
		"another teacher, before anything about the item's dependencies is revealed": {
			arrange: func(_ *testing.T, f commercialContentFixture) { f.store.deleted[releaseLesson] = true },
			caller:  contentDan, expect: 1, wantErr: domain.ErrForbidden,
		},
		"a student": {
			caller: contentStudent, expect: 1, wantErr: domain.ErrForbidden,
		},
		"while the rollout is OFF": {
			arrange: func(_ *testing.T, f commercialContentFixture) { f.switchContentVersions(false) },
			caller:  contentBea, expect: 1, wantErr: domain.ErrForbidden,
		},
		"a draft path": {
			arrange: func(_ *testing.T, f commercialContentFixture) {
				draft := f.store.snapshots[releasePath]
				draft.Published = false
				f.store.snapshots[releasePath] = draft
			},
			caller: contentBea, expect: 1, wantErr: domain.ErrConflict, message: "not published",
		},
		"a lesson without a published version": {
			arrange: func(_ *testing.T, f commercialContentFixture) {
				draft := f.store.snapshots[releaseLesson]
				draft.Published, draft.VersionRef = false, nil
				f.store.snapshots[releaseLesson] = draft
			},
			caller: contentBea, expect: 1, wantErr: domain.ErrConflict, message: "Shells",
		},
		"a dependency with unresolved ownership": {
			arrange: func(t *testing.T, f commercialContentFixture) {
				f.ownUnresolved(t, domain.ContentExercise, releaseExercise.ID)
			},
			caller: contentAdmin, expect: 1, wantErr: domain.ErrConflict, message: "unresolved",
		},
		"a dependency with another rights holder": {
			arrange: func(t *testing.T, f commercialContentFixture) {
				f.own(t, domain.ContentExercise, releaseExercise.ID, contentAdmin)
			},
			caller: contentBea, expect: 1, wantErr: domain.ErrConflict, message: "Shell voicings in C",
		},
		"a stated revision the item is not at": {
			caller: contentBea, expect: 2, wantErr: domain.ErrConflict,
		},
		"a deleted path": {
			arrange: func(_ *testing.T, f commercialContentFixture) { f.store.deleted[releasePath] = true },
			caller:  contentAdmin, expect: 1, wantErr: domain.ErrNotFound,
		},
	}
	for name, tc := range refusals {
		t.Run("refused: "+name, func(t *testing.T) {
			f := newCommercialContentFixture(t)
			f.switchContentVersions(true)
			f.publishedPath(t, "Jazz foundations", contentBea)
			if tc.arrange != nil {
				tc.arrange(t, f)
			}

			_, err := f.publish(ctx, tc.caller, releasePath, tc.expect)

			assert.ErrorIs(t, err, tc.wantErr)
			if tc.message != "" {
				assert.ErrorContains(t, err, tc.message)
			}
			assert.Empty(t, f.store.state.releases, "a refusal releases nothing, not even a dependency")
			assert.Contains(t, f.store.auditOutcomes(), domain.AuditOutcomeDenied)
		})
	}

	t.Run("a request without a reason is invalid", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "Jazz foundations", contentBea)

		_, err := f.service.PublishRelease(ctx, contentBea, "path", releasePath.ID, uuid.NewString(), application.PublishReleaseRequest{ExpectedRevision: 1})

		assert.ErrorIs(t, err, domain.ErrValidation)
	})
}

func TestCommercialContentService_GetRelease(t *testing.T) {
	ctx := context.Background()
	released := func(t *testing.T) (commercialContentFixture, domain.CommercialRelease) {
		t.Helper()
		f := newCommercialContentFixture(t)
		f.switchContentVersions(true)
		f.publishedPath(t, "Jazz foundations", contentBea)
		release, err := f.publish(ctx, contentBea, releasePath, 1)
		require.NoError(t, err)
		return f, release
	}

	readers := map[string]domain.User{"the owner": contentBea, "an admin": contentAdmin}
	for name, caller := range readers {
		t.Run(name+" reads the release", func(t *testing.T) {
			f, release := released(t)

			got, err := f.service.GetRelease(ctx, caller, release.ReleaseID)

			require.NoError(t, err)
			assert.Equal(t, release, got)
		})
	}

	hidden := map[string]domain.User{"another teacher": contentDan, "a student": contentStudent}
	for name, caller := range hidden {
		t.Run(name+" cannot tell the release exists", func(t *testing.T) {
			f, release := released(t)

			_, err := f.service.GetRelease(ctx, caller, release.ReleaseID)

			assert.ErrorIs(t, err, domain.ErrNotFound)
		})
	}

	t.Run("the recorded owner and the new owner both read it after an ownership correction", func(t *testing.T) {
		f, release := released(t)
		corrected := f.store.state.ownership[releasePath][0]
		corrected.OwnerID, corrected.Revision = ptrTo(contentDanID), 2
		f.store.state.ownership[releasePath] = append(f.store.state.ownership[releasePath], corrected)

		for _, caller := range []domain.User{contentBea, contentDan} {
			got, err := f.service.GetRelease(ctx, caller, release.ReleaseID)
			require.NoError(t, err, caller.ID)
			assert.Equal(t, contentBeaID, *got.Ownership.OwnerID)
		}
	})

	t.Run("it stays readable after the path is deleted and while the rollout is OFF", func(t *testing.T) {
		f, release := released(t)
		f.store.deleted[releasePath] = true
		f.switchContentVersions(false)

		got, err := f.service.GetRelease(ctx, contentAdmin, release.ReleaseID)

		require.NoError(t, err)
		assert.Equal(t, release, got)
	})

	t.Run("an unknown release is not found", func(t *testing.T) {
		f := newCommercialContentFixture(t)

		_, err := f.service.GetRelease(ctx, contentAdmin, contentUnknownID)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
