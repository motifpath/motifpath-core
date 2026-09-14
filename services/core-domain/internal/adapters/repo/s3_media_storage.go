package repo

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/motifpath/core-domain/internal/domain"
)

// presignTTL is how long an issued upload URL remains valid. Short-lived by
// design — the browser is expected to PUT immediately after requesting it,
// not hold the URL for later use.
const presignTTL = 15 * time.Minute

// S3MediaStorage issues presigned upload URLs via the S3 API — the same
// client works against real S3 in production and MinIO in local dev,
// differing only in how the *s3.Client was constructed.
type S3MediaStorage struct {
	presign       *s3.PresignClient
	bucket        string
	publicBaseURL string // e.g. a CloudFront domain in prod, MinIO's local URL in dev
}

// NewS3MediaStorage constructs an S3MediaStorage. publicBaseURL is the read
// base every object_url is built from (scheme + host, no trailing slash) —
// it is deliberately independent of the client's own upload endpoint, since
// production reads go through CloudFront rather than S3 directly.
func NewS3MediaStorage(client *s3.Client, bucket, publicBaseURL string) *S3MediaStorage {
	return &S3MediaStorage{
		presign:       s3.NewPresignClient(client),
		bucket:        bucket,
		publicBaseURL: strings.TrimSuffix(publicBaseURL, "/"),
	}
}

func (s *S3MediaStorage) PresignUpload(ctx context.Context, objectKey string, contentType domain.MediaContentType) (domain.MediaUploadURL, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &objectKey,
		ContentType: mimeType(contentType),
	}, s3.WithPresignExpires(presignTTL))
	if err != nil {
		return domain.MediaUploadURL{}, err
	}

	return domain.MediaUploadURL{
		UploadURL: req.URL,
		ObjectURL: s.publicBaseURL + "/" + objectKey,
		ExpiresAt: time.Now().Add(presignTTL),
	}, nil
}

// mimeType returns a permissive content type per MediaContentType — good
// enough to let browsers and S3 handle the object sensibly. Callers upload
// exactly one file per request, so a single representative MIME type per
// category (rather than deriving one from the file extension) is
// sufficient; nothing downstream inspects it beyond the browser's own PUT
// and S3 storing it as object metadata.
func mimeType(contentType domain.MediaContentType) *string {
	var mt string
	switch contentType {
	case domain.MediaContentTypeAudio:
		mt = "audio/mpeg"
	case domain.MediaContentTypeImage:
		mt = "image/png"
	default:
		mt = "application/octet-stream"
	}
	return &mt
}
