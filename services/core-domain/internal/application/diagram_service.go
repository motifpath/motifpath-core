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
// with another value or leave it as-is; Color behaves the same way.
// Per-position colors travel with Positions and so can be cleared by
// resending Positions without them.
type DiagramUpdate struct {
	Name         *string
	Positions    []domain.Position
	SkillIDs     []string
	ConceptIDs   []string
	RootNote     *string
	LabelDisplay *domain.LabelDisplay
	Color        *string
}

// CreateDiagram creates a diagram against an existing instrument, owned by
// caller. Only teachers and admins may create one, and only an admin may
// create a basic one; opts.Kind defaults to custom. Positions without an id
// are assigned one; a client-supplied id is kept. See domain.DiagramOptions
// for the other optional settings.
func (s *DiagramService) CreateDiagram(ctx context.Context, caller domain.User, instrumentID, name string, positions []domain.Position, skillIDs, conceptIDs []string, opts domain.DiagramOptions) (domain.Diagram, error) {
	if !canManageContent(caller.Role) {
		return domain.Diagram{}, domain.ErrForbidden
	}
	if opts.Kind == domain.DiagramKindBasic && caller.Role != domain.RoleAdmin {
		return domain.Diagram{}, domain.ErrForbidden
	}

	instrument, err := s.instruments.GetByID(ctx, instrumentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Diagram{}, domain.NewValidationError("instrument_id", "does not reference an existing instrument")
		}
		return domain.Diagram{}, err
	}

	diagram, err := domain.NewDiagram(s.newID(), caller.ID, instrument, name, s.withPositionIDs(positions), skillIDs, conceptIDs, opts, s.now())
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
// Any authenticated user may call it for a specific known id, whatever the
// diagram's kind or creator: students render the diagrams embedded in their
// content. ListDiagrams' role scoping governs discovery only.
func (s *DiagramService) GetDiagram(ctx context.Context, id string) (domain.Diagram, error) {
	return s.diagrams.GetByID(ctx, id)
}

// ListDiagrams returns one page of the diagrams matching filter that caller
// may discover. Students may not list diagrams at all. A teacher sees every
// basic diagram plus their own custom ones, and may pass only their own id
// as filter.CreatedBy. An admin sees every diagram. filter.VisibleTo is set
// here from caller's role; any value the caller supplied is overwritten.
func (s *DiagramService) ListDiagrams(ctx context.Context, caller domain.User, filter domain.DiagramListFilter, page domain.PageRequest) (domain.Page[domain.Diagram], error) {
	if filter.Kind != "" && !filter.Kind.Valid() {
		return domain.Page[domain.Diagram]{}, domain.NewValidationError("kind", "must be one of: basic, custom")
	}
	filter.VisibleTo = ""
	switch caller.Role {
	case domain.RoleTeacher:
		if filter.CreatedBy != "" && filter.CreatedBy != caller.ID {
			return domain.Page[domain.Diagram]{}, domain.ErrForbidden
		}
		filter.VisibleTo = caller.ID
	case domain.RoleAdmin:
	case domain.RoleStudent:
		return domain.Page[domain.Diagram]{}, domain.ErrForbidden
	default:
		return domain.Page[domain.Diagram]{}, domain.ErrForbidden
	}
	return s.diagrams.List(ctx, filter, page)
}

// UpdateDiagram applies update to an existing diagram. Only an admin may
// update a basic diagram; a custom one may be updated by its creator or an
// admin. Kind and CreatedBy are never changed. The result is re-validated as a whole, so replaced
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
	if err := requireDiagramEditor(caller, current); err != nil {
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
	updated, err := domain.NewDiagram(current.ID, current.CreatedBy, instrument, name, positions, skillIDs, conceptIDs, updatedDiagramOptions(current, update), current.CreatedAt)
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

// updatedDiagramOptions returns current's options with update's non-nil
// root note, label display and color applied. Kind always stays current's.
func updatedDiagramOptions(current domain.Diagram, update DiagramUpdate) domain.DiagramOptions {
	opts := domain.DiagramOptions{RootNote: current.RootNote, LabelDisplay: current.LabelDisplay, Color: current.Color, Kind: current.Kind}
	if update.RootNote != nil {
		opts.RootNote = update.RootNote
	}
	if update.LabelDisplay != nil {
		opts.LabelDisplay = *update.LabelDisplay
	}
	if update.Color != nil {
		opts.Color = update.Color
	}
	return opts
}

// requireDiagramEditor returns domain.ErrForbidden unless caller may update
// diagram: an admin may update any diagram, a teacher only their own custom
// ones — never a basic one, whoever created it.
func requireDiagramEditor(caller domain.User, diagram domain.Diagram) error {
	if diagram.Kind == domain.DiagramKindBasic && caller.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	return requireOwner(caller, diagram.CreatedBy)
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
