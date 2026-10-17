package application_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

var hearProgressions = fakeProgressions{
	{Key: "I-V-vi-IV", Bars: []string{"I", "V", "vi", "IV"}, Confusable: []string{"I-vi-IV-V"}, NodeIDs: []string{"skill-ear"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)"}},
	{Key: "I-vi-IV-V", Bars: []string{"I", "vi", "IV", "V"}, Confusable: []string{"I-V-vi-IV"}, NodeIDs: []string{"skill-ear"}, Names: domain.LocalizedText{"en": "I–vi–IV–V (doo-wop)"}},
	{Key: "IV-V-I", Bars: []string{"IV", "V", "I", "I"}, NodeIDs: []string{"skill-ear"}, Names: domain.LocalizedText{"en": "IV–V–I"}},
	{Key: "ii-V-I", Bars: []string{"ii", "V", "I", "I"}, NodeIDs: []string{"skill-ear"}, Names: domain.LocalizedText{"en": "ii–V–I"}},
}

// hearFixture is a practice fixture with the ear drills installed: every
// heard chord and heard progression under skill-ear, the only skill on the
// student's path, classified for guitar and bass.
func hearFixture(t *testing.T, plays ...string) *practiceFixture {
	f := newPracticeFixture(t)
	f.skill("skill-ear")
	f.onPathFor(plays, "skill-ear")
	f.svc.WithHarmonyDrills(hearProgressions, harmonyChordCatalog(), fakeFretboardLayouts{
		practiceGuitar: {{ID: practiceGuitar}},
		practiceBass:   {{ID: practiceBass}},
	})
	cadence, seen := "heard_chord:cadence_then_chord", "heard_chord:see_then_name"
	f.templates.extra = []domain.DrillTemplate{
		{Key: cadence, ItemKind: domain.PracticeItemKindHeardChord, Rung: domain.PracticeRungHear, SecondWay: &seen, ComesBack: true, Timed: true},
		{Key: seen, ItemKind: domain.PracticeItemKindHeardChord, Rung: domain.PracticeRungHear, SecondWay: &cadence, Timed: true},
		{Key: "heard_progression:progression_by_number", ItemKind: domain.PracticeItemKindHeardProgression, Rung: domain.PracticeRungHear, ComesBack: true, Timed: true},
	}
	var items []domain.ClassifiedItem
	for _, numeral := range domain.HarmonyItemNumerals {
		items = append(items, domain.ClassifiedItem{ItemKey: domain.HeardChordItem{Numeral: numeral}.ItemKey(), NodeIDs: []string{"skill-ear"}})
	}
	for _, p := range hearProgressions {
		items = append(items, domain.ClassifiedItem{ItemKey: domain.HeardProgressionItemKey(p.Key), NodeIDs: []string{"skill-ear"}})
	}
	for _, id := range []string{practiceGuitar, practiceBass} {
		f.cells[id] = append(f.cells[id], items...)
	}
	return f
}

// heardItems lists the plan's heard chords and heard progressions, in plan
// order.
func heardItems(plan domain.PracticeSessionPlan) []domain.PracticeSessionItem {
	return slices.DeleteFunc(slices.Clone(plan.Items), func(item domain.PracticeSessionItem) bool {
		return item.Kind != domain.PracticeItemKindHeardChord && item.Kind != domain.PracticeItemKindHeardProgression
	})
}

func TestPracticeSessionService_ComposePlanHearDrills(t *testing.T) {
	t.Run("a session in the head asks new heard chords by ear, with a playback, on the hear rung", func(t *testing.T) {
		f := hearFixture(t, practiceGuitar)

		plan := f.composeInTheHead(t, 10)

		items := itemsOfKind(plan, domain.PracticeItemKindHeardChord)
		require.NotEmpty(t, items)
		for _, item := range items {
			require.NotNil(t, item.GeneratedChoice, item.ItemKey)
			assert.Equal(t, "heard_chord:cadence_then_chord", item.DrillTemplateKey(), "never answered, it is heard first")
			assert.Equal(t, domain.PracticeRungHear, item.Rung)
			require.NotNil(t, item.GeneratedChoice.Playback)
			assert.Equal(t, 70, item.GeneratedChoice.Playback.TempoBPM)
		}
	})

	t.Run("every item asked a way that comes back is asked again later, with at least three items between", func(t *testing.T) {
		f := hearFixture(t, practiceGuitar)

		plan := f.composeInTheHead(t, 10)

		positions := map[string][]int{}
		for i, item := range plan.Items {
			positions[item.ItemKey] = append(positions[item.ItemKey], i)
		}
		heard := heardItems(plan)
		require.NotEmpty(t, heard)
		for _, item := range heard {
			at := positions[item.ItemKey]
			require.Len(t, at, 2, item.ItemKey)
			assert.GreaterOrEqual(t, at[1]-at[0]-1, 3, item.ItemKey+": at least three other items between")
			again := plan.Items[at[1]]
			assert.Equal(t, item.DrillTemplateKey(), again.DrillTemplateKey(), "asked the same way again")
			require.NotNil(t, again.GeneratedChoice.Playback, "asked by ear again")
		}
	})

	t.Run("a heard chord asked by sight doesn't come back", func(t *testing.T) {
		f := hearFixture(t, practiceGuitar)
		chord := domain.HeardChordItem{Numeral: "vi"}.ItemKey()
		f.stateOf(chord, domain.PracticeItemState{
			Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(1),
			RightByResponse: map[string]int{"cadence_then_chord": 2},
		})

		plan := f.composeInTheHead(t, 10)

		var asked []domain.PracticeSessionItem
		for _, item := range plan.Items {
			if item.ItemKey == chord {
				asked = append(asked, item)
			}
		}
		require.Len(t, asked, 1)
		assert.Equal(t, "heard_chord:see_then_name", asked[0].DrillTemplateKey())
		assert.Nil(t, asked[0].GeneratedChoice.Playback)
	})

	t.Run("a session's items, comebacks included, fit in its time", func(t *testing.T) {
		f := hearFixture(t, practiceGuitar)

		plan := f.composeInTheHead(t, 5)

		total := 0
		for _, item := range plan.Items {
			total += item.EstimatedSeconds
		}
		assert.LessOrEqual(t, total, 5*60)
	})

	t.Run("confusable heard progressions are asked next to each other", func(t *testing.T) {
		f := hearFixture(t, practiceGuitar)
		f.svc.WithShuffle(reverseOrder)

		plan := f.composeInTheHead(t, 10)

		var progressions []string
		for _, item := range itemsOfKind(plan, domain.PracticeItemKindHeardProgression) {
			if !slices.Contains(progressions, item.ItemKey) {
				progressions = append(progressions, item.ItemKey)
			}
		}
		pop, doowop := domain.HeardProgressionItemKey("I-V-vi-IV"), domain.HeardProgressionItemKey("I-vi-IV-V")
		require.Contains(t, progressions, pop)
		require.Contains(t, progressions, doowop)
		heard := heardItems(plan)
		first := slices.IndexFunc(heard, func(item domain.PracticeSessionItem) bool { return item.ItemKey == pop || item.ItemKey == doowop })
		require.Less(t, first+1, len(heard))
		assert.ElementsMatch(t, []string{pop, doowop}, []string{heard[first].ItemKey, heard[first+1].ItemKey},
			"no other heard progression between them")
	})
}

func TestPracticeSessionService_ComposePlanHearBassLine(t *testing.T) {
	tests := []struct {
		name  string
		plays []string
		want  string
	}{
		{name: "a bassist hears a root line under the chords, on the bass", plays: []string{practiceBass}, want: practiceBass},
		{name: "a guitarist hears no root line", plays: []string{practiceGuitar}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := hearFixture(t, tt.plays...)

			plan := f.composeInTheHead(t, 10)

			items := heardItems(plan)
			require.NotEmpty(t, items)
			for _, item := range items {
				assert.Equal(t, tt.want, item.GeneratedChoice.Playback.BassInstrumentID, item.ItemKey)
				assert.NotEmpty(t, item.GeneratedChoice.Voicings, "the chords keep their guitar voicing")
			}
		})
	}
}
