package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// SongChartRepository stores song charts with their drafts, and the
// revisions they were published as. A revision is never changed once
// stored.
type SongChartRepository interface {
	// Create stores a new chart.
	Create(ctx context.Context, chart domain.SongChart) error
	// GetByID returns the chart with id, or domain.ErrNotFound.
	GetByID(ctx context.Context, id string) (domain.SongChart, error)
	// List returns the charts matching filter, most recently updated first,
	// then by id.
	List(ctx context.Context, filter domain.SongChartFilter, page domain.PageRequest) (domain.Page[domain.SongChart], error)
	// Save replaces a stored chart's status, draft and withdrawal.
	Save(ctx context.Context, chart domain.SongChart) error
	// Publish saves chart and stores rev, its new revision, together: either
	// both happen or neither does.
	Publish(ctx context.Context, chart domain.SongChart, rev domain.SongChartRevision) error
	// ListRevisions returns a chart's revisions, newest first.
	ListRevisions(ctx context.Context, chartID string) ([]domain.SongChartRevision, error)
	// GetRevision returns one revision of a chart, or domain.ErrNotFound.
	GetRevision(ctx context.Context, chartID string, number int) (domain.SongChartRevision, error)
}
