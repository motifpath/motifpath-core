//go:build integration

package bdd

import (
	"context"
	"sort"
	"sync"

	"github.com/motifpath/core-domain/internal/domain"
)

// fakeSongChartRepo is an in-memory ports.SongChartRepository.
type fakeSongChartRepo struct {
	mu        sync.Mutex
	charts    map[string]domain.SongChart
	revisions map[string][]domain.SongChartRevision
}

func newFakeSongChartRepo() *fakeSongChartRepo {
	return &fakeSongChartRepo{charts: map[string]domain.SongChart{}, revisions: map[string][]domain.SongChartRevision{}}
}

func (f *fakeSongChartRepo) Create(_ context.Context, chart domain.SongChart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.charts[chart.ID] = chart
	return nil
}

func (f *fakeSongChartRepo) GetByID(_ context.Context, id string) (domain.SongChart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	chart, ok := f.charts[id]
	if !ok {
		return domain.SongChart{}, domain.ErrNotFound
	}
	return chart, nil
}

func (f *fakeSongChartRepo) List(_ context.Context, filter domain.SongChartFilter, page domain.PageRequest) (domain.Page[domain.SongChart], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var matched []domain.SongChart
	for _, c := range f.charts {
		if filter.Status != nil && c.Status != *filter.Status {
			continue
		}
		if filter.Q != "" && !domain.ContainsLoosely(c.Draft.Title, filter.Q) && !domain.ContainsLoosely(c.Draft.Artist, filter.Q) {
			continue
		}
		matched = append(matched, c)
	}
	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].Draft.UpdatedAt.Equal(matched[j].Draft.UpdatedAt) {
			return matched[i].Draft.UpdatedAt.After(matched[j].Draft.UpdatedAt)
		}
		return matched[i].ID < matched[j].ID
	})
	total := len(matched)
	end := min(page.Offset+page.Limit, total)
	if page.Offset >= total {
		return domain.Page[domain.SongChart]{Total: total}, nil
	}
	return domain.Page[domain.SongChart]{Items: matched[page.Offset:end], Total: total}, nil
}

func (f *fakeSongChartRepo) Save(_ context.Context, chart domain.SongChart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.charts[chart.ID]; !ok {
		return domain.ErrNotFound
	}
	f.charts[chart.ID] = chart
	return nil
}

func (f *fakeSongChartRepo) Publish(_ context.Context, chart domain.SongChart, rev domain.SongChartRevision) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.charts[chart.ID] = chart
	f.revisions[chart.ID] = append([]domain.SongChartRevision{rev}, f.revisions[chart.ID]...)
	return nil
}

func (f *fakeSongChartRepo) ListRevisions(_ context.Context, chartID string) ([]domain.SongChartRevision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.SongChartRevision(nil), f.revisions[chartID]...), nil
}

func (f *fakeSongChartRepo) GetRevision(_ context.Context, chartID string, number int) (domain.SongChartRevision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, rev := range f.revisions[chartID] {
		if rev.Number == number {
			return rev, nil
		}
	}
	return domain.SongChartRevision{}, domain.ErrNotFound
}

func (f *fakeSongChartRepo) ExistingIDs(_ context.Context, ids []string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, id := range ids {
		if _, ok := f.charts[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}
