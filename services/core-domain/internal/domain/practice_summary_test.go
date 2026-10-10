package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestDaysInLast7(t *testing.T) {
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	// Monday 2026-10-05 12:00 in São Paulo.
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	local := func(day, hour, minute int) time.Time {
		return time.Date(2026, 10, day, hour, minute, 0, 0, saoPaulo)
	}

	for _, tc := range []struct {
		name  string
		times []time.Time
		want  int
	}{
		{name: "nothing counts no day"},
		{name: "two times on one day count one day", times: []time.Time{local(5, 8, 0), local(5, 9, 0)}, want: 1},
		{name: "times on different local days count each day", times: []time.Time{local(5, 8, 0), local(4, 23, 0)}, want: 2},
		{name: "the first day of the window counts", times: []time.Time{local(29, 0, 0).AddDate(0, -1, 0)}, want: 1},
		{name: "the day before the window doesn't", times: []time.Time{local(28, 23, 59).AddDate(0, -1, 0)}},
		{name: "a missed day never resets the count", times: []time.Time{local(29, 9, 0).AddDate(0, -1, 0), local(30, 9, 0).AddDate(0, -1, 0), local(1, 9, 0), local(2, 9, 0), local(3, 9, 0), local(4, 9, 0)}, want: 6},
		{name: "a time late on a local day counts on that day, whatever the UTC day", times: []time.Time{local(28, 23, 30).AddDate(0, -1, 0).Add(24 * time.Hour)}, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.DaysInLast7(tc.times, now, saoPaulo))
		})
	}
}

func TestDaysOfLast7(t *testing.T) {
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	// Monday 2026-10-05 12:00 in São Paulo.
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	local := func(month time.Month, day, hour, minute int) time.Time {
		return time.Date(2026, month, day, hour, minute, 0, 0, saoPaulo)
	}
	marked := func(days []domain.CalendarDay) []bool {
		out := make([]bool, len(days))
		for i, day := range days {
			out[i] = day.Marked
		}
		return out
	}

	t.Run("lists the 7 local days, oldest first and today last", func(t *testing.T) {
		got := domain.DaysOfLast7(nil, now, saoPaulo)

		require.Len(t, got, 7)
		for i, want := range []time.Time{
			local(9, 29, 0, 0), local(9, 30, 0, 0), local(10, 1, 0, 0), local(10, 2, 0, 0),
			local(10, 3, 0, 0), local(10, 4, 0, 0), local(10, 5, 0, 0),
		} {
			assert.True(t, want.Equal(got[i].Date), "day %d is %s, got %s", i, want, got[i].Date)
		}
		assert.Equal(t, make([]bool, 7), marked(got), "nothing happened, so no day is marked")
	})

	for _, tc := range []struct {
		name  string
		times []time.Time
		want  []bool
	}{
		{name: "two times on one day mark that day once", times: []time.Time{local(10, 5, 8, 0), local(10, 5, 9, 0)}, want: []bool{false, false, false, false, false, false, true}},
		{name: "a missed day is just unmarked", times: []time.Time{local(9, 29, 9, 0), local(10, 1, 9, 0)}, want: []bool{true, false, true, false, false, false, false}},
		{name: "the day before the window marks nothing", times: []time.Time{local(9, 28, 23, 59)}, want: make([]bool, 7)},
		{name: "a time after now's local day marks nothing", times: []time.Time{local(10, 6, 0, 0)}, want: make([]bool, 7)},
		{name: "a time late on a local day marks that day, whatever the UTC day", times: []time.Time{local(10, 4, 23, 30)}, want: []bool{false, false, false, false, false, true, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, marked(domain.DaysOfLast7(tc.times, now, saoPaulo)))
		})
	}

	t.Run("a daylight saving change still gives 7 calendar days", func(t *testing.T) {
		newYork, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		// Clocks fall back on Sunday 2026-11-01; now is Wednesday 2026-11-04 noon.
		dstNow := time.Date(2026, 11, 4, 17, 0, 0, 0, time.UTC)

		got := domain.DaysOfLast7([]time.Time{time.Date(2026, 11, 1, 23, 30, 0, 0, newYork)}, dstNow, newYork)

		require.Len(t, got, 7)
		assert.True(t, time.Date(2026, 10, 29, 0, 0, 0, 0, newYork).Equal(got[0].Date))
		assert.True(t, time.Date(2026, 11, 4, 0, 0, 0, 0, newYork).Equal(got[6].Date))
		assert.Equal(t, []bool{false, false, false, true, false, false, false}, marked(got))
	})
}

func TestDaysInLast7CountsTheMarkedDays(t *testing.T) {
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	times := []time.Time{now, now.Add(-24 * time.Hour), now.Add(-30 * time.Hour), now.Add(-10 * 24 * time.Hour)}

	marked := 0
	for _, day := range domain.DaysOfLast7(times, now, time.UTC) {
		if day.Marked {
			marked++
		}
	}
	assert.Equal(t, marked, domain.DaysInLast7(times, now, time.UTC))
}

func TestLast7DaysStart(t *testing.T) {
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	now := time.Date(2026, 10, 5, 2, 0, 0, 0, time.UTC) // Sunday 23:00 in São Paulo

	assert.Equal(t, time.Date(2026, 9, 28, 0, 0, 0, 0, saoPaulo), domain.Last7DaysStart(now, saoPaulo),
		"the window opens at local midnight six days before the local today")
}

func TestSkillProgressLines(t *testing.T) {
	bpm := func(v int) *int { return &v }
	skill := domain.KnowledgeNode{ID: "skill", Kind: domain.KnowledgeNodeKindSkill}
	practised := func(accuracy, fluency float64, best *int) domain.PracticeItemState {
		return domain.PracticeItemState{Counted: 3, Accuracy: accuracy, Fluency: fluency, BestCleanBPM: best}
	}
	snapshot := func(accuracy, fluency float64, best *int) domain.PracticeItemSnapshot {
		return domain.PracticeItemSnapshot{Counted: 2, Accuracy: accuracy, Fluency: fluency, BestCleanBPM: best}
	}

	t.Run("accuracy and fluency are the means over the skill's practised items, then and now", func(t *testing.T) {
		now := map[string]domain.PracticeItemState{"a": practised(0.9, 0.5, nil), "b": practised(0.82, 0.3, nil)}
		before := map[string]domain.PracticeItemSnapshot{"a": snapshot(0.72, 0.5, nil), "b": snapshot(0.72, 0.1, nil)}

		got := domain.SkillProgressLines(skill, []string{"a", "b", "c"}, now, before)

		require.Len(t, got, 2)
		assert.Equal(t, domain.SkillProgressMeasureAccuracy, got[0].Measure)
		assert.InDelta(t, 0.72, got[0].Before, 1e-9)
		assert.InDelta(t, 0.86, got[0].After, 1e-9)
		assert.Equal(t, domain.SkillProgressMeasureFluency, got[1].Measure)
		assert.InDelta(t, 0.3, got[1].Before, 1e-9)
		assert.InDelta(t, 0.4, got[1].After, 1e-9)
	})

	t.Run("a skill first practised this week starts from 0", func(t *testing.T) {
		got := domain.SkillProgressLines(skill, []string{"a"}, map[string]domain.PracticeItemState{"a": practised(0.8, 0, nil)}, nil)

		require.Len(t, got, 1)
		assert.Equal(t, domain.SkillProgress{NodeID: "skill", Measure: domain.SkillProgressMeasureAccuracy, Before: 0, After: 0.8}, got[0])
	})

	t.Run("a measure that didn't improve has no line", func(t *testing.T) {
		got := domain.SkillProgressLines(skill, []string{"a"},
			map[string]domain.PracticeItemState{"a": practised(0.7, 0.2, nil)},
			map[string]domain.PracticeItemSnapshot{"a": snapshot(0.8, 0.2, nil)})

		assert.Empty(t, got)
	})

	t.Run("a tempo line is the best clean tempo, with a clean take on both sides", func(t *testing.T) {
		now := map[string]domain.PracticeItemState{"a": practised(0.5, 0, bpm(105)), "b": practised(0.5, 0, bpm(90))}

		got := domain.SkillProgressLines(skill, []string{"a", "b"}, now, map[string]domain.PracticeItemSnapshot{"a": snapshot(0.5, 0, bpm(80))})
		require.Len(t, got, 1)
		assert.Equal(t, domain.SkillProgress{NodeID: "skill", Measure: domain.SkillProgressMeasureBestCleanTempo, Before: 80, After: 105}, got[0])

		got = domain.SkillProgressLines(skill, []string{"a"}, now, map[string]domain.PracticeItemSnapshot{"a": snapshot(0.5, 0, nil)})
		assert.Empty(t, got, "no clean take seven days ago, so no tempo line")
	})

	t.Run("a skill with nothing practised has no lines", func(t *testing.T) {
		assert.Empty(t, domain.SkillProgressLines(skill, []string{"a"}, nil, nil))
	})
}

func TestRankSkillProgress(t *testing.T) {
	lines := []domain.SkillProgress{
		{NodeID: "small", Measure: domain.SkillProgressMeasureAccuracy, Before: 0.7, After: 0.75},
		{NodeID: "tempo", Measure: domain.SkillProgressMeasureBestCleanTempo, Before: 100, After: 120},
		{NodeID: "big", Measure: domain.SkillProgressMeasureAccuracy, Before: 0.2, After: 0.8},
	}

	domain.RankSkillProgress(lines)

	assert.Equal(t, []string{"big", "tempo", "small"}, []string{lines[0].NodeID, lines[1].NodeID, lines[2].NodeID},
		"most improved first: a ratio by its gain, a tempo by its gain over where it started")
}

func TestRankNextSteps(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	inDays := func(days int) *time.Time {
		at := now.AddDate(0, 0, days)
		return &at
	}
	level := func(l domain.KnowledgeLevel) *domain.KnowledgeLevel { return &l }
	skill := func(id string, parentID *string) domain.KnowledgeNode {
		return domain.KnowledgeNode{ID: id, Kind: domain.KnowledgeNodeKindSkill, ParentID: parentID}
	}
	wide := "wide"
	nodes := []domain.KnowledgeNode{
		skill("fading", nil),
		skill("fluent", nil),
		skill("ready", nil),
		skill("strengthen", nil),
		{ID: "concept", Kind: domain.KnowledgeNodeKindConcept},
		skill("unlinked", nil),
		skill(wide, nil),
		skill("leaf", &wide),
		skill("empty", nil),
	}
	standings := map[string]domain.NodeStanding{
		"fading":     {NodeRollup: domain.NodeRollup{Total: 1, Level: level(domain.KnowledgeLevelAccurate), Fading: true}},
		"fluent":     {NodeRollup: domain.NodeRollup{Total: 1, Level: level(domain.KnowledgeLevelFluent)}},
		"ready":      {NodeRollup: domain.NodeRollup{Total: 1, Level: level(domain.KnowledgeLevelNew)}},
		"strengthen": {NodeRollup: domain.NodeRollup{Total: 1, Level: level(domain.KnowledgeLevelLearning)}},
		"concept":    {NodeRollup: domain.NodeRollup{Total: 1, Level: level(domain.KnowledgeLevelLearning)}},
		"unlinked":   {NodeRollup: domain.NodeRollup{Total: 1, Level: level(domain.KnowledgeLevelNew)}},
		wide:         {NodeRollup: domain.NodeRollup{Total: 2, Level: level(domain.KnowledgeLevelNew)}},
		"leaf":       {NodeRollup: domain.NodeRollup{Total: 1, Level: level(domain.KnowledgeLevelNew)}},
		"empty":      {NodeRollup: domain.NodeRollup{Total: 0}},
	}
	practisedState := domain.PracticeItemState{RulesVersion: domain.PracticeRulesVersion, Counted: 2, DueAt: inDays(3)}
	m := domain.KnowledgeView{
		Nodes:     nodes,
		Standings: standings,
		Subtrees: map[string][]string{
			"fading": {"i-fading"}, "fluent": {"i-fluent"}, "ready": {"i-ready"}, "strengthen": {"i-strengthen"},
			"concept": {"i-concept"}, "unlinked": {"i-unlinked"}, wide: {"i-leaf", "i-wide"}, "leaf": {"i-leaf"},
		},
		States: map[string]domain.PracticeItemState{
			"i-fading": practisedState, "i-fluent": practisedState, "i-strengthen": practisedState, "i-concept": practisedState,
		},
	}

	got := domain.RankNextSteps(m, []string{"fading", "fluent", "ready", "strengthen", "concept", "leaf"})

	var steps []string
	for _, s := range got {
		steps = append(steps, string(s.Kind)+":"+s.Node.ID)
	}
	assert.Equal(t, []string{"refresh:fading", "strengthen:strengthen", "ready_to_start:ready", "ready_to_start:leaf"}, steps,
		"fluent is secure; concepts and wide nodes are no next steps; unlinked isn't connected; empty has nothing to practise")
}

func TestGroupPracticeNodes(t *testing.T) {
	level := domain.KnowledgeLevelNew
	some := domain.NodeStanding{NodeRollup: domain.NodeRollup{Total: 1, Level: &level}}
	area, fretboard := "area", "fretboard"
	nodes := []domain.KnowledgeNode{
		{ID: area, InstrumentIDs: []string{"guitar"}},
		{ID: fretboard, InstrumentIDs: []string{"guitar"}},
		{ID: "intervals"},
		{ID: "low", ParentID: &fretboard, InstrumentIDs: []string{"guitar"}},
		{ID: "picking", ParentID: &area, InstrumentIDs: []string{"guitar"}},
		{ID: "nothing", ParentID: &area, InstrumentIDs: []string{"guitar"}},
	}
	standings := map[string]domain.NodeStanding{
		area: some, fretboard: some, "intervals": some, "low": some, "picking": some,
		"nothing": {NodeRollup: domain.NodeRollup{Total: 0}},
	}

	got := domain.GroupPracticeNodes(nodes, standings)

	require.Len(t, got, 3)
	ids := func(g domain.PracticeNodeGroup) []string {
		var out []string
		for _, n := range g.Nodes {
			out = append(out, n.ID)
		}
		return out
	}
	require.NotNil(t, got[0].Area)
	assert.Equal(t, area, got[0].Area.ID)
	assert.Equal(t, []string{area, "picking"}, ids(got[0]), "a node with nothing to practise is left out")
	require.NotNil(t, got[1].Area)
	assert.Equal(t, fretboard, got[1].Area.ID)
	assert.Equal(t, []string{fretboard, "low"}, ids(got[1]))
	assert.True(t, got[2].AnyInstrument)
	assert.Nil(t, got[2].Area)
	assert.Equal(t, []string{"intervals"}, ids(got[2]), "a node for every instrument goes to the Any instrument group")
}
