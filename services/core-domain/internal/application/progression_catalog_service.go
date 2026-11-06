package application

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ProgressionCatalogService keeps the progressions: the catalog's, which clients
// read to show and grade the progression drills as the server does, and those
// admins create for song charts.
type ProgressionCatalogService struct {
	progressions ports.ProgressionRepository
	uses         ports.ProgressionUseReader
	languages    ports.LanguageRepository
	knowledge    ports.KnowledgeNodeRepository
	references   ports.PracticeReferenceWriter
	newID        func() string
	now          func() time.Time
}

func NewProgressionCatalogService(
	progressions ports.ProgressionRepository,
	uses ports.ProgressionUseReader,
	languages ports.LanguageRepository,
	knowledge ports.KnowledgeNodeRepository,
	references ports.PracticeReferenceWriter,
	newID func() string,
	now func() time.Time,
) *ProgressionCatalogService {
	return &ProgressionCatalogService{progressions: progressions, uses: uses, languages: languages, knowledge: knowledge, references: references, newID: newID, now: now}
}

// List returns every progression, ordered by key: the catalog keeps no
// position of its own, so key order is the one that stays put. Only an admin
// sees who created one, so learners never get admins' names.
func (s *ProgressionCatalogService) List(ctx context.Context, caller domain.User) ([]domain.Progression, error) {
	progressions, err := s.progressions.List(ctx)
	if err != nil {
		return nil, err
	}
	uses, err := s.uses.ProgressionUses(ctx)
	if err != nil {
		return nil, err
	}
	sorted := slices.Clone(progressions)
	slices.SortFunc(sorted, func(a, b domain.Progression) int { return strings.Compare(a.Key, b.Key) })
	for i := range sorted {
		sorted[i].SongChartCount = songChartCount(uses, sorted[i].Key)
		if caller.Role != domain.RoleAdmin {
			sorted[i].CreatedBy = nil
		}
	}
	return sorted, nil
}

// ProgressionInput is a progression an admin creates. SkillIDs and ConceptIDs
// are what it practises, and count only when it enters the drills.
type ProgressionInput struct {
	Mode       string
	Bars       []string
	Nicknames  map[string]string
	InDrills   bool
	SkillIDs   []string
	ConceptIDs []string
}

// Create stores a new progression, for song charts and, when in.InDrills, for the
// drills too. Mode and numerals another one has are refused with a
// *domain.ProgressionExistsError naming it; one that can't enter the drills, with a
// *domain.ProgressionNotAddableError, and nothing is created.
func (s *ProgressionCatalogService) Create(ctx context.Context, caller domain.User, in ProgressionInput) (domain.Progression, error) {
	if err := requireAdmin(caller); err != nil {
		return domain.Progression{}, err
	}
	created, err := s.newProgression(ctx, in)
	if err != nil {
		return domain.Progression{}, err
	}
	now := s.now()
	created.ID, created.CreatedBy, created.CreatedAt = s.newID(), &caller.ID, &now
	err = s.progressions.Create(ctx, created)
	if errors.Is(err, domain.ErrConflict) {
		// Another admin created it since the check above.
		if existing := s.refuseExisting(ctx, created); existing != nil {
			return domain.Progression{}, existing
		}
	}
	if err != nil {
		return domain.Progression{}, err
	}
	if created.InDrills {
		putProgressionReference(ctx, s.references, created)
	}
	return created, nil
}

// newProgression reads in as a progression no other has, ready to store, in the
// drills when in.InDrills says so.
func (s *ProgressionCatalogService) newProgression(ctx context.Context, in ProgressionInput) (domain.Progression, error) {
	if in.InDrills {
		if err := s.checkDrillNodes(ctx, in.SkillIDs, in.ConceptIDs); err != nil {
			return domain.Progression{}, err
		}
	}
	languages, err := s.nameLanguages(ctx)
	if err != nil {
		return domain.Progression{}, err
	}
	created, err := domain.NewCreatedProgression(in.Mode, in.Bars, in.Nicknames, languages)
	if err != nil {
		return domain.Progression{}, err
	}
	if err := s.refuseExisting(ctx, created); err != nil {
		return domain.Progression{}, err
	}
	if !in.InDrills {
		return created, nil
	}
	drills, err := s.progressions.ListInDrills(ctx)
	if err != nil {
		return domain.Progression{}, err
	}
	return created.AddedToDrills(slices.Concat(in.SkillIDs, in.ConceptIDs), drills)
}

// checkDrillNodes refuses a progression for the drills that practises nothing,
// or something the knowledge map doesn't have. Any instrument may play a
// progression, so a node for one instrument is as good as any.
func (s *ProgressionCatalogService) checkDrillNodes(ctx context.Context, skillIDs, conceptIDs []string) error {
	if len(skillIDs) == 0 && len(conceptIDs) == 0 {
		return domain.NewValidationError("skill_ids", "a progression in the drills needs a skill or a concept")
	}
	return checkClassificationSuits(ctx, s.knowledge, skillIDs, conceptIDs, nil)
}

// UpdateNicknames replaces a created progression's nicknames. Nothing else about
// it ever changes; a catalog progression's names don't either.
func (s *ProgressionCatalogService) UpdateNicknames(ctx context.Context, caller domain.User, id string, nicknames map[string]string) (domain.Progression, error) {
	if err := requireAdmin(caller); err != nil {
		return domain.Progression{}, err
	}
	stored, err := s.progressions.GetByID(ctx, id)
	if err != nil {
		return domain.Progression{}, err
	}
	if stored.IsCatalog() {
		return domain.Progression{}, domain.ErrProgressionIsCatalog
	}
	languages, err := s.nameLanguages(ctx)
	if err != nil {
		return domain.Progression{}, err
	}
	renamed, err := stored.WithNicknames(nicknames, languages)
	if err != nil {
		return domain.Progression{}, err
	}
	if err := s.progressions.SaveNames(ctx, renamed); err != nil {
		return domain.Progression{}, err
	}
	return s.withSongChartCount(ctx, renamed)
}

// Delete removes a created progression no chart's draft or latest published
// revision tags. A drill progression is never removed, since learners' practice
// points at it. Older revisions keep the key, which spells the numerals.
func (s *ProgressionCatalogService) Delete(ctx context.Context, caller domain.User, id string) error {
	if err := requireAdmin(caller); err != nil {
		return err
	}
	stored, err := s.progressions.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if stored.InDrills {
		return &domain.ProgressionNotRemovableError{Reason: domain.ProgressionNotRemovableInDrills}
	}
	all, err := s.uses.ProgressionUses(ctx)
	if err != nil {
		return err
	}
	uses := slices.DeleteFunc(all, func(u domain.ProgressionUse) bool { return u.ProgressionKey != stored.Key })
	if len(uses) > 0 {
		return &domain.ProgressionNotRemovableError{Reason: domain.ProgressionNotRemovableInUse, Uses: uses}
	}
	return s.progressions.Delete(ctx, id)
}

func (s *ProgressionCatalogService) refuseExisting(ctx context.Context, p domain.Progression) error {
	existing, err := s.progressions.FindByNumerals(ctx, p.Mode, p.Bars)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if existing, err = s.withSongChartCount(ctx, existing); err != nil {
		return err
	}
	return &domain.ProgressionExistsError{Existing: existing}
}

func (s *ProgressionCatalogService) withSongChartCount(ctx context.Context, p domain.Progression) (domain.Progression, error) {
	uses, err := s.uses.ProgressionUses(ctx)
	if err != nil {
		return domain.Progression{}, err
	}
	p.SongChartCount = songChartCount(uses, p.Key)
	return p, nil
}

// nameLanguages are the codes a progression's names are written in: every
// language but the language-agnostic marker.
func (s *ProgressionCatalogService) nameLanguages(ctx context.Context) ([]string, error) {
	languages, err := s.languages.List(ctx)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(languages))
	for _, l := range languages {
		if l.Code != domain.LanguageCodeAny {
			codes = append(codes, l.Code)
		}
	}
	return codes, nil
}

// songChartCount is how many charts tag key, however many sections do.
func songChartCount(uses []domain.ProgressionUse, key string) int {
	charts := map[string]bool{}
	for _, u := range uses {
		if u.ProgressionKey == key {
			charts[u.SongChartID] = true
		}
	}
	return len(charts)
}
