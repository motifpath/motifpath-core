package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// PracticeSubjectCatalogRepository keeps the precomputed practice subject
// catalog, one row per scope, and the queue of signals that it needs
// rebuilding.
type PracticeSubjectCatalogRepository interface {
	// Catalogs returns the catalog of each of scopes, keyed by scope. A scope
	// with no catalog is simply absent.
	Catalogs(ctx context.Context, scopes []string) (map[string]domain.PracticeSubjectCatalog, error)

	// Signal queues a signal that the catalog needs rebuilding.
	Signal(ctx context.Context, reason domain.PracticeCatalogSignalReason) error

	// Rebuild rebuilds the catalog for every signal pending when it starts,
	// holding a lock no other rebuild can hold at the same time: the
	// catalogs build returns replace every row, and those signals are
	// removed, together. A signal queued while build runs stays for the
	// next rebuild. It reports false, building nothing, when nothing is
	// pending or another rebuild holds the lock; on an error from build,
	// the rows and the signals stay as they were.
	Rebuild(ctx context.Context, build func(context.Context) ([]domain.PracticeSubjectCatalog, error)) (bool, error)
}
