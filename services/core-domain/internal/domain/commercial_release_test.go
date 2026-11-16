package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

const (
	releasePathID     = "2b3c4d5e-0000-4000-8000-0000000000c1"
	releaseLessonID   = "2b3c4d5e-0000-4000-8000-0000000000c2"
	releaseExerciseID = "2b3c4d5e-0000-4000-8000-0000000000c3"
	releaseEvidence   = "2b3c4d5e-0000-4000-8000-0000000000e1"
	releaseID1        = "2b3c4d5e-0000-4000-8000-0000000000f1"
	releaseID2        = "2b3c4d5e-0000-4000-8000-0000000000f2"
)

func pathSnapshot(title string) domain.ReleaseSnapshot {
	summary := "Shell voicings and guide tones"
	return domain.ReleaseSnapshot{
		Content:   domain.ContentRef{Kind: domain.ContentPath, ID: releasePathID},
		Title:     title,
		Published: true,
		State: domain.PathReleaseState{
			Title: title, Summary: &summary, Instruments: []string{"guitar"},
			Items: []domain.PathReleaseItem{{LessonID: releaseLessonID}},
		},
		Children: []domain.ContentRef{{Kind: domain.ContentContentNode, ID: releaseLessonID}},
	}
}

func teacherOwnership(content domain.ContentRef, ownerID string) domain.ContentOwnership {
	return domain.ContentOwnership{
		Content: content, OwnerKind: domain.OwnerTeacher, OwnerID: &ownerID, CreatorID: &ownerID,
		Source: domain.SourceNative, EvidenceEntryID: ownershipEvidenceID, Revision: 1,
	}
}

func TestReleaseSnapshotContentSHA256(t *testing.T) {
	first, err := pathSnapshot("Jazz foundations").ContentSHA256()
	require.NoError(t, err)
	assert.Regexp(t, "^[a-f0-9]{64}$", first)

	again, err := pathSnapshot("Jazz foundations").ContentSHA256()
	require.NoError(t, err)
	assert.Equal(t, first, again, "the same learner-visible state has the same digest")

	renamed, err := pathSnapshot("Jazz foundations I").ContentSHA256()
	require.NoError(t, err)
	assert.NotEqual(t, first, renamed, "a learner-visible edit changes the digest")

	reordered := pathSnapshot("Jazz foundations")
	reordered.State = domain.PathReleaseState{
		Title: "Jazz foundations", Instruments: []string{"guitar"},
		Items: []domain.PathReleaseItem{{LessonID: releaseLessonID}, {LessonID: releaseExerciseID}},
	}
	other, err := reordered.ContentSHA256()
	require.NoError(t, err)
	assert.NotEqual(t, first, other)
}

func TestReleaseSnapshotContentSHA256CoversTheVersion(t *testing.T) {
	v1, v2 := "version-1", "version-2"
	lesson := func(version *string) domain.ReleaseSnapshot {
		return domain.ReleaseSnapshot{
			Content: domain.ContentRef{Kind: domain.ContentContentNode, ID: releaseLessonID}, Title: "Shells",
			Published: true, VersionRef: version, State: domain.LessonReleaseState{Title: "Shells", ContentType: "text"},
		}
	}
	first, err := lesson(&v1).ContentSHA256()
	require.NoError(t, err)
	second, err := lesson(&v2).ContentSHA256()
	require.NoError(t, err)
	assert.NotEqual(t, first, second, "a new published version is a new released state")
}

func TestCheckReleasable(t *testing.T) {
	bea := domain.ContentRef{Kind: domain.ContentPath, ID: releasePathID}
	root := teacherOwnership(bea, beaID)
	lessonRef := domain.ContentRef{Kind: domain.ContentContentNode, ID: releaseLessonID}

	t.Run("a published item with the root's rights holder can be released", func(t *testing.T) {
		require.NoError(t, domain.CheckReleasable(pathSnapshot("Jazz foundations"), root, root))
	})
	t.Run("an unpublished item is refused", func(t *testing.T) {
		draft := pathSnapshot("Jazz foundations")
		draft.Published = false
		err := domain.CheckReleasable(draft, root, root)
		assert.ErrorIs(t, err, domain.ErrConflict)
		assert.ErrorContains(t, err, "not published")
		assert.ErrorContains(t, err, "Jazz foundations")
	})
	t.Run("unresolved ownership is refused", func(t *testing.T) {
		unresolved := domain.ContentOwnership{Content: bea, OwnerKind: domain.OwnerUnresolved, Revision: 1}
		err := domain.CheckReleasable(pathSnapshot("Jazz foundations"), unresolved, unresolved)
		assert.ErrorIs(t, err, domain.ErrConflict)
		assert.ErrorContains(t, err, "unresolved")
	})
	t.Run("a dependency with another rights holder is refused, naming it", func(t *testing.T) {
		lesson := domain.ReleaseSnapshot{Content: lessonRef, Title: "Open C chord", Published: true, State: domain.LessonReleaseState{Title: "Open C chord"}}
		platform := domain.ContentOwnership{Content: lessonRef, OwnerKind: domain.OwnerPlatform, OwnerID: ptr(domain.CatalogProfileID), Revision: 1}
		err := domain.CheckReleasable(lesson, platform, root)
		assert.ErrorIs(t, err, domain.ErrConflict)
		assert.ErrorContains(t, err, "Open C chord")
		assert.ErrorContains(t, err, releaseLessonID)
	})
	t.Run("another teacher's dependency is refused", func(t *testing.T) {
		lesson := domain.ReleaseSnapshot{Content: lessonRef, Title: "Shells", Published: true, State: domain.LessonReleaseState{Title: "Shells"}}
		err := domain.CheckReleasable(lesson, teacherOwnership(lessonRef, adaID), root)
		assert.ErrorIs(t, err, domain.ErrConflict)
	})
}

func TestReleaseManifestSHA256(t *testing.T) {
	content := domain.ContentRef{Kind: domain.ContentPath, ID: releasePathID}
	ownership := teacherOwnership(content, beaID)
	dependency := domain.ReleaseDependency{
		Content:       domain.ContentRef{Kind: domain.ContentContentNode, ID: releaseLessonID},
		ContentSHA256: "aa", ReleaseID: releaseID1, Permission: domain.PermissionOwn, EvidenceEntryID: ownershipEvidenceID,
	}
	manifest := func(contentSHA string, owner domain.ContentOwnership, deps ...domain.ReleaseDependency) string {
		t.Helper()
		digest, err := domain.ReleaseManifestSHA256(content, contentSHA, nil, owner, deps)
		require.NoError(t, err)
		return digest
	}

	base := manifest("c1", ownership, dependency)
	assert.Regexp(t, "^[a-f0-9]{64}$", base)
	assert.Equal(t, base, manifest("c1", ownership, dependency))
	assert.NotEqual(t, base, manifest("c2", ownership, dependency), "the content digest is covered")

	moved := dependency
	moved.ReleaseID = releaseID2
	assert.NotEqual(t, base, manifest("c1", ownership, moved), "dependency releases are covered")

	corrected := ownership
	corrected.OwnerID = ptr(adaID)
	corrected.Revision = 2
	assert.NotEqual(t, base, manifest("c1", corrected, dependency), "the rights holder is covered")

	reEvidenced := ownership
	reEvidenced.EvidenceEntryID = releaseEvidence
	assert.Equal(t, base, manifest("c1", reEvidenced, dependency), "evidence ids are not part of what was offered")
}

func TestNextRelease(t *testing.T) {
	content := domain.ContentRef{Kind: domain.ContentPath, ID: releasePathID}
	candidate := domain.CommercialRelease{
		ReleaseID: releaseID2, Content: content, ManifestSHA256: "m1", ContentSHA256: "c1",
		Ownership: teacherOwnership(content, beaID), EvidenceEntryID: releaseEvidence, CreatedAt: time.Unix(0, 0),
	}

	t.Run("the first release is revision 1", func(t *testing.T) {
		got, created := domain.NextRelease(nil, candidate)
		assert.True(t, created)
		assert.Equal(t, 1, got.Revision)
		assert.Equal(t, releaseID2, got.ReleaseID)
	})
	t.Run("an unchanged manifest returns the latest release", func(t *testing.T) {
		latest := candidate
		latest.ReleaseID, latest.Revision = releaseID1, 1
		got, created := domain.NextRelease(&latest, candidate)
		assert.False(t, created)
		assert.Equal(t, latest, got)
	})
	t.Run("a new manifest is the next revision", func(t *testing.T) {
		latest := candidate
		latest.ReleaseID, latest.Revision, latest.ManifestSHA256 = releaseID1, 3, "m0"
		got, created := domain.NextRelease(&latest, candidate)
		assert.True(t, created)
		assert.Equal(t, 4, got.Revision)
	})
}

func TestCommercialStateAdvance(t *testing.T) {
	content := domain.ContentRef{Kind: domain.ContentPath, ID: releasePathID}
	state := domain.NewCommercialState(content)
	assert.Equal(t, 1, state.Revision)
	assert.Nil(t, state.LatestReleaseID)

	_, err := state.Advance(2, releaseID1)
	assert.ErrorIs(t, err, domain.ErrConflict, "a stale or future observed revision is refused")

	next, err := state.Advance(1, releaseID1)
	require.NoError(t, err)
	assert.Equal(t, 2, next.Revision)
	require.NotNil(t, next.LatestReleaseID)
	assert.Equal(t, releaseID1, *next.LatestReleaseID)

	assert.Equal(t, 3, next.AdvancedTo(releaseID2).Revision, "a dependency's state advances without an observed revision")
}
