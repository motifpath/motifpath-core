package ports

import "github.com/motifpath/core-domain/internal/domain"

// SkillRepository persists Skill records — the tree ContentNode and
// Exercise classify against. See TaxonomyRepository's doc comment for the
// shared shape.
type SkillRepository = TaxonomyRepository[domain.Skill]
