//go:build integration

package repo

import (
	"testing"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// testOwnership is the ownership record a teacher's newly created item is
// stored with, built the way production builds it.
func testOwnership(t *testing.T, kind domain.ContentKind, id string) ports.OwnershipRecord {
	t.Helper()
	teacher := domain.User{ID: uuid.NewString(), Role: domain.RoleTeacher}
	return ownershipBy(t, kind, id, teacher)
}

// ownershipBy is the ownership record author's newly created item is stored
// with.
func ownershipBy(t *testing.T, kind domain.ContentKind, id string, author domain.User) ports.OwnershipRecord {
	t.Helper()
	recorder := application.NewOwnershipRecorder(commercialEnvironment(t, "test"), uuid.NewString)
	return recorder.Native(domain.ContentRef{Kind: kind, ID: id}, author, commercialAt)
}
