package application

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// maxChordSymbolLength is the longest chord symbol, in characters, a search
// accepts.
const maxChordSymbolLength = 32

// ChordCatalogService finds chords of the chord catalog and their voicings.
// Only teachers and admins read the catalog; students see voicings only
// where content embeds their diagrams.
type ChordCatalogService struct {
	chords ports.ChordCatalogRepository
}

func NewChordCatalogService(chords ports.ChordCatalogRepository) *ChordCatalogService {
	return &ChordCatalogService{chords: chords}
}

// ChordSearch is what reading one chord symbol found. Chord is the catalog's
// chord with the parsed root, quality and bass pitch classes; when the
// symbol is a slash chord the catalog doesn't have, ChordWithoutBass is the
// same chord without its bass. Both are nil unless the symbol parsed.
type ChordSearch struct {
	WrittenSymbol    string
	Reading          domain.ChordSymbolReading
	Chord            *domain.ChordDefinition
	ChordWithoutBass *domain.ChordDefinition
}

// SearchChords parses symbol as written and looks its chord up by pitch
// class, so every supported spelling finds the same chord and a symbol that
// doesn't parse is reported rather than guessed at.
func (s *ChordCatalogService) SearchChords(ctx context.Context, caller domain.User, symbol string) (ChordSearch, error) {
	if !canManageContent(caller.Role) {
		return ChordSearch{}, domain.ErrForbidden
	}
	if length := utf8.RuneCountInString(symbol); length == 0 || length > maxChordSymbolLength {
		return ChordSearch{}, domain.NewValidationError("symbol", "must be 1 to 32 characters")
	}
	search := ChordSearch{WrittenSymbol: symbol, Reading: domain.ParseChordSymbol(symbol)}
	parsed := search.Reading.Parsed
	if parsed == nil {
		return search, nil
	}
	chord, err := s.findChord(ctx, parsed.RootPitchClass, parsed.Quality, parsed.BassPitchClass)
	if err != nil || chord != nil || parsed.BassPitchClass == nil {
		search.Chord = chord
		return search, err
	}
	search.ChordWithoutBass, err = s.findChord(ctx, parsed.RootPitchClass, parsed.Quality, nil)
	return search, err
}

// findChord returns the catalog's chord, or nil when there is none.
func (s *ChordCatalogService) findChord(ctx context.Context, rootPitchClass int, quality domain.ChordQuality, bassPitchClass *int) (*domain.ChordDefinition, error) {
	chord, err := s.chords.FindChord(ctx, rootPitchClass, quality, bassPitchClass)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &chord, nil
}

// GetChord returns one chord of the catalog with its active voicings.
func (s *ChordCatalogService) GetChord(ctx context.Context, caller domain.User, id string) (domain.ChordDefinition, error) {
	if !canManageContent(caller.Role) {
		return domain.ChordDefinition{}, domain.ErrForbidden
	}
	return s.chords.GetChord(ctx, id)
}
