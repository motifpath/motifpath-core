package application

import (
	"context"
	"path/filepath"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// MediaService issues presigned upload URLs for content-authoring media. It
// never touches the uploaded bytes — the caller PUTs directly to the
// returned upload_url.
type MediaService struct {
	exercises ports.ExerciseRepository
	storage   ports.MediaStorage
	newID     func() string
}

func NewMediaService(exercises ports.ExerciseRepository, storage ports.MediaStorage, newID func() string) *MediaService {
	return &MediaService{exercises: exercises, storage: storage, newID: newID}
}

// CreateUploadURL issues a presigned upload URL for the given purpose. Only
// teachers and admins may request one. When purpose is exercise_asset, the
// referenced exercise must exist.
func (s *MediaService) CreateUploadURL(ctx context.Context, caller domain.User, purpose domain.MediaUploadPurpose, exerciseID *string, contentType domain.MediaContentType, fileName string) (domain.MediaUploadURL, error) {
	if !canManageContent(caller.Role) {
		return domain.MediaUploadURL{}, domain.ErrForbidden
	}

	req, err := domain.NewMediaUploadRequest(purpose, exerciseID, contentType, fileName)
	if err != nil {
		return domain.MediaUploadURL{}, err
	}

	if req.Purpose == domain.MediaUploadPurposeExerciseAsset {
		if _, err := s.exercises.GetByID(ctx, *req.ExerciseID); err != nil {
			return domain.MediaUploadURL{}, err
		}
	}

	return s.storage.PresignUpload(ctx, s.objectKey(req), req.ContentType)
}

// objectKey lays objects out as exercises/{exercise_id}/... for
// exercise-specific uploads, library/... for the shared image-picker
// library. The stored key never reuses the caller's file name — only its
// extension — so two uploads with the same original name never collide.
func (s *MediaService) objectKey(req domain.MediaUploadRequest) string {
	ext := filepath.Ext(req.FileName)
	id := s.newID()
	if req.Purpose == domain.MediaUploadPurposeExerciseAsset {
		return "exercises/" + *req.ExerciseID + "/" + id + ext
	}
	return "library/" + id + ext
}
