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

func newMediaService(exercises *fakeExerciseRepository, storage *fakeMediaStorage) *application.MediaService {
	return application.NewMediaService(exercises, storage, idSequence())
}

func TestMediaService_CreateUploadURL(t *testing.T) {
	t.Run("a teacher requests an upload URL for an exercise asset", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		storage := newFakeMediaStorage()
		svc := newMediaService(exercises, storage)
		exerciseID := "exercise-1"

		url, err := svc.CreateUploadURL(context.Background(), teacherCaller(), domain.MediaUploadPurposeExerciseAsset, &exerciseID, domain.MediaContentTypeImage, "diagram.png")

		require.NoError(t, err)
		assert.NotEmpty(t, url.UploadURL)
		assert.NotEmpty(t, url.ObjectURL)
		assert.False(t, url.ExpiresAt.IsZero())
	})

	t.Run("an admin requests an upload URL for the predefined image library", func(t *testing.T) {
		svc := newMediaService(newFakeExerciseRepository(), newFakeMediaStorage())

		url, err := svc.CreateUploadURL(context.Background(), adminCaller(), domain.MediaUploadPurposeLibraryAsset, nil, domain.MediaContentTypeImage, "c-major-scale.png")

		require.NoError(t, err)
		assert.NotEmpty(t, url.UploadURL)
	})

	t.Run("requesting an exercise_asset upload URL without an exercise_id is rejected", func(t *testing.T) {
		svc := newMediaService(newFakeExerciseRepository(), newFakeMediaStorage())

		_, err := svc.CreateUploadURL(context.Background(), teacherCaller(), domain.MediaUploadPurposeExerciseAsset, nil, domain.MediaContentTypeImage, "diagram.png")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_id")
	})

	t.Run("requesting an upload URL without a content type is rejected", func(t *testing.T) {
		svc := newMediaService(newFakeExerciseRepository(), newFakeMediaStorage())

		_, err := svc.CreateUploadURL(context.Background(), teacherCaller(), domain.MediaUploadPurposeLibraryAsset, nil, "", "diagram.png")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "content_type")
	})

	t.Run("requesting an exercise_asset upload URL for a non-existent exercise returns not found", func(t *testing.T) {
		svc := newMediaService(newFakeExerciseRepository(), newFakeMediaStorage())
		missing := "missing"

		_, err := svc.CreateUploadURL(context.Background(), teacherCaller(), domain.MediaUploadPurposeExerciseAsset, &missing, domain.MediaContentTypeImage, "diagram.png")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot request a media upload URL", func(t *testing.T) {
		svc := newMediaService(newFakeExerciseRepository(), newFakeMediaStorage())

		_, err := svc.CreateUploadURL(context.Background(), studentCaller(), domain.MediaUploadPurposeLibraryAsset, nil, domain.MediaContentTypeImage, "diagram.png")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}
