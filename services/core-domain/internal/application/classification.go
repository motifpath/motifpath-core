package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// checkClassificationSuits reports a domain.ValidationError under
// "skill_ids"/"concept_ids" if any id in skillIDs/conceptIDs does not
// reference an existing skill/concept, or references one that does not suit
// instrumentIDs (empty meaning every instrument) — see
// domain.KnowledgeNode.Suits. Every id is looked up in one batched call;
// skill_ids is checked first, so the same request always reports the same
// field.
func checkClassificationSuits(ctx context.Context, nodes ports.KnowledgeNodeRepository, skillIDs, conceptIDs, instrumentIDs []string) error {
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
			if !node.Suits(instrumentIDs) {
				return domain.NewValidationError(field.name, "references a "+string(field.kind)+" that is for none of this item's instruments: "+id)
			}
		}
	}
	return nil
}
