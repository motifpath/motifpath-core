package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// LanguageRepository looks up the Language lookup rows seeded by the
// database migration (en, pt_BR, any).
type LanguageRepository interface {
	// GetByCode returns the language with the given code. Returns
	// domain.ErrNotFound if no such language exists.
	GetByCode(ctx context.Context, code string) (domain.Language, error)
}
