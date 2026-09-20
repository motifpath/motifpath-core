package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// SkillRepository persists Skill records — the tree ContentNode and
// Exercise classify against. There is no update/delete method — creating
// this repository's counterpart for a resource with no update/delete
// endpoint yet, per ADR-026.
type SkillRepository interface {
	Create(ctx context.Context, skill domain.Skill) error

	// GetByID returns domain.ErrNotFound if no skill exists with the given id.
	GetByID(ctx context.Context, id string) (domain.Skill, error)

	// GetByIDs returns the skills matching ids, keyed by ID. An id with no
	// matching skill is simply absent from the result — the same
	// "not found, simply absent" convention ContentNodeRepository.GetByIDs
	// documents, letting a caller detect a bad id by checking for the key.
	GetByIDs(ctx context.Context, ids []string) (map[string]domain.Skill, error)

	// List returns every known skill, sorted by name within each level.
	List(ctx context.Context) ([]domain.Skill, error)

	// ExistsSibling reports whether a skill named name already exists under
	// parentID (nil meaning a root skill) — the sibling-uniqueness rule
	// ADR-026 requires but a plain unique index on a nullable parent_id
	// column can't enforce (Postgres treats NULL as distinct from NULL).
	ExistsSibling(ctx context.Context, parentID *string, name string) (bool, error)
}
