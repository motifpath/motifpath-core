package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// putDiagramReference refreshes d's practice reference after d is saved.
// A failure is logged, not returned: the diagram is already committed, and
// the sync on start repairs the snapshot (ADR-047). Until then, an answer
// on d is rejected and kept for a regrade.
func putDiagramReference(ctx context.Context, references ports.PracticeReferenceWriter, d domain.Diagram) {
	if err := references.PutDiagram(ctx, domain.NewDiagramReference(d)); err != nil {
		slog.ErrorContext(ctx, "write the diagram's practice reference", "diagram_id", d.ID, "error", err)
	}
}

// PracticeReferenceService rebuilds the practice reference snapshot from
// PostgreSQL (ADR-047). It runs on start and from a maintenance command,
// repairing any write lost after a commit and covering rows installed by
// migrations.
type PracticeReferenceService struct {
	diagrams   ports.DiagramRepository
	references ports.PracticeReferenceWriter
}

func NewPracticeReferenceService(diagrams ports.DiagramRepository, references ports.PracticeReferenceWriter) *PracticeReferenceService {
	return &PracticeReferenceService{diagrams: diagrams, references: references}
}

// SyncDiagrams writes every diagram's reference and returns how many it
// wrote. It keeps going past a failed write and reports all of them.
func (s *PracticeReferenceService) SyncDiagrams(ctx context.Context) (int, error) {
	page := domain.PageRequest{Limit: domain.MaxPageLimit}
	synced := 0
	var errs []error
	for {
		got, err := s.diagrams.List(ctx, domain.DiagramListFilter{}, page)
		if err != nil {
			return synced, err
		}
		for _, d := range got.Items {
			if err := s.references.PutDiagram(ctx, domain.NewDiagramReference(d)); err != nil {
				errs = append(errs, err)
				continue
			}
			synced++
		}
		page.Offset += len(got.Items)
		if len(got.Items) == 0 || page.Offset >= got.Total {
			return synced, errors.Join(errs...)
		}
	}
}
