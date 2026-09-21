package repo

import (
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// parseUUIDs parses each of ids, failing on the first value that isn't a
// valid uuid. Unlike GetByIDs' "not found, simply absent" convention, a
// malformed id here is a hard error: by the time ids reach this layer they
// have already been checked to reference real Skill/Concept rows (and are
// therefore expected to be well-formed uuids), so a parse failure signals a
// bug upstream rather than a legitimately absent reference.
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

// parseOptionalUUID parses *s if s is non-nil and non-empty, returning nil
// otherwise — the shape a self-referential optional parent_id field takes
// at this layer.
func parseOptionalUUID(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// parseUUIDsSkippingInvalid is parseUUIDs' counterpart for lookup filters
// following the "not found, simply absent" convention: an id that isn't a
// valid uuid can never match a row, so it's dropped rather than failing the
// whole lookup.
func parseUUIDsSkippingInvalid(ids []string) []uuid.UUID {
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if u, err := uuid.Parse(id); err == nil {
			parsed = append(parsed, u)
		}
	}
	return parsed
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
