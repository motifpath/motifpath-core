package ports

import "github.com/motifpath/core-domain/internal/domain"

// ConceptRepository persists Concept records. See TaxonomyRepository's doc
// comment for the shared shape — Concept and Skill are otherwise
// independent trees.
type ConceptRepository = TaxonomyRepository[domain.Concept]
