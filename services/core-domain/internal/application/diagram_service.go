package application

import (
	"context"
	"errors"
	"slices"
	"time"
	"unicode/utf8"

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
	languages   ports.LanguageRepository
	users       ports.UserRepository
	newID       func() string
	now         func() time.Time
}

func NewDiagramService(diagrams ports.DiagramRepository, instruments ports.InstrumentRepository, skills ports.SkillRepository, concepts ports.ConceptRepository, languages ports.LanguageRepository, users ports.UserRepository, newID func() string, now func() time.Time) *DiagramService {
	return &DiagramService{diagrams: diagrams, instruments: instruments, skills: skills, concepts: concepts, languages: languages, users: users, newID: newID, now: now}
}

const (
	// maxDiagramNameSearchLength bounds a diagram list's name search.
	maxDiagramNameSearchLength = 200
	// maxRootNoteFilterLength fits the longest note spelling, e.g. "C#" or "Bb", with room for a double accidental.
	maxRootNoteFilterLength = 3
)

// DiagramUpdate carries the fields UpdateDiagram may replace. A nil field
// leaves the current value untouched. Classification is replaced as a unit:
// supplying either SkillIDs or ConceptIDs replaces both, so a caller
// changing one must resend the other. There is no way to clear an
// already-set RootNote back to nil through an update — only to replace it
// with another value or leave it as-is; Color behaves the same way.
// Per-position colors, custom labels and notes travel with Positions and so
// can be cleared by resending Positions without them.
type DiagramUpdate struct {
	// Names replaces every name when non-nil; nil keeps the current ones.
	Names     map[string]string
	Positions []domain.Position
	// Regions replaces every region when non-nil — an empty, non-nil slice
	// removes them all; nil keeps the current ones.
	Regions      []domain.Region
	SkillIDs     []string
	ConceptIDs   []string
	RootNote     *string
	LabelDisplay *domain.LabelDisplay
	Color        *string
	// Mode replaces the mode when Set; a nil Value clears it.
	Mode Nullable[domain.DiagramMode]
	// TempoBPM replaces the tempo when Set; a nil Value clears it, which is
	// only valid when the resulting sequence is empty.
	TempoBPM      Nullable[int]
	TimeSignature *domain.TimeSignature
	// Sequence replaces every step when non-nil — an empty, non-nil slice
	// removes the playback; nil keeps the current steps.
	Sequence []domain.SequenceStep
}

// Nullable is an update field with three states: left out (Set false)
// keeps the current value, Set with a nil Value clears it, and Set with a
// Value replaces it.
type Nullable[T any] struct {
	Set   bool
	Value *T
}

// CreateDiagram creates a diagram against an existing instrument, owned by
// caller. Only teachers and admins may create one, and only an admin may
// create a basic one; opts.Kind defaults to custom. Positions and regions
// without an id are assigned one; a client-supplied id is kept. See
// domain.DiagramOptions for the other optional settings.
func (s *DiagramService) CreateDiagram(ctx context.Context, caller domain.User, instrumentID string, names map[string]string, positions []domain.Position, skillIDs, conceptIDs []string, opts domain.DiagramOptions) (domain.Diagram, error) {
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

	offered, err := offeredLanguages(ctx, s.languages)
	if err != nil {
		return domain.Diagram{}, err
	}
	opts.Regions = s.withRegionIDs(opts.Regions)
	diagram, err := domain.NewDiagram(s.newID(), caller.ID, instrument, names, offered, s.withPositionIDs(positions), skillIDs, conceptIDs, opts, s.now())
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
// may discover, ordered by the name caller reads (their locale, then English,
// then the first language a diagram is named in). Students may not list
// diagrams at all. A teacher sees every basic diagram plus their own custom
// ones, and an admin every diagram; the other filters, filter.CreatedBy
// included, only narrow that. filter.VisibleTo is set here from caller's
// role; any value the caller supplied is overwritten.
func (s *DiagramService) ListDiagrams(ctx context.Context, caller domain.User, filter domain.DiagramListFilter, page domain.PageRequest) (domain.Page[domain.Diagram], error) {
	if filter.Kind != "" && !filter.Kind.Valid() {
		return domain.Page[domain.Diagram]{}, domain.NewValidationError("kind", "must be one of: basic, custom")
	}
	if utf8.RuneCountInString(filter.Name) > maxDiagramNameSearchLength {
		return domain.Page[domain.Diagram]{}, domain.NewValidationError("name", "must be at most 200 characters")
	}
	if utf8.RuneCountInString(filter.RootNote) > maxRootNoteFilterLength {
		return domain.Page[domain.Diagram]{}, domain.NewValidationError("root_note", "must be at most 3 characters")
	}
	if filter.Language != "" {
		offered, err := offeredLanguages(ctx, s.languages)
		if err != nil {
			return domain.Page[domain.Diagram]{}, err
		}
		if !slices.Contains(offered, filter.Language) {
			return domain.Page[domain.Diagram]{}, domain.NewValidationError("language", "must be one of the languages MotifPath offers")
		}
	}
	filter.Locale = caller.Locale.Code
	visibleTo, err := diagramVisibility(caller)
	if err != nil {
		return domain.Page[domain.Diagram]{}, err
	}
	filter.VisibleTo = visibleTo
	return s.diagrams.List(ctx, filter, page)
}

// ListDiagramCreators returns the distinct creators of the diagrams caller
// may discover in ListDiagrams, so a picker can offer a complete creator
// filter without paging: a teacher gets the creators of basic diagrams plus
// themselves once they have a custom diagram, an admin every creator.
// Students are refused. A non-empty nameQuery keeps only the creators whose
// display name contains it; see namedCreators for matching and ordering.
func (s *DiagramService) ListDiagramCreators(ctx context.Context, caller domain.User, nameQuery string) ([]Creator, error) {
	visibleTo, err := diagramVisibility(caller)
	if err != nil {
		return nil, err
	}
	ids, err := s.diagrams.ListCreatorIDs(ctx, domain.DiagramListFilter{VisibleTo: visibleTo})
	if err != nil {
		return nil, err
	}
	return namedCreators(ctx, s.users, ids, nameQuery)
}

// diagramVisibility returns the DiagramListFilter.VisibleTo scoping caller's
// role gets in the library: their own id for a teacher, none for an admin.
// Students never browse the library — they read embedded diagrams by id —
// and any role this doesn't know is refused the same way.
func diagramVisibility(caller domain.User) (string, error) {
	switch caller.Role {
	case domain.RoleTeacher:
		return caller.ID, nil
	case domain.RoleAdmin:
		return "", nil
	case domain.RoleStudent:
		return "", domain.ErrForbidden
	default:
		return "", domain.ErrForbidden
	}
}

// UpdateDiagram applies update to an existing diagram. Only an admin may
// update a basic diagram; a custom one may be updated by its creator or an
// admin. Kind and CreatedBy are never changed. The result is re-validated
// as a whole, so replaced positions and regions are checked against the
// diagram's own instrument, and every custom label, note and region
// description against the languages of the names as updated — a rename that
// drops a language the existing notes still use is refused. The instrument
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

	names := map[string]string(current.Names)
	if update.Names != nil {
		names = update.Names
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
	offered, err := offeredLanguages(ctx, s.languages)
	if err != nil {
		return domain.Diagram{}, err
	}
	opts := updatedDiagramOptions(current, update)
	if update.Regions != nil {
		opts.Regions = s.withRegionIDs(update.Regions)
	}
	updated, err := domain.NewDiagram(current.ID, current.CreatedBy, instrument, names, offered, positions, skillIDs, conceptIDs, opts, current.CreatedAt)
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

// updatedDiagramOptions returns current's options with update's given root
// note, label display, color, mode, tempo, time signature and sequence
// applied. Kind always stays current's; regions are current's too, for the
// caller to replace.
func updatedDiagramOptions(current domain.Diagram, update DiagramUpdate) domain.DiagramOptions {
	opts := domain.DiagramOptions{
		RootNote: current.RootNote, LabelDisplay: current.LabelDisplay, Color: current.Color, Kind: current.Kind, Regions: current.Regions,
		Mode: current.Mode, TempoBPM: current.TempoBPM, TimeSignature: current.TimeSignature, Sequence: current.Sequence,
	}
	if update.RootNote != nil {
		opts.RootNote = update.RootNote
	}
	if update.LabelDisplay != nil {
		opts.LabelDisplay = *update.LabelDisplay
	}
	if update.Color != nil {
		opts.Color = update.Color
	}
	if update.Mode.Set {
		opts.Mode = update.Mode.Value
	}
	if update.TempoBPM.Set {
		opts.TempoBPM = update.TempoBPM.Value
	}
	if update.TimeSignature != nil {
		opts.TimeSignature = *update.TimeSignature
	}
	if update.Sequence != nil {
		opts.Sequence = update.Sequence
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

// withRegionIDs returns a copy of regions with an id assigned to every
// region that lacks one, leaving the caller's slice untouched. A nil slice
// stays nil.
func (s *DiagramService) withRegionIDs(regions []domain.Region) []domain.Region {
	if regions == nil {
		return nil
	}
	out := make([]domain.Region, len(regions))
	copy(out, regions)
	for i := range out {
		if out[i].ID == "" {
			out[i].ID = s.newID()
		}
	}
	return out
}
