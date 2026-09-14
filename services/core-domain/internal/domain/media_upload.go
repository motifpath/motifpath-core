package domain

import "time"

// MediaUploadPurpose identifies what an uploaded object is for, which
// determines where it is stored (ADR-021's bucket layout).
type MediaUploadPurpose string

const (
	MediaUploadPurposeExerciseAsset MediaUploadPurpose = "exercise_asset"
	MediaUploadPurposeLibraryAsset  MediaUploadPurpose = "library_asset"
)

// MediaContentType is the media type of the file being uploaded.
type MediaContentType string

const (
	MediaContentTypeImage MediaContentType = "image"
	MediaContentTypeAudio MediaContentType = "audio"
)

// MediaUploadRequest is a validated request for a presigned media upload
// URL. It carries no persisted identity — nothing is stored until the
// caller actually PUTs a file to the issued URL.
type MediaUploadRequest struct {
	Purpose     MediaUploadPurpose
	ExerciseID  *string
	ContentType MediaContentType
	FileName    string
}

// MediaUploadURL is a presigned upload URL and the object's eventual read
// URL, per ADR-021's presigned-PUT flow.
type MediaUploadURL struct {
	UploadURL string
	ObjectURL string
	ExpiresAt time.Time
}

// NewMediaUploadRequest validates and constructs a MediaUploadRequest.
// Whether ExerciseID refers to an exercise that actually exists is an
// application-layer concern — it requires a repository round trip this
// constructor can't perform.
func NewMediaUploadRequest(purpose MediaUploadPurpose, exerciseID *string, contentType MediaContentType, fileName string) (MediaUploadRequest, error) {
	var errs []FieldError

	switch purpose {
	case MediaUploadPurposeExerciseAsset, MediaUploadPurposeLibraryAsset:
	default:
		errs = append(errs, FieldError{Field: "purpose", Reason: "must be one of exercise_asset, library_asset"})
	}

	switch contentType {
	case MediaContentTypeImage, MediaContentTypeAudio:
	default:
		errs = append(errs, FieldError{Field: "content_type", Reason: "must be one of image, audio"})
	}

	if fileName == "" {
		errs = append(errs, FieldError{Field: "file_name", Reason: "must not be empty"})
	}

	hasExerciseID := exerciseID != nil && *exerciseID != ""
	if purpose == MediaUploadPurposeExerciseAsset && !hasExerciseID {
		errs = append(errs, FieldError{Field: "exercise_id", Reason: "required when purpose is exercise_asset"})
	}
	if purpose == MediaUploadPurposeLibraryAsset && hasExerciseID {
		errs = append(errs, FieldError{Field: "exercise_id", Reason: "must be absent when purpose is library_asset"})
	}

	if len(errs) > 0 {
		return MediaUploadRequest{}, &ValidationError{Fields: errs}
	}

	return MediaUploadRequest{
		Purpose:     purpose,
		ExerciseID:  exerciseID,
		ContentType: contentType,
		FileName:    fileName,
	}, nil
}
