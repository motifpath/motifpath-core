package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// DrillTemplateRepository reads the installed drill templates.
type DrillTemplateRepository interface {
	// List returns every template, by key.
	List(ctx context.Context) ([]domain.DrillTemplate, error)
}
