package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestPracticeItemStateWeak(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	inDays := func(days int) *time.Time {
		at := now.AddDate(0, 0, days)
		return &at
	}

	for _, tc := range []struct {
		name  string
		state domain.PracticeItemState
		want  bool
	}{
		{
			name:  "practised, not due and below fluent is weak",
			state: domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: inDays(2)},
			want:  true,
		},
		{
			name:  "learning and not due is weak",
			state: domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelLearning, Counted: 1, Box: 1, DueAt: inDays(1)},
			want:  true,
		},
		{
			name:  "fluent and not due is never weak",
			state: domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: inDays(5)},
		},
		{
			name:  "retained and not due is never weak",
			state: domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelRetained, Counted: 9, Box: 6, DueAt: inDays(20)},
		},
		{
			name:  "a due item is due, not weak",
			state: domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: inDays(0)},
		},
		{
			name:  "a state folded under older rules is due, not weak",
			state: domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion - 1, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: inDays(2)},
		},
		{
			name:  "an item never counted is new, not weak",
			state: domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelNew},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.state.Weak(now))
		})
	}
}

func TestExerciseSeconds(t *testing.T) {
	estimate := 45

	assert.Equal(t, 45, domain.ExerciseSeconds(domain.Exercise{EstimatedDurationSeconds: &estimate}), "an authored estimate is used as it is")
	assert.Equal(t, 30, domain.ExerciseSeconds(domain.Exercise{}), "an exercise with no estimate takes 30 seconds")
}

func TestRankStretchNodes(t *testing.T) {
	level := domain.KnowledgeLevelNew
	ready := func(met, total, depth int) domain.NodeStanding {
		return domain.NodeStanding{NodeRollup: domain.NodeRollup{Total: 4, Level: &level}, Readiness: domain.Readiness{Met: met, Total: total}, RequiresDepth: depth}
	}
	nodes := []domain.KnowledgeNode{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}, {ID: "f"}, {ID: "g"}}
	standings := map[string]domain.NodeStanding{
		"a": ready(0, 0, 0),                            // requires nothing
		"b": ready(1, 1, 2),                            // builds on, deeper
		"c": ready(1, 1, 1),                            // builds on, shallower
		"d": ready(0, 1, 1),                            // a requirement unmet
		"e": {NodeRollup: domain.NodeRollup{Total: 0}}, // nothing to practise
		"f": ready(2, 2, 1),                            // builds on, as shallow as c, later in catalog order
		// g is not for the instrument: no standing.
	}

	assert.Equal(t, []string{"c", "f", "b"}, domain.RankStretchNodes(nodes, standings, nil, nil),
		"a requires nothing and has no link to the path, so it is not connected to what the student is learning")
	assert.Equal(t, []string{"b", "a", "c", "f"}, domain.RankStretchNodes(nodes, standings, []string{"a", "b", "d"}, nil),
		"the student's path skills come first, ranked by the same rules among themselves; d stays unready")
}

func TestRankStretchNodesReachesPathNeighbours(t *testing.T) {
	level := domain.KnowledgeLevelNew
	fromScratch := domain.NodeStanding{NodeRollup: domain.NodeRollup{Total: 4, Level: &level}}
	path, parent := "path", "parent"
	nodes := []domain.KnowledgeNode{
		{ID: "parent"},
		{ID: "path", ParentID: &parent},
		{ID: "child", ParentID: &path},
		{ID: "applied"},
		{ID: "applier"},
		{ID: "sibling", ParentID: &parent},
		{ID: "unlinked"},
	}
	standings := map[string]domain.NodeStanding{}
	for _, n := range nodes {
		standings[n.ID] = fromScratch
	}
	applies := []domain.KnowledgeEdge{
		{Type: domain.KnowledgeEdgeTypeApplies, FromID: "path", ToID: "applied"},
		{Type: domain.KnowledgeEdgeTypeApplies, FromID: "applier", ToID: "path"},
		{Type: domain.KnowledgeEdgeTypeApplies, FromID: "unlinked", ToID: "sibling"},
	}

	assert.Equal(t, []string{"path", "parent", "child", "applied", "applier"}, domain.RankStretchNodes(nodes, standings, []string{"path"}, applies),
		"a path skill's parent, children and applies neighbours either way are connected; a sibling and a node linked only to it are not")
}
