package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// InstrumentService manages Instrument — what a Diagram is authored against.
// There is deliberately no update/delete method, matching the OpenAPI
// surface: changing an instrument's family or shape once diagrams exist
// against it is an open question.
type InstrumentService struct {
	instruments ports.InstrumentRepository
	newID       func() string
}

func NewInstrumentService(instruments ports.InstrumentRepository, newID func() string) *InstrumentService {
	return &InstrumentService{instruments: instruments, newID: newID}
}

// CreateInstrument creates a new instrument. Only teachers and admins may
// create one — instruments are an authoring surface.
func (s *InstrumentService) CreateInstrument(ctx context.Context, caller domain.User, name string, family domain.InstrumentFamily, stringCount *int, tuning []string, keyRange *domain.KeyRange) (domain.Instrument, error) {
	if !canManageContent(caller.Role) {
		return domain.Instrument{}, domain.ErrForbidden
	}

	instrument, err := domain.NewInstrument(s.newID(), name, family, stringCount, tuning, keyRange)
	if err != nil {
		return domain.Instrument{}, err
	}
	if err := s.instruments.Create(ctx, instrument); err != nil {
		return domain.Instrument{}, err
	}
	return instrument, nil
}

// ListInstruments returns every known instrument. Any authenticated user may
// list them.
func (s *InstrumentService) ListInstruments(ctx context.Context) ([]domain.Instrument, error) {
	return s.instruments.List(ctx)
}
