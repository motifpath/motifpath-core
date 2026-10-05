package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// referenceWriteTimeout bounds the snapshot write that follows a save.
const referenceWriteTimeout = 5 * time.Second

// putDiagramReference refreshes d's practice reference after d is saved.
// A failure is logged, not returned: the diagram is already committed, and
// the sync on start repairs the snapshot (ADR-047). Until then, an answer
// on d is rejected and kept for a regrade. The write outlives the caller's
// cancellation — a client leaving right after the commit must not leave the
// snapshot stale until the next start.
func putDiagramReference(ctx context.Context, references ports.PracticeReferenceWriter, d domain.Diagram) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), referenceWriteTimeout)
	defer cancel()
	if err := references.PutDiagrams(ctx, []domain.DiagramReference{domain.NewDiagramReference(d)}); err != nil {
		slog.ErrorContext(ctx, "write the diagram's practice reference", "diagram_id", d.ID, "error", err)
	}
}

// putExerciseReference refreshes e's practice reference after e is saved,
// under the same rules as putDiagramReference.
func putExerciseReference(ctx context.Context, references ports.PracticeReferenceWriter, e domain.Exercise) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), referenceWriteTimeout)
	defer cancel()
	if err := references.PutExercises(ctx, []domain.ExerciseReference{domain.NewExerciseReference(e)}); err != nil {
		slog.ErrorContext(ctx, "write the exercise's practice reference", "exercise_id", e.ID, "error", err)
	}
}

// PracticeReferenceService rebuilds the practice reference snapshot from
// PostgreSQL (ADR-047). It runs on start and from a maintenance command,
// repairing any write lost after a commit and covering rows installed by
// migrations.
type PracticeReferenceService struct {
	diagrams   ports.DiagramRepository
	exercises  ports.ExerciseRepository
	thresholds ports.DrillThresholdRepository
	references ports.PracticeReferenceWriter
}

func NewPracticeReferenceService(diagrams ports.DiagramRepository, exercises ports.ExerciseRepository, thresholds ports.DrillThresholdRepository, references ports.PracticeReferenceWriter) *PracticeReferenceService {
	return &PracticeReferenceService{diagrams: diagrams, exercises: exercises, thresholds: thresholds, references: references}
}

// PracticeReferenceSynced counts the references a Sync wrote, by kind.
type PracticeReferenceSynced struct {
	Diagrams        int
	Exercises       int
	DrillThresholds int
}

// Sync rebuilds the whole snapshot. The fluent times go first and every part
// runs whatever the others did: an exercise answered while they're missing would
// fold without fluency, then gain it when its item is rebuilt. It reports every
// part that failed.
func (s *PracticeReferenceService) Sync(ctx context.Context) (PracticeReferenceSynced, error) {
	var synced PracticeReferenceSynced
	var errs [3]error
	synced.DrillThresholds, errs[0] = s.SyncDrillThresholds(ctx)
	synced.Diagrams, errs[1] = s.SyncDiagrams(ctx)
	synced.Exercises, errs[2] = s.SyncExercises(ctx)
	return synced, errors.Join(errs[:]...)
}

// SyncDrillThresholds writes every installed fluent time version, all in one
// write, and returns how many it wrote. Versions come only from migrations,
// so this sync is the only way they reach the snapshot.
func (s *PracticeReferenceService) SyncDrillThresholds(ctx context.Context) (int, error) {
	thresholds, err := s.thresholds.List(ctx)
	if err != nil || len(thresholds) == 0 {
		return 0, err
	}
	if err := s.references.PutDrillThresholds(ctx, thresholds); err != nil {
		return 0, err
	}
	return len(thresholds), nil
}

// SyncDiagrams writes every diagram's reference, one bulk write per page,
// and returns how many it wrote. It keeps going past a failed page and
// reports all of them.
func (s *PracticeReferenceService) SyncDiagrams(ctx context.Context) (int, error) {
	return syncPages(ctx,
		func(page domain.PageRequest) (domain.Page[domain.Diagram], error) {
			return s.diagrams.List(ctx, domain.DiagramListFilter{}, page)
		},
		domain.NewDiagramReference,
		s.references.PutDiagrams)
}

// SyncExercises writes every exercise's reference, one bulk write per page,
// and returns how many it wrote. Like SyncDiagrams, it keeps going past a
// failed page and reports all of them.
func (s *PracticeReferenceService) SyncExercises(ctx context.Context) (int, error) {
	return syncPages(ctx,
		func(page domain.PageRequest) (domain.Page[domain.Exercise], error) {
			return s.exercises.List(ctx, domain.ExerciseFilter{}, page)
		},
		domain.NewExerciseReference,
		s.references.PutExercises)
}

// syncPages walks every page list returns, writing the references of each
// page in one call to put.
func syncPages[T, R any](ctx context.Context, list func(domain.PageRequest) (domain.Page[T], error), reference func(T) R, put func(context.Context, []R) error) (int, error) {
	page := domain.PageRequest{Limit: domain.MaxPageLimit}
	synced := 0
	var errs []error
	for {
		got, err := list(page)
		if err != nil {
			return synced, err
		}
		refs := make([]R, len(got.Items))
		for i, item := range got.Items {
			refs[i] = reference(item)
		}
		if len(refs) > 0 {
			if err := put(ctx, refs); err != nil {
				errs = append(errs, err)
			} else {
				synced += len(refs)
			}
		}
		page.Offset += len(got.Items)
		if len(got.Items) == 0 || page.Offset >= got.Total {
			return synced, errors.Join(errs...)
		}
	}
}
