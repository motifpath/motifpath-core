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
	paths ports.LearningPathRepository
	users ports.UserRepository
}

func NewPathCatalogService(paths ports.LearningPathRepository, users ports.UserRepository) *PathCatalogService {
	return &PathCatalogService{paths: paths, users: users}
}

// ListCatalogPaths returns one page of the published paths matching filter,
// ordered by title then id. Whatever status or sort filter carries, the
// catalog lists published paths only, by title.
func (s *PathCatalogService) ListCatalogPaths(ctx context.Context, filter domain.LearningPathFilter, page domain.PageRequest) (domain.Page[domain.LearningPath], error) {
	filter.Status = domain.LearningPathStatusPublished
	filter.Sort = domain.LearningPathSortTitle
	return s.paths.List(ctx, filter, page)
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
	ids, err := s.paths.ListCreatorIDs(ctx, domain.LearningPathFilter{Status: domain.LearningPathStatusPublished})
	if err != nil {
		return nil, err
	}
	return namedCreators(ctx, s.users, ids, nameQuery)
}
