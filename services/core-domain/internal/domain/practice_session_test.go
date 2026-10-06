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

func TestTapCheckDue(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	daysAgo := func(days int) *time.Time {
		at := now.AddDate(0, 0, -days)
		return &at
	}
	cell := domain.PracticeSessionItem{Kind: domain.PracticeItemKindFretboardCell}
	exercise := domain.PracticeSessionItem{Kind: domain.PracticeItemKindExercise}
	playAlong := domain.PracticeSessionItem{Kind: domain.PracticeItemKindPlayAlong}

	for _, tc := range []struct {
		name     string
		items    []domain.PracticeSessionItem
		lastDone *time.Time
		want     bool
	}{
		{"a plan with a cell and no tap check ever", []domain.PracticeSessionItem{exercise, cell}, nil, true},
		{"a tap check 12 days ago still stands", []domain.PracticeSessionItem{cell}, daysAgo(12), false},
		{"a tap check exactly 30 days ago still stands", []domain.PracticeSessionItem{cell}, daysAgo(30), false},
		{"a tap check 31 days ago is asked for again", []domain.PracticeSessionItem{cell}, daysAgo(31), true},
		{"a plan without a cell never asks", []domain.PracticeSessionItem{exercise, playAlong}, nil, false},
		{"an empty plan never asks", nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.TapCheckDue(tc.items, tc.lastDone, now))
		})
	}
}

func TestPracticeSessionItemDrillTemplateKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		item domain.PracticeSessionItem
		want string
	}{
		{"a fretboard cell is timed by the way it is asked",
			domain.PracticeSessionItem{Kind: domain.PracticeItemKindFretboardCell, FretboardCell: &domain.PlannedFretboardCell{Drill: domain.FretboardDrillFindTheNote}},
			"fretboard_cell:find_the_note"},
		{"an exercise is timed by its type",
			domain.PracticeSessionItem{Kind: domain.PracticeItemKindExercise, Exercise: &domain.Exercise{ExerciseType: domain.ExerciseTypeTextResponse}},
			"exercise:text_response"},
		{"a play-along is rated, not timed",
			domain.PracticeSessionItem{Kind: domain.PracticeItemKindPlayAlong, PlayAlong: &domain.PlannedPlayAlong{}},
			""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.item.DrillTemplateKey())
		})
	}
}

func TestFeltQuestions(t *testing.T) {
	cell := func(drill domain.FretboardDrill) domain.PracticeSessionItem {
		return domain.PracticeSessionItem{Kind: domain.PracticeItemKindFretboardCell, FretboardCell: &domain.PlannedFretboardCell{Drill: drill}}
	}
	exercise := func(exerciseType domain.ExerciseType) domain.PracticeSessionItem {
		return domain.PracticeSessionItem{Kind: domain.PracticeItemKindExercise, Exercise: &domain.Exercise{ExerciseType: exerciseType}}
	}
	playAlong := domain.PracticeSessionItem{Kind: domain.PracticeItemKindPlayAlong, PlayAlong: &domain.PlannedPlayAlong{}}
	name, find := cell(domain.FretboardDrillNameTheNote), cell(domain.FretboardDrillFindTheNote)
	text := exercise(domain.ExerciseTypeTextResponse)

	for _, tc := range []struct {
		name      string
		items     []domain.PracticeSessionItem
		feltRated map[string]int
		want      []string
	}{
		{"the two templates with the fewest felt-rated sessions, fewest first",
			[]domain.PracticeSessionItem{name, find, text, name},
			map[string]int{"fretboard_cell:name_the_note": 40, "fretboard_cell:find_the_note": 12, "exercise:text_response": 3},
			[]string{"exercise:text_response", "fretboard_cell:find_the_note"}},
		{"a template never felt-rated has none",
			[]domain.PracticeSessionItem{name, text},
			map[string]int{"fretboard_cell:name_the_note": 1},
			[]string{"exercise:text_response", "fretboard_cell:name_the_note"}},
		{"a tie keeps the plan's order",
			[]domain.PracticeSessionItem{text, find, name},
			map[string]int{},
			[]string{"exercise:text_response", "fretboard_cell:find_the_note"}},
		{"a single timed template is the only question",
			[]domain.PracticeSessionItem{playAlong, find, find},
			nil,
			[]string{"fretboard_cell:find_the_note"}},
		{"play-alongs are never asked about",
			[]domain.PracticeSessionItem{playAlong},
			nil,
			[]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.FeltQuestions(tc.items, tc.feltRated))
		})
	}
}

func TestPlanDrillTemplates(t *testing.T) {
	items := []domain.PracticeSessionItem{
		{Kind: domain.PracticeItemKindPlayAlong, PlayAlong: &domain.PlannedPlayAlong{}},
		{Kind: domain.PracticeItemKindExercise, Exercise: &domain.Exercise{ExerciseType: domain.ExerciseTypeImageChoice}},
		{Kind: domain.PracticeItemKindFretboardCell, FretboardCell: &domain.PlannedFretboardCell{Drill: domain.FretboardDrillNameTheNote}},
		{Kind: domain.PracticeItemKindExercise, Exercise: &domain.Exercise{ExerciseType: domain.ExerciseTypeImageChoice}},
	}

	assert.Equal(t, []string{"exercise:image_choice", "fretboard_cell:name_the_note"}, domain.PlanDrillTemplates(items))
}

func TestPlayAlongStartTempo(t *testing.T) {
	bpm := func(v int) *int { return &v }

	for _, tc := range []struct {
		name      string
		target    int
		bestClean *int
		want      int
	}{
		{name: "starts at the best clean tempo", target: 120, bestClean: bpm(90), want: 90},
		{name: "starts at 60% of the target, rounded down to 5 BPM, with no clean take yet", target: 120, bestClean: nil, want: 70},
		{name: "keeps a best clean tempo past the target", target: 120, bestClean: bpm(150), want: 150},
		{name: "never starts below the slowest playable tempo", target: 30, bestClean: nil, want: domain.MinTempoBPM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.PlayAlongStartTempo(tc.target, tc.bestClean))
		})
	}
}

func TestWarmUpTempo(t *testing.T) {
	for _, tc := range []struct {
		name      string
		target    int
		bestClean int
		want      int
	}{
		{name: "is 80% of the best clean tempo, rounded down to 5 BPM", target: 120, bestClean: 100, want: 80},
		{name: "is 80% of a best clean tempo past the target", target: 120, bestClean: 150, want: 120},
		{name: "may itself be past the target", target: 120, bestClean: 200, want: 160},
		{name: "never goes below the slowest playable tempo", target: 120, bestClean: 20, want: domain.MinTempoBPM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.WarmUpTempo(tc.target, tc.bestClean))
		})
	}
}
