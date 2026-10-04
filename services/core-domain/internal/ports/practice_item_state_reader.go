package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// PracticeItemStateReader reads per-student practice item states from the
// Aggregation Worker's MongoDB `practice_item_state` collection. It is
// read-only from this service's perspective — the worker's evidence
// processor is the only writer.
type PracticeItemStateReader interface {
	// GetStates returns the student's state for each of itemKeys. An item
	// never practised has no state and is simply absent from the result.
	GetStates(ctx context.Context, studentID string, itemKeys []string) (map[string]domain.PracticeItemState, error)
}
