package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ChordDrillRepository reads the installed chord drills, change pairs and
// change targets.
type ChordDrillRepository interface {
	// Catalog returns them all, drills and pairs in catalog order and targets
	// by version.
	Catalog(ctx context.Context) (domain.ChordDrillCatalog, error)
}
