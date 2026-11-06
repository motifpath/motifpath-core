//go:build integration

package bdd

import (
	"context"
	"slices"
	"strings"

	"github.com/motifpath/core-domain/internal/domain"
)

// fakeProgressionRepo is the catalog, read from motifpath-specs, and the
// progressions a scenario creates.
type fakeProgressionRepo struct {
	created []domain.Progression
}

func (f *fakeProgressionRepo) List(ctx context.Context) ([]domain.Progression, error) {
	catalog, err := catalogProgressions{}.List(ctx)
	if err != nil {
		return nil, err
	}
	all := slices.Concat(catalog, f.created)
	slices.SortFunc(all, func(a, b domain.Progression) int { return strings.Compare(a.Key, b.Key) })
	return all, nil
}

func (f *fakeProgressionRepo) ListInDrills(ctx context.Context) ([]domain.Progression, error) {
	all, err := f.List(ctx)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(p domain.Progression) bool { return !p.InDrills }), nil
}

func (f *fakeProgressionRepo) find(ctx context.Context, match func(domain.Progression) bool) (domain.Progression, error) {
	all, err := f.List(ctx)
	if err != nil {
		return domain.Progression{}, err
	}
	i := slices.IndexFunc(all, match)
	if i < 0 {
		return domain.Progression{}, domain.ErrNotFound
	}
	return all[i], nil
}

func (f *fakeProgressionRepo) GetByID(ctx context.Context, id string) (domain.Progression, error) {
	return f.find(ctx, func(p domain.Progression) bool { return p.ID == id })
}

func (f *fakeProgressionRepo) FindByNumerals(ctx context.Context, mode string, bars []string) (domain.Progression, error) {
	return f.find(ctx, func(p domain.Progression) bool { return p.Mode == mode && slices.Equal(p.Bars, bars) })
}

func (f *fakeProgressionRepo) Create(_ context.Context, p domain.Progression) error {
	f.created = append(f.created, p)
	return nil
}

func (f *fakeProgressionRepo) SaveNames(_ context.Context, p domain.Progression) error {
	for i := range f.created {
		if f.created[i].ID == p.ID {
			f.created[i].Names, f.created[i].Nicknames = p.Names, p.Nicknames
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeProgressionRepo) Delete(_ context.Context, id string) error {
	f.created = slices.DeleteFunc(f.created, func(p domain.Progression) bool { return p.ID == id })
	return nil
}

// ProgressionUses finds the tags in every chart's draft and latest published
// revision, as the stored charts hold them.
func (f *fakeSongChartRepo) ProgressionUses(ctx context.Context) ([]domain.ProgressionUse, error) {
	var uses []domain.ProgressionUse
	for id, chart := range f.charts {
		uses = append(uses, sectionUses(chart.Draft.Body, id, chart.Draft.Title, domain.ProgressionUseDraft)...)
		if chart.PublishedRevision == nil {
			continue
		}
		rev, err := f.GetRevision(ctx, id, chart.PublishedRevision.Number)
		if err != nil {
			return nil, err
		}
		uses = append(uses, sectionUses(rev.Body, id, rev.Title, domain.ProgressionUsePublished)...)
	}
	return uses, nil
}

func sectionUses(doc domain.SongChartDocument, chartID, title string, revision domain.ProgressionUseRevision) []domain.ProgressionUse {
	var uses []domain.ProgressionUse
	for i, section := range doc.Sections {
		for _, key := range section.ProgressionKeys {
			uses = append(uses, domain.ProgressionUse{
				ProgressionKey: key, SongChartID: chartID, Title: title, Revision: revision, SectionIndex: i, SectionLabel: section.Label,
			})
		}
	}
	return uses
}
