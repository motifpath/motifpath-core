package domain_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

var rollupNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

// itemKeys returns n item keys with prefix.
func itemKeys(prefix string, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("%s:%d", prefix, i)
	}
	return keys
}

// heldAt returns a practised item's state at level, reviewed long ago and
// due again in a week, so its shown level is its earned level.
func heldAt(key string, level domain.KnowledgeLevel) domain.PracticeItemState {
	due := rollupNow.AddDate(0, 0, 7)
	last := rollupNow.AddDate(0, 0, -1)
	return domain.PracticeItemState{
		ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: level,
		Counted: 5, Box: 3, DueAt: &due, LastAt: &last,
	}
}

// statesAt puts each of keys at level into states.
func statesAt(states map[string]domain.PracticeItemState, keys []string, level domain.KnowledgeLevel) {
	for _, k := range keys {
		states[k] = heldAt(k, level)
	}
}

func levelPtr(l domain.KnowledgeLevel) *domain.KnowledgeLevel { return &l }

func TestRollUpNode(t *testing.T) {
	t.Run("a node is at the highest level that 80% of its items reach", func(t *testing.T) {
		keys := itemKeys("cell", 26)
		states := map[string]domain.PracticeItemState{}
		statesAt(states, keys[:21], domain.KnowledgeLevelFluent)
		statesAt(states, keys[21:], domain.KnowledgeLevelAccurate)

		got := domain.RollUpNode(keys, states, rollupNow)

		assert.Equal(t, levelPtr(domain.KnowledgeLevelFluent), got.Level)
	})

	t.Run("a node falls to the next level when fewer than 80% reach the higher one", func(t *testing.T) {
		keys := itemKeys("cell", 26)
		states := map[string]domain.PracticeItemState{}
		statesAt(states, keys[:20], domain.KnowledgeLevelFluent)
		statesAt(states, keys[20:], domain.KnowledgeLevelAccurate)

		got := domain.RollUpNode(keys, states, rollupNow)

		assert.Equal(t, levelPtr(domain.KnowledgeLevelAccurate), got.Level)
	})

	t.Run("unseen items count as new", func(t *testing.T) {
		keys := itemKeys("cell", 26)
		states := map[string]domain.PracticeItemState{}
		statesAt(states, keys[:10], domain.KnowledgeLevelFluent)

		got := domain.RollUpNode(keys, states, rollupNow)

		assert.Equal(t, levelPtr(domain.KnowledgeLevelNew), got.Level)
	})

	t.Run("an item's shown level counts, one step down once its review is long overdue", func(t *testing.T) {
		keys := itemKeys("exercise", 5)
		states := map[string]domain.PracticeItemState{}
		statesAt(states, keys, domain.KnowledgeLevelFluent)
		for _, k := range keys {
			s := states[k]
			overdue := rollupNow.AddDate(0, 0, -30)
			s.DueAt = &overdue
			states[k] = s
		}

		got := domain.RollUpNode(keys, states, rollupNow)

		assert.Equal(t, levelPtr(domain.KnowledgeLevelAccurate), got.Level)
	})

	t.Run("a node with nothing to practise has no level", func(t *testing.T) {
		got := domain.RollUpNode(nil, map[string]domain.PracticeItemState{}, rollupNow)

		assert.Nil(t, got.Level)
		assert.Zero(t, got.Total)
	})

	t.Run("coverage counts the items at accurate or above", func(t *testing.T) {
		keys := itemKeys("cell", 72)
		states := map[string]domain.PracticeItemState{}
		statesAt(states, keys[:20], domain.KnowledgeLevelAccurate)
		statesAt(states, keys[20:26], domain.KnowledgeLevelRetained)
		statesAt(states, keys[26:30], domain.KnowledgeLevelLearning)

		got := domain.RollUpNode(keys, states, rollupNow)

		assert.Equal(t, 26, got.Covered)
		assert.Equal(t, 72, got.Total)
	})

	t.Run("a node is fading when a practised item's review is due", func(t *testing.T) {
		keys := itemKeys("exercise", 3)
		states := map[string]domain.PracticeItemState{}
		statesAt(states, keys, domain.KnowledgeLevelAccurate)
		s := states[keys[1]]
		due := rollupNow.Add(-time.Hour)
		s.DueAt = &due
		states[keys[1]] = s

		got := domain.RollUpNode(keys, states, rollupNow)

		assert.True(t, got.Fading)
	})

	t.Run("a state folded under older mastery rules is not fading while its review isn't due", func(t *testing.T) {
		keys := itemKeys("exercise", 1)
		states := map[string]domain.PracticeItemState{}
		statesAt(states, keys, domain.KnowledgeLevelAccurate)
		s := states[keys[0]]
		s.RulesVersion = domain.PracticeRulesVersion - 1
		states[keys[0]] = s

		got := domain.RollUpNode(keys, states, rollupNow)

		assert.False(t, got.Fading)
	})

	t.Run("a node is not fading while no review is due, nor for unseen items", func(t *testing.T) {
		keys := itemKeys("exercise", 3)
		states := map[string]domain.PracticeItemState{}
		statesAt(states, keys[:2], domain.KnowledgeLevelAccurate)

		got := domain.RollUpNode(keys, states, rollupNow)

		assert.False(t, got.Fading)
	})
}

func TestSubtreeItemKeys(t *testing.T) {
	root := domain.KnowledgeNode{ID: "fretboard"}
	low := domain.KnowledgeNode{ID: "low", ParentID: &root.ID}
	high := domain.KnowledgeNode{ID: "high", ParentID: &root.ID}
	other := domain.KnowledgeNode{ID: "reading"}
	nodes := []domain.KnowledgeNode{root, low, high, other}

	t.Run("a node holds the items classified under it or any node below it", func(t *testing.T) {
		items := []domain.ClassifiedItem{
			{ItemKey: "a", NodeIDs: []string{"low"}},
			{ItemKey: "b", NodeIDs: []string{"high"}},
			{ItemKey: "c", NodeIDs: []string{"fretboard"}},
		}

		got := domain.SubtreeItemKeys(nodes, items)

		assert.Equal(t, []string{"a", "b", "c"}, got["fretboard"])
		assert.Equal(t, []string{"a"}, got["low"])
		assert.Equal(t, []string{"b"}, got["high"])
		assert.Empty(t, got["reading"])
	})

	t.Run("an item classified under two nodes of one subtree counts once", func(t *testing.T) {
		items := []domain.ClassifiedItem{{ItemKey: "a", NodeIDs: []string{"low", "high", "fretboard"}}}

		got := domain.SubtreeItemKeys(nodes, items)

		assert.Equal(t, []string{"a"}, got["fretboard"])
	})

	t.Run("an item under an unknown node is skipped", func(t *testing.T) {
		items := []domain.ClassifiedItem{{ItemKey: "a", NodeIDs: []string{"gone"}}}

		got := domain.SubtreeItemKeys(nodes, items)

		assert.NotContains(t, got, "gone")
		assert.Empty(t, got["fretboard"])
	})
}

func requires(from, to string, level domain.MasteryLevel) domain.KnowledgeEdge {
	return domain.KnowledgeEdge{ID: from + "->" + to, FromID: from, ToID: to, Type: domain.KnowledgeEdgeTypeRequires, Level: &level}
}

func nodesByID(nodes ...domain.KnowledgeNode) map[string]domain.KnowledgeNode {
	byID := map[string]domain.KnowledgeNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	return byID
}

func TestNodeReadiness(t *testing.T) {
	all := domain.KnowledgeNode{ID: "all-strings"}
	low := domain.KnowledgeNode{ID: "low-strings"}
	reading := domain.KnowledgeNode{ID: "reading"}
	bassOnly := domain.KnowledgeNode{ID: "slap", InstrumentIDs: []string{"bass"}}
	nodes := nodesByID(all, low, reading, bassOnly)

	t.Run("a requirement is met once its target reaches the required level", func(t *testing.T) {
		edges := []domain.KnowledgeEdge{requires("all-strings", "low-strings", domain.MasteryLevelAccurate)}
		rollups := map[string]domain.NodeRollup{"low-strings": {Total: 26, Level: levelPtr(domain.KnowledgeLevelFluent)}}

		got := domain.NodeReadiness("all-strings", edges, nodes, "guitar", rollups)

		assert.Equal(t, domain.Readiness{Met: 1, Total: 1}, got)
	})

	t.Run("a requirement is unmet below the required level", func(t *testing.T) {
		edges := []domain.KnowledgeEdge{requires("all-strings", "low-strings", domain.MasteryLevelFluent)}
		rollups := map[string]domain.NodeRollup{"low-strings": {Total: 26, Level: levelPtr(domain.KnowledgeLevelAccurate)}}

		got := domain.NodeReadiness("all-strings", edges, nodes, "guitar", rollups)

		assert.Equal(t, domain.Readiness{Met: 0, Total: 1}, got)
	})

	t.Run("a requirement on a node with nothing to practise is never met", func(t *testing.T) {
		edges := []domain.KnowledgeEdge{requires("all-strings", "reading", domain.MasteryLevelAccurate)}
		rollups := map[string]domain.NodeRollup{"reading": {}}

		got := domain.NodeReadiness("all-strings", edges, nodes, "guitar", rollups)

		assert.Equal(t, domain.Readiness{Met: 0, Total: 1}, got)
	})

	t.Run("only requires edges from the node whose two ends are for the instrument count", func(t *testing.T) {
		applies := domain.KnowledgeEdge{FromID: "all-strings", ToID: "reading", Type: domain.KnowledgeEdgeTypeApplies}
		edges := []domain.KnowledgeEdge{
			requires("all-strings", "low-strings", domain.MasteryLevelAccurate),
			requires("all-strings", "slap", domain.MasteryLevelAccurate),
			requires("reading", "low-strings", domain.MasteryLevelAccurate),
			applies,
		}
		rollups := map[string]domain.NodeRollup{"low-strings": {Total: 1, Level: levelPtr(domain.KnowledgeLevelAccurate)}}

		assert.Equal(t, domain.Readiness{Met: 1, Total: 1}, domain.NodeReadiness("all-strings", edges, nodes, "guitar", rollups))
		assert.Equal(t, domain.Readiness{Met: 1, Total: 2}, domain.NodeReadiness("all-strings", edges, nodes, "bass", rollups))
	})

	t.Run("a node with no requirements is ready with 0 of 0", func(t *testing.T) {
		got := domain.NodeReadiness("low-strings", nil, nodes, "guitar", nil)

		assert.Equal(t, domain.Readiness{}, got)
		assert.True(t, got.Complete())
	})
}

func TestRequiresDepth(t *testing.T) {
	a := domain.KnowledgeNode{ID: "a"}
	b := domain.KnowledgeNode{ID: "b"}
	c := domain.KnowledgeNode{ID: "c"}
	d := domain.KnowledgeNode{ID: "d"}
	bassOnly := domain.KnowledgeNode{ID: "e", InstrumentIDs: []string{"bass"}}
	nodes := nodesByID(a, b, c, d, bassOnly)

	t.Run("a node with no requirements has depth 0", func(t *testing.T) {
		assert.Equal(t, 0, domain.RequiresDepth("a", nil, nodes, "guitar"))
	})

	t.Run("depth is the longest chain of requirements below the node", func(t *testing.T) {
		edges := []domain.KnowledgeEdge{
			requires("a", "b", domain.MasteryLevelAccurate),
			requires("b", "c", domain.MasteryLevelAccurate),
			requires("a", "d", domain.MasteryLevelAccurate),
		}

		assert.Equal(t, 2, domain.RequiresDepth("a", edges, nodes, "guitar"))
		assert.Equal(t, 1, domain.RequiresDepth("b", edges, nodes, "guitar"))
	})

	t.Run("only edges whose two ends are for the instrument count", func(t *testing.T) {
		edges := []domain.KnowledgeEdge{
			requires("a", "e", domain.MasteryLevelAccurate),
			requires("e", "b", domain.MasteryLevelAccurate),
		}

		assert.Equal(t, 0, domain.RequiresDepth("a", edges, nodes, "guitar"))
		assert.Equal(t, 2, domain.RequiresDepth("a", edges, nodes, "bass"))
	})
}

func TestKnowledgeNodeFor(t *testing.T) {
	assert.True(t, domain.KnowledgeNode{}.For("guitar"))
	assert.True(t, domain.KnowledgeNode{InstrumentIDs: []string{"bass", "guitar"}}.For("guitar"))
	assert.False(t, domain.KnowledgeNode{InstrumentIDs: []string{"bass"}}.For("guitar"))
}

func TestExerciseItemKey(t *testing.T) {
	assert.Equal(t, "exercise:e-1", domain.ExerciseItemKey("e-1"))
}
