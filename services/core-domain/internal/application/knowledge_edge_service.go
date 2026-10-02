package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// KnowledgeEdgeService manages the typed links between knowledge nodes. Any
// authenticated user may read them; only an admin may change them.
type KnowledgeEdgeService struct {
	edges ports.KnowledgeEdgeRepository
	nodes ports.KnowledgeNodeRepository
	newID func() string
}

func NewKnowledgeEdgeService(edges ports.KnowledgeEdgeRepository, nodes ports.KnowledgeNodeRepository, newID func() string) *KnowledgeEdgeService {
	return &KnowledgeEdgeService{edges: edges, nodes: nodes, newID: newID}
}

// CreateKnowledgeEdgeInput is a new edge; Level is nil for applies.
type CreateKnowledgeEdgeInput struct {
	FromID string
	ToID   string
	Type   domain.KnowledgeEdgeType
	Level  *domain.MasteryLevel
}

// Create links two nodes. An edge of the same type between the same nodes,
// or a requires edge that would close a cycle, is a domain.ErrConflict.
func (s *KnowledgeEdgeService) Create(ctx context.Context, caller domain.User, input CreateKnowledgeEdgeInput) (domain.KnowledgeEdge, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.KnowledgeEdge{}, domain.ErrForbidden
	}
	found, err := s.nodes.GetByIDs(ctx, []string{input.FromID, input.ToID})
	if err != nil {
		return domain.KnowledgeEdge{}, err
	}
	from, ok := found[input.FromID]
	if !ok {
		return domain.KnowledgeEdge{}, domain.NewValidationError("from_id", "does not reference an existing knowledge node")
	}
	to, ok := found[input.ToID]
	if !ok {
		return domain.KnowledgeEdge{}, domain.NewValidationError("to_id", "does not reference an existing knowledge node")
	}
	edge, err := domain.NewKnowledgeEdge(s.newID(), from, to, input.Type, input.Level)
	if err != nil {
		return domain.KnowledgeEdge{}, err
	}
	if edge.Type == domain.KnowledgeEdgeTypeRequires {
		// The new edge closes a cycle exactly when to already reaches from.
		cycle, err := s.edges.RequiresPathExists(ctx, edge.ToID, edge.FromID)
		if err != nil {
			return domain.KnowledgeEdge{}, err
		}
		if cycle {
			return domain.KnowledgeEdge{}, fmt.Errorf("%w: this requires edge would close a cycle of requires edges", domain.ErrConflict)
		}
	}
	if err := s.edges.Create(ctx, edge); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return domain.KnowledgeEdge{}, fmt.Errorf("%w: an edge of this type already links these nodes in this direction", domain.ErrConflict)
		}
		return domain.KnowledgeEdge{}, err
	}
	return edge, nil
}

// List returns the edges matching filter.
func (s *KnowledgeEdgeService) List(ctx context.Context, filter ports.KnowledgeEdgeFilter) ([]domain.KnowledgeEdge, error) {
	return s.edges.List(ctx, filter)
}

// UpdateLevel changes the level a requires edge asks for.
func (s *KnowledgeEdgeService) UpdateLevel(ctx context.Context, caller domain.User, id string, level domain.MasteryLevel) (domain.KnowledgeEdge, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.KnowledgeEdge{}, domain.ErrForbidden
	}
	edge, err := s.edges.GetByID(ctx, id)
	if err != nil {
		return domain.KnowledgeEdge{}, err
	}
	edge, err = edge.WithLevel(level)
	if err != nil {
		return domain.KnowledgeEdge{}, err
	}
	if err := s.edges.UpdateLevel(ctx, edge); err != nil {
		return domain.KnowledgeEdge{}, err
	}
	return edge, nil
}

// Delete removes an edge.
func (s *KnowledgeEdgeService) Delete(ctx context.Context, caller domain.User, id string) error {
	if caller.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	return s.edges.Delete(ctx, id)
}
