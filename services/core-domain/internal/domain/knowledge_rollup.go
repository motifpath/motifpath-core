package domain

import (
	"slices"
	"time"
)

// knowledgeLevels lists the levels from lowest to highest.
var knowledgeLevels = []KnowledgeLevel{
	KnowledgeLevelNew, KnowledgeLevelLearning, KnowledgeLevelAccurate,
	KnowledgeLevelFluent, KnowledgeLevelRetained,
}

// Rank orders levels from new (0) to retained (4); an unknown level ranks
// below new.
func (l KnowledgeLevel) Rank() int {
	return slices.Index(knowledgeLevels, l)
}

// nodeLevelShare is the share of a node's items, in percent, that must
// reach a level for the node to be at it.
const nodeLevelShare = 80

// coveredLevel is the level at which an item counts as met in a node's
// coverage: the lowest level a requirement can ask for.
const coveredLevel = KnowledgeLevelAccurate

// ClassifiedItem is a practice item and the knowledge nodes it is
// classified under directly.
type ClassifiedItem struct {
	ItemKey string
	NodeIDs []string
}

// NodeRollup is what a student's item states say about one knowledge node
// for one instrument.
type NodeRollup struct {
	// Total counts the items in the node's subtree for the instrument, and
	// Covered those of them at accurate or above. A wide node, one with
	// children, is shown by this coverage and its children's levels rather
	// than by a level of its own.
	Total   int
	Covered int
	// Level is the highest level at least 80% of the items reach, unseen
	// items counting as new; nil when the node has nothing to practise.
	Level *KnowledgeLevel
	// Fading reports whether a practised item's review is due. A state
	// folded under older rules is only waiting to be rebuilt, which is not
	// a review the student owes.
	Fading bool
}

// RollUpNode rolls the states of a node's items, keyed by item key, up
// into the node's standing at now. An item with no state was never
// practised.
func RollUpNode(itemKeys []string, states map[string]PracticeItemState, now time.Time) NodeRollup {
	rollup := NodeRollup{Total: len(itemKeys)}
	if rollup.Total == 0 {
		return rollup
	}
	reached := make([]int, len(knowledgeLevels))
	for _, key := range itemKeys {
		level := KnowledgeLevelNew
		if state, ok := states[key]; ok && state.Counted > 0 {
			level = state.ShownLevel(now)
			rollup.Fading = rollup.Fading || state.ReviewDue(now)
		}
		for rank := 0; rank <= level.Rank(); rank++ {
			reached[rank]++
		}
		if level.Rank() >= coveredLevel.Rank() {
			rollup.Covered++
		}
	}
	for rank := len(knowledgeLevels) - 1; rank >= 0; rank-- {
		if reached[rank]*100 >= nodeLevelShare*rollup.Total {
			rollup.Level = &knowledgeLevels[rank]
			break
		}
	}
	return rollup
}

// SubtreeItemKeys maps each of nodes to the keys of the items classified
// under it or any node below it, each once, in items' order. An item under
// a node not in nodes is skipped.
func SubtreeItemKeys(nodes []KnowledgeNode, items []ClassifiedItem) map[string][]string {
	parents := make(map[string]*string, len(nodes))
	subtrees := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		parents[n.ID] = n.ParentID
		subtrees[n.ID] = nil
	}
	for _, item := range items {
		added := map[string]bool{}
		for _, nodeID := range item.NodeIDs {
			for id := &nodeID; id != nil && !added[*id]; {
				parent, known := parents[*id]
				if !known {
					break
				}
				added[*id] = true
				subtrees[*id] = append(subtrees[*id], item.ItemKey)
				id = parent
			}
		}
	}
	return subtrees
}

// NodeStanding is where a student stands on one knowledge node for one
// instrument.
type NodeStanding struct {
	NodeRollup
	Readiness     Readiness
	RequiresDepth int
}

// Readiness counts a node's requirements for an instrument and how many of
// them the student meets.
type Readiness struct {
	Met   int
	Total int
}

// Complete reports whether every requirement is met; a node with none is
// complete.
func (r Readiness) Complete() bool { return r.Met == r.Total }

// NodeReadiness counts the requires edges from nodeID whose two ends are
// for instrumentID, and those whose target's level, in rollups, reaches
// the edge's level. A target with nothing to practise has no level, so a
// requirement on it is never met.
func NodeReadiness(nodeID string, edges []KnowledgeEdge, nodes map[string]KnowledgeNode, instrumentID string, rollups map[string]NodeRollup) Readiness {
	var r Readiness
	for _, e := range requiresFrom(nodeID, edges, nodes, instrumentID) {
		r.Total++
		target := rollups[e.ToID].Level
		if target != nil && e.Level != nil && target.Rank() >= KnowledgeLevel(*e.Level).Rank() {
			r.Met++
		}
	}
	return r
}

// RequiresDepth is the length of the longest chain of requirements below
// nodeID for instrumentID, counting only requires edges whose two ends are
// for the instrument; 0 for a node that requires nothing. Requires edges
// never form a cycle.
func RequiresDepth(nodeID string, edges []KnowledgeEdge, nodes map[string]KnowledgeNode, instrumentID string) int {
	depths := map[string]int{}
	var depth func(id string) int
	depth = func(id string) int {
		if d, ok := depths[id]; ok {
			return d
		}
		d := 0
		for _, e := range requiresFrom(id, edges, nodes, instrumentID) {
			d = max(d, 1+depth(e.ToID))
		}
		depths[id] = d
		return d
	}
	return depth(nodeID)
}

// requiresFrom lists the requires edges from nodeID whose two ends are
// known nodes for instrumentID.
func requiresFrom(nodeID string, edges []KnowledgeEdge, nodes map[string]KnowledgeNode, instrumentID string) []KnowledgeEdge {
	var found []KnowledgeEdge
	for _, e := range edges {
		if e.Type != KnowledgeEdgeTypeRequires || e.FromID != nodeID {
			continue
		}
		from, okFrom := nodes[e.FromID]
		to, okTo := nodes[e.ToID]
		if okFrom && okTo && from.For(instrumentID) && to.For(instrumentID) {
			found = append(found, e)
		}
	}
	return found
}
