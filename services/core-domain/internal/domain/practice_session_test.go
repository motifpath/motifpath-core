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
	everyInstance := domain.PracticeSessionItem{Kind: domain.PracticeItemKindFretboardCell, FretboardCell: &domain.PlannedFretboardCell{Drill: domain.FretboardDrillFindEveryInstance}}
	named := domain.PracticeSessionItem{Kind: domain.PracticeItemKindFretboardCell, FretboardCell: &domain.PlannedFretboardCell{Drill: domain.FretboardDrillNameTheNote}}

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
		{"finding every instance isn't timed, so a plan with only that never asks", []domain.PracticeSessionItem{everyInstance, exercise}, nil, false},
		{"a cell named beside one found everywhere still asks", []domain.PracticeSessionItem{everyInstance, named}, nil, true},
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

func TestPlayAlongSeconds(t *testing.T) {
	steps := make([]domain.SequenceStep, 8)
	for i := range steps {
		steps[i] = domain.SequenceStep{PositionIDs: []string{"p1"}, Value: domain.NoteValue{Num: 1, Den: 4}}
	}
	d := domain.Diagram{
		Playbacks:         []domain.DiagramPlayback{{ID: "pb", TempoBPM: 120, TimeSignature: domain.DefaultTimeSignature, Steps: steps}},
		DefaultPlaybackID: func() *string { id := "pb"; return &id }(),
	}

	for _, tc := range []struct {
		name      string
		noteValue domain.TempoNoteValue
		warmUp    bool
		want      int
	}{
		// A take is a count-in bar of 4 clicks plus 8 quarter notes, each followed by a
		// 10-second rating: at 60 BPM, 4 takes of (4 + 8) s + 10 s.
		{name: "four takes in quarters", noteValue: domain.TempoNoteValueQuarters, want: 4 * (12 + 10)},
		{name: "the notes play twice as fast in eighths, the count-in doesn't", noteValue: domain.TempoNoteValueEighths, want: 4 * (8 + 10)},
		{name: "halves play the notes at half speed", noteValue: domain.TempoNoteValueHalves, want: 4 * (20 + 10)},
		{name: "a warm-up is two takes", noteValue: domain.TempoNoteValueEighths, warmUp: true, want: 2 * (8 + 10)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.PlayAlongSeconds(d, 60, tc.noteValue, tc.warmUp))
		})
	}
}

func TestPracticeItemState_BestCleanTempo(t *testing.T) {
	bpm, eighths := 60, domain.TempoNoteValueEighths

	t.Run("is the best clean take as played", func(t *testing.T) {
		got, v := domain.PracticeItemState{BestCleanBPM: &bpm, BestCleanNoteValue: &eighths}.BestCleanTempo()
		assert.Equal(t, &bpm, got)
		assert.Equal(t, domain.TempoNoteValueEighths, v)
	})
	t.Run("a best clean take with no note value was played in quarters", func(t *testing.T) {
		got, v := domain.PracticeItemState{BestCleanBPM: &bpm}.BestCleanTempo()
		assert.Equal(t, &bpm, got)
		assert.Equal(t, domain.TempoNoteValueQuarters, v)
	})
	t.Run("with no clean take yet, a play-along starts in quarters", func(t *testing.T) {
		got, v := domain.PracticeItemState{}.BestCleanTempo()
		assert.Nil(t, got)
		assert.Equal(t, domain.TempoNoteValueQuarters, v)
	})
}

func TestTempoNoteValue_EffectiveSpeed(t *testing.T) {
	for v, want := range map[domain.TempoNoteValue]float64{
		domain.TempoNoteValueHalves: 40, domain.TempoNoteValueQuarters: 80, domain.TempoNoteValueEighths: 160,
		domain.TempoNoteValueTriplets: 240, domain.TempoNoteValueSixteenths: 320, domain.TempoNoteValueQuintuplets: 400,
		domain.TempoNoteValueSextuplets: 480, domain.TempoNoteValueThirtySeconds: 640,
	} {
		assert.InDelta(t, want, v.EffectiveSpeed(80), 1e-9, v)
	}
}

func TestItemsFocus(t *testing.T) {
	node := func(id string) domain.KnowledgeNode { return domain.KnowledgeNode{ID: id, Key: id} }
	nodes := map[string]domain.KnowledgeNode{"a": node("a"), "b": node("b"), "c": node("c"), "d": node("d")}
	item := func(nodeID string, seconds int) domain.PracticeSessionItem {
		return domain.PracticeSessionItem{NodeID: &nodeID, EstimatedSeconds: seconds}
	}

	t.Run("names the nodes by planned time, most first, at most three", func(t *testing.T) {
		got := domain.ItemsFocus([]domain.PracticeSessionItem{
			item("a", 10), item("b", 30), item("c", 20), item("d", 5), item("a", 15),
		}, nodes)
		assert.Equal(t, []domain.KnowledgeNode{node("b"), node("a"), node("c")}, got)
	})

	t.Run("a tie keeps the node served first", func(t *testing.T) {
		got := domain.ItemsFocus([]domain.PracticeSessionItem{item("c", 10), item("a", 10)}, nodes)
		assert.Equal(t, []domain.KnowledgeNode{node("c"), node("a")}, got)
	})

	t.Run("items serving no known node are left out", func(t *testing.T) {
		got := domain.ItemsFocus([]domain.PracticeSessionItem{{EstimatedSeconds: 60}, item("gone", 60), item("a", 5)}, nodes)
		assert.Equal(t, []domain.KnowledgeNode{node("a")}, got)
	})
}
