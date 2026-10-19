package application

import (
	"context"
	"slices"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ProgressionCatalogService lists the progression catalog, the reference
// data clients read to show and grade the progression drills as the
// server does.
type ProgressionCatalogService struct {
	progressions ports.ProgressionRepository
}

func NewProgressionCatalogService(progressions ports.ProgressionRepository) *ProgressionCatalogService {
	return &ProgressionCatalogService{progressions: progressions}
}

// List returns every progression, ordered by key: the catalog keeps no
// position of its own, so key order is the one that stays put.
func (s *ProgressionCatalogService) List(ctx context.Context) ([]domain.Progression, error) {
	progressions, err := s.progressions.List(ctx)
	if err != nil {
		return nil, err
	}
	sorted := slices.Clone(progressions)
	slices.SortFunc(sorted, func(a, b domain.Progression) int { return strings.Compare(a.Key, b.Key) })
	return sorted, nil
}
