package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// PracticeSubjectCatalogRepository keeps the precomputed practice subject
// catalog, one row per scope, and the content version that tells which rows
// are current: a content change raises it, and a row built from an older
// version is out of date.
type PracticeSubjectCatalogRepository interface {
	// Catalogs returns the catalog of each of scopes, keyed by scope, and the
	// current content version, 0 before it is first raised. A scope with no
	// catalog is simply absent.
	Catalogs(ctx context.Context, scopes []string) (map[string]domain.PracticeSubjectCatalog, int64, error)

	// BumpContentVersion raises the content version by one.
	BumpContentVersion(ctx context.Context) error

	// Store replaces the rows of catalogs' scopes with catalogs, built from
	// content version version, unless the version has been raised since: a
	// raise at the same time waits for the store, or the store sees it. It
	// reports whether it stored them.
	Store(ctx context.Context, version int64, catalogs []domain.PracticeSubjectCatalog) (bool, error)
}
