package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// checkDiagramRefsExist reports an error if diagramRef or diagramStackRef
// names a diagram that does not exist, or if diagramStackRef's entries
// reference diagrams on more than one instrument — both require a
// repository round trip, so cannot be checked in the domain layer.
// diagramRef/diagramStackRef's own structural validity (e.g. having a
// diagram_id at all) is already checked by the domain layer before this
// runs. At most one of diagramRef/diagramStackRef is ever non-nil.
func checkDiagramRefsExist(ctx context.Context, diagrams ports.DiagramRepository, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef) error {
	refs := []domain.DiagramRef{}
	if diagramRef != nil {
		refs = append(refs, *diagramRef)
	}
	if diagramStackRef != nil {
		refs = append(refs, diagramStackRef.Stack...)
	}

	var instrumentID string
	for i, ref := range refs {
		diagram, err := diagrams.GetByID(ctx, ref.DiagramID)
		if err != nil {
			return err
		}
		if i == 0 {
			instrumentID = diagram.InstrumentID
		} else if diagram.InstrumentID != instrumentID {
			return domain.NewValidationError("diagram_stack_ref", "every diagram in a stack must share the same instrument")
		}
	}
	return nil
}
