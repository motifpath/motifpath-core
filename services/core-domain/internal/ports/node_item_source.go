package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// NodeItemSource lists the practice items knowledge-node levels roll up.
type NodeItemSource interface {
	// ClassifiedItems returns the practice items that suit instrumentID —
	// items for it and items for every instrument — each with the
	// knowledge nodes it is classified under directly: play-alongs (basic
	// diagrams with a sequence and a tempo) and authored exercises.
	ClassifiedItems(ctx context.Context, instrumentID string) ([]domain.ClassifiedItem, error)
}
