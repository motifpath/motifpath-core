package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/ports"
)

// withSubtrees widens ids to every skill or concept below each of them, so a
// classification filter that names an area also finds what is classified
// with its sub-skills or sub-concepts. ids themselves stay in the result,
// including one that doesn't exist, which then simply matches nothing.
func withSubtrees(ctx context.Context, knowledge ports.KnowledgeNodeRepository, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return ids, nil
	}
	below, err := knowledge.SubtreeIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	widened := append([]string{}, ids...)
	seen := make(map[string]bool, len(ids)+len(below))
	for _, id := range ids {
		seen[id] = true
	}
	for _, id := range below {
		if !seen[id] {
			seen[id] = true
			widened = append(widened, id)
		}
	}
	return widened, nil
}

// withClassificationSubtrees applies withSubtrees to a filter's skill and
// concept ids.
func withClassificationSubtrees(ctx context.Context, knowledge ports.KnowledgeNodeRepository, skillIDs, conceptIDs []string) ([]string, []string, error) {
	skills, err := withSubtrees(ctx, knowledge, skillIDs)
	if err != nil {
		return nil, nil, err
	}
	concepts, err := withSubtrees(ctx, knowledge, conceptIDs)
	if err != nil {
		return nil, nil, err
	}
	return skills, concepts, nil
}
