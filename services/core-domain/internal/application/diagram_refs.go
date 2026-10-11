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
// of its diagram's instrument family, and one that chooses a playback must
// choose one of its diagram's, unless stored already holds that choice; both
// are reported under "diagram_ref". A stack doesn't play, so its entries'
// playback is never checked.
func resolveDiagramRefs(ctx context.Context, repos diagramRefRepos, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef, stored storedPlaybacks) ([]diagramRefWithDiagram, error) {
	resolved, err := resolveDiagrams(ctx, repos.diagrams, diagramRef, diagramStackRef)
	if err != nil {
		return nil, err
	}
	if diagramRef != nil {
		if err := checkPlayback(ctx, repos, "diagram_ref", resolved[0], stored); err != nil {
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

// storedPlaybacks are the playbacks a saved usage already chose, each with
// the diagram it chose it from. A playback removed from its diagram leaves
// the usages that chose it unchanged, and they play the default instead, so
// saving such a usage again with the same choice must not be refused: only a
// choice the usage didn't already hold has to be one of the diagram's
// playbacks. Nil (a usage being created) holds none.
type storedPlaybacks map[storedPlayback]struct{}

type storedPlayback struct{ diagramID, playbackID string }

// storedPlaybacksOf returns the playbacks refs choose.
func storedPlaybacksOf(refs ...domain.DiagramRef) storedPlaybacks {
	stored := storedPlaybacks{}
	for _, ref := range refs {
		if ref.Playback != nil && ref.Playback.PlaybackID != nil {
			stored[storedPlayback{ref.DiagramID, *ref.Playback.PlaybackID}] = struct{}{}
		}
	}
	return stored
}

// holds reports whether stored already holds ref's playback choice.
func (s storedPlaybacks) holds(ref domain.DiagramRef) bool {
	_, ok := s[storedPlayback{ref.DiagramID, *ref.Playback.PlaybackID}]
	return ok
}

// checkPlayback returns a validation error on field when resolved's
// playback chooses a playback its diagram doesn't have (and stored doesn't
// already hold), or picks a voice that doesn't exist or doesn't play its
// diagram's instrument family.
func checkPlayback(ctx context.Context, repos diagramRefRepos, field string, resolved diagramRefWithDiagram, stored storedPlaybacks) error {
	playback := resolved.ref.Playback
	if playback == nil {
		return nil
	}
	if playback.PlaybackID != nil && !resolved.diagram.HasPlayback(*playback.PlaybackID) && !stored.holds(resolved.ref) {
		return domain.NewValidationError(field, "playback.playback_id is not a playback of the diagram: "+*playback.PlaybackID)
	}
	if playback.VoiceID == nil {
		return nil
	}
	voice, err := repos.voices.GetByID(ctx, *playback.VoiceID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.NewValidationError(field, "playback.voice_id references a voice that does not exist: "+*playback.VoiceID)
	}
	if err != nil {
		return err
	}
	instrument, err := repos.instruments.GetByID(ctx, resolved.diagram.InstrumentID)
	if err != nil {
		return err
	}
	if voice.Family != instrument.Family {
		return domain.NewValidationError(field, "playback.voice_id must be a voice of the diagram's instrument family")
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
func checkDiagramRefs(ctx context.Context, repos diagramRefRepos, diagramRef *domain.DiagramRef, diagramStackRef *domain.DiagramStackRef, stored storedPlaybacks) error {
	_, err := resolveDiagramRefs(ctx, repos, diagramRef, diagramStackRef, stored)
	return err
}

// checkEmbeddedPlaybacks applies checkPlayback to every diagram embedded in
// a document or an option, reporting under field. An embed that picks no
// voice and chooses no playback isn't resolved at all, so its diagram need
// not exist; one that does needs its diagram, to know which family plays it
// and which playbacks it has.
func checkEmbeddedPlaybacks(ctx context.Context, repos diagramRefRepos, field string, refs []domain.DiagramRef, stored storedPlaybacks) error {
	for _, ref := range refs {
		if ref.Playback == nil || (ref.Playback.VoiceID == nil && ref.Playback.PlaybackID == nil) {
			continue
		}
		diagram, err := repos.diagrams.GetByID(ctx, ref.DiagramID)
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewValidationError(field, "embeds a diagram that does not exist, playing with a voice or playback of its own: "+ref.DiagramID)
		}
		if err != nil {
			return err
		}
		if err := checkPlayback(ctx, repos, field, diagramRefWithDiagram{ref: ref, diagram: diagram}, stored); err != nil {
			return err
		}
	}
	return nil
}

// embeddedRefs is every diagram embedded in doc, or none for no document.
func embeddedRefs(doc *domain.PromptDocument) []domain.DiagramRef {
	if doc == nil {
		return nil
	}
	return doc.EmbeddedDiagramRefs()
}

// refsOf is ref as a list: empty when ref is nil.
func refsOf(ref *domain.DiagramRef) []domain.DiagramRef {
	if ref == nil {
		return nil
	}
	return []domain.DiagramRef{*ref}
}
