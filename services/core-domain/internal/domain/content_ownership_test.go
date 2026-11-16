package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

const (
	ownershipEvidenceID = "1a2b3c4d-0000-4000-8000-0000000000e1"
	correctionEvidence  = "1a2b3c4d-0000-4000-8000-0000000000e2"
	adaID = "1a2b3c4d-0000-4000-8000-0000000000a1"
	ownDanID = "1a2b3c4d-0000-4000-8000-0000000000d1"
	pathID              = "1a2b3c4d-0000-4000-8000-0000000000c1"
)

func ref(t *testing.T, kind domain.ContentKind) domain.ContentRef {
	t.Helper()
	return domain.ContentRef{Kind: kind, ID: pathID}
}


func TestParseContentKind(t *testing.T) {
	for _, name := range []string{"course", "path", "content_node", "exercise"} {
		kind, err := domain.ParseContentKind(name)
		require.NoError(t, err, name)
		assert.Equal(t, name, string(kind))
	}
	for _, name := range []string{"", "student_path", "Course"} {
		_, err := domain.ParseContentKind(name)
		assert.ErrorIs(t, err, domain.ErrValidation, name)
	}
}

func TestNativeOwnership(t *testing.T) {
	tests := map[string]struct {
		author        domain.User
		wantOwnerKind domain.OwnerKind
		wantOwnerID   string
	}{
		"an admin's content is owned by the platform": {
			author: domain.User{ID: adaID, Role: domain.RoleAdmin}, wantOwnerKind: domain.OwnerPlatform, wantOwnerID: domain.CatalogProfileID,
		},
		"a teacher's content is owned by that teacher": {
			author: domain.User{ID: beaID, Role: domain.RoleTeacher}, wantOwnerKind: domain.OwnerTeacher, wantOwnerID: beaID,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := domain.NativeOwnership(ref(t, domain.ContentPath), tc.author, ownershipEvidenceID)

			assert.Equal(t, tc.wantOwnerKind, got.OwnerKind)
			require.NotNil(t, got.OwnerID)
			assert.Equal(t, tc.wantOwnerID, *got.OwnerID)
			require.NotNil(t, got.CreatorID)
			assert.Equal(t, tc.author.ID, *got.CreatorID, "the creator is the author, even for platform content")
			assert.Equal(t, domain.SourceNative, got.Source)
			assert.Equal(t, 1, got.Revision)
			assert.Equal(t, ownershipEvidenceID, got.EvidenceEntryID)
			assert.Nil(t, got.CopiedFrom)
		})
	}
}

func TestLegacyOwnership(t *testing.T) {
	tests := map[string]struct {
		author        *domain.LegacyAuthor
		wantOwnerKind domain.OwnerKind
		wantOwnerID   *string
		wantCreatorID *string
	}{
		"an admin author gives platform ownership": {
			author:        &domain.LegacyAuthor{ID: adaID, IsAdmin: true},
			wantOwnerKind: domain.OwnerPlatform, wantOwnerID: strPtr(domain.CatalogProfileID), wantCreatorID: strPtr(adaID),
		},
		"the catalog profile gives platform ownership": {
			author:        &domain.LegacyAuthor{ID: domain.CatalogProfileID},
			wantOwnerKind: domain.OwnerPlatform, wantOwnerID: strPtr(domain.CatalogProfileID), wantCreatorID: strPtr(domain.CatalogProfileID),
		},
		"any other author gives teacher ownership": {
			author:        &domain.LegacyAuthor{ID: beaID},
			wantOwnerKind: domain.OwnerTeacher, wantOwnerID: strPtr(beaID), wantCreatorID: strPtr(beaID),
		},
		"no recorded author leaves it unresolved": {
			author:        nil,
			wantOwnerKind: domain.OwnerUnresolved, wantOwnerID: nil, wantCreatorID: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := domain.LegacyOwnership(ref(t, domain.ContentExercise), tc.author, ownershipEvidenceID)

			assert.Equal(t, tc.wantOwnerKind, got.OwnerKind)
			assert.Equal(t, tc.wantOwnerID, got.OwnerID)
			assert.Equal(t, tc.wantCreatorID, got.CreatorID)
			assert.Equal(t, domain.SourceReviewedLegacy, got.Source)
			assert.Equal(t, 1, got.Revision)
		})
	}
}

func TestContentOwnershipCorrected(t *testing.T) {
	current := domain.NativeOwnership(ref(t, domain.ContentPath), domain.User{ID: adaID, Role: domain.RoleAdmin}, ownershipEvidenceID)

	t.Run("a correction moves the rights holder, keeps the creator and adds one revision", func(t *testing.T) {
		got, err := current.Corrected(domain.OwnershipCorrection{
			ExpectedRevision: 1, OwnerKind: domain.OwnerTeacher, OwnerID: beaID,
		}, correctionEvidence)

		require.NoError(t, err)
		assert.Equal(t, domain.OwnerTeacher, got.OwnerKind)
		assert.Equal(t, beaID, *got.OwnerID)
		assert.Equal(t, adaID, *got.CreatorID)
		assert.Equal(t, domain.SourceNative, got.Source)
		assert.Equal(t, 2, got.Revision)
		assert.Equal(t, correctionEvidence, got.EvidenceEntryID)
		assert.Equal(t, domain.OwnerPlatform, current.OwnerKind, "the earlier revision is not changed")
	})

	refusals := map[string]struct {
		correction domain.OwnershipCorrection
		want       error
	}{
		"a stale revision": {
			correction: domain.OwnershipCorrection{ExpectedRevision: 2, OwnerKind: domain.OwnerTeacher, OwnerID: beaID},
			want:       domain.ErrConflict,
		},
		"an unresolved target": {
			correction: domain.OwnershipCorrection{ExpectedRevision: 1, OwnerKind: domain.OwnerUnresolved, OwnerID: beaID},
			want:       domain.ErrValidation,
		},
		"the platform with an owner other than the catalog profile": {
			correction: domain.OwnershipCorrection{ExpectedRevision: 1, OwnerKind: domain.OwnerPlatform, OwnerID: ownDanID},
			want:       domain.ErrValidation,
		},
		"a teacher who is the catalog profile": {
			correction: domain.OwnershipCorrection{ExpectedRevision: 1, OwnerKind: domain.OwnerTeacher, OwnerID: domain.CatalogProfileID},
			want:       domain.ErrValidation,
		},
	}
	for name, tc := range refusals {
		t.Run("refuses "+name, func(t *testing.T) {
			_, err := current.Corrected(tc.correction, correctionEvidence)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
