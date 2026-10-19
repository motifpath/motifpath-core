package domain

import (
	"cmp"
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

// RankStretchNodes lists the ids of nodes a student may stretch to, in the
// order to start them: those with something to practise and every
// requirement met, keyed in standings (the nodes for the instrument), and
// connected to what the student is learning (see ConnectedNodes). The
// nodes in pathNodeIDs, the student's own path, come first; then nodes that
// build on something the student meets, having requirements; then the
// shallowest requires depth; then nodes' own order.
func RankStretchNodes(nodes []KnowledgeNode, standings map[string]NodeStanding, pathNodeIDs []string, applies []KnowledgeEdge) []string {
	connected := ConnectedNodes(nodes, standings, pathNodeIDs, applies)
	var ready []KnowledgeNode
	for _, n := range nodes {
		if s, ok := standings[n.ID]; ok && s.Level != nil && s.Readiness.Complete() && connected[n.ID] {
			ready = append(ready, n)
		}
	}
	slices.SortStableFunc(ready, func(a, b KnowledgeNode) int {
		return cmp.Or(
			cmp.Compare(offPath(a.ID, pathNodeIDs), offPath(b.ID, pathNodeIDs)),
			cmp.Compare(buildsOnRank(standings[a.ID]), buildsOnRank(standings[b.ID])),
			cmp.Compare(standings[a.ID].RequiresDepth, standings[b.ID].RequiresDepth),
		)
	})
	ids := make([]string, len(ready))
	for i, n := range ready {
		ids[i] = n.ID
	}
	return ids
}

// ConnectedNodes reports which of nodes connect to what the student is
// learning: the nodes in pathNodeIDs; their parents and children; nodes
// linked to them by an applies edge, either way; and nodes that build on
// something the student meets, having every requirement in standings met.
func ConnectedNodes(nodes []KnowledgeNode, standings map[string]NodeStanding, pathNodeIDs []string, applies []KnowledgeEdge) map[string]bool {
	connected := map[string]bool{}
	for _, id := range pathNodeIDs {
		connected[id] = true
	}
	for _, n := range nodes {
		if s, ok := standings[n.ID]; ok && s.Readiness.Total > 0 && s.Readiness.Complete() {
			connected[n.ID] = true
		}
	}
	for _, link := range pathLinks(nodes, applies) {
		if slices.Contains(pathNodeIDs, link[0]) {
			connected[link[1]] = true
		}
		if slices.Contains(pathNodeIDs, link[1]) {
			connected[link[0]] = true
		}
	}
	return connected
}

// pathLinks lists the pairs of node ids that connect each other to what a
// student is learning: each node and its parent, and the two ends of each
// applies edge.
func pathLinks(nodes []KnowledgeNode, applies []KnowledgeEdge) [][2]string {
	var links [][2]string
	for _, n := range nodes {
		if n.ParentID != nil {
			links = append(links, [2]string{n.ID, *n.ParentID})
		}
	}
	for _, e := range applies {
		if e.Type == KnowledgeEdgeTypeApplies {
			links = append(links, [2]string{e.FromID, e.ToID})
		}
	}
	return links
}

// KnowledgeView is where a student stands on the knowledge map for one
// instrument.
type KnowledgeView struct {
	// Nodes lists the nodes for the instrument in catalog order.
	Nodes []KnowledgeNode
	// Standings holds each of Nodes' standing, keyed by node id.
	Standings map[string]NodeStanding
	// Subtrees maps each of Nodes to the keys of the items that suit the
	// instrument in its subtree.
	Subtrees map[string][]string
	// States holds the student's states on those items, keyed by item key;
	// an item never practised has none.
	States map[string]PracticeItemState
	// Applies lists every applies edge, which connects nodes without
	// requiring anything.
	Applies []KnowledgeEdge
	// Items lists the items that suit the instrument, each with the nodes
	// it is classified under directly.
	Items []ClassifiedItem
}

// Practised reports whether the student has a counted answer on an item in
// nodeID's subtree.
func (v KnowledgeView) Practised(nodeID string) bool {
	for _, key := range v.Subtrees[nodeID] {
		if state, ok := v.States[key]; ok && state.Counted > 0 {
			return true
		}
	}
	return false
}

// Wide reports whether nodeID has a child among the view's nodes with
// something to practise: such a node is shown through its coverage and its
// children, never by a level of its own.
func (v KnowledgeView) Wide(nodeID string) bool {
	for _, n := range v.Nodes {
		if n.ParentID != nil && *n.ParentID == nodeID && v.Standings[n.ID].Total > 0 {
			return true
		}
	}
	return false
}
