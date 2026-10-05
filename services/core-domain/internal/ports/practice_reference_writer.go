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
	// PutDiagrams creates or replaces each diagram's reference, in one
	// round trip to the store.
	PutDiagrams(ctx context.Context, refs []domain.DiagramReference) error

	// PutExercises creates or replaces each exercise's reference, in one
	// round trip to the store.
	PutExercises(ctx context.Context, refs []domain.ExerciseReference) error

	// PutDrillThresholds creates or replaces each fluent time version, in one
	// round trip to the store.
	PutDrillThresholds(ctx context.Context, thresholds []domain.DrillThreshold) error
}
