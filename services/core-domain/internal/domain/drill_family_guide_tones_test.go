package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

var iiVIProgression = domain.Progression{Key: "ii-V-I", Bars: []string{"ii", "V", "I", "I"}, Names: domain.LocalizedText{"en": "ii–V–I"}}

// firstChoice always picks the first of the choices.
func firstChoice(int) int { return 0 }

func plannedGuideTone(t *testing.T, item domain.GuideToneItem, planning domain.DrillPlanning) domain.PlannedGuideTone {
	t.Helper()
	if planning.Pick == nil {
		planning.Pick = firstChoice
	}
	if planning.Voicing == nil {
		planning.Voicing = everyChordHasABox
	}
	planned := plannedItem(t, item, planning)
	require.NotNil(t, planned.GuideTone)
	assert.Equal(t, domain.PracticeItemKindGuideTone, planned.Kind)
	return *planned.GuideTone
}

func TestAGuideToneIsTappedInTheHeadAtTheTargetRung(t *testing.T) {
	item := domain.GuideToneItem{Layout: tourGuitar, Progression: iiVIProgression}

	assert.Equal(t, "guide_tone:guitar:ii-V-I", item.ItemKey())
	assert.Equal(t, domain.PracticeItemKindGuideTone, item.Kind())
	assert.True(t, item.SuitsSession(false))
	assert.False(t, item.SuitsSession(true), "a guide tone is tapped on the screen, never played in hand")

	planned := plannedItem(t, item, domain.DrillPlanning{Pick: firstChoice, Voicing: everyChordHasABox})
	assert.Equal(t, domain.PracticeRungTarget, domain.ItemRung(planned, nil))
	assert.Empty(t, planned.DrillTemplateKey(), "a guide tone is measured by tempo, so it practises no timed template")
	drill, most, capped := planned.SessionCap()
	assert.True(t, capped)
	assert.Equal(t, "guide_tone", drill)
	assert.Equal(t, 1, most, "at most one guide tone a session")
}

func TestAGuideToneIsSizedByThreeTakesAtItsStartTempo(t *testing.T) {
	// Each take: a bar of count-in and the progression once at 60 BPM in 4/4,
	// then 5 seconds to see the result.
	assert.Equal(t, 3*(5*4+5), domain.GuideToneItem{Layout: tourGuitar, Progression: iiVIProgression}.Seconds())
	assert.Equal(t, 3*(13*4+5), domain.GuideToneItem{Layout: tourGuitar, Progression: bluesProgression}.Seconds())
}

func TestAGuideToneNeverPractisedAsksTheThirdAtSixty(t *testing.T) {
	g := plannedGuideTone(t, domain.GuideToneItem{Layout: tourGuitar, Progression: iiVIProgression}, domain.DrillPlanning{})

	assert.Equal(t, "guitar", g.LayoutInstrumentID)
	assert.Equal(t, "ii-V-I", g.ProgressionKey)
	assert.Equal(t, iiVIProgression.Names, g.ProgressionNames)
	assert.Equal(t, "3", g.Tone)
	assert.Equal(t, 60, g.TempoBPM)
	assert.Equal(t, 90, g.TargetTempoBPM)
	assert.Equal(t, 3, g.Takes)
	assert.Equal(t, domain.FretWindow{FromFret: 0}, g.FretWindow)
}

func TestAGuideToneAsksHarderAsTheLevelRises(t *testing.T) {
	cases := []struct {
		level domain.KnowledgeLevel
		tone  string
		tempo int
	}{
		{domain.KnowledgeLevelNew, "3", 60},
		{domain.KnowledgeLevelLearning, "3", 60},
		{domain.KnowledgeLevelAccurate, "7", 72},
		{domain.KnowledgeLevelFluent, "7", 90},
		{domain.KnowledgeLevelRetained, "7", 90},
	}
	for _, tc := range cases {
		t.Run(string(tc.level), func(t *testing.T) {
			g := plannedGuideTone(t, domain.GuideToneItem{Layout: tourGuitar, Progression: iiVIProgression}, domain.DrillPlanning{Level: tc.level})

			assert.Equal(t, tc.tone, g.Tone)
			assert.Equal(t, tc.tempo, g.TempoBPM)
		})
	}
}

func TestAGuideTonesBarsAreInItsKeyWithAVoicingPerChord(t *testing.T) {
	// The keys but the last take's, in order, are picked from: index 6 of all
	// 12 but C is G.
	pickG := func(n int) int {
		require.Equal(t, 11, n)
		return 6
	}
	state := &domain.PracticeItemState{LastMusicKey: "C"}

	g := plannedGuideTone(t, domain.GuideToneItem{Layout: tourGuitar, Progression: iiVIProgression}, domain.DrillPlanning{Pick: pickG, State: state})

	assert.Equal(t, "G", g.MusicKey)
	assert.Equal(t, []domain.GuideToneBar{{Numeral: "ii", Chord: "Am"}, {Numeral: "V", Chord: "D"}, {Numeral: "I", Chord: "G"}, {Numeral: "I", Chord: "G"}}, g.Bars)
	assert.Equal(t, []domain.ChordBox{{Chord: "Am", DiagramID: "box-Am"}, {Chord: "D", DiagramID: "box-D"}, {Chord: "G", DiagramID: "box-G"}}, g.Voicings)
}

func TestAGuideToneIsNeverPlannedInItsLastTakesKey(t *testing.T) {
	item := domain.GuideToneItem{Layout: tourGuitar, Progression: iiVIProgression}
	state := &domain.PracticeItemState{LastMusicKey: "G"}
	seen := map[string]bool{}
	for choice := range 11 {
		g := plannedGuideTone(t, item, domain.DrillPlanning{Pick: func(int) int { return choice }, State: state})
		seen[g.MusicKey] = true
	}

	assert.NotContains(t, seen, "G")
	assert.Len(t, seen, 11, "every other major key can be picked")
}

func TestABluesGuideToneKeepsItsDominantSevenths(t *testing.T) {
	pickA := func(int) int { return 9 } // A, with no last key
	g := plannedGuideTone(t, domain.GuideToneItem{Layout: tourGuitar, Progression: bluesProgression}, domain.DrillPlanning{Pick: pickA, Level: domain.KnowledgeLevelAccurate})

	require.Len(t, g.Bars, 12)
	assert.Equal(t, domain.GuideToneBar{Numeral: "I7", Chord: "A7"}, g.Bars[0])
	assert.Equal(t, "7", g.Tone)
}

func TestAGuideToneMarksNoAnswer(t *testing.T) {
	item := domain.GuideToneItem{Layout: tourGuitar, Progression: iiVIProgression}
	planned := item.Plan(item.Ways()[0], domain.DrillPlanning{Pick: firstChoice, Voicing: everyChordHasABox})

	assert.Equal(t, domain.DrillStimulus{}, planned.Stimulus(), "the plan names no note to tap and no place")
}
