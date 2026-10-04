package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// PracticeReferenceWriter keeps the practice reference snapshot the
// Aggregation Worker's graders read (ADR-047). This service is its only
// writer. A document is replaced in place and never removed, so old
// evidence can always be regraded.
type PracticeReferenceWriter interface {
	// PutDiagram creates or replaces the diagram's reference.
	PutDiagram(ctx context.Context, ref domain.DiagramReference) error
}
