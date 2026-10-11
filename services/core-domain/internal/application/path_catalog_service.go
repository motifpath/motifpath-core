package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// PathCatalogService is the learner path catalog: the published learning
// paths any user can browse, read and enroll in. It is the same for every
// caller whatever their role, and never shows a draft.
type PathCatalogService struct {
	paths     ports.LearningPathRepository
	users     ports.UserRepository
	knowledge ports.KnowledgeNodeRepository
}

func NewPathCatalogService(paths ports.LearningPathRepository, users ports.UserRepository, knowledge ports.KnowledgeNodeRepository) *PathCatalogService {
	return &PathCatalogService{paths: paths, users: users, knowledge: knowledge}
}

// ListCatalogPaths returns one page of the published paths matching filter,
// ordered by title then id; a skill or concept also matches everything
// classified below it. Whatever status or sort filter carries, the catalog
// lists published paths only, by title.
func (s *PathCatalogService) ListCatalogPaths(ctx context.Context, filter domain.LearningPathFilter, page domain.PageRequest) (domain.Page[domain.LearningPath], error) {
	var err error
	filter.SkillIDs, filter.ConceptIDs, err = withClassificationSubtrees(ctx, s.knowledge, filter.SkillIDs, filter.ConceptIDs)
	if err != nil {
		return domain.Page[domain.LearningPath]{}, err
	}
	return s.paths.List(ctx, catalogPathFilter(filter), page)
}

// catalogPathFilter narrows filter to what the learner catalog shows: published paths only, by
// title.
func catalogPathFilter(filter domain.LearningPathFilter) domain.LearningPathFilter {
	filter.Status = domain.LearningPathStatusPublished
	filter.Sort = domain.ListSortTitle
	return filter
}

// GetCatalogPath returns the published path with the given id. A draft is
// domain.ErrNotFound, even for its author: the catalog is not a preview.
func (s *PathCatalogService) GetCatalogPath(ctx context.Context, id string) (domain.LearningPath, error) {
	path, err := s.paths.GetByID(ctx, id)
	if err != nil {
		return domain.LearningPath{}, err
	}
	if path.Status != domain.LearningPathStatusPublished {
		return domain.LearningPath{}, domain.ErrNotFound
	}
	return path, nil
}

// ListCatalogPathCreators returns every user who created at least one
// published path, each once, ordered by name as a person reads names;
// nameQuery, when non-empty, keeps those whose name contains it.
func (s *PathCatalogService) ListCatalogPathCreators(ctx context.Context, nameQuery string) ([]Creator, error) {
	counts, err := s.paths.ListCreatorCounts(ctx, domain.LearningPathFilter{Status: domain.LearningPathStatusPublished})
	if err != nil {
		return nil, err
	}
	return namedCreators(ctx, s.users, counts, nameQuery)
}
