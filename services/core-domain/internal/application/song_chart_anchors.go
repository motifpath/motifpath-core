package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/motifpath/core-domain/internal/domain"
)

// resolveAnchors resolves every anchor's written symbol to a catalog chord
// and lists the anchors that don't fully resolve. A chord counts only if the
// catalog offers a voicing of it; a slash chord the catalog lacks resolves to
// the chord without its bass. A picked voicing must be one of the resolved
// chord's voicings, or the document is refused; one that has been withdrawn
// is a warning.
func (s *SongChartService) resolveAnchors(ctx context.Context, doc domain.SongChartDocument) (domain.SongChartDocument, []domain.AnchorWarning, error) {
	anchors := doc.Anchors()
	picked, err := s.pickedVoicings(ctx, anchors)
	if err != nil {
		return domain.SongChartDocument{}, nil, err
	}

	r := anchorResolver{service: s, found: map[string]*domain.ChordDefinition{}}
	resolved := map[string]*string{}
	var warnings []domain.AnchorWarning
	for _, a := range anchors {
		chordID, kind, err := r.resolve(ctx, a.Anchor.WrittenSymbol)
		if err != nil {
			return domain.SongChartDocument{}, nil, err
		}
		if kind, err = checkPick(doc, a.Anchor, chordID, kind, picked); err != nil {
			return domain.SongChartDocument{}, nil, err
		}
		resolved[a.Anchor.ID] = chordID
		if kind != "" {
			warnings = append(warnings, domain.AnchorWarning{AnchorID: a.Anchor.ID, Position: a.Position, WrittenSymbol: a.Anchor.WrittenSymbol, Kind: kind})
		}
	}
	body := doc.WithAnchors(func(a domain.ChordAnchor) domain.ChordAnchor {
		a.ChordDefinitionID = resolved[a.ID]
		return a
	})
	return body, warnings, nil
}

// pickedVoicings reads every voicing an anchor picked, withdrawn ones
// included.
func (s *SongChartService) pickedVoicings(ctx context.Context, anchors []domain.PositionedAnchor) (map[string]domain.ChordVoicing, error) {
	var ids []string
	for _, a := range anchors {
		if a.Anchor.ChordVoicingID != nil {
			ids = append(ids, *a.Anchor.ChordVoicingID)
		}
	}
	return s.chords.GetVoicings(ctx, ids)
}

// checkPick refuses a picked voicing that isn't one of the resolved
// chord's, and returns the anchor's warning: a withdrawn pick blocks
// publication, so it outranks the only other warning a resolved chord can
// carry, a missing bass.
func checkPick(doc domain.SongChartDocument, anchor domain.ChordAnchor, chordID *string, kind domain.AnchorWarningKind, picked map[string]domain.ChordVoicing) (domain.AnchorWarningKind, error) {
	if anchor.ChordVoicingID == nil {
		return kind, nil
	}
	voicing, ok := picked[*anchor.ChordVoicingID]
	if !ok || chordID == nil || voicing.ChordDefinitionID != *chordID {
		return "", domain.NewValidationError(voicingField(doc, anchor.ID), "must be a voicing of the chord its symbol names")
	}
	if voicing.Status != domain.ChordVoicingActive {
		return domain.AnchorVoicingUnavailable, nil
	}
	return kind, nil
}

// voicingField is the request-body path of an anchor's chordVoicingId.
func voicingField(doc domain.SongChartDocument, anchorID string) string {
	for si, section := range doc.Sections {
		for li, line := range section.Lines {
			for ri, run := range line.Runs {
				if run.Anchor != nil && run.Anchor.ID == anchorID {
					return fmt.Sprintf("body/content/%d/content/%d/content/%d/marks/0/attrs/chordVoicingId", si, li, ri)
				}
			}
		}
	}
	return "body"
}

// anchorResolver looks each chord up once, however many anchors use it.
type anchorResolver struct {
	service *SongChartService
	found   map[string]*domain.ChordDefinition
}

// resolve returns the catalog chord a symbol resolves to, if any, and the
// warning it carries, if any.
func (r anchorResolver) resolve(ctx context.Context, symbol string) (*string, domain.AnchorWarningKind, error) {
	reading := domain.ParseChordSymbol(symbol)
	switch reading.Status {
	case domain.ChordSymbolNoChord:
		return nil, "", nil
	case domain.ChordSymbolUnparsed:
		if reading.Warning == domain.ChordSymbolWarningUnsupportedQuality {
			return nil, domain.AnchorUnsupportedQuality, nil
		}
		return nil, domain.AnchorUnparsedSymbol, nil
	case domain.ChordSymbolParsed:
		return r.resolveParsed(ctx, *reading.Parsed)
	}
	return nil, domain.AnchorUnparsedSymbol, nil
}

// resolveParsed finds the catalog chord for a parsed symbol, falling back to
// the chord without its bass for a slash chord the catalog lacks.
func (r anchorResolver) resolveParsed(ctx context.Context, p domain.ParsedChordSymbol) (*string, domain.AnchorWarningKind, error) {
	chord, err := r.playable(ctx, p.RootPitchClass, p.Quality, p.BassPitchClass)
	if err != nil || chord != nil {
		return chordIDOf(chord), "", err
	}
	if p.BassPitchClass != nil {
		chord, err = r.playable(ctx, p.RootPitchClass, p.Quality, nil)
		if err != nil || chord != nil {
			return chordIDOf(chord), domain.AnchorBassNotInCatalog, err
		}
	}
	return nil, domain.AnchorChordNotInCatalog, nil
}

func chordIDOf(c *domain.ChordDefinition) *string {
	if c == nil {
		return nil
	}
	return &c.ID
}

// playable returns the catalog's chord when it has a voicing to offer, or
// nil.
func (r anchorResolver) playable(ctx context.Context, rootPitchClass int, quality domain.ChordQuality, bassPitchClass *int) (*domain.ChordDefinition, error) {
	key := fmt.Sprintf("%d/%s", rootPitchClass, quality)
	if bassPitchClass != nil {
		key += fmt.Sprintf("/%d", *bassPitchClass)
	}
	if chord, seen := r.found[key]; seen {
		return chord, nil
	}
	chord, err := r.service.chords.FindChord(ctx, rootPitchClass, quality, bassPitchClass)
	if errors.Is(err, domain.ErrNotFound) {
		r.found[key] = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(chord.Voicings) == 0 {
		r.found[key] = nil
		return nil, nil
	}
	r.found[key] = &chord
	return &chord, nil
}
