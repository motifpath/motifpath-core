package application

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// diagramRefWithDiagram pairs a requested DiagramRef with the Diagram it
// resolved to, so a caller that already paid for the existence round trip
// (resolveDiagramRefs) never has to fetch the same row again to use its
// positions.
type diagramRefWithDiagram struct {
	ref     domain.DiagramRef
	diagram domain.Diagram
}

// resolveDiagramRefs fetches the Diagram(s) diagramRef or diagramStackRef
// names, in the same order the caller supplied them (single ref first, else
// stack order). It reports a domain.ValidationError — not a bare
// domain.ErrNotFound — when a named diagram does not exist, or when
// diagramStackRef's entries span more than one instrument: both require a
// repository round trip, so cannot be checked in the domain layer, but a
// missing referenced entity is still a client-input problem (400), the same
// convention checkSkillsAndConceptsExist/checkRemediationTargetsExist
// already use for a missing skill/concept/content node — not a bare 404
// that a caller further up (e.g. an HTTP handler mapping ErrNotFound to a
// message about a wholly different resource) could misattribute.
// diagramRef/diagramStackRef's own structural validity (e.g. having a
// diagram_id at all) is already checked by the domain layer before this
// runs. At most one of diagramRef/diagramStackRef is ever non-nil.
func resolveDiagramRefs(ctx context.Context, diagrams ports.DiagramRepository, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef) ([]diagramRefWithDiagram, error) {
	refs := []domain.DiagramRef{}
	if diagramRef != nil {
		refs = append(refs, *diagramRef)
	}
	if diagramStackRef != nil {
		refs = append(refs, diagramStackRef.Stack...)
	}

	// A missing diagram in a lone diagram_ref is reported under
	// "diagram_ref"; anywhere within a diagram_stack_ref, under
	// "diagram_stack_ref" — matching whichever field the caller actually
	// sent.
	missingDiagramField := "diagram_ref"
	if diagramStackRef != nil {
		missingDiagramField = "diagram_stack_ref"
	}

	resolved := make([]diagramRefWithDiagram, 0, len(refs))
	var instrumentID string
	for i, ref := range refs {
		diagram, err := diagrams.GetByID(ctx, ref.DiagramID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, domain.NewValidationError(missingDiagramField, "references a diagram that does not exist: "+ref.DiagramID)
			}
			return nil, err
		}
		if i == 0 {
			instrumentID = diagram.InstrumentID
		} else if diagram.InstrumentID != instrumentID {
			return nil, domain.NewValidationError("diagram_stack_ref", "every diagram in a stack must share the same instrument")
		}
		resolved = append(resolved, diagramRefWithDiagram{ref: ref, diagram: diagram})
	}
	return resolved, nil
}

// checkDiagramRefsExist is resolveDiagramRefs for a caller that only needs
// the existence/same-instrument check, not the resolved Diagrams themselves.
func checkDiagramRefsExist(ctx context.Context, diagrams ports.DiagramRepository, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef) error {
	_, err := resolveDiagramRefs(ctx, diagrams, diagramRef, diagramStackRef)
	return err
}
