package ports

import (
	"context"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// PracticeReferenceReader reads core's practice_reference snapshot, the
// only reference data the graders use. The worker never writes it.
type PracticeReferenceReader interface {
	// Diagrams returns the diagrams among ids that exist, by id. A missing id is
	// simply absent from the map.
	Diagrams(ctx context.Context, ids []string) (map[string]domain.DiagramReference, error)
}

// PracticeEvidenceRepository stores evidence, once per evidence id. Evidence is
// never modified or deleted.
type PracticeEvidenceRepository interface {
	// Insert stores e and reports false, without error, when evidence with its id
	// is already stored.
	Insert(ctx context.Context, e domain.PracticeEvidence) (inserted bool, err error)

	// ListForItem returns a student's evidence for one item in the order it was
	// stored, which domain.RebuildFold keeps for evidence sharing a timestamp.
	ListForItem(ctx context.Context, studentID, itemKey string) ([]domain.PracticeEvidence, error)
}

// PracticeItemStateRepository keeps each item's folded state: one document per
// (student_id, item_key). Like CompletionStateRepository, its read-modify-write is
// safe without locking because motifpath.events is partitioned by student_id, which makes
// this worker the single writer for each student.
type PracticeItemStateRepository interface {
	// Get reports the stored fold; found is false when the student has no state for
	// the item yet.
	Get(ctx context.Context, studentID, itemKey string) (fold domain.ItemFold, found bool, err error)

	// Put replaces the stored fold, stamped with domain.PracticeRulesVersion.
	Put(ctx context.Context, studentID, itemKey string, fold domain.ItemFold) error
}
