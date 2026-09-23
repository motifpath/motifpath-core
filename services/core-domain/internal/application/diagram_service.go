package application

import (
	"context"
	"errors"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// DiagramService manages Diagram — a prebuilt, reusable set of positions on
// one instrument, classified against the shared Skill/Concept trees.
type DiagramService struct {
	diagrams    ports.DiagramRepository
	instruments ports.InstrumentRepository
	skills      ports.SkillRepository
	concepts    ports.ConceptRepository
	newID       func() string
	now         func() time.Time
}

func NewDiagramService(diagrams ports.DiagramRepository, instruments ports.InstrumentRepository, skills ports.SkillRepository, concepts ports.ConceptRepository, newID func() string, now func() time.Time) *DiagramService {
	return &DiagramService{diagrams: diagrams, instruments: instruments, skills: skills, concepts: concepts, newID: newID, now: now}
}

// DiagramUpdate carries the fields UpdateDiagram may replace. A nil field
// leaves the current value untouched. Classification is replaced as a unit:
// supplying either SkillIDs or ConceptIDs replaces both, so a caller
// changing one must resend the other. There is no way to clear an
// already-set RootNote back to nil through an update — only to replace it
// with another value or leave it as-is.
type DiagramUpdate struct {
	Name         *string
	Positions    []domain.Position
	SkillIDs     []string
	ConceptIDs   []string
	RootNote     *string
	LabelDisplay *domain.LabelDisplay
}

// CreateDiagram creates a diagram against an existing instrument. Only
// teachers and admins may create one. Positions without an id are assigned
// one; a client-supplied id is kept. rootNote may be nil (unrecorded);
// labelDisplay defaults to domain.LabelDisplayInterval when left as the
// zero value.
func (s *DiagramService) CreateDiagram(ctx context.Context, caller domain.User, instrumentID, name string, positions []domain.Position, skillIDs, conceptIDs []string, rootNote *string, labelDisplay domain.LabelDisplay) (domain.Diagram, error) {
	if !canManageContent(caller.Role) {
		return domain.Diagram{}, domain.ErrForbidden
	}

	instrument, err := s.instruments.GetByID(ctx, instrumentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Diagram{}, domain.NewValidationError("instrument_id", "does not reference an existing instrument")
		}
		return domain.Diagram{}, err
	}

	diagram, err := domain.NewDiagram(s.newID(), instrument, name, s.withPositionIDs(positions), skillIDs, conceptIDs, rootNote, labelDisplay, s.now())
	if err != nil {
		return domain.Diagram{}, err
	}
	if err := checkSkillsAndConceptsExist(ctx, s.skills, s.concepts, skillIDs, conceptIDs); err != nil {
		return domain.Diagram{}, err
	}

	if err := s.diagrams.Create(ctx, diagram); err != nil {
		return domain.Diagram{}, err
	}
	return diagram, nil
}

// GetDiagram returns the diagram with the given id, or domain.ErrNotFound.
// Any authenticated user may call it for a specific known id.
func (s *DiagramService) GetDiagram(ctx context.Context, id string) (domain.Diagram, error) {
	return s.diagrams.GetByID(ctx, id)
}

// ListDiagrams returns diagrams matching the given filters; an empty filter
// value means "no filter" on that dimension. Any authenticated user may list
// them.
func (s *DiagramService) ListDiagrams(ctx context.Context, instrumentID, skillID, conceptID string) ([]domain.Diagram, error) {
	return s.diagrams.List(ctx, instrumentID, skillID, conceptID)
}

// UpdateDiagram applies update to an existing diagram. Only teachers and
// admins may update one. The result is re-validated as a whole, so replaced
// positions are checked against the diagram's own instrument; the instrument
// itself can't change.
func (s *DiagramService) UpdateDiagram(ctx context.Context, caller domain.User, id string, update DiagramUpdate) (domain.Diagram, error) {
	if !canManageContent(caller.Role) {
		return domain.Diagram{}, domain.ErrForbidden
	}

	current, err := s.diagrams.GetByID(ctx, id)
	if err != nil {
		return domain.Diagram{}, err
	}
	instrument, err := s.instruments.GetByID(ctx, current.InstrumentID)
	if err != nil {
		return domain.Diagram{}, err
	}

	name := current.Name
	if update.Name != nil {
		name = *update.Name
	}
	positions := current.Positions
	if update.Positions != nil {
		positions = s.withPositionIDs(update.Positions)
	}
	skillIDs, conceptIDs := current.SkillIDs(), current.ConceptIDs()
	classificationChanged := update.SkillIDs != nil || update.ConceptIDs != nil
	if classificationChanged {
		skillIDs, conceptIDs = update.SkillIDs, update.ConceptIDs
	}
	rootNote := current.RootNote
	if update.RootNote != nil {
		rootNote = update.RootNote
	}
	labelDisplay := current.LabelDisplay
	if update.LabelDisplay != nil {
		labelDisplay = *update.LabelDisplay
	}

	updated, err := domain.NewDiagram(current.ID, instrument, name, positions, skillIDs, conceptIDs, rootNote, labelDisplay, current.CreatedAt)
	if err != nil {
		return domain.Diagram{}, err
	}
	if classificationChanged {
		if err := checkSkillsAndConceptsExist(ctx, s.skills, s.concepts, skillIDs, conceptIDs); err != nil {
			return domain.Diagram{}, err
		}
	}

	if err := s.diagrams.Update(ctx, updated); err != nil {
		return domain.Diagram{}, err
	}
	return updated, nil
}

// withPositionIDs returns a copy of positions with an id assigned to every
// position that lacks one, leaving the caller's slice untouched.
func (s *DiagramService) withPositionIDs(positions []domain.Position) []domain.Position {
	out := make([]domain.Position, len(positions))
	copy(out, positions)
	for i := range out {
		if out[i].ID == "" {
			out[i].ID = s.newID()
		}
	}
	return out
}
