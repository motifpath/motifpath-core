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
// A lone diagramRef whose playback picks a voice must pick an existing voice
// of its diagram's instrument family, reported under "diagram_ref". A stack
// doesn't play, so its entries' playback is never checked.
func resolveDiagramRefs(ctx context.Context, repos diagramRefRepos, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef) ([]diagramRefWithDiagram, error) {
	resolved, err := resolveDiagrams(ctx, repos.diagrams, diagramRef, diagramStackRef)
	if err != nil {
		return nil, err
	}
	if diagramRef != nil {
		if err := checkPlaybackVoice(ctx, repos, resolved[0]); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

// diagramRefRepos are the repositories resolving a diagram reference reads.
type diagramRefRepos struct {
	diagrams    ports.DiagramRepository
	instruments ports.InstrumentRepository
	voices      ports.VoiceRepository
}

// checkPlaybackVoice returns a validation error on "diagram_ref" when
// resolved's playback picks a voice that doesn't exist or doesn't play its
// diagram's instrument family.
func checkPlaybackVoice(ctx context.Context, repos diagramRefRepos, resolved diagramRefWithDiagram) error {
	playback := resolved.ref.Playback
	if playback == nil || playback.VoiceID == nil {
		return nil
	}
	voice, err := repos.voices.GetByID(ctx, *playback.VoiceID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NewValidationError("diagram_ref", "playback.voice_id references a voice that does not exist: "+*playback.VoiceID)
	}
	if err != nil {
		return err
	}
	instrument, err := repos.instruments.GetByID(ctx, resolved.diagram.InstrumentID)
	if err != nil {
		return err
	}
	if voice.Family != instrument.Family {
		return domain.NewValidationError("diagram_ref", "playback.voice_id must be a voice of the diagram's instrument family")
	}
	return nil
}

// resolveDiagrams fetches the Diagrams diagramRef or diagramStackRef names,
// checking that they exist and that a stack's share one instrument.
func resolveDiagrams(ctx context.Context, diagrams ports.DiagramRepository, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef) ([]diagramRefWithDiagram, error) {
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

// checkDiagramRefs is resolveDiagramRefs for a caller that only needs its
// checks, not the resolved Diagrams themselves.
func checkDiagramRefs(ctx context.Context, repos diagramRefRepos, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef) error {
	_, err := resolveDiagramRefs(ctx, repos, diagramRef, diagramStackRef)
	return err
}
