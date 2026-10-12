package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// MediaStorage issues presigned upload URLs for content-authoring media.
// Implementations speak the S3 API — real S3 in production, MinIO in local
// dev — so no calling code branches on environment.
type MediaStorage interface {
	// PresignUpload issues a short-lived presigned PUT URL for objectKey,
	// along with the URL the object will be readable at once uploaded. A
	// non-nil contentLength is signed into the URL, so storage refuses an
	// upload of any other size.
	PresignUpload(ctx context.Context, objectKey string, contentType domain.MediaContentType, contentLength *int64) (domain.MediaUploadURL, error)
}
