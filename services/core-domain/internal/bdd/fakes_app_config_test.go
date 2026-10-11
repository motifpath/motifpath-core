//go:build integration

package bdd

import (
	"context"
	"sort"
	"sync"

	"github.com/motifpath/core-domain/internal/domain"
)

type fakeAppConfigRepo struct {
	mu    sync.Mutex
	byKey map[string]domain.AppConfigEntry
}

func newFakeAppConfigRepo() *fakeAppConfigRepo {
	return &fakeAppConfigRepo{byKey: map[string]domain.AppConfigEntry{}}
}

func (f *fakeAppConfigRepo) Get(_ context.Context, key string) (domain.AppConfigEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, ok := f.byKey[key]
	if !ok {
		return domain.AppConfigEntry{}, domain.ErrNotFound
	}
	return entry, nil
}

func (f *fakeAppConfigRepo) Put(_ context.Context, entry domain.AppConfigEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byKey[entry.Key] = entry
	return nil
}

func (f *fakeAppConfigRepo) List(_ context.Context) ([]domain.AppConfigEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.AppConfigEntry, 0, len(f.byKey))
	for _, entry := range f.byKey {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}
