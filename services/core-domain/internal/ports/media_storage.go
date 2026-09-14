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
	// along with the URL the object will be readable at once uploaded.
	PresignUpload(ctx context.Context, objectKey string, contentType domain.MediaContentType) (domain.MediaUploadURL, error)
}
