package repo

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// newTestS3MediaStorage presigns against a made-up endpoint: presigning is
// local signing only, so no storage has to be running.
func newTestS3MediaStorage() *S3MediaStorage {
	client := s3.New(s3.Options{
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("key", "secret", ""),
		BaseEndpoint: aws.String("http://storage.test:8333"),
		UsePathStyle: true,
	})
	return NewS3MediaStorage(client, "media", "https://media.test")
}

func signedHeaders(t *testing.T, uploadURL string) []string {
	t.Helper()
	u, err := url.Parse(uploadURL)
	require.NoError(t, err)
	return strings.Split(u.Query().Get("X-Amz-SignedHeaders"), ";")
}

func TestS3MediaStorage_PresignUpload(t *testing.T) {
	storage := newTestS3MediaStorage()

	tests := []struct {
		name          string
		contentType   domain.MediaContentType
		contentLength *int64
		wantSigned    []string
		wantUnsigned  []string
	}{
		{
			name:          "a video signs video/mp4 and its exact size",
			contentType:   domain.MediaContentTypeVideo,
			contentLength: aws.Int64(980 << 20),
			wantSigned:    []string{"content-type", "content-length"},
		},
		{
			name:         "an image without a declared size signs no size",
			contentType:  domain.MediaContentTypeImage,
			wantSigned:   []string{"content-type"},
			wantUnsigned: []string{"content-length"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := storage.PresignUpload(context.Background(), "lessons/id-1.mp4", tc.contentType, tc.contentLength)

			require.NoError(t, err)
			assert.Equal(t, "https://media.test/lessons/id-1.mp4", got.ObjectURL)
			headers := signedHeaders(t, got.UploadURL)
			for _, h := range tc.wantSigned {
				assert.Contains(t, headers, h)
			}
			for _, h := range tc.wantUnsigned {
				assert.NotContains(t, headers, h)
			}
		})
	}

	t.Run("each content type maps to the MIME type the browser must send", func(t *testing.T) {
		assert.Equal(t, "video/mp4", *mimeType(domain.MediaContentTypeVideo))
		assert.Equal(t, "image/png", *mimeType(domain.MediaContentTypeImage))
		assert.Equal(t, "audio/mpeg", *mimeType(domain.MediaContentTypeAudio))
	})
}
