package repo

import (
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// parseUUIDs parses each of ids, skipping (rather than failing on) any value
// that isn't a valid uuid — the same "not found, simply absent" convention
// ContentNodeRepository.GetByIDs documents, since ids reaching this layer
// have already been validated non-empty by the domain/application layers.
func parseUUIDs(ids []string) ([]uuid.UUID, error) {
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, u)
	}
	return parsed, nil
}

// domainSkillsFromEdges converts an eager-loaded []*ent.Skill edge slice to
// its domain shape, in full (id, name, parent_id) — the shape ContentNode/
// Exercise responses embed rather than bare ids.
func domainSkillsFromEdges(rows []*ent.Skill) []domain.Skill {
	result := make([]domain.Skill, len(rows))
	for i, row := range rows {
		result[i] = toDomainSkill(row)
	}
	return result
}

// domainConceptsFromEdges is domainSkillsFromEdges' counterpart for Concept.
func domainConceptsFromEdges(rows []*ent.Concept) []domain.Concept {
	result := make([]domain.Concept, len(rows))
	for i, row := range rows {
		result[i] = toDomainConcept(row)
	}
	return result
}

// skillIDsOf extracts the ids from a []domain.Skill — used where a domain
// value (e.g. domain.Exercise.Skills) carries full Skill structs but only
// the ids are needed to persist the join edge.
func skillIDsOf(skills []domain.Skill) []string {
	ids := make([]string, len(skills))
	for i, s := range skills {
		ids[i] = s.ID
	}
	return ids
}

// conceptIDsOf is skillIDsOf's counterpart for []domain.Concept.
func conceptIDsOf(concepts []domain.Concept) []string {
	ids := make([]string, len(concepts))
	for i, c := range concepts {
		ids[i] = c.ID
	}
	return ids
}
