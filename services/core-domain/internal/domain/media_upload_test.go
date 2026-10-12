package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func int64Ptr(v int64) *int64 { return &v }

const mb = int64(1 << 20)

func TestNewMediaUploadRequest_LessonVideo(t *testing.T) {
	t.Run("a lesson video is an MP4 with its size", func(t *testing.T) {
		req, err := domain.NewMediaUploadRequest(domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "open-chords-lesson.mp4", int64Ptr(980*mb))

		require.NoError(t, err)
		assert.Equal(t, domain.MediaUploadPurposeLessonVideo, req.Purpose)
		assert.Equal(t, domain.MediaContentTypeVideo, req.ContentType)
		assert.Equal(t, int64Ptr(980*mb), req.FileSize)
	})

	t.Run("exactly 1 GB is accepted", func(t *testing.T) {
		_, err := domain.NewMediaUploadRequest(domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "long-lesson.mp4", int64Ptr(domain.MaxVideoUploadBytes))

		require.NoError(t, err)
		assert.Equal(t, int64(1073741824), domain.MaxVideoUploadBytes)
	})

	t.Run("the .mp4 extension is matched in any case", func(t *testing.T) {
		_, err := domain.NewMediaUploadRequest(domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "OPEN-CHORDS.MP4", int64Ptr(300*mb))

		require.NoError(t, err)
	})

	exercise := "exercise-1"
	for name, tc := range map[string]struct {
		purpose     domain.MediaUploadPurpose
		exerciseID  *string
		contentType domain.MediaContentType
		fileName    string
		fileSize    *int64
		field       string
	}{
		"over 1 GB":                 {domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "long-lesson.mp4", int64Ptr(domain.MaxVideoUploadBytes + 1), "file_size"},
		"without a file size":       {domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "lesson.mp4", nil, "file_size"},
		"of zero bytes":             {domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "lesson.mp4", int64Ptr(0), "file_size"},
		"a .mov":                    {domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "lesson.mov", int64Ptr(300 * mb), "file_name"},
		"a .webm":                   {domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "lesson.webm", int64Ptr(300 * mb), "file_name"},
		"a .mp4.zip":                {domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeVideo, "lesson.mp4.zip", int64Ptr(300 * mb), "file_name"},
		"for the image library":     {domain.MediaUploadPurposeLibraryAsset, nil, domain.MediaContentTypeVideo, "lesson.mp4", int64Ptr(300 * mb), "content_type"},
		"for an exercise":           {domain.MediaUploadPurposeExerciseAsset, &exercise, domain.MediaContentTypeVideo, "clip.mp4", int64Ptr(50 * mb), "content_type"},
		"as an image":               {domain.MediaUploadPurposeLessonVideo, nil, domain.MediaContentTypeImage, "poster.png", nil, "content_type"},
		"with an exercise":          {domain.MediaUploadPurposeLessonVideo, &exercise, domain.MediaContentTypeVideo, "clip.mp4", int64Ptr(50 * mb), "exercise_id"},
		"an image of negative size": {domain.MediaUploadPurposeLibraryAsset, nil, domain.MediaContentTypeImage, "a.png", int64Ptr(-1), "file_size"},
	} {
		t.Run("rejected: a video "+name, func(t *testing.T) {
			_, err := domain.NewMediaUploadRequest(tc.purpose, tc.exerciseID, tc.contentType, tc.fileName, tc.fileSize)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tc.field, valErr.Fields[0].Field)
		})
	}

	t.Run("images and audio keep an optional size", func(t *testing.T) {
		withSize, err := domain.NewMediaUploadRequest(domain.MediaUploadPurposeLibraryAsset, nil, domain.MediaContentTypeImage, "a.png", int64Ptr(2*mb))
		require.NoError(t, err)
		assert.Equal(t, int64Ptr(2*mb), withSize.FileSize)

		withoutSize, err := domain.NewMediaUploadRequest(domain.MediaUploadPurposeLibraryAsset, nil, domain.MediaContentTypeAudio, "a.mp3", nil)
		require.NoError(t, err)
		assert.Nil(t, withoutSize.FileSize)
	})
}
