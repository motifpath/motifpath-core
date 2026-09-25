package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// InstrumentService manages Instrument — what a Diagram is authored against.
// Only an instrument's names can change after creation: changing its family
// or shape once diagrams exist against it is an open question.
type InstrumentService struct {
	instruments ports.InstrumentRepository
	languages   ports.LanguageRepository
	newID       func() string
}

func NewInstrumentService(instruments ports.InstrumentRepository, languages ports.LanguageRepository, newID func() string) *InstrumentService {
	return &InstrumentService{instruments: instruments, languages: languages, newID: newID}
}

// CreateInstrument creates a new instrument. Only teachers and admins may
// create one — instruments are an authoring surface. names must cover every
// language MotifPath offers, since every user sees the instrument.
func (s *InstrumentService) CreateInstrument(ctx context.Context, caller domain.User, names map[string]string, family domain.InstrumentFamily, stringCount *int, tuning []string, keyRange *domain.KeyRange) (domain.Instrument, error) {
	if !canManageContent(caller.Role) {
		return domain.Instrument{}, domain.ErrForbidden
	}
	offered, err := offeredLanguages(ctx, s.languages)
	if err != nil {
		return domain.Instrument{}, err
	}

	instrument, err := domain.NewInstrument(s.newID(), names, offered, family, stringCount, tuning, keyRange)
	if err != nil {
		return domain.Instrument{}, err
	}
	if err := s.instruments.Create(ctx, instrument); err != nil {
		return domain.Instrument{}, err
	}
	return instrument, nil
}

// UpdateInstrumentNames replaces an instrument's names — the only part of an
// instrument that can change. Only admins may, since instruments are shared
// by every user; the new names must cover every language MotifPath offers.
func (s *InstrumentService) UpdateInstrumentNames(ctx context.Context, caller domain.User, id string, names map[string]string) (domain.Instrument, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.Instrument{}, domain.ErrForbidden
	}
	current, err := s.instruments.GetByID(ctx, id)
	if err != nil {
		return domain.Instrument{}, err
	}
	offered, err := offeredLanguages(ctx, s.languages)
	if err != nil {
		return domain.Instrument{}, err
	}

	updated, err := domain.NewInstrument(current.ID, names, offered, current.Family, current.StringCount, current.Tuning, current.KeyRange)
	if err != nil {
		return domain.Instrument{}, err
	}
	if err := s.instruments.UpdateNames(ctx, updated.ID, updated.Names); err != nil {
		return domain.Instrument{}, err
	}
	return updated, nil
}

// ListInstruments returns every known instrument. Any authenticated user may
// list them.
func (s *InstrumentService) ListInstruments(ctx context.Context) ([]domain.Instrument, error) {
	return s.instruments.List(ctx)
}

// offeredLanguages returns the code of every language MotifPath offers —
// every Language row except the language-agnostic LanguageCodeAny marker,
// which is never a language text can be written in.
func offeredLanguages(ctx context.Context, languages ports.LanguageRepository) ([]string, error) {
	all, err := languages.List(ctx)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(all))
	for _, lang := range all {
		if lang.Code != domain.LanguageCodeAny {
			codes = append(codes, lang.Code)
		}
	}
	return codes, nil
}

// checkInstrumentsExist returns a validation error on "instrument_ids" when
// any of ids does not reference an existing instrument.
func checkInstrumentsExist(ctx context.Context, instruments ports.InstrumentRepository, ids []string) error {
	for _, id := range ids {
		if _, err := instruments.GetByID(ctx, id); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return domain.NewValidationError("instrument_ids", fmt.Sprintf("%q does not reference an existing instrument", id))
			}
			return err
		}
	}
	return nil
}
