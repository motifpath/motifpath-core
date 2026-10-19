package domain

import (
	"fmt"
	"strings"
	"time"
)

// MediaUploadPurpose identifies what an uploaded object is for, which
// determines where it is stored.
type MediaUploadPurpose string

const (
	MediaUploadPurposeExerciseAsset MediaUploadPurpose = "exercise_asset"
	MediaUploadPurposeLibraryAsset  MediaUploadPurpose = "library_asset"
	// MediaUploadPurposeThumbnail is an image shown for a course, learning
	// path or content node in lists and cards.
	MediaUploadPurposeThumbnail MediaUploadPurpose = "thumbnail"
	// MediaUploadPurposeLessonVideo is the MP4 a video content node plays.
	// It is the only purpose that accepts video.
	MediaUploadPurposeLessonVideo MediaUploadPurpose = "lesson_video"
)

// MaxVideoUploadBytes is the largest video a teacher may upload: 1 GB. A
// single upload of that size still fits comfortably under object storage's
// one-request limit, and a lesson longer than that is better split in two.
const MaxVideoUploadBytes int64 = 1 << 30

// MediaContentType is the media type of the file being uploaded.
type MediaContentType string

const (
	MediaContentTypeImage MediaContentType = "image"
	MediaContentTypeAudio MediaContentType = "audio"
	// MediaContentTypeVideo is an MP4 file (H.264 video, AAC audio) — the
	// one video format every supported browser plays without transcoding.
	MediaContentTypeVideo MediaContentType = "video"
)

// MediaUploadRequest is a validated request for a presigned media upload
// URL. It carries no persisted identity — nothing is stored until the
// caller actually PUTs a file to the issued URL.
type MediaUploadRequest struct {
	Purpose     MediaUploadPurpose
	ExerciseID  *string
	ContentType MediaContentType
	FileName    string
	// FileSize is the declared size in bytes. When set, the issued URL
	// accepts an upload of exactly this size and nothing else.
	FileSize *int64
}

// MediaUploadURL is a presigned upload URL and the object's eventual read
// URL: the caller PUTs the file to UploadURL, then reads it back from
// ObjectURL.
type MediaUploadURL struct {
	UploadURL string
	ObjectURL string
	ExpiresAt time.Time
}

// NewMediaUploadRequest validates and constructs a MediaUploadRequest.
// Whether ExerciseID refers to an exercise that actually exists is an
// application-layer concern — it requires a repository round trip this
// constructor can't perform.
func NewMediaUploadRequest(purpose MediaUploadPurpose, exerciseID *string, contentType MediaContentType, fileName string, fileSize *int64) (MediaUploadRequest, error) {
	var errs []FieldError

	switch purpose {
	case MediaUploadPurposeExerciseAsset, MediaUploadPurposeLibraryAsset, MediaUploadPurposeThumbnail, MediaUploadPurposeLessonVideo:
	default:
		errs = append(errs, FieldError{Field: "purpose", Reason: "must be one of exercise_asset, library_asset, thumbnail, lesson_video"})
	}

	switch contentType {
	case MediaContentTypeImage, MediaContentTypeAudio, MediaContentTypeVideo:
	default:
		errs = append(errs, FieldError{Field: "content_type", Reason: "must be one of image, audio, video"})
	}

	if fileName == "" {
		errs = append(errs, FieldError{Field: "file_name", Reason: "must not be empty"})
	}

	hasExerciseID := exerciseID != nil && *exerciseID != ""
	if purpose == MediaUploadPurposeExerciseAsset && !hasExerciseID {
		errs = append(errs, FieldError{Field: "exercise_id", Reason: "required when purpose is exercise_asset"})
	}
	if purpose != MediaUploadPurposeExerciseAsset && hasExerciseID {
		errs = append(errs, FieldError{Field: "exercise_id", Reason: fmt.Sprintf("must be absent when purpose is %s", purpose)})
	}
	if purpose == MediaUploadPurposeThumbnail && contentType != MediaContentTypeImage {
		errs = append(errs, FieldError{Field: "content_type", Reason: "must be image when purpose is thumbnail"})
	}
	errs = append(errs, validateVideo(purpose, contentType, fileName)...)
	errs = append(errs, validateFileSize(contentType, fileSize)...)

	if len(errs) > 0 {
		return MediaUploadRequest{}, &ValidationError{Fields: errs}
	}

	return MediaUploadRequest{
		Purpose:     purpose,
		ExerciseID:  exerciseID,
		ContentType: contentType,
		FileName:    fileName,
		FileSize:    fileSize,
	}, nil
}

// validateVideo pairs video with lesson_video both ways, and accepts only
// an .mp4 file name for a video.
func validateVideo(purpose MediaUploadPurpose, contentType MediaContentType, fileName string) []FieldError {
	var errs []FieldError
	if purpose == MediaUploadPurposeLessonVideo && contentType != MediaContentTypeVideo {
		errs = append(errs, FieldError{Field: "content_type", Reason: "must be video when purpose is lesson_video"})
	}
	if contentType == MediaContentTypeVideo && purpose != MediaUploadPurposeLessonVideo {
		errs = append(errs, FieldError{Field: "content_type", Reason: "video is accepted only when purpose is lesson_video"})
	}
	if contentType == MediaContentTypeVideo && fileName != "" && !strings.HasSuffix(strings.ToLower(fileName), ".mp4") {
		errs = append(errs, FieldError{Field: "file_name", Reason: "a video must be an .mp4 file"})
	}
	return errs
}

// validateFileSize requires a size for video, caps it at
// MaxVideoUploadBytes, and refuses a non-positive size for any content type.
func validateFileSize(contentType MediaContentType, fileSize *int64) []FieldError {
	switch {
	case fileSize == nil && contentType == MediaContentTypeVideo:
		return []FieldError{{Field: "file_size", Reason: "required when content_type is video"}}
	case fileSize == nil:
		return nil
	case *fileSize < 1:
		return []FieldError{{Field: "file_size", Reason: "must be at least 1 byte"}}
	case contentType == MediaContentTypeVideo && *fileSize > MaxVideoUploadBytes:
		return []FieldError{{Field: "file_size", Reason: fmt.Sprintf("must be at most %d bytes (1 GB) for a video", MaxVideoUploadBytes)}}
	}
	return nil
}
