package application

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// KnowledgeRollupService rolls a student's practice item states up the
// knowledge map, one instrument at a time. Levels are derived on every
// read, never stored.
type KnowledgeRollupService struct {
	nodes  ports.KnowledgeNodeRepository
	edges  ports.KnowledgeEdgeRepository
	items  ports.NodeItemSource
	states ports.PracticeItemStateReader
	now    func() time.Time
}

func NewKnowledgeRollupService(
	nodes ports.KnowledgeNodeRepository,
	edges ports.KnowledgeEdgeRepository,
	items ports.NodeItemSource,
	states ports.PracticeItemStateReader,
	now func() time.Time,
) *KnowledgeRollupService {
	return &KnowledgeRollupService{nodes: nodes, edges: edges, items: items, states: states, now: now}
}

// KnowledgeMap is where a student stands on the knowledge map for one
// instrument.
type KnowledgeMap struct {
	// Nodes lists the nodes for the instrument in catalog order.
	Nodes []domain.KnowledgeNode
	// Standings holds each of Nodes' standing, keyed by node id.
	Standings map[string]domain.NodeStanding
	// Subtrees maps each of Nodes to the keys of the items that suit the
	// instrument in its subtree.
	Subtrees map[string][]string
	// States holds the student's states on those items, keyed by item key;
	// an item never practised has none.
	States map[string]domain.PracticeItemState
}

// Standings returns the student's standing on every knowledge node for
// instrumentID, keyed by node id: its level over the items in its subtree
// that suit the instrument, its coverage, whether it is fading, its
// readiness and its requires depth.
func (s *KnowledgeRollupService) Standings(ctx context.Context, studentID, instrumentID string) (map[string]domain.NodeStanding, error) {
	m, err := s.Map(ctx, studentID, instrumentID)
	if err != nil {
		return nil, err
	}
	return m.Standings, nil
}

// Map returns the student's standings on the knowledge map for
// instrumentID, with the items and states they were rolled up from.
func (s *KnowledgeRollupService) Map(ctx context.Context, studentID, instrumentID string) (KnowledgeMap, error) {
	nodes, err := s.nodes.List(ctx, ports.KnowledgeNodeFilter{})
	if err != nil {
		return KnowledgeMap{}, err
	}
	requires := domain.KnowledgeEdgeTypeRequires
	edges, err := s.edges.List(ctx, ports.KnowledgeEdgeFilter{Type: &requires})
	if err != nil {
		return KnowledgeMap{}, err
	}
	items, err := s.items.ClassifiedItems(ctx, instrumentID)
	if err != nil {
		return KnowledgeMap{}, err
	}
	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.ItemKey
	}
	states := map[string]domain.PracticeItemState{}
	if len(keys) > 0 {
		if states, err = s.states.GetStates(ctx, studentID, keys); err != nil {
			return KnowledgeMap{}, err
		}
	}

	now := s.now()
	subtrees := domain.SubtreeItemKeys(nodes, items)
	byID := make(map[string]domain.KnowledgeNode, len(nodes))
	rollups := make(map[string]domain.NodeRollup, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
		rollups[n.ID] = domain.RollUpNode(subtrees[n.ID], states, now)
	}
	m := KnowledgeMap{Standings: map[string]domain.NodeStanding{}, Subtrees: map[string][]string{}, States: states}
	for _, n := range nodes {
		if !n.For(instrumentID) {
			continue
		}
		m.Nodes = append(m.Nodes, n)
		m.Subtrees[n.ID] = subtrees[n.ID]
		m.Standings[n.ID] = domain.NodeStanding{
			NodeRollup:    rollups[n.ID],
			Readiness:     domain.NodeReadiness(n.ID, edges, byID, instrumentID, rollups),
			RequiresDepth: domain.RequiresDepth(n.ID, edges, byID, instrumentID),
		}
	}
	return m, nil
}
