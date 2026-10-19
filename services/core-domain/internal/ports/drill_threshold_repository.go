package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// DrillThresholdRepository reads the installed fluent time versions.
type DrillThresholdRepository interface {
	// List returns every version, by template key then version.
	List(ctx context.Context) ([]domain.DrillThreshold, error)
}
