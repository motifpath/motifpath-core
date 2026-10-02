package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// checkClassification reports a domain.ValidationError under "skill_ids"/
// "concept_ids" if any id in skillIDs/conceptIDs does not reference an
// existing skill/concept. skill_ids is checked first, so the same request
// always reports the same field.
func checkClassification(ctx context.Context, nodes ports.KnowledgeNodeRepository, skillIDs, conceptIDs []string) error {
	return checkClassificationFor(ctx, nodes, skillIDs, conceptIDs, nil)
}

// checkClassificationSuits is checkClassification for content meant for
// instrumentIDs (empty meaning every instrument): each node must also suit
// those instruments — see domain.KnowledgeNode.Suits.
func checkClassificationSuits(ctx context.Context, nodes ports.KnowledgeNodeRepository, skillIDs, conceptIDs, instrumentIDs []string) error {
	return checkClassificationFor(ctx, nodes, skillIDs, conceptIDs, &instrumentIDs)
}

// checkClassificationFor looks every id up in one batched call, then checks
// each field in turn; instrumentIDs nil skips the instrument rule.
func checkClassificationFor(ctx context.Context, nodes ports.KnowledgeNodeRepository, skillIDs, conceptIDs []string, instrumentIDs *[]string) error {
	if len(skillIDs) == 0 && len(conceptIDs) == 0 {
		return nil
	}
	found, err := nodes.GetByIDs(ctx, append(append([]string{}, skillIDs...), conceptIDs...))
	if err != nil {
		return err
	}
	for _, field := range []struct {
		name string
		kind domain.KnowledgeNodeKind
		ids  []string
	}{
		{"skill_ids", domain.KnowledgeNodeKindSkill, skillIDs},
		{"concept_ids", domain.KnowledgeNodeKindConcept, conceptIDs},
	} {
		for _, id := range field.ids {
			node, ok := found[id]
			if !ok || node.Kind != field.kind {
				return domain.NewValidationError(field.name, "references a "+string(field.kind)+" that does not exist: "+id)
			}
			if instrumentIDs != nil && !node.Suits(*instrumentIDs) {
				return domain.NewValidationError(field.name, "references a "+string(field.kind)+" that is for none of this item's instruments: "+id)
			}
		}
	}
	return nil
}
