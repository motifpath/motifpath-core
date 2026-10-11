package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// AppConfigRepository stores the app's settings, one value per key.
type AppConfigRepository interface {
	// Get returns the setting with the given key, or domain.ErrNotFound if it was never set.
	Get(ctx context.Context, key string) (domain.AppConfigEntry, error)
	// Put stores entry, replacing any value its key had.
	Put(ctx context.Context, entry domain.AppConfigEntry) error
	// List returns every setting that has a value, ordered by key.
	List(ctx context.Context) ([]domain.AppConfigEntry, error)
}
